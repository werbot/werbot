package key

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal"
	keypb "github.com/werbot/werbot/internal/core/key/proto/key"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/storage/postgres"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
	"github.com/werbot/werbot/pkg/uuid"
)

// Keys fetches a list of keys based on the request parameters.
func (h *Handler) Keys(ctx context.Context, in *keypb.Keys_Request) (*keypb.Keys_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &keypb.Keys_Response{}
	sqlUserLimit := postgres.SQLColumnsNull(in.GetIsAdmin(), true, []string{`"locked_at"`}) // if not admin

	// Total count for pagination
	baseQuery := postgres.SQLGluing(`
    SELECT COUNT(*)
    FROM "profile_public_key"
    WHERE "profile_id" = $1
  `, sqlUserLimit)
	err := h.DB.Conn.QueryRowContext(ctx, baseQuery, in.GetProfileId()).Scan(&response.Total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}
	if response.Total == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgProjectNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// List records
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	baseQuery = postgres.SQLGluing(`
    SELECT
      "id",
      "profile_id",
      "title",
      "key",
      "fingerprint",
      "locked_at",
      "archived_at",
      "updated_at",
      "created_at"
    FROM "profile_public_key"
    WHERE "profile_id" = $1
  `, sqlUserLimit, sqlFooter)
	rows, err := h.DB.Conn.QueryContext(ctx, baseQuery, in.GetProfileId())
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer rows.Close()

	for rows.Next() {
		var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp
		publicKey := &keypb.Key_Response{}
		err = rows.Scan(
			&publicKey.KeyId,
			&publicKey.ProfileId,
			&publicKey.Title,
			&publicKey.Key,
			&publicKey.Fingerprint,
			&lockedAt,
			&archivedAt,
			&updatedAt,
			&createdAt,
		)
		if err != nil {
			return nil, trace.Error(err, log, nil)
		}

		protoutils.SetPgtypeTimestamps(publicKey, map[string]pgtype.Timestamp{
			"locked_at":   lockedAt,
			"archived_at": archivedAt,
			"updated_at":  updatedAt,
			"created_at":  createdAt,
		})

		if !in.GetIsAdmin() {
			ghoster.Secrets(publicKey, true)
		}

		response.PublicKeys = append(response.PublicKeys, publicKey)
	}

	return response, nil
}

// Key fetches a single key based on the request parameters.
func (h *Handler) Key(ctx context.Context, in *keypb.Key_Request) (*keypb.Key_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &keypb.Key_Response{
		KeyId: in.GetKeyId(),
	}
	var lockedAt, archivedAt, updatedAt, createdAt pgtype.Timestamp

	sqlHook := postgres.SQLColumnsNull(in.GetIsAdmin(), true, []string{`"locked_at"`}) // if not admin
	baseQuery := postgres.SQLGluing(`
    SELECT
      "id",
      "profile_id",
      "title",
      "key",
      "fingerprint",
      "locked_at",
      "archived_at",
      "updated_at",
      "created_at"
    FROM "profile_public_key"
    WHERE "id" = $1 AND "profile_id" = $2
  `, sqlHook)
	err := h.DB.Conn.QueryRowContext(ctx, baseQuery, in.GetKeyId(), in.GetProfileId()).Scan(
		&response.KeyId,
		&response.ProfileId,
		&response.Title,
		&response.Key,
		&response.Fingerprint,
		&lockedAt,
		&archivedAt,
		&updatedAt,
		&createdAt,
	)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	protoutils.SetPgtypeTimestamps(response, map[string]pgtype.Timestamp{
		"locked_at":   lockedAt,
		"archived_at": archivedAt,
		"updated_at":  updatedAt,
		"created_at":  createdAt,
	})

	if !in.GetIsAdmin() {
		ghoster.Secrets(response, true)
	}

	return response, nil
}

// AddKey adds a new key to the database.
func (h *Handler) AddKey(ctx context.Context, in *keypb.AddKey_Request) (*keypb.AddKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	sshKey, err := crypto.ParseSSHKey([]byte(in.GetKey()))
	if err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, trace.MsgPublicKeyIsBroken)
	}

	response := &keypb.AddKey_Response{
		Fingerprint: sshKey.Fingerprint,
	}

	// Check public key fingerprint
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT "id"
    FROM "profile_public_key"
    WHERE
      "fingerprint" = $1
      AND "profile_id" = $2
      AND "archived_at" IS NULL
  `,
		response.GetFingerprint(),
		in.GetProfileId(),
	).Scan(&response.KeyId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, nil)
	}

	if response.GetKeyId() != "" {
		return nil, trace.Error(status.Error(codes.AlreadyExists, ""), log, nil)
	}

	// Set key comment if title is provided
	if in.GetTitle() != "" {
		sshKey.Comment = in.GetTitle()
	}

	// Insert new public key
	err = h.DB.Conn.QueryRowContext(ctx, `
    INSERT INTO "profile_public_key" ("profile_id", "title", "key", "fingerprint")
    VALUES ($1, $2, $3, $4)
    RETURNING "id"
  `,
		in.GetProfileId(),
		sshKey.Comment,
		in.GetKey(),
		response.GetFingerprint(),
	).Scan(&response.KeyId)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToAdd)
	}

	return response, nil
}

// UpdateKey is ...
func (h *Handler) UpdateKey(ctx context.Context, in *keypb.UpdateKey_Request) (*keypb.UpdateKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &keypb.UpdateKey_Response{}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "profile_public_key"
    SET "title" = $1
    WHERE "id" = $2 AND "profile_id" = $3
  `,
		in.GetTitle(),
		in.GetKeyId(),
		in.GetProfileId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgKeyNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}

// DeleteKey is ...
func (h *Handler) DeleteKey(ctx context.Context, in *keypb.DeleteKey_Request) (*keypb.DeleteKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &keypb.DeleteKey_Response{}

	result, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "profile_public_key"
    SET "archived_at" = NOW()
    WHERE "id" = $1 AND "profile_id" = $2
  `,
		in.GetKeyId(),
		in.GetProfileId(),
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}

	if rows, _ := result.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgUserNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}

// GenerateSSHKey is ...
func (h *Handler) GenerateSSHKey(ctx context.Context, in *keypb.GenerateSSHKey_Request) (*keypb.GenerateSSHKey_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	// Generate SSH key
	key, err := crypto.NewSSHKey(in.GetKeyType().String())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedCreatingSSHKey)
	}

	// Populate response fields
	response := &keypb.GenerateSSHKey_Response{
		KeyType:     in.GetKeyType(),
		Public:      string(key.PublicKey),
		Passphrase:  key.Passphrase,
		Uuid:        uuid.New(),
		FingerPrint: key.FingerPrint,
	}

	// Prepare cache key
	cacheKey := &keypb.SchemeKey{
		Private:     string(key.PrivateKey),
		Public:      string(key.PublicKey),
		Passphrase:  key.Passphrase,
		FingerPrint: key.FingerPrint,
	}
	mapB, err := json.Marshal(cacheKey)
	if err != nil {
		return nil, trace.Error(err, log, "Failed to marshal cache key")
	}

	// Set cache with expiration duration
	cacheKeyStr := fmt.Sprintf("tmp_key_ssh:%s", response.Uuid)
	expiration := internal.GetDuration("SSH_KEY_REFRESH_DURATION", "10m")
	if err := h.Redis.Client.Set(ctx, cacheKeyStr, mapB, expiration).Err(); err != nil {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}
