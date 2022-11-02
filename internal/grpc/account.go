package grpc

import (
	"context"
	"errors"
	"time"

	pb_account "github.com/werbot/werbot/internal/grpc/proto/account"
	"github.com/werbot/werbot/internal/utils/parse"
)

type account struct {
	pb_account.UnimplementedAccountHandlersServer
}

// GetAccountByID is ...
// TODO: проверка на invite
// TODO: включить проверку в Firewall
func (s *account) GetAccountByID(ctx context.Context, in *pb_account.GetAccountByID_Request) (*pb_account.GetAccountByID_Response, error) {
	nameArray := parse.UsernameParseInfo(in.GetUsername())
	var id string

	row := db.Conn.QueryRow(`SELECT
			"user"."id" 
		FROM
			"user"
			JOIN "user_public_key" ON "user"."id" = "user_public_key"."user_id" 
		WHERE
			"user"."name" = $1 
			AND "user_public_key".fingerprint = $2`, nameArray[0], in.GetFingerprint())

	if err := row.Scan(&id); err != nil {
		return nil, errors.New("select to user_public_key")
	}

	/*
		if id > 0 {
				if actx.userType() == UserTypeInvite {
					actx.err = errors.New("invites are only supported for new SSH keys; your ssh key is already associated with the user")
				}
	*/
	/*
				firewall_setting := security.Setting{
					db,
					ctx,
					config.Settings.ConfigPath,
					config.Settings.FirewallWorkCountry,
					config.Settings.FirewallBlacklistUris,
				}
				if !security.FirewallIpCheck(firewall_setting) {
					return false
				}
		}
	*/

	return &pb_account.GetAccountByID_Response{
		UserId: id,
	}, nil
}

// SetAccountStatus is ...
func (s *account) SetAccountStatus(ctx context.Context, in *pb_account.SetAccountStatus_Request) (*pb_account.SetAccountStatus_Response, error) {
	if in.GetStatus() == 1 {
		if _, err := db.Conn.Exec(`UPDATE "server_account" SET "online" = true, "last_activity" = $1 WHERE "id" = $2`, time.Now(), in.GetAccountId()); err != nil {
			return &pb_account.SetAccountStatus_Response{}, errors.New("SetAccountStatus update server account failed")
		}
	}
	if in.Status == 2 {
		if _, err := db.Conn.Exec(`UPDATE "server_account" SET "online" = false WHERE "id" = $1`, in.GetAccountId()); err != nil {
			return &pb_account.SetAccountStatus_Response{}, errors.New("SetAccountStatus update server account failed")
		}
	}
	return &pb_account.SetAccountStatus_Response{}, nil
}

// TODO SessionAccount is ...
func (s *account) SessionAccount(ctx context.Context, in *pb_account.SessionAccount_Request) (*pb_account.SessionAccount_Response, error) {
	return &pb_account.SessionAccount_Response{}, nil
}

// TODO FindByTokenAccount is ...
func (s *account) FindByTokenAccount(ctx context.Context, in *pb_account.FindByTokenAccount_Request) (*pb_account.FindByTokenAccount_Response, error) {
	return &pb_account.FindByTokenAccount_Response{}, nil
}
