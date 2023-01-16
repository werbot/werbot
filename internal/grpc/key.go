package grpc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"

	keypb "github.com/werbot/werbot/api/proto/key"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"
)

type key struct {
	keypb.UnimplementedKeyHandlersServer
}

var (
	errPublicKeyIsBroken = errors.New("the public key has a broken structure")
)

// ListKeys is ...
func (k *key) ListKeys(ctx context.Context, in *keypb.ListKeys_Request) (*keypb.ListKeys_Response, error) {
	response := new(keypb.ListKeys_Response)

	sqlSearch := service.db.SQLAddWhere(in.GetQuery())
	sqlFooter := service.db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())
	rows, err := service.db.Conn.Query(`SELECT
			"user_public_key"."id" AS "key_id",
			"user_public_key"."user_id",
			"user"."name" AS "user_name",
			"user_public_key"."title",
			"user_public_key"."key_",
			"user_public_key"."fingerprint",
			"user_public_key"."last_used",
			"user_public_key"."created"
		FROM "user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	for rows.Next() {
		var lastUsed, created pgtype.Timestamp
		publicKey := new(keypb.Key_Response)
		err = rows.Scan(&publicKey.KeyId,
			&publicKey.UserId,
			&publicKey.UserName,
			&publicKey.Title,
			&publicKey.Key,
			&publicKey.Fingerprint,
			&lastUsed,
			&created,
		)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, errNotFound
			}
			service.log.FromGRPC(err).Send()
			return nil, errServerError
		}
		publicKey.LastUsed = timestamppb.New(lastUsed.Time)
		publicKey.Created = timestamppb.New(created.Time)
		response.PublicKeys = append(response.PublicKeys, publicKey)
	}
	defer rows.Close()

	// Total count for pagination
	err = service.db.Conn.QueryRow(`SELECT COUNT (*)
		FROM "user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch,
	).Scan(&response.Total)
	if err != nil && err != sql.ErrNoRows {
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	return response, nil
}

// PublicKey is ...
func (k *key) PublicKey(ctx context.Context, in *keypb.Key_Request) (*keypb.Key_Response, error) {
	var lastUsed, created pgtype.Timestamp
	response := new(keypb.Key_Response)
	response.KeyId = in.GetKeyId()

	err := service.db.Conn.QueryRow(`SELECT
			"user_public_key"."id" AS "key_id",
			"user_public_key"."user_id",
			"user"."name" AS "user_name",
			"user_public_key"."title",
			"user_public_key"."key_",
			"user_public_key"."fingerprint",
			"user_public_key"."last_used",
			"user_public_key"."created"
		FROM "user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"
        WHERE "user_public_key"."id" = $1
          AND "user_public_key"."user_id" = $2`,
		in.GetKeyId(),
		in.GetUserId(),
	).Scan(&response.KeyId,
		&response.UserId,
		&response.UserName,
		&response.Title,
		&response.Key,
		&response.Fingerprint,
		&lastUsed,
		&created,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}

	response.LastUsed = timestamppb.New(lastUsed.Time)
	response.Created = timestamppb.New(created.Time)

	return response, nil
}

// AddKey is ...
func (k *key) AddKey(ctx context.Context, in *keypb.AddKey_Request) (*keypb.AddKey_Response, error) {
	response := new(keypb.AddKey_Response)

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errPublicKeyIsBroken
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	err = service.db.Conn.QueryRow(`SELECT "id" FROM "user_public_key" WHERE "fingerprint" = $1`,
		fingerprint,
	).Scan(&response.KeyId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if response.KeyId != "" {
		return nil, errObjectAlreadyExists
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	err = service.db.Conn.QueryRow(`INSERT INTO "user_public_key" ("user_id", "title", "key_", "fingerprint", "created")
    VALUES ($1, $2, $3, $4, NOW()) RETURNING id`,
		in.GetUserId(),
		comment,
		in.GetKey(),
		fingerprint,
	).Scan(&response.KeyId)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToAdd
	}

	return response, nil
}

// UpdateKey is ...
func (k *key) UpdateKey(ctx context.Context, in *keypb.UpdateKey_Request) (*keypb.UpdateKey_Response, error) {
	var keyID string
	response := new(keypb.UpdateKey_Response)

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errPublicKeyIsBroken
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	err = service.db.Conn.QueryRow(`SELECT "id" FROM "user_public_key" WHERE "fingerprint" = $1`,
		fingerprint,
	).Scan(&keyID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
	}
	if keyID != "" {
		return nil, errObjectAlreadyExists // This key has already been added
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	data, err := service.db.Conn.Exec(`UPDATE "user_public_key"
    SET "title" = $1, "key_" = $2, "fingerprint" = $3
		WHERE "id" = $4 AND "user_id" = $5`,
		comment,
		in.GetKey(),
		fingerprint,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// DeleteKey is ...
func (k *key) DeleteKey(ctx context.Context, in *keypb.DeleteKey_Request) (*keypb.DeleteKey_Response, error) {
	response := new(keypb.DeleteKey_Response)

	data, err := service.db.Conn.Exec(`DELETE FROM "user_public_key" WHERE "id" = $1 AND "user_id" = $2`,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// GenerateSSHKey is ...
func (k *key) GenerateSSHKey(ctx context.Context, in *keypb.GenerateSSHKey_Request) (*keypb.GenerateSSHKey_Response, error) {
	response := new(keypb.GenerateSSHKey_Response)
	response.KeyType = in.GetKeyType()

	key, err := crypto.NewSSHKey(in.GetKeyType().String())
	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, crypto.ErrFailedCreatingSSHKey
	}

	response.Public = key.PublicKey
	response.Passphrase = key.Passphrase
	response.Uuid = uuid.New().String()

	if err := service.cache.Set(fmt.Sprintf("tmp_key_ssh::%s", response.Uuid), key.PrivateKey, internal.GetDuration("SSH_KEY_REFRESH_DURATION", "5m")); err != nil {
		return nil, errIncorrectParameters
	}

	return response, nil
}
