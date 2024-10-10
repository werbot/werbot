package access

import (
	"google.golang.org/protobuf/proto"

	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
)

func NewContainerDocker() proto.Message { return &schemeaccesspb.AccessScheme_Container_Docker{} }
func SetContainerDocker(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ContainerDocker{ContainerDocker: msg.(*schemeaccesspb.AccessScheme_Container_Docker)}
}

func NewContainerK8S() proto.Message { return &schemeaccesspb.AccessScheme_Container_K8S{} }
func SetContainerK8S(core *schemeaccesspb.AccessScheme, msg proto.Message) {
	core.Access = &schemeaccesspb.AccessScheme_ContainerK8S{ContainerK8S: msg.(*schemeaccesspb.AccessScheme_Container_K8S)}
}

func (s *handler) handleContainerDocker(in *schemeaccesspb.AccessScheme_ContainerDocker) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_container_docker
	newAccess := in.ContainerDocker
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Container Docker #%s",
	}, nil
}

func (s *handler) handleContainerK8S(in *schemeaccesspb.AccessScheme_ContainerK8S) (*SchemeDescription, error) {
	schemeType := schemeaccesspb.SchemeType_container_k8s
	newAccess := in.ContainerK8S
	return &SchemeDescription{
		SchemeType:   schemeType,
		Access:       newAccess,
		TitlePattern: "Container Kubernetes #%s",
	}, nil
}
