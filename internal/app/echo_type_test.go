package app

import (
	"errors"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/internal/connector"
)

// echo is a second resource type registered only in tests (contract §3.2; on
// the gateway connectortest.Echo serves it, stage 5). This is the only
// registration of "echo" in package app: stage 5 reuses it and a second
// Register panics.
func init() {
	connector.Register(connector.Type{Name: "echo", Category: connector.CategoryApplications, Level: "L1",
		NewConfig: func() proto.Message { return &wrapperspb.StringValue{} },
		Validate: func(m proto.Message) error {
			if m.(*wrapperspb.StringValue).GetValue() == "" {
				return errors.New("value required")
			}
			return nil
		},
		Credentials: []connector.CredentialKind{connector.CredentialPassword}, Actions: []string{"echo.connect"},
		Kinds:    []connector.Kind{connector.KindStream},
		Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{AnyTarget: true, NoHealth: true} }})
}
