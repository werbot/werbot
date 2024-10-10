package event

import (
	eventpb "github.com/werbot/werbot/internal/core/event/proto/event"
)

// ProfileSection represents a section within a profile.
type ProfileSection eventpb.Profile_Section

// Constants for different profile sections.
const (
	ProfileUnspecified ProfileSection = ProfileSection(eventpb.Profile_section_unspecified)
	ProfileProfile     ProfileSection = ProfileSection(eventpb.Profile_profile)
	ProfileSetting     ProfileSection = ProfileSection(eventpb.Profile_setting)
	ProfilePassword    ProfileSection = ProfileSection(eventpb.Profile_password)
	ProfileSSHKey      ProfileSection = ProfileSection(eventpb.Profile_ssh_key)
	ProfileLicense     ProfileSection = ProfileSection(eventpb.Profile_license)
	ProfileProject     ProfileSection = ProfileSection(eventpb.Profile_project)
)

// Profile logs an event for a specific profile section.
func (e *WebEvent) Profile(userID string, section ProfileSection, event EventType, metaData ...any) {
	if envMode != "test" {
		e.addEvent(userID, section, event, metaData...)
	}
}
