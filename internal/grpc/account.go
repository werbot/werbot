package grpc

import (
	"context"
	"database/sql"

	accountpb "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/pkg/strutil"
)

type account struct {
	accountpb.UnimplementedAccountHandlersServer
}

// TODO ListAccounts is ...
func (s *account) ListAccounts(ctx context.Context, in *accountpb.ListAccounts_Request) (*accountpb.ListAccounts_Response, error) {
	response := new(accountpb.ListAccounts_Response)
	return response, nil
}

// TODO Account is ...
func (s *account) Account(ctx context.Context, in *accountpb.Account_Request) (*accountpb.Account_Response, error) {
	response := new(accountpb.Account_Response)
	return response, nil
}

// TODO AddAccount is ...
func (s *account) AddAccount(ctx context.Context, in *accountpb.AddAccount_Request) (*accountpb.AddAccount_Response, error) {
	response := new(accountpb.AddAccount_Response)
	return response, nil
}

// TODO UpdateAccount is ...
func (s *account) UpdateAccount(ctx context.Context, in *accountpb.UpdateAccount_Request) (*accountpb.UpdateAccount_Response, error) {
	response := new(accountpb.UpdateAccount_Response)
	return response, nil
}

// TODO DeleteAccount is ...
func (s *account) DeleteAccount(ctx context.Context, in *accountpb.DeleteAccount_Request) (*accountpb.DeleteAccount_Response, error) {
	response := new(accountpb.DeleteAccount_Response)
	return response, nil
}

// AccountIDByLogin is ...
// TODO: Check bu invite
// TODO: Enable check in Firewall
func (s *account) AccountIDByLogin(ctx context.Context, in *accountpb.AccountIDByLogin_Request) (*accountpb.AccountIDByLogin_Response, error) {
	response := new(accountpb.AccountIDByLogin_Response)
	nameArray := strutil.SplitNTrimmed(in.GetLogin(), "_", 3)

	err := service.db.Conn.QueryRow(`SELECT "user"."id"
		FROM "user"
			JOIN "user_public_key" ON "user"."id" = "user_public_key"."user_id"
		WHERE "user"."login" = $1
			AND "user_public_key".fingerprint = $2`,
		nameArray[0],
		in.GetFingerprint(),
	).Scan(&response.UserId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		service.log.FromGRPC(err).Send()
		return nil, errServerError
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

	return response, nil
}

// UpdateStatus is ...
func (s *account) UpdateStatus(ctx context.Context, in *accountpb.UpdateStatus_Request) (*accountpb.UpdateStatus_Response, error) {
	var data sql.Result
	var err error
	response := new(accountpb.UpdateStatus_Response)

	switch in.GetStatus() {
	case 1:
		data, err = service.db.Conn.Exec(`UPDATE "server_member" SET "online" = true, "last_update" = NOW() WHERE "id" = $1`,
			in.GetAccountId(),
		)
	case 2:
		data, err = service.db.Conn.Exec(`UPDATE "server_member" SET "online" = false, "last_update" = NOW() WHERE "id" = $1`,
			in.GetAccountId(),
		)
	}

	if err != nil {
		service.log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// TODO SessionAccount is ...
func (s *account) SessionAccount(ctx context.Context, in *accountpb.SessionAccount_Request) (*accountpb.SessionAccount_Response, error) {
	response := new(accountpb.SessionAccount_Response)
	return response, nil
}

// TODO FindByTokenAccount is ...
func (s *account) FindByTokenAccount(ctx context.Context, in *accountpb.FindByTokenAccount_Request) (*accountpb.FindByTokenAccount_Response, error) {
	response := new(accountpb.FindByTokenAccount_Response)
	return response, nil
}
