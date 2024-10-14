package recorder

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"

	"github.com/werbot/werbot/internal"
	eventenum "github.com/werbot/werbot/internal/core/event/proto/enum"
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
	eventpb "github.com/werbot/werbot/internal/core/event/proto/rpc"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/logger"
)

// Metadata is ...
type Metadata map[string]any

// Type represents different types of events.
type Type eventenum.Type

// Constants for different event types.
const (
	Unspecified Type = Type(eventenum.Type_event_unspecified)
	OnOnline    Type = Type(eventenum.Type_onOnline)
	OnOffline   Type = Type(eventenum.Type_onOffline)
	OnCreate    Type = Type(eventenum.Type_onCreate)
	OnEdit      Type = Type(eventenum.Type_onEdit)
	OnRemove    Type = Type(eventenum.Type_onRemove)
	OnActive    Type = Type(eventenum.Type_onActive)
	OnInactive  Type = Type(eventenum.Type_onInactive)
	OnChange    Type = Type(eventenum.Type_onChange)
	OnLogin     Type = Type(eventenum.Type_onLogin)
	OnLogoff    Type = Type(eventenum.Type_onLogoff)
	OnReset     Type = Type(eventenum.Type_onReset)
	OnUpdate    Type = Type(eventenum.Type_onUpdate)
	OnRequest   Type = Type(eventenum.Type_onRequest)
	OnMessage   Type = Type(eventenum.Type_onMessage)
)

// Event holds the gRPC client for event handlers.
type Event struct {
	client eventpb.EventHandlersClient
}

// WebEvent holds the gRPC client and request for web events.
type WebEvent struct {
	client  eventpb.EventHandlersClient
	request *eventpb.AddEvent_Request
}

var (
	log     logger.Logger
	envMode string
)

// New creates a new Event instance with the given gRPC client connection.
func New(grpcConn *grpc.ClientConn) *Event {
	log = logger.New()
	envMode = internal.GetString("ENV_MODE", "prod")

	return &Event{
		client: eventpb.NewEventHandlersClient(grpcConn),
	}
}

// Web initializes a WebEvent with user agent and IP information from the fiber context.
func (e *Event) Web(c *fiber.Ctx, session *session.ProfileParameters) *WebEvent {
	return &WebEvent{
		client: e.client,
		request: &eventpb.AddEvent_Request{
			OwnerId: session.ProfileID(c.Query("owner_id")),
			Session: &eventmessage.Session{
				Id:        session.SessionId(),
				UserAgent: string(c.Request().Header.UserAgent()),
				Ip:        c.IP(),
			},
		},
	}
}

// addEvent adds an event to the system with the provided user ID, section, event type, and metadata.
func (e *WebEvent) addEvent(sectionID string, section interface{}, event Type, metaData ...any) {
	switch s := section.(type) {
	case ProfileSection:
		e.request.Section = &eventpb.AddEvent_Request_Profile{
			Profile: &eventmessage.Profile{
				Id:      sectionID,
				Section: eventmessage.Profile_Section(s),
			},
		}
	case ProjectSection:
		e.request.Section = &eventpb.AddEvent_Request_Project{
			Project: &eventmessage.Project{
				Id:      sectionID,
				Section: eventmessage.Project_Section(s),
			},
		}
	case SchemeSection:
		e.request.Section = &eventpb.AddEvent_Request_Scheme{
			Scheme: &eventmessage.Scheme{
				Id:      sectionID,
				Section: eventmessage.Scheme_Section(s),
			},
		}
	default:
		log.Error(nil).Msg("Invalid section type")
		return
	}

	e.request.Type = eventenum.Type(event)

	if len(metaData) > 0 {
		data, err := json.Marshal(metaData)
		if err != nil {
			log.Error(err).Msg("Failed to marshal metaData")
		} else {
			e.request.MetaData = data
		}
	}

	if _, err := e.client.AddEvent(context.Background(), e.request); err != nil {
		log.Error(err).Send()
	}
}
