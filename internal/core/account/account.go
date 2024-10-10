package account

import (
	"context"

	accountpb "github.com/werbot/werbot/internal/core/account/proto/account"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/strutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListAccounts is ...
func (h *Handler) ListAccounts(ctx context.Context, in *accountpb.ListAccounts_Request) (*accountpb.ListAccounts_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.ListAccounts_Response{}
	return response, nil
}

// Account is ...
func (h *Handler) Account(ctx context.Context, in *accountpb.Account_Request) (*accountpb.Account_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.Account_Response{}
	return response, nil
}

// AddAccount is ...
func (h *Handler) AddAccount(ctx context.Context, in *accountpb.AddAccount_Request) (*accountpb.AddAccount_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.AddAccount_Response{}
	return response, nil
}

// UpdateAccount is ...
func (h *Handler) UpdateAccount(ctx context.Context, in *accountpb.UpdateAccount_Request) (*accountpb.UpdateAccount_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.UpdateAccount_Response{}
	return response, nil
}

// DeleteAccount is ...
func (h *Handler) DeleteAccount(ctx context.Context, in *accountpb.DeleteAccount_Request) (*accountpb.DeleteAccount_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.DeleteAccount_Response{}
	return response, nil
}

// TODO Check bu invite and Enable check in Firewall
// AccountIDByLogin is a function that takes a context and an AccountIDByLogin_Request as input,
// and returns an AccountIDByLogin_Response and an error as output.
func (h *Handler) AccountIDByLogin(ctx context.Context, in *accountpb.AccountIDByLogin_Request) (*accountpb.AccountIDByLogin_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.AccountIDByLogin_Response{}
	nameArray := strutil.SplitNTrimmed(in.GetLogin(), "_", 3)

	stmt, err := h.DB.Conn.PrepareContext(ctx, `
    SELECT "user"."id"
    FROM
      "user"
      JOIN "user_public_key" ON "user".i"d = "user_public_key"."user_id"
    WHERE
      "user"."login" = $1
      AND "user_public_key"."fingerprint" = $2
  `)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}
	defer stmt.Close()

	err = stmt.QueryRowContext(ctx, nameArray[0], in.GetFingerprint()).Scan(&response.UserId)
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	return response, nil
}

// UpdateStatus is a method implemented by Handler struct which accepts
// a context and an UpdateStatus_Request object and returns an UpdateStatus_Response object and an error
func (h *Handler) UpdateStatus(ctx context.Context, in *accountpb.UpdateStatus_Request) (*accountpb.UpdateStatus_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.UpdateStatus_Response{}

	online := false
	if in.GetStatus() == 1 {
		online = true
	}

	res, err := h.DB.Conn.ExecContext(ctx, `
    UPDATE "scheme_member"
    SET "online" = $2
    WHERE "id" = $1
  `,
		in.GetAccountId(),
		online,
	)
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToUpdate)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		errGRPC := status.Error(codes.NotFound, trace.MsgAccountNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	return response, nil
}

// SessionAccount is ...
func (h *Handler) SessionAccount(ctx context.Context, in *accountpb.SessionAccount_Request) (*accountpb.SessionAccount_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.SessionAccount_Response{}
	return response, nil
}

// FindByTokenAccount is ...
func (h *Handler) FindByTokenAccount(ctx context.Context, in *accountpb.FindByTokenAccount_Request) (*accountpb.FindByTokenAccount_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		errGRPC := status.Error(codes.InvalidArgument, err.Error())
		return nil, trace.Error(errGRPC, log, nil)
	}

	response := &accountpb.FindByTokenAccount_Response{}
	return response, nil
}
