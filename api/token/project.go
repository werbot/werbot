package token

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	notificationenum "github.com/werbot/werbot/internal/core/notification/proto/enum"
	notificationmessage "github.com/werbot/werbot/internal/core/notification/proto/message"
	notificationrpc "github.com/werbot/werbot/internal/core/notification/proto/rpc"
	tokenenum "github.com/werbot/werbot/internal/core/token/proto/enum"
	tokenmessage "github.com/werbot/werbot/internal/core/token/proto/message"
	tokenpb "github.com/werbot/werbot/internal/core/token/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/utils/protoutils"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary List project invites
// @Description Returns pending invitations for a project owned by the session profile.
// @Tags tokens
// @Produce json
// @Param project_id path string true "Project UUID"
// @Param owner_id query string false "Owner UUID. Accessible with ROLE_ADMIN rights"
// @Param limit query int false "Limit for pagination"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.ProjectTokens_Response}
// @Failure 401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/project/{project_id} [get]
func (h *Handler) projectTokens(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)
	pagination := webutil.GetPaginationFromCtx(c)

	request := &tokenmessage.ProjectTokens_Request{
		IsAdmin:   true, // the SQL filters by owner; unmask the token id for the owner
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
		Action:    tokenenum.Action_request,
		Limit:     pagination.Limit,
		Offset:    pagination.Offset,
		SortBy:    `"created_at":ASC`,
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokens, err := rClient.ProjectTokens(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokens)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Project tokens", result)
}

// @Summary Invite a project member
// @Description Creates a project invite for a new profile (email, name, surname) or an existing profile (profile_id).
// @Tags tokens
// @Accept json
// @Produce json
// @Param project_id path string true "Project UUID"
// @Param owner_id query string false "Owner UUID. Accessible with ROLE_ADMIN rights"
// @Param body body addProjectTokenRequest true "Project invite"
// @Success 200 {object} webutil.HTTPResponse{result=tokenmessage.AddTokenProjectMember_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/tokens/project/{project_id} [post]
func (h *Handler) addProjectToken(c *fiber.Ctx) error {
	sessionData := session.AuthProfile(c)

	body := &addProjectTokenRequest{}
	if err := c.BodyParser(body); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	request := &tokenmessage.AddTokenProjectMember_Request{
		OwnerId:   sessionData.ProfileID(c.Query("owner_id")),
		ProjectId: c.Params("project_id"),
	}

	if body.Email != "" && body.Name != "" && body.Surname != "" {
		request.Profile = &tokenmessage.AddTokenProjectMember_Request_CreateNewProfile{
			CreateNewProfile: &tokenmessage.MetaDataProfile{
				Name:    body.Name,
				Surname: body.Surname,
				Email:   body.Email,
			},
		}
	} else {
		request.Profile = &tokenmessage.AddTokenProjectMember_Request_ProfileId{
			ProfileId: body.ProfileID,
		}
	}

	rClient := tokenpb.NewTokenHandlersClient(h.Grpc)
	tokenData, err := rClient.AddTokenProjectMember(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	// Email delivery must not fail the invite.
	if body.Email != "" {
		h.sendProjectInvite(c.UserContext(), body.Email, tokenData.GetToken())
	}

	result, err := protoutils.ConvertProtoMessageToMap(tokenData)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Member invited", result)
}

// sendProjectInvite enqueues the project invitation email. Errors are logged
// because the invite itself has already been created.
func (h *Handler) sendProjectInvite(ctx context.Context, email, token string) {
	link := fmt.Sprintf("%s/invite/project/%s", internal.GetString("APP_DSN", "http://localhost:5173"), token)

	rClient := notificationrpc.NewNotificationHandlersClient(h.Grpc)
	_, err := rClient.SendMail(ctx, &notificationmessage.SendMail_Request{
		Email:    email,
		Subject:  "project invitation",
		Template: notificationenum.MailTemplate_project_invite,
		Data: map[string]string{
			"Link": link,
		},
	})
	if err != nil {
		h.log.Error(err).Send()
	}
}
