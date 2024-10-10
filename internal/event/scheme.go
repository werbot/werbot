package event

import (
	eventpb "github.com/werbot/werbot/internal/core/event/proto/event"
)

// SchemeSection represents a section within a scheme.
type SchemeSection eventpb.Scheme_Section

// Constants for different scheme sections.
const (
	SchemeUnspecified SchemeSection = SchemeSection(eventpb.Scheme_section_unspecified)
	SchemeScheme      SchemeSection = SchemeSection(eventpb.Scheme_scheme)
	SchemeMember      SchemeSection = SchemeSection(eventpb.Scheme_member)
	SchemeActivity    SchemeSection = SchemeSection(eventpb.Scheme_activity)
	SchemeFirewall    SchemeSection = SchemeSection(eventpb.Scheme_firewall)
	SchemeSetting     SchemeSection = SchemeSection(eventpb.Scheme_setting)
)

// Scheme logs an event for a specific scheme section.
func (e *WebEvent) Scheme(userID string, section SchemeSection, event EventType, metaData ...any) {
	if envMode != "test" {
		e.addEvent(userID, section, event, metaData...)
	}
}
