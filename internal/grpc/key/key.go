package key

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
	keypb "github.com/werbot/werbot/internal/grpc/key/proto"
)

// ListKeys is ...
func (h *Handler) ListKeys(ctx context.Context, in *keypb.ListKeys_Request) (*keypb.ListKeys_Response, error) {
	response := new(keypb.ListKeys_Response)

	sqlSearch := h.DB.SQLAddWhere(in.GetQuery())
	sqlFooter := h.DB.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := h.DB.Conn.Query(`SELECT
			"user_public_key"."id" AS "key_id",
			"user_public_key"."user_id",
			"user"."login" AS "user_login",
			"user_public_key"."title",
			"user_public_key"."key_",
			"user_public_key"."fingerprint",
			"user_public_key"."last_update",
			"user_public_key"."created"
		FROM "user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch + sqlFooter)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var lastUpdate, created pgtype.Timestamp
		publicKey := new(keypb.Key_Response)
		err = rows.Scan(&publicKey.KeyId,
			&publicKey.UserId,
			&publicKey.UserLogin,
			&publicKey.Title,
			&publicKey.Key,
			&publicKey.Fingerprint,
			&lastUpdate,
			&created,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			log.FromGRPC(err).Send()
			return nil, errServerError
		}
		publicKey.LastUpdate = timestamppb.New(lastUpdate.Time)
		publicKey.Created = timestamppb.New(created.Time)
		response.PublicKeys = append(response.PublicKeys, publicKey)
	}
	defer rows.Close()

	// Total count for pagination
	err = h.DB.Conn.QueryRow(`SELECT COUNT (*)
		FROM "user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch,
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// PublicKey is ...
func (h *Handler) PublicKey(ctx context.Context, in *keypb.Key_Request) (*keypb.Key_Response, error) {
	var lastUpdate, created pgtype.Timestamp
	response := new(keypb.Key_Response)
	response.KeyId = in.GetKeyId()

	err := h.DB.Conn.QueryRow(`SELECT
			"user_public_key"."id" AS "key_id",
			"user_public_key"."user_id",
			"user"."login" AS "user_login",
			"user_public_key"."title",
			"user_public_key"."key_",
			"user_public_key"."fingerprint",
			"user_public_key"."last_update",
			"user_public_key"."created"
		FROM "user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"
        WHERE "user_public_key"."id" = $1
          AND "user_public_key"."user_id" = $2`,
		in.GetKeyId(),
		in.GetUserId(),
	).Scan(&response.KeyId,
		&response.UserId,
		&response.UserLogin,
		&response.Title,
		&response.Key,
		&response.Fingerprint,
		&lastUpdate,
		&created,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}

	response.LastUpdate = timestamppb.New(lastUpdate.Time)
	response.Created = timestamppb.New(created.Time)

	return response, nil
}

// AddKey is ...
func (h *Handler) AddKey(ctx context.Context, in *keypb.AddKey_Request) (*keypb.AddKey_Response, error) {
	response := new(keypb.AddKey_Response)

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errPublicKeyIsBroken
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	err = h.DB.Conn.QueryRow(`SELECT "id" FROM "user_public_key" WHERE "fingerprint" = $1`,
		fingerprint,
	).Scan(&response.KeyId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if response.KeyId != "" {
		return nil, errObjectAlreadyExists
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	err = h.DB.Conn.QueryRow(`INSERT INTO "user_public_key" ("user_id", "title", "key_", "fingerprint")
    VALUES ($1, $2, $3, $4) RETURNING id`,
		in.GetUserId(),
		comment,
		in.GetKey(),
		fingerprint,
	).Scan(&response.KeyId)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// UpdateKey is ...
func (h *Handler) UpdateKey(ctx context.Context, in *keypb.UpdateKey_Request) (*keypb.UpdateKey_Response, error) {
	var keyID string
	response := new(keypb.UpdateKey_Response)

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errPublicKeyIsBroken
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	err = h.DB.Conn.QueryRow(`SELECT "id" FROM "user_public_key" WHERE "fingerprint" = $1`,
		fingerprint,
	).Scan(&keyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if keyID != "" {
		return nil, errObjectAlreadyExists // This key has already been added
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	data, err := h.DB.Conn.Exec(`UPDATE "user_public_key"
    SET "title" = $1, "key_" = $2, "fingerprint" = $3, "last_update" = NOW()
		WHERE "id" = $4 AND "user_id" = $5`,
		comment,
		in.GetKey(),
		fingerprint,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteKey is ...
func (h *Handler) DeleteKey(ctx context.Context, in *keypb.DeleteKey_Request) (*keypb.DeleteKey_Response, error) {
	response := new(keypb.DeleteKey_Response)

	data, err := h.DB.Conn.Exec(`DELETE FROM "user_public_key" WHERE "id" = $1 AND "user_id" = $2`,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// GenerateSSHKey is ...
func (h *Handler) GenerateSSHKey(ctx context.Context, in *keypb.GenerateSSHKey_Request) (*keypb.GenerateSSHKey_Response, error) {
	response := new(keypb.GenerateSSHKey_Response)
	response.KeyType = in.GetKeyType()

	key, err := crypto.NewSSHKey(in.GetKeyType().String())
	if err != nil {
		log.FromGRPC(err).Send()
		return nil, crypto.ErrFailedCreatingSSHKey
	}

	response.Public = key.PublicKey
	response.Passphrase = key.Passphrase
	response.Uuid = uuid.New().String()

	cacheKey := new(keypb.GenerateSSHKey_Key)
	cacheKey.Private = string(key.PrivateKey)
	cacheKey.Public = string(key.PublicKey)
	mapB, _ := json.Marshal(cacheKey)

	if err := h.Cache.Set(fmt.Sprintf("tmp_key_ssh::%s", response.Uuid), mapB, internal.GetDuration("SSH_KEY_REFRESH_DURATION", "10m")); err != nil {
		return nil, errIncorrectParameters
	}

	return response, nil
}