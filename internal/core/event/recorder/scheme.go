package recorder

import (
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
)

// SchemeSection represents a section within a scheme.
type SchemeSection eventmessage.Scheme_Section

// Constants for different scheme sections.
const (
	SchemeUnspecified SchemeSection = SchemeSection(eventmessage.Scheme_section_unspecified)
	SchemeScheme      SchemeSection = SchemeSection(eventmessage.Scheme_scheme)
	SchemeMember      SchemeSection = SchemeSection(eventmessage.Scheme_member)
	SchemeActivity    SchemeSection = SchemeSection(eventmessage.Scheme_activity)
	SchemeFirewall    SchemeSection = SchemeSection(eventmessage.Scheme_firewall)
	SchemeSetting     SchemeSection = SchemeSection(eventmessage.Scheme_setting)
)

// Scheme logs an event for a specific scheme section.
func (e *WebEvent) Scheme(userID string, section SchemeSection, event Type, metaData ...any) {
	if envMode != "test" {
		e.addEvent(userID, section, event, metaData...)
	}
}
