package access

import (
	"google.golang.org/protobuf/encoding/protojson"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// DecodedScheme wraps a decoded scheme access payload with helpers.
type DecodedScheme struct {
	*schemeaccesspb.AccessScheme
}

// Unmarshal is ...
func Unmarshal(data []byte, schemeType schemeaccesspb.SchemeType) (*DecodedScheme, error) {
	typeToScheme := TypeToScheme(schemeType)
	instance := typeToScheme.NewInstance()
	if err := protojson.Unmarshal(data, instance); err != nil {
		return nil, err
	}

	access := &DecodedScheme{
		AccessScheme: &schemeaccesspb.AccessScheme{},
	}
	typeToScheme.SetAccess(access.AccessScheme, instance)

	return access, nil
}

// Ghoster is ...
func (a *DecodedScheme) Ghoster() {
	ghoster.Secrets(a.AccessScheme, false)
}
