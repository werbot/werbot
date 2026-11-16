// Package inventory owns resources: connector-typed configuration, labels and
// the pinned SSH host key. Credentials, access and policy live in other domains.
package inventory

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/inventory/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
)

const maxLabels = 32

// The label rules are repeated in internal/access: domains never import each other.
var (
	nameRe       = regexp.MustCompile(`^[a-z0-9-]{1,63}$`)
	labelKeyRe   = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]{0,61}[a-z0-9])?$`)
	labelValueRe = regexp.MustCompile(`^[A-Za-z0-9._-]{0,63}$`)
)

// HostKey is a pinned SSH host key.
type HostKey struct {
	KeyType, PublicKey, FingerprintSHA256 string
	CreatedAt                             time.Time
}

// Resource is a protected target of a registered connector type.
type Resource struct {
	ID, OrgID uuid.UUID
	Name      string
	Type      string
	Category  connector.Category
	GatewayID *uuid.UUID
	Config    proto.Message // nil when the type is no longer registered
	Labels    map[string]string
	HostKey   *HostKey // nil: no key pinned
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateInput describes a new resource.
type CreateInput struct {
	Name, Type string
	GatewayID  *uuid.UUID
	Config     proto.Message
	Labels     map[string]string
}

// UpdateInput replaces name, gateway and labels; the type never changes and a nil Config keeps the old one.
type UpdateInput struct {
	ID        uuid.UUID
	Name      string
	GatewayID *uuid.UUID
	Config    proto.Message
	Labels    map[string]string
}

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", apperr.ErrInvalidArgument, fmt.Sprintf(format, a...))
}

func validateName(name string) error {
	if !nameRe.MatchString(name) {
		return invalid("resource name must match %s", nameRe)
	}
	return nil
}

func validateLabels(labels map[string]string) error {
	if len(labels) > maxLabels {
		return invalid("at most %d labels", maxLabels)
	}
	for k, v := range labels {
		if !labelKeyRe.MatchString(k) {
			return invalid("label key %q must match %s", k, labelKeyRe)
		}
		if !labelValueRe.MatchString(v) {
			return invalid("label value of %q must match %s", k, labelValueRe)
		}
	}
	return nil
}

// validateConfig checks cfg against the type (same message and Validate) and returns its JSON.
func validateConfig(t connector.Type, cfg proto.Message) ([]byte, error) {
	if cfg == nil || cfg.ProtoReflect().Descriptor().FullName() != t.NewConfig().ProtoReflect().Descriptor().FullName() {
		return nil, invalid("config does not match type %q", t.Name)
	}
	if err := t.Validate(cfg); err != nil {
		return nil, invalid("%v", err)
	}
	return protojson.Marshal(cfg)
}

func toResource(r dbq.GetResourceRow) (Resource, error) {
	res := Resource{
		ID: r.ID, OrgID: r.OrgID, Name: r.Name, Type: r.Type, Category: connector.Category(r.Category),
		GatewayID: r.GatewayID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	if err := json.Unmarshal(r.Labels, &res.Labels); err != nil {
		return Resource{}, fmt.Errorf("inventory: decode labels: %w", err)
	}
	if t, ok := connector.Lookup(r.Type); ok {
		res.Config = t.NewConfig()
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(r.Config, res.Config); err != nil {
			return Resource{}, fmt.Errorf("inventory: decode config of %s: %w", r.ID, err)
		}
	}
	if r.FingerprintSha256 != nil {
		res.HostKey = &HostKey{KeyType: *r.KeyType, PublicKey: *r.PublicKey, FingerprintSHA256: *r.FingerprintSha256, CreatedAt: *r.HostKeyCreatedAt}
	}
	return res, nil
}
