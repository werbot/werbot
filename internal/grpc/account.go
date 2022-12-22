package grpc

import (
	"context"
	"time"

	pb_account "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/internal/utils/parse"
)

type account struct {
	pb_account.UnimplementedAccountHandlersServer
}

// AccountIDByName is ...
// TODO: Check bu invite
// TODO: Enable check in Firewall
func (s *account) AccountIDByName(ctx context.Context, in *pb_account.AccountIDByName_Request) (*pb_account.AccountIDByName_Response, error) {
	var id string
	nameArray := parse.Username(in.GetUsername())
	err := service.db.Conn.QueryRow(`SELECT
			"user"."id"
		FROM
			"user"
			JOIN "user_public_key" ON "user"."id" = "user_public_key"."user_id"
		WHERE
			"user"."name" = $1
			AND "user_public_key".fingerprint = $2`,
		nameArray[0],
		in.GetFingerprint(),
	).Scan(&id)
	if err != nil {
		service.log.ErrorGRPC(err)
		return nil, errFailedToScan
	}

	/*
		// OLD CODE
		if id > 0 {
			if actx.userType() == UserTypeInvite {
				actx.err = errors.New("invites are only supported for new SSH keys; your ssh key is already associated with the user")
			}
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

	return &pb_account.AccountIDByName_Response{
		UserId: id,
	}, nil
}

// UpdateAccountStatus is ...
func (s *account) UpdateAccountStatus(ctx context.Context, in *pb_account.UpdateAccountStatus_Request) (*pb_account.UpdateAccountStatus_Response, error) {
	if in.GetStatus() == 1 {
		data, err := service.db.Conn.Exec(`UPDATE "server_member"
			SET
				"online" = true,
				"last_activity" = $1
			WHERE
				"id" = $2`,
			time.Now(),
			in.GetAccountId(),
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}
	if in.Status == 2 {
		data, err := service.db.Conn.Exec(`UPDATE "server_member"
			SET
				"online" = false
			WHERE
				"id" = $1`,
			in.GetAccountId(),
		)
		if err != nil {
			service.log.ErrorGRPC(err)
			return nil, errFailedToUpdate
		}
		if affected, _ := data.RowsAffected(); affected == 0 {
			return nil, errNotFound
		}
	}

	return &pb_account.UpdateAccountStatus_Response{}, nil
}

// TODO SessionAccount is ...
func (s *account) SessionAccount(ctx context.Context, in *pb_account.SessionAccount_Request) (*pb_account.SessionAccount_Response, error) {
	return &pb_account.SessionAccount_Response{}, nil
}

// TODO FindByTokenAccount is ...
func (s *account) FindByTokenAccount(ctx context.Context, in *pb_account.FindByTokenAccount_Request) (*pb_account.FindByTokenAccount_Response, error) {
	return &pb_account.FindByTokenAccount_Response{}, nil
}
