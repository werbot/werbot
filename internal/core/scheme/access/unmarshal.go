package access

import (
	"encoding/json"
	"strconv"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	"github.com/werbot/werbot/pkg/utils/protoutils/ghoster"
)

// normalizeNumericBools rewrites legacy numeric booleans (0/1) into JSON
// booleans, which protojson accepts. Older scheme records stored bool fields
// such as tls_required as numbers; without this, reading those schemes fails.
func normalizeNumericBools(data []byte, msg proto.Message) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil || fields == nil {
		return data, nil
	}

	descriptor := msg.ProtoReflect().Descriptor()
	changed := false

	for key, raw := range fields {
		field := descriptor.Fields().ByJSONName(key)
		if field == nil {
			field = descriptor.Fields().ByName(protoreflect.Name(key))
		}
		if field == nil || field.Kind() != protoreflect.BoolKind {
			continue
		}

		var value int
		if err := json.Unmarshal(raw, &value); err != nil || (value != 0 && value != 1) {
			continue
		}

		fields[key] = json.RawMessage(strconv.FormatBool(value == 1))
		changed = true
	}

	if !changed {
		return data, nil
	}

	return json.Marshal(fields)
}

// DecodedScheme wraps a decoded scheme access payload with helpers.
type DecodedScheme struct {
	*schemeaccesspb.AccessScheme
}

// Unmarshal is ...
func Unmarshal(data []byte, schemeType schemeaccesspb.SchemeType) (*DecodedScheme, error) {
	typeToScheme := TypeToScheme(schemeType)
	instance := typeToScheme.NewInstance()

	normalized, err := normalizeNumericBools(data, instance)
	if err != nil {
		return nil, err
	}

	if err := protojson.Unmarshal(normalized, instance); err != nil {
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
