package access

import (
	"time"

	"github.com/google/uuid"
)

// ApprovalKind tells what an approval lets through.
type ApprovalKind string

// Approval kinds.
const (
	KindAccess  ApprovalKind = "access"
	KindCommand ApprovalKind = "command"
)

// Approval statuses.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"
	StatusExpired  = "expired"
	StatusCanceled = "canceled"
)

// Approval policy modes.
const (
	ModeManual = "manual"
	ModeAuto   = "auto"
)

// Selector picks resources by id or by labels: exactly one of the two.
type Selector struct {
	ResourceIDs []uuid.UUID       `json:"resource_ids,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// Subjects names the users and teams a rule applies to, or all users or all agents.
type Subjects struct {
	Users     []uuid.UUID `json:"users,omitempty"`
	Teams     []uuid.UUID `json:"teams,omitempty"`
	AllUsers  bool        `json:"all_users,omitempty"`
	AllAgents bool        `json:"all_agents,omitempty"`
}

// ApprovalPolicy says who may request access to which resources, who approves, and for how long.
type ApprovalPolicy struct {
	ID, OrgID                           uuid.UUID
	Name                                string
	Selector                            Selector
	Requesters, Approvers               Subjects // approvers: users and teams only; empty: owner and admin
	Mode                                string   // ModeManual | ModeAuto
	MaxDuration                         time.Duration
	RequireReason, FourEyes, BreakGlass bool
	CreatedBy                           *uuid.UUID
	CreatedAt                           time.Time
}

// Approval is an access request or a command approval.
type Approval struct {
	ID, OrgID, RequesterID                                  uuid.UUID
	Kind                                                    ApprovalKind
	AgentRunID, ResourceID, PolicyID, DecidedBy, InstanceID *uuid.UUID
	Selector                                                map[string]string
	Duration                                                time.Duration
	Command                                                 string
	CommandSHA256                                           []byte
	Classes                                                 []string
	Reason, Status, Comment                                 string
	BreakGlass                                              bool
	ExpiresAt, CreatedAt                                    time.Time
	DecidedAt                                               *time.Time
}

// CreateRequestInput asks for access to one resource or to the resources matching Selector.
type CreateRequestInput struct {
	ResourceID *uuid.UUID
	Selector   map[string]string
	Duration   time.Duration
	Reason     string
	BreakGlass bool
}

// CommandApprovalInput asks for approval of one command of a session.
type CommandApprovalInput struct {
	OrgID, UserID, ResourceID, InstanceID uuid.UUID
	AgentRunID                            *uuid.UUID
	Command                               string
	Classes, Reasons                      []string
}

// Grant is time-limited access of a user to one resource.
type Grant struct {
	ID, OrgID, UserID, ResourceID uuid.UUID
	ApprovalID                    *uuid.UUID
	ExpiresAt                     time.Time
	BreakGlass, NeedsReview       bool
}

// Lock denies access to a target until ExpiresAt (nil: until deleted).
type Lock struct {
	ID, OrgID  uuid.UUID
	TargetType string
	TargetID   *uuid.UUID
	Reason     string
	ExpiresAt  *time.Time
}
