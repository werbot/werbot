package event

import (
	"context"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"

	"github.com/werbot/werbot/internal"
	eventpb "github.com/werbot/werbot/internal/core/event/proto/event"
	"github.com/werbot/werbot/internal/web/session"
	"github.com/werbot/werbot/pkg/logger"
)

// Metadata is ...
type Metadata map[string]any

// EventType represents different types of events.
type EventType eventpb.EventType

// Constants for different event types.
const (
	Unspecified EventType = EventType(eventpb.EventType_event_unspecified)
	OnOnline    EventType = EventType(eventpb.EventType_onOnline)
	OnOffline   EventType = EventType(eventpb.EventType_onOffline)
	OnCreate    EventType = EventType(eventpb.EventType_onCreate)
	OnEdit      EventType = EventType(eventpb.EventType_onEdit)
	OnRemove    EventType = EventType(eventpb.EventType_onRemove)
	OnActive    EventType = EventType(eventpb.EventType_onActive)
	OnInactive  EventType = EventType(eventpb.EventType_onInactive)
	OnChange    EventType = EventType(eventpb.EventType_onChange)
	OnLogin     EventType = EventType(eventpb.EventType_onLogin)
	OnLogoff    EventType = EventType(eventpb.EventType_onLogoff)
	OnReset     EventType = EventType(eventpb.EventType_onReset)
	OnUpdate    EventType = EventType(eventpb.EventType_onUpdate)
	OnRequest   EventType = EventType(eventpb.EventType_onRequest)
	OnMessage   EventType = EventType(eventpb.EventType_onMessage)
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
			ProfileId: session.ProfileID(c.Query("profile_id")),
			SessionId: session.SessionId(),
			UserAgent: string(c.Request().Header.UserAgent()),
			Ip:        c.IP(),
		},
	}
}

// addEvent adds an event to the system with the provided user ID, section, event type, and metadata.
func (e *WebEvent) addEvent(sectionID string, section interface{}, event EventType, metaData ...any) {
	switch s := section.(type) {
	case ProfileSection:
		e.request.Section = &eventpb.AddEvent_Request_Profile{
			Profile: &eventpb.Profile{
				Id:      sectionID,
				Section: eventpb.Profile_Section(s),
			},
		}
	case ProjectSection:
		e.request.Section = &eventpb.AddEvent_Request_Project{
			Project: &eventpb.Project{
				Id:      sectionID,
				Section: eventpb.Project_Section(s),
			},
		}
	case SchemeSection:
		e.request.Section = &eventpb.AddEvent_Request_Scheme{
			Scheme: &eventpb.Scheme{
				Id:      sectionID,
				Section: eventpb.Scheme_Section(s),
			},
		}
	default:
		log.Error(nil).Msg("Invalid section type")
		return
	}

	e.request.Event = eventpb.EventType(event)

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
