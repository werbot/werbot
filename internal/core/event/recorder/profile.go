package recorder

import (
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
)

// ProfileSection represents a section within a profile.
type ProfileSection eventmessage.Profile_Section

// Constants for different profile sections.
const (
	ProfileUnspecified ProfileSection = ProfileSection(eventmessage.Profile_section_unspecified)
	ProfileProfile     ProfileSection = ProfileSection(eventmessage.Profile_profile)
	ProfileSetting     ProfileSection = ProfileSection(eventmessage.Profile_setting)
	ProfilePassword    ProfileSection = ProfileSection(eventmessage.Profile_password)
	ProfileSSHKey      ProfileSection = ProfileSection(eventmessage.Profile_ssh_key)
	ProfileLicense     ProfileSection = ProfileSection(eventmessage.Profile_license)
	ProfileProject     ProfileSection = ProfileSection(eventmessage.Profile_project)
)

// Profile logs an event for a specific profile section.
func (e *WebEvent) Profile(userID string, section ProfileSection, event Type, metaData ...any) {
	if envMode != "test" {
		e.addEvent(userID, section, event, metaData...)
	}
}
