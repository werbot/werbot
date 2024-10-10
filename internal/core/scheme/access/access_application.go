package access

import (
	"google.golang.org/protobuf/proto"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
)

func NewApplicationSite() proto.Message { return &schemeaccesspb.AccessScheme_Application_Site{} }
func SetApplicationSite(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ApplicationSite{ApplicationSite: msg.(*schemeaccesspb.AccessScheme_Application_Site)}
}

func (s *handler) handleApplicationSite(in *schemeaccesspb.AccessScheme_ApplicationSite) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_application_site
	newAccess := in.ApplicationSite
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Application site #%s",
	}, nil
}
