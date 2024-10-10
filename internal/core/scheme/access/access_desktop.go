package access

import (
	"google.golang.org/protobuf/proto"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
)

func NewDesktopRDP() proto.Message { return &schemeaccesspb.AccessScheme_Desktop_RDP{} }
func SetDesktopRDP(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DesktopRdp{DesktopRdp: msg.(*schemeaccesspb.AccessScheme_Desktop_RDP)}
}

func NewDesktopVNC() proto.Message { return &schemeaccesspb.AccessScheme_Desktop_VNC{} }
func SetDesktopVNC(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_DesktopVnc{DesktopVnc: msg.(*schemeaccesspb.AccessScheme_Desktop_VNC)}
}

func (s *handler) handleDesktopRDP(in *schemeaccesspb.AccessScheme_DesktopRdp) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_desktop_rdp
	newAccess := in.DesktopRdp
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Desktop RDP #%s",
	}, nil
}

func (s *handler) handleDesktopVNC(in *schemeaccesspb.AccessScheme_DesktopVnc) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_desktop_vnc
	newAccess := in.DesktopVnc
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Desktop VNC #%s",
	}, nil
}
