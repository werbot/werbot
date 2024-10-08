package access

import (
	"google.golang.org/protobuf/encoding/protojson"

	schemeaccesspb "github.com/werbot/werbot/internal/grpc/scheme/proto/access"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

type accessScheme struct {
	*schemeaccesspb.AccessScheme
}

// Unmarshal is ...
func Unmarshal(data []byte, schemeType schemeaccesspb.SchemeType) (*accessScheme, error) {
	typeToScheme := TypeToScheme(schemeType)
	instance := typeToScheme.NewInstance()
	if err := protojson.Unmarshal(data, instance); err != nil {
		return nil, err
	}

	access := &accessScheme{
		AccessScheme: &schemeaccesspb.AccessScheme{},
	}
	typeToScheme.SetAccess(access.AccessScheme, instance)

	return access, nil
}

// Ghoster is ...
func (a *accessScheme) Ghoster() {
	ghoster.Secrets(a.AccessScheme, false)
}
