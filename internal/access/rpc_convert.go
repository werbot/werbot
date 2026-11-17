package access

import (
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid id: %w", apperr.ErrInvalidArgument)
	}
	return id, nil
}

func bindingToProto(b Binding) *werbotv1.Binding {
	out := &werbotv1.Binding{Id: b.ID.String(), SubjectType: b.SubjectType, SubjectId: b.SubjectID.String(),
		LabelSelector: b.LabelSelector, CreatedBy: b.CreatedBy.String(), CreatedAt: timestamppb.New(b.CreatedAt)}
	if b.ResourceID != nil {
		out.ResourceId = b.ResourceID.String()
	}
	for _, r := range b.Ports {
		out.Ports = append(out.Ports, &connectorv1.PortRange{Port: uint32(r.Port), EndPort: uint32(r.EndPort)})
	}
	return out
}
