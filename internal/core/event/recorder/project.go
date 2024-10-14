package recorder

import (
	eventmessage "github.com/werbot/werbot/internal/core/event/proto/message"
)

// ProjectSection represents a section within a project.
type ProjectSection eventmessage.Project_Section

// Constants for different project sections.
const (
	ProjectUnspecified ProjectSection = ProjectSection(eventmessage.Project_section_unspecified)
	ProjectProject     ProjectSection = ProjectSection(eventmessage.Project_project)
	ProjectSetting     ProjectSection = ProjectSection(eventmessage.Project_setting)
	ProjectMember      ProjectSection = ProjectSection(eventmessage.Project_member)
	ProjectTeam        ProjectSection = ProjectSection(eventmessage.Project_team)
	ProjectServer      ProjectSection = ProjectSection(eventmessage.Project_server)
	ProjectDatabase    ProjectSection = ProjectSection(eventmessage.Project_database)
	ProjectApplication ProjectSection = ProjectSection(eventmessage.Project_application)
	ProjectDesktop     ProjectSection = ProjectSection(eventmessage.Project_desktop)
	ProjectContainer   ProjectSection = ProjectSection(eventmessage.Project_container)
	ProjectCloud       ProjectSection = ProjectSection(eventmessage.Project_cloud)
)

// Project logs an event for a specific project section.
func (e *WebEvent) Project(userID string, section ProjectSection, event Type, metaData ...any) {
	if envMode != "test" {
		e.addEvent(userID, section, event, metaData...)
	}
}
