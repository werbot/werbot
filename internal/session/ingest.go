package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/hashchain"
	"github.com/werbot/werbot/internal/session/dbq"
)

// MaxChunkGz caps a compressed chunk: 64 KiB of raw output plus gzip overhead.
const MaxChunkGz = 128 << 10

// Recordings ingests the chunks of session recordings and signs finished ones.
type Recordings struct {
	pool   *pgxpool.Pool
	store  RecordingStore
	kind   string // recording.store: "fs" | "s3"
	signer *Signer
}

// NewRecordings returns Recordings writing chunks to store of kind storeKind.
func NewRecordings(pool *pgxpool.Pool, store RecordingStore, storeKind string, signer *Signer) *Recordings {
	return &Recordings{pool: pool, store: store, kind: storeKind, signer: signer}
}

// Begin opens the recording of a session whose resource type records its kind
// as querylog, so the recording has its format before the first chunk; a
// repeat is a no-op. Other recordings are asciicast, opened by IngestChunk. A
// session outside orgID or without its resource is apperr.ErrNotFound.
func (r *Recordings) Begin(ctx context.Context, orgID, sessionID uuid.UUID) error {
	return dbx.InOrg(ctx, r.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		s, err := st.SessionResourceType(ctx, dbq.SessionResourceTypeParams{ID: sessionID, OrgID: orgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("session not found: %w", apperr.ErrNotFound)
		}
		if err != nil {
			return err
		}
		t, ok := connector.Lookup(s.Type)
		if !ok || t.RecordFormat != connector.RecordQuerylog || !t.Records(connector.Kind(s.Kind)) {
			return nil
		}
		return st.CreateRecording(ctx, dbq.CreateRecordingParams{SessionID: sessionID, OrgID: orgID, Store: r.kind,
			Format: connector.RecordQuerylog})
	})
}

// IngestChunk stores chunk seq of a session served by instanceID. A seq that
// was already accepted is apperr.ErrAlreadyExists and keeps the stored object;
// a store failure is apperr.ErrUnavailable and leaves nothing committed.
func (r *Recordings) IngestChunk(ctx context.Context, orgID, instanceID, sessionID uuid.UUID, seq int, hash, gz []byte) error {
	return dbx.InOrg(ctx, r.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := checkInstance(ctx, st, orgID, instanceID, sessionID); err != nil {
			return err
		}
		if err := st.CreateRecording(ctx, dbq.CreateRecordingParams{SessionID: sessionID, OrgID: orgID, Store: r.kind,
			Format: connector.RecordAsciicast}); err != nil {
			return err
		}
		rec, err := st.LockRecording(ctx, dbq.LockRecordingParams{SessionID: sessionID, OrgID: orgID})
		if err != nil {
			return err
		}
		switch chunks := int(rec.Chunks); {
		case seq < 0:
			return fmt.Errorf("recording chunk %d: negative seq: %w", seq, apperr.ErrInvalidArgument)
		case seq < chunks:
			return fmt.Errorf("recording chunk %d: %w", seq, apperr.ErrAlreadyExists)
		case rec.FinalHash != nil:
			return fmt.Errorf("recording chunk %d: recording finished: %w", seq, apperr.ErrFailedPrecondition)
		case seq > chunks:
			return fmt.Errorf("recording chunk %d: expected seq %d: %w", seq, chunks, apperr.ErrFailedPrecondition)
		case len(gz) == 0 || len(gz) > MaxChunkGz:
			return fmt.Errorf("recording chunk %d: size %d outside 1..%d: %w", seq, len(gz), MaxChunkGz, apperr.ErrInvalidArgument)
		case !bytes.Equal(hashchain.Next(rec.LastHash, gz), hash):
			return fmt.Errorf("recording chunk %d: hash breaks the chain: %w", seq, apperr.ErrInvalidArgument)
		}
		// ponytail: Put runs inside the row-locked tx, so a slow store holds a pool
		// connection and the recording row lock for its duration (bounded by the
		// gateway's 10 s unary deadline); stage 7a measures it under load.
		if err := r.store.Put(ctx, orgID, sessionID, seq, gz); err != nil {
			return fmt.Errorf("recording store: %w: %w", apperr.ErrUnavailable, err)
		}
		return st.AppendRecordingChunk(ctx, dbq.AppendRecordingChunkParams{
			Size: int64(len(gz)), LastHash: hash, SessionID: sessionID, OrgID: orgID})
	})
}

// FinishRecording checks the gateway's totals against the ingested chain,
// signs the manifest and audits "recording.finish". A repeat with the same
// final hash is a no-op. Without a signer it is apperr.ErrUnavailable.
func (r *Recordings) FinishRecording(ctx context.Context, orgID, instanceID, sessionID uuid.UUID, chunks int, size int64, finalHash []byte) error {
	if r.signer == nil {
		return fmt.Errorf("recording signer not loaded: %w", apperr.ErrUnavailable)
	}
	return dbx.InOrg(ctx, r.pool, orgID, uuid.Nil, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := checkInstance(ctx, st, orgID, instanceID, sessionID); err != nil {
			return err
		}
		rec, err := st.LockRecording(ctx, dbq.LockRecordingParams{SessionID: sessionID, OrgID: orgID})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("recording has no chunks: %w", apperr.ErrFailedPrecondition)
		}
		if err != nil {
			return err
		}
		if rec.FinalHash != nil {
			if bytes.Equal(rec.FinalHash, finalHash) {
				return nil
			}
			return fmt.Errorf("recording finished with another hash: %w", apperr.ErrFailedPrecondition)
		}
		if rec.Chunks == 0 || int(rec.Chunks) != chunks || rec.Bytes != size || !bytes.Equal(rec.LastHash, finalHash) {
			return fmt.Errorf("recording mismatch: server has %d chunks, %d bytes: %w", rec.Chunks, rec.Bytes, apperr.ErrFailedPrecondition)
		}
		sig := r.signer.Sign(NewManifest(sessionID, chunks, size, finalHash).Canonical())
		if err := st.FinishRecording(ctx, dbq.FinishRecordingParams{
			FinalHash: finalHash, ManifestSignature: sig, SessionID: sessionID, OrgID: orgID}); err != nil {
			return err
		}
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &orgID, ActorKind: "gateway", Action: "recording.finish",
			TargetType: "session", TargetID: sessionID.String(),
			Data: map[string]any{"gateway_instance_id": instanceID.String(), "chunks": chunks, "bytes": size},
		})
	})
}

// checkInstance denies access to a session not served by instanceID of orgID.
func checkInstance(ctx context.Context, st Store, orgID, instanceID, sessionID uuid.UUID) error {
	ok, err := st.SessionOfInstanceExists(ctx, dbq.SessionOfInstanceExistsParams{ID: sessionID, OrgID: orgID, InstanceID: instanceID})
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session of another instance: %w", apperr.ErrPermissionDenied)
	}
	return nil
}
