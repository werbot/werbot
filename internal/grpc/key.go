package grpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/config"
	"github.com/werbot/werbot/internal/crypto"
	"github.com/werbot/werbot/internal/message"

	pb_key "github.com/werbot/werbot/internal/grpc/proto/key"
)

type key struct {
	pb_key.UnimplementedKeyHandlersServer
}

// ListPublicKeys is ...
func (k *key) ListPublicKeys(ctx context.Context, in *pb_key.ListPublicKeys_Request) (*pb_key.ListPublicKeys_Response, error) {
	sqlSearch := db.SQLAddWhere(in.GetQuery())
	sqlFooter := db.SQLPagination(in.GetLimit(), in.GetOffset(), in.GetSortBy())

	rows, err := db.Conn.Query(`SELECT
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
		return nil, errors.New("ListPublicKeys failed")
	}

	keys := []*pb_key.GetPublicKey_Response{}
	for rows.Next() {
		publicKey := pb_key.GetPublicKey_Response{}
		var lastUsed, created pgtype.Timestamp

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
			return nil, errors.New("ListCustomers Scan")
		}

		publicKey.LastUsed = timestamppb.New(lastUsed.Time)
		publicKey.Created = timestamppb.New(created.Time)

		keys = append(keys, &publicKey)
	}
	defer rows.Close()

	// Total count for pagination
	var total int32
	db.Conn.QueryRow(`SELECT COUNT (*)
		FROM
			"user_public_key"
			INNER JOIN "user" ON "user_public_key"."user_id" = "user"."id"` + sqlSearch).Scan(&total)

	return &pb_key.ListPublicKeys_Response{
		Total:      total,
		PublicKeys: keys,
	}, nil
}

// GetPublicKey is ...
func (k *key) GetPublicKey(ctx context.Context, in *pb_key.GetPublicKey_Request) (*pb_key.GetPublicKey_Response, error) {
	publicKey := pb_key.GetPublicKey_Response{}
	publicKey.KeyId = in.GetKeyId()
	var lastUsed, created pgtype.Timestamp

	err := db.Conn.QueryRow(`SELECT
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
		return nil, errors.New(message.ErrNotFound)
	}

	publicKey.LastUsed = timestamppb.New(lastUsed.Time)
	publicKey.Created = timestamppb.New(created.Time)

	return &publicKey, nil
}

// CreatePublicKey is ...
func (k *key) CreatePublicKey(ctx context.Context, in *pb_key.CreatePublicKey_Request) (*pb_key.CreatePublicKey_Response, error) {
	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		return nil, errors.New("The public key has a broken structure")
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	var count int32
	db.Conn.QueryRow(`SELECT COUNT(*) FROM "user_public_key" WHERE "fingerprint" = $1`, fingerprint).Scan(&count)
	if count > 0 {
		return nil, errors.New("This key has already been added")
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	var id string
	err = db.Conn.QueryRow(`INSERT INTO "user_public_key" ("user_id", "title", "key_", "fingerprint", "created") VALUES ($1, $2, $3, $4, NOW()) RETURNING id`,
		in.GetUserId(),
		comment,
		in.GetKey(),
		fingerprint,
	).Scan(&id)
	if err != nil {
		return nil, errors.New("CreatePublicKey failed")
	}

	return &pb_key.CreatePublicKey_Response{
		KeyId: id,
	}, nil
}

// UpdatePublicKey is ...
func (k *key) UpdatePublicKey(ctx context.Context, in *pb_key.UpdatePublicKey_Request) (*pb_key.UpdatePublicKey_Response, error) {
	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(in.GetKey()))
	if err != nil {
		return nil, errors.New("The public key has a broken structure")
	}
	fingerprint := ssh.FingerprintLegacyMD5(publicKey)

	// Check public key fingerprint
	var count int32
	db.Conn.QueryRow(`SELECT COUNT(*) FROM "user_public_key" WHERE "fingerprint" = $1`, fingerprint).Scan(&count)
	if count > 1 {
		return nil, errors.New("This key has already been added")
	}

	if in.GetTitle() != "" {
		comment = in.GetTitle()
	}

	data, err := db.Conn.Exec(`UPDATE "user_public_key" SET "title" = $1, "key_" = $2, "fingerprint" = $3 WHERE "id" = $4 AND "user_id" = $5`,
		comment,
		in.GetKey(),
		fingerprint,
		in.GetKeyId(),
		in.GetUserId(),
	)
	if err != nil {
		return nil, errors.New("UpdatePublicKey failed")
	}

	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errors.New(message.ErrNotFound)
	}

	return &pb_key.UpdatePublicKey_Response{}, nil
}

// DeletePublicKey is ...
func (k *key) DeletePublicKey(ctx context.Context, in *pb_key.DeletePublicKey_Request) (*pb_key.DeletePublicKey_Response, error) {
	data, err := db.Conn.Exec(`DELETE FROM "user_public_key" WHERE "id" = $1 AND "user_id" = $2`, in.GetKeyId(), in.GetUserId())
	if err != nil {
		return nil, errors.New("DeletePublicKey failed")
	}

	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errors.New(message.ErrNotFound)
	}

	return &pb_key.DeletePublicKey_Response{}, nil
}

// GenerateSSHKey is ...
func (k *key) GenerateSSHKey(ctx context.Context, in *pb_key.GenerateSSHKey_Request) (*pb_key.GenerateSSHKey_Response, error) {
	key, err := crypto.NewSSHKey(in.GetKeyType().String())
	if err != nil {
		//return nil, errors.New("Error generating SSH key pair")
		return nil, err
	}

	sub := uuid.New().String()
	if err := cache.Set(fmt.Sprintf("tmp_key_ssh::%s", sub), key.PrivateKey, config.GetDuration("SSH_KEY_REFRESH_DURATION", "5m")); err != nil {
		return nil, err
	}

	return &pb_key.GenerateSSHKey_Response{
		KeyType:    in.GetKeyType(),
		Uuid:       sub,
		Public:     key.PublicKey,
		Passphrase: key.Passphrase,
	}, nil
}
