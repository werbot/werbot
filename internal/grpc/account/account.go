package account

import (
	"context"
	"database/sql"

	accountpb "github.com/werbot/werbot/api/proto/account"
	"github.com/werbot/werbot/pkg/strutil"
)

// ListAccounts is ...
func (h *Handler) ListAccounts(ctx context.Context, in *accountpb.ListAccounts_Request) (*accountpb.ListAccounts_Response, error) {
	response := new(accountpb.ListAccounts_Response)
	return response, nil
}

// Account is ...
func (h *Handler) Account(ctx context.Context, in *accountpb.Account_Request) (*accountpb.Account_Response, error) {
	response := new(accountpb.Account_Response)
	return response, nil
}

// AddAccount is ...
func (h *Handler) AddAccount(ctx context.Context, in *accountpb.AddAccount_Request) (*accountpb.AddAccount_Response, error) {
	response := new(accountpb.AddAccount_Response)
	return response, nil
}

// UpdateAccount is ...
func (h *Handler) UpdateAccount(ctx context.Context, in *accountpb.UpdateAccount_Request) (*accountpb.UpdateAccount_Response, error) {
	response := new(accountpb.UpdateAccount_Response)
	return response, nil
}

// DeleteAccount is ...
func (h *Handler) DeleteAccount(ctx context.Context, in *accountpb.DeleteAccount_Request) (*accountpb.DeleteAccount_Response, error) {
	response := new(accountpb.DeleteAccount_Response)
	return response, nil
}

// TODO Check bu invite and Enable check in Firewall
// AccountIDByLogin is ...
func (h *Handler) AccountIDByLogin(ctx context.Context, in *accountpb.AccountIDByLogin_Request) (*accountpb.AccountIDByLogin_Response, error) {
	response := new(accountpb.AccountIDByLogin_Response)
	nameArray := strutil.SplitNTrimmed(in.GetLogin(), "_", 3)

	sqlRpw := h.DB.Conn.QueryRow(`SELECT "user"."id"
		FROM "user"
			JOIN "user_public_key" ON "user"."id" = "user_public_key"."user_id"
		WHERE "user"."login" = $1
			AND "user_public_key".fingerprint = $2`,
		nameArray[0],
		in.GetFingerprint(),
	)
	err := sqlRpw.Scan(&response.UserId)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errNotFound
		}
		log.FromGRPC(err).Send()
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
func (h *Handler) UpdateStatus(ctx context.Context, in *accountpb.UpdateStatus_Request) (*accountpb.UpdateStatus_Response, error) {
	var data sql.Result
	var err error
	response := new(accountpb.UpdateStatus_Response)

	switch in.GetStatus() {
	case 1:
		data, err = h.DB.Conn.Exec(`UPDATE "server_member" SET "online" = true, "last_update" = NOW() WHERE "id" = $1`,
			in.GetAccountId(),
		)
	case 2:
		data, err = h.DB.Conn.Exec(`UPDATE "server_member" SET "online" = false, "last_update" = NOW() WHERE "id" = $1`,
			in.GetAccountId(),
		)
	}

	if err != nil {
		log.FromGRPC(err).Send()
		return nil, errFailedToUpdate
	}
	if affected, _ := data.RowsAffected(); affected == 0 {
		return nil, errNotFound
	}

	return response, nil
}

// SessionAccount is ...
func (h *Handler) SessionAccount(ctx context.Context, in *accountpb.SessionAccount_Request) (*accountpb.SessionAccount_Response, error) {
	response := new(accountpb.SessionAccount_Response)
	return response, nil
}

// FindByTokenAccount is ...
func (h *Handler) FindByTokenAccount(ctx context.Context, in *accountpb.FindByTokenAccount_Request) (*accountpb.FindByTokenAccount_Response, error) {
	response := new(accountpb.FindByTokenAccount_Response)
	return response, nil
}
