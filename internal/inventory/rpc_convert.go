package inventory

import (
	"fmt"
	"slices"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid id: %w", apperr.ErrInvalidArgument)
	}
	return id, nil
}

// parseGatewayID maps "" to nil.
func parseGatewayID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := parseID(s)
	return &id, err
}

// parseConfig decodes the config of type t from the Any or the JSON, whichever is set.
// It returns nil when neither is set; both set is invalid.
func parseConfig(t connector.Type, a *anypb.Any, js string) (proto.Message, error) {
	if a != nil && js != "" {
		return nil, invalid("set either config or config_json, not both")
	}
	cfg := t.NewConfig()
	switch {
	case a != nil:
		if err := a.UnmarshalTo(cfg); err != nil {
			return nil, invalid("config does not match type %q", t.Name)
		}
	case js != "":
		if err := protojson.Unmarshal([]byte(js), cfg); err != nil {
			return nil, invalid("config_json: %v", err)
		}
	default:
		return nil, nil
	}
	return cfg, nil
}

func typeToProto(t connector.Type) *werbotv1.ResourceType {
	out := &werbotv1.ResourceType{
		Name: t.Name, Category: string(t.Category), Actions: slices.Clone(t.Actions), Level: t.Level,
		ConfigTypeUrl: "type.googleapis.com/" + string(t.NewConfig().ProtoReflect().Descriptor().FullName()),
	}
	for _, k := range t.Credentials {
		out.CredentialKinds = append(out.CredentialKinds, string(k))
	}
	for _, k := range t.Kinds {
		out.Kinds = append(out.Kinds, string(k))
	}
	return out
}

// resourceToProto never carries a credential value, only its kind.
func resourceToProto(r Resource, kinds map[uuid.UUID]connector.CredentialKind) (*werbotv1.Resource, error) {
	out := &werbotv1.Resource{
		Id: r.ID.String(), Name: r.Name, Type: r.Type, Category: string(r.Category), Labels: r.Labels,
		CredentialKind: string(kinds[r.ID]),
		CreatedAt:      timestamppb.New(r.CreatedAt), UpdatedAt: timestamppb.New(r.UpdatedAt),
	}
	if r.GatewayID != nil {
		out.GatewayId = r.GatewayID.String()
	}
	if r.Config != nil {
		js, err := protojson.Marshal(r.Config)
		if err != nil {
			return nil, err
		}
		if out.Config, err = anypb.New(r.Config); err != nil {
			return nil, err
		}
		out.ConfigJson = string(js)
	}
	if h := r.HostKey; h != nil {
		out.HostKey = &werbotv1.HostKey{KeyType: h.KeyType, FingerprintSha256: h.FingerprintSHA256, CreatedAt: timestamppb.New(h.CreatedAt)}
	}
	return out, nil
}
