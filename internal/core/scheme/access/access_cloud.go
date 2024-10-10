package access

import (
	"google.golang.org/protobuf/proto"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
)

func NewCloudAWS() proto.Message { return &schemeaccesspb.AccessScheme_Cloud_AWS{} }
func SetCloudAWS(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_CloudAws{CloudAws: msg.(*schemeaccesspb.AccessScheme_Cloud_AWS)}
}

func NewCloudGCP() proto.Message { return &schemeaccesspb.AccessScheme_Cloud_GCP{} }
func SetCloudGCP(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_CloudGcp{CloudGcp: msg.(*schemeaccesspb.AccessScheme_Cloud_GCP)}
}

func NewCloudAzure() proto.Message { return &schemeaccesspb.AccessScheme_Cloud_Azure{} }
func SetCloudAzure(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_CloudAzure{CloudAzure: msg.(*schemeaccesspb.AccessScheme_Cloud_Azure)}
}

func NewCloudDO() proto.Message { return &schemeaccesspb.AccessScheme_Cloud_DO{} }
func SetCloudDO(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_CloudDo{CloudDo: msg.(*schemeaccesspb.AccessScheme_Cloud_DO)}
}

func NewCloudHetzner() proto.Message { return &schemeaccesspb.AccessScheme_Cloud_Hetzner{} }
func SetCloudHetzner(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_CloudHetzner{CloudHetzner: msg.(*schemeaccesspb.AccessScheme_Cloud_Hetzner)}
}

func (s *handler) handleCloudAWS(in *schemeaccesspb.AccessScheme_CloudAws) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_cloud_aws
	newAccess := in.CloudAws
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Cloud AWS #%s",
	}, nil
}

func (s *handler) handleCloudGCP(in *schemeaccesspb.AccessScheme_CloudGcp) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_cloud_gcp
	newAccess := in.CloudGcp
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Cloud GCP #%s",
	}, nil
}

func (s *handler) handleCloudAzure(in *schemeaccesspb.AccessScheme_CloudAzure) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_cloud_azure
	newAccess := in.CloudAzure
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Cloud Azure #%s",
	}, nil
}

func (s *handler) handleCloudDO(in *schemeaccesspb.AccessScheme_CloudDo) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_cloud_do
	newAccess := in.CloudDo
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Cloud Digital Ocean #%s",
	}, nil
}

func (s *handler) handleCloudHetzner(in *schemeaccesspb.AccessScheme_CloudHetzner) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_cloud_hetzner
	newAccess := in.CloudHetzner
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Cloud Hetzner #%s",
	}, nil
}
