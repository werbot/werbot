package session

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/hashchain"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session/dbq"
)

// Verification is the outcome of checking a recording; Reason is empty when Verified.
type Verification struct {
	Verified bool
	Reason   string
}

// Reasons a recording fails verification.
const (
	ReasonNoRecording      = "no_recording"
	ReasonNotFinished      = "not_finished"
	ReasonSignatureInvalid = "signature_invalid"
	ReasonChunkMissing     = "chunk_missing"
	ReasonChainMismatch    = "chain_mismatch"
)

// Verify re-reads every chunk from the store and checks the hash chain, the
// totals and the manifest signature. Tampering is a Verification, not an error;
// an error means the database or the store failed. p is the caller, who may see
// the session.
func (r *Recordings) Verify(ctx context.Context, p principal.Principal, sessionID uuid.UUID) (Verification, error) {
	return r.copyChain(ctx, p, sessionID, io.Discard)
}

// copyChain verifies like Verify and writes the compressed chunks to w in order,
// stopping at the first missing one. The chain is recomputed from store objects
// and recording.chunks; last_hash is ingest state and takes no part.
func (r *Recordings) copyChain(ctx context.Context, p principal.Principal, sessionID uuid.UUID, w io.Writer) (Verification, error) {
	orgID := p.OrgID
	var rec dbq.GetRecordingRow
	err := dbx.InOrg(ctx, r.pool, orgID, p.UserID, func(tx pgx.Tx) (err error) {
		rec, err = newStore(tx).GetRecording(ctx, dbq.GetRecordingParams{SessionID: sessionID, OrgID: orgID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Verification{Reason: ReasonNoRecording}, nil
	}
	if err != nil {
		return Verification{}, err
	}

	if rec.Store != r.kind {
		return Verification{}, fmt.Errorf("recording kept in %q store, server configured for %q: %w", rec.Store, r.kind, apperr.ErrUnavailable)
	}

	prev, size, missing := hashchain.Genesis(), int64(0), false
	for seq := range int(rec.Chunks) {
		gz, err := r.readChunk(ctx, orgID, sessionID, seq)
		if errors.Is(err, fs.ErrNotExist) {
			missing = true
			break
		}
		if err != nil {
			return Verification{}, fmt.Errorf("recording store: %w: %w", apperr.ErrUnavailable, err)
		}
		if _, err := w.Write(gz); err != nil {
			return Verification{}, err
		}
		prev, size = hashchain.Next(prev, gz), size+int64(len(gz))
	}

	switch {
	case missing:
		return Verification{Reason: ReasonChunkMissing}, nil
	case rec.FinalHash == nil:
		return Verification{Reason: ReasonNotFinished}, nil
	case !r.signer.Verify(NewManifest(sessionID, int(rec.Chunks), rec.Bytes, rec.FinalHash).Canonical(), rec.ManifestSignature):
		return Verification{Reason: ReasonSignatureInvalid}, nil
	case !bytes.Equal(prev, rec.FinalHash) || size != rec.Bytes:
		return Verification{Reason: ReasonChainMismatch}, nil
	}
	return Verification{Verified: true}, nil
}

func (r *Recordings) readChunk(ctx context.Context, orgID, sessionID uuid.UUID, seq int) ([]byte, error) {
	rc, err := r.store.Get(ctx, orgID, sessionID, seq)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, MaxChunkGz+1))
}
