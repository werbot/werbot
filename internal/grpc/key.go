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

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/crypto"

	pb_key "github.com/werbot/werbot/api/proto/key"
)

type key struct {
	pb_key.UnimplementedKeyHandlersServer
}

var (
	errPublicKeyIsBroken = errors.New("The public key has a broken structure")
)

// ListPublicKeys is ...
func (k *key) ListPublicKeys(ctx context.Context, in *pb_key.ListPublicKeys_Request) (*pb_key.ListPublicKeys_Response, error) {
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
		FROM
			"user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch + sqlFooter)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToSelect
	}

	keys := []*pb_key.PublicKey_Response{}
	for rows.Next() {
		var lastUsed, created pgtype.Timestamp
		publicKey := new(pb_key.PublicKey_Response)
		err = rows.Scan(
			&publicKey.KeyId,
			&publicKey.UserId,
			&publicKey.UserName,
			&publicKey.Title,
			&publicKey.Key,
			&publicKey.Fingerprint,
			&lastUsed,
			&created,
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToScan
		}
		publicKey.LastUsed = timestamppb.New(lastUsed.Time)
		publicKey.Created = timestamppb.New(created.Time)
		keys = append(keys, publicKey)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	err = service.db.Conn.QueryRow(`SELECT
      COUNT (*)
		FROM
			"user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch).
		Scan(&total)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	return &pb_key.ListPublicKeys_Response{
		Total:      total,
		PublicKeys: keys,
	}, nil
}

// PublicKey is ...
func (k *key) PublicKey(ctx context.Context, in *pb_key.PublicKey_Request) (*pb_key.PublicKey_Response, error) {
	var lastUsed, created pgtype.Timestamp
	publicKey := new(pb_key.PublicKey_Response)
	publicKey.KeyId = in.GetKeyId()
	err := service.db.Conn.QueryRow(`SELECT
			"user_public_key"."id" AS "key_id",
			"user_public_key"."user_id",
			"user"."name" AS "user_name",
			"user_public_key"."title",
			"user_public_key"."key_",
			"user_public_key"."fingerprint",
			"user_public_key"."last_used",
			"user_public_key"."created"
		FROM
			"user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id" WHERE "user_public_key"."id" = $1 AND "user_public_key"."user_id" = $2`, in.GetKeyId(), in.GetUserId()).
		Scan(
			&publicKey.KeyId,
			&publicKey.UserId,
			&publicKey.UserName,
			&publicKey.Title,
			&publicKey.Key,
			&publicKey.Fingerprint,
			&lastUsed,
			&created,
		)
	if err != nil {
		service.log.ErrorGRPC(err)
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		return nil, errFailedToScan
	}

	publicKey.LastUsed = timestamppb.New(lastUsed.Time)
	publicKey.Created = timestamppb.New(created.Time)
	return publicKey, nil
}

// AddPublicKey is ...
func (k *key) AddPublicKey(ctx context.Context, in *pb_key.AddPublicKey_Request) (*pb_key.AddPublicKey_Response, error) {
	var count int32
	var id string

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errPublicKeyIsBroken
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	err = service.db.Conn.QueryRow(`SELECT
			COUNT(*)
		FROM
			"user_public_key"
		WHERE
			"fingerprint" = $1`,
		fingerprint,
	).Scan(&count)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}
	if count > 0 {
		return nil, errObjectAlreadyExists // This key has already been added
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	err = service.db.Conn.QueryRow(`INSERT
		INTO "user_public_key" (
			"user_id",
			"title",
			"key_",
			"fingerprint",
			"created")
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id`,
		in.GetUserId(),
		comment,
		in.GetKey(),
		fingerprint,
	).Scan(&id)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToAdd
	}

	return &pb_key.AddPublicKey_Response{
		KeyId: id,
	}, nil
}

// UpdatePublicKey is ...
func (k *key) UpdatePublicKey(ctx context.Context, in *pb_key.UpdatePublicKey_Request) (*pb_key.UpdatePublicKey_Response, error) {
	var count int32

	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errPublicKeyIsBroken
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	service.db.Conn.QueryRow(`SELECT
			COUNT(*)
		FROM
			"user_public_key"
		WHERE
			"fingerprint" = $1`,
		fingerprint,
	).Scan(&count)
	if count > 1 {
		return nil, errObjectAlreadyExists // This key has already been added
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	data, err := service.db.Conn.Exec(`UPDATE "user_public_key"
		SET
			"title" = $1,
			"key_" = $2,
			"fingerprint" = $3
		WHERE
			"id" = $4
			AND "user_id" = $5`,
		comment,
		in.GetKey(),
		fingerprint,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_key.UpdatePublicKey_Response{}, nil
}

// DeletePublicKey is ...
func (k *key) DeletePublicKey(ctx context.Context, in *pb_key.DeletePublicKey_Request) (*pb_key.DeletePublicKey_Response, error) {
	data, err := service.db.Conn.Exec(`DELETE
		FROM
			"user_public_key"
		WHERE
			"id" = $1
			AND "user_id" = $2`,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToDelete
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return &pb_key.DeletePublicKey_Response{}, nil
}

// GenerateSSHKey is ...
func (k *key) GenerateSSHKey(ctx context.Context, in *pb_key.GenerateSSHKey_Request) (*pb_key.GenerateSSHKey_Response, error) {
	key, err := crypto.NewSSHKey(in.GetKeyType().String())
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, crypto.ErrFailedCreatingSSHKey
	}

	sub := uuid.New().String()
	if err := service.cache.Set(fmt.Sprintf("tmp_key_ssh::%s", sub), key.PrivateKey, internal.GetDuration("SSH_KEY_REFRESH_DURATION", "5m")); err != nil {
		return nil, errIncorrectParameters
	}

	return &pb_key.GenerateSSHKey_Response{
		KeyType:    in.GetKeyType(),
		Uuid:       sub,
		Public:     key.PublicKey,
		Passphrase: key.Passphrase,
	}, nil
}
