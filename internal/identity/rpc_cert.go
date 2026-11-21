package identity

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// IssueUserCert signs the caller's ed25519 key with the organization's user CA.
func (h *Handler) IssueUserCert(ctx context.Context, req *connect.Request[werbotv1.IssueUserCertRequest]) (*connect.Response[werbotv1.IssueUserCertResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.Msg.GetPublicKey()))
	if err != nil { // the parser's text may quote the input
		return nil, apperr.ToConnect(fmt.Errorf("invalid public key: %w", apperr.ErrInvalidArgument))
	}
	cert, err := h.svc.IssueUserCert(ctx, p, pub)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.IssueUserCertResponse{
		Certificate: authorizedKey(cert),
		ValidBefore: timestamppb.New(time.Unix(int64(cert.ValidBefore), 0)), //nolint:gosec // set by sshca.Sign from a unix time
	}), nil
}
