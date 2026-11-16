package identity

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func sshKeyToProto(k SSHKey) *werbotv1.SSHKey {
	return &werbotv1.SSHKey{
		Id: k.ID.String(), Name: k.Name, PublicKey: k.PublicKey,
		FingerprintSha256: k.FingerprintSHA256, CreatedAt: timestamppb.New(k.CreatedAt),
	}
}

func parseKeyID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid id: %w", apperr.ErrInvalidArgument)
	}
	return id, nil
}

// ListSSHKeys returns the caller's SSH keys.
func (h *Handler) ListSSHKeys(ctx context.Context, _ *connect.Request[werbotv1.ListSSHKeysRequest]) (*connect.Response[werbotv1.ListSSHKeysResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	keys, err := h.svc.ListSSHKeys(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListSSHKeysResponse{}
	for _, k := range keys {
		res.Keys = append(res.Keys, sshKeyToProto(k))
	}
	return connect.NewResponse(res), nil
}

// AddSSHKey stores a public key for the caller.
func (h *Handler) AddSSHKey(ctx context.Context, req *connect.Request[werbotv1.AddSSHKeyRequest]) (*connect.Response[werbotv1.AddSSHKeyResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	k, err := h.svc.AddSSHKey(ctx, req.Msg.GetName(), req.Msg.GetPublicKey())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.AddSSHKeyResponse{Key: sshKeyToProto(k)}), nil
}

// UpdateSSHKey renames the caller's key.
func (h *Handler) UpdateSSHKey(ctx context.Context, req *connect.Request[werbotv1.UpdateSSHKeyRequest]) (*connect.Response[werbotv1.UpdateSSHKeyResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseKeyID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	k, err := h.svc.UpdateSSHKey(ctx, id, req.Msg.GetName())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateSSHKeyResponse{Key: sshKeyToProto(k)}), nil
}

// DeleteSSHKey removes the caller's key.
func (h *Handler) DeleteSSHKey(ctx context.Context, req *connect.Request[werbotv1.DeleteSSHKeyRequest]) (*connect.Response[werbotv1.DeleteSSHKeyResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseKeyID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.DeleteSSHKey(ctx, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteSSHKeyResponse{}), nil
}
