package account

import (
	"context"

	accountpb "github.com/werbot/werbot/internal/grpc/account/proto"
	"github.com/werbot/werbot/internal/trace"
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
// AccountIDByLogin is a function that takes a context and an AccountIDByLogin_Request as input,
// and returns an AccountIDByLogin_Response and an error as output.
func (h *Handler) AccountIDByLogin(ctx context.Context, in *accountpb.AccountIDByLogin_Request) (*accountpb.AccountIDByLogin_Response, error) {
	response := new(accountpb.AccountIDByLogin_Response)
	nameArray := strutil.SplitNTrimmed(in.GetLogin(), "_", 3)

	stmt, err := h.DB.Conn.PrepareContext(ctx, `
    SELECT
      "user"."id"
    FROM
      "user"
      JOIN "user_public_key" ON "user"."id" = "user_public_key"."user_id"
    WHERE
      "user"."login" = $1
      AND "user_public_key"."fingerprint" = $2
  `)
	if err != nil {
		return nil, trace.ErrorAborted(err, log)
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, nameArray[0], in.GetFingerprint()).Scan(&response.UserId)
	if err != nil {
		return nil, trace.ErrorAborted(err, log)
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

// UpdateStatus is a method implemented by Handler struct which accepts
// a context and an UpdateStatus_Request object and returns an UpdateStatus_Response object and an error
func (h *Handler) UpdateStatus(ctx context.Context, in *accountpb.UpdateStatus_Request) (*accountpb.UpdateStatus_Response, error) {
	response := new(accountpb.UpdateStatus_Response)

	online := false
	if in.GetStatus() == 1 {
		online = true
	}

	_, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "server_member"
    SET
      "online" = $2
    WHERE
      "id" = $1
  `, in.GetAccountId(), online)
	if err != nil {
		return nil, trace.ErrorAborted(err, log, trace.MsgFailedToUpdate)
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
