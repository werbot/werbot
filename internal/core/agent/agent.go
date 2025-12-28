package agent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	agentmessage "github.com/werbot/werbot/internal/core/agent/proto/message"
	"github.com/werbot/werbot/internal/core/scheme"
	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemeauthpb "github.com/werbot/werbot/internal/core/scheme/proto/auth"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/pkg/crypto"
	"github.com/werbot/werbot/pkg/utils/protoutils"
)

// Auth is ...
func (h *Handler) Auth(ctx context.Context, in *agentmessage.Auth_Request) (*agentmessage.Auth_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	response := &agentmessage.Auth_Response{}

	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "agent_token"."scheme_type",
      "project_api"."api_key",
      "project_api"."api_secret"
    FROM
      "agent_token"
      INNER JOIN "project_api" ON "agent_token"."project_id" = "project_api"."project_id"
    WHERE
      "agent_token"."token" = $1
      AND "agent_token"."active" = TRUE
      AND "project_api"."active" = TRUE
  `, in.GetToken()).Scan(
		&response.SchemeType,
		&response.ApiKey,
		&response.ApiSecret,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.NotFound, trace.MsgTokenNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	return response, nil
}

// AddScheme is ...
func (h *Handler) AddScheme(ctx context.Context, in *agentmessage.AddScheme_Request) (*agentmessage.AddScheme_Response, error) {
	if err := protoutils.ValidateRequest(in); err != nil {
		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
	}

	// core info about scheme described in token
	var ownerID, projectID string
	var schemeType schemeaccesspb.SchemeType
	err := h.DB.Conn.QueryRowContext(ctx, `
    SELECT
      "project"."owner_id",
      "agent_token"."project_id",
      "agent_token"."scheme_type"
    FROM
      "agent_token"
      INNER JOIN "project" ON "agent_token"."project_id" = "project"."id"
    WHERE
      "token" = $1
      AND "active" = TRUE
  `, in.GetToken()).Scan(
		&ownerID,
		&projectID,
		&schemeType,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			errGRPC := status.Error(codes.NotFound, trace.MsgTokenNotFound)
			return nil, trace.Error(errGRPC, log, nil)
		}
		return nil, trace.Error(err, log, "")
	}

	// define access scheme
	// now - only SSH access :)
	accessScheme := &schemeaccesspb.AccessScheme{}
	switch schemeType {
	case schemeaccesspb.SchemeType_server_ssh: // server ssh
		accessScheme.Access = &schemeaccesspb.AccessScheme_ServerSsh{
			ServerSsh: &schemeaccesspb.AccessScheme_Server_SSH{
				Alias:   crypto.NewPassword(6, false),
				Address: in.GetAddress(),
				Port:    in.GetPort(),
				Access: &schemeaccesspb.AccessScheme_Server_SSH_Key{
					Key: &schemeauthpb.Auth_Key{
						Login: in.GetLogin(),
						Access: &schemeauthpb.Auth_Key_KeyId{
							KeyId: "00000000-0000-0000-0000-000000000000",
						},
					},
				},
			},
		}
	default:
		errGRPC := status.Error(codes.NotFound, trace.MsgSchemeNotFound)
		return nil, trace.Error(errGRPC, log, nil)
	}

	// add core record about scheme
	title := in.GetTitle()
	if title == "" {
		title = fmt.Sprintf("Agent server #%s", crypto.NewPassword(6, false))
	}

	schemeHandler := &scheme.Handler{
		DB:    h.DB,
		Redis: h.Redis,
	}

	// add new scheme
	newSchemeData, err := schemeHandler.AddScheme(ctx, &schemepb.AddScheme_Request{
		OwnerId:   ownerID,
		ProjectId: projectID,
		Title:     title,
		Scheme:    accessScheme,
	})
	if err != nil {
		return nil, trace.Error(err, log, nil)
	}

	response := &agentmessage.AddScheme_Response{
		SchemeId: newSchemeData.GetSchemeId(),
	}

	// read public key
	err = h.DB.Conn.QueryRowContext(ctx, `
    SELECT "access"->'key'->'key'->>'public'
    FROM "scheme"
    WHERE "id" = $1
  `, response.GetSchemeId()).Scan(&response.PublicKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, trace.Error(err, log, "")
	}

	// block token
	_, err = h.DB.Conn.ExecContext(ctx, `
    UPDATE "agent_token"
    SET "active" = FALSE
    WHERE "token" = $1 AND "one_time" = TRUE
  `, in.GetToken())
	if err != nil {
		return nil, trace.Error(err, log, trace.MsgFailedToDelete)
	}

	return response, nil
}

// UpdateScheme is ...
//func (h *Handler) UpdateScheme(ctx context.Context, in *agentmessage.UpdateScheme_Request) (*agentmessage.UpdateScheme_Response, error) {
//	if err := protoutils.ValidateRequest(in); err != nil {
//		return nil, trace.Error(status.Error(codes.InvalidArgument, err.Error()), log, nil)
//	}
//
//	response := &agentmessage.UpdateScheme_Response{}
//
//	return response, nil
//}
