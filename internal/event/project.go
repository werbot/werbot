package event

import (
	eventpb "github.com/werbot/werbot/internal/core/event/proto/event"
)

// ProjectSection represents a section within a project.
type ProjectSection eventpb.Project_Section

// Constants for different project sections.
const (
	ProjectUnspecified ProjectSection = ProjectSection(eventpb.Project_section_unspecified)
	ProjectProject     ProjectSection = ProjectSection(eventpb.Project_project)
	ProjectSetting     ProjectSection = ProjectSection(eventpb.Project_setting)
	ProjectMember      ProjectSection = ProjectSection(eventpb.Project_member)
	ProjectTeam        ProjectSection = ProjectSection(eventpb.Project_team)
	ProjectServer      ProjectSection = ProjectSection(eventpb.Project_server)
	ProjectDatabase    ProjectSection = ProjectSection(eventpb.Project_database)
	ProjectApplication ProjectSection = ProjectSection(eventpb.Project_application)
	ProjectDesktop     ProjectSection = ProjectSection(eventpb.Project_desktop)
	ProjectContainer   ProjectSection = ProjectSection(eventpb.Project_container)
	ProjectCloud       ProjectSection = ProjectSection(eventpb.Project_cloud)
)

// Project logs an event for a specific project section.
func (e *WebEvent) Project(userID string, section ProjectSection, event EventType, metaData ...any) {
	if envMode != "test" {
		e.addEvent(userID, section, event, metaData...)
	}
}
