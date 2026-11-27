package agentic

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/rpcid"
)

// timestamp converts t; nil stays unset.
func timestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// limitsFromProto converts l; unset is all defaults.
func limitsFromProto(l *werbotv1.AgentLimits) Limits {
	return Limits{MaxActions: int(l.GetMaxActions()), MaxDuration: l.GetMaxDuration().AsDuration(),
		MaxConcurrent: int(l.GetMaxConcurrent()), MaxRows: int(l.GetMaxRows()), MaxResultBytes: l.GetMaxResultBytes(),
		StatementTimeout: l.GetStatementTimeout().AsDuration(), MaxQueries: int(l.GetMaxQueries())}
}

// limitsToProto converts l; Limits.Validate bounds every field to int32.
func limitsToProto(l Limits) *werbotv1.AgentLimits {
	return &werbotv1.AgentLimits{MaxActions: int32(l.MaxActions), MaxDuration: durationpb.New(l.MaxDuration), //nolint:gosec // Limits.Validate bounds them
		MaxConcurrent: int32(l.MaxConcurrent), MaxRows: int32(l.MaxRows), MaxResultBytes: l.MaxResultBytes, //nolint:gosec // Limits.Validate bounds them
		StatementTimeout: durationpb.New(l.StatementTimeout), MaxQueries: int32(l.MaxQueries)} //nolint:gosec // Limits.Validate bounds it
}

func agentToProto(v AgentView) *werbotv1.Agent {
	return &werbotv1.Agent{Id: v.ID.String(), Name: v.Name, SponsorUserId: v.SponsorID.String(), SponsorLogin: v.SponsorLogin,
		Kind: v.Kind, Status: v.Status, CreatedAt: timestamppb.New(v.CreatedAt), Limits: limitsToProto(v.Limits),
		ActiveRuns: int32(v.ActiveRuns)} //nolint:gosec // live runs of one agent
}

func runToProto(r RunView) *werbotv1.AgentRun {
	return &werbotv1.AgentRun{Id: r.ID.String(), AgentId: r.AgentID.String(), AgentName: r.AgentName,
		SponsorUserId: r.SponsorID.String(), Client: r.Client, Reason: r.Reason, Status: r.Status,
		Actions: int32(r.Actions), MaxActions: int32(r.MaxActions), DenialsInRow: int32(r.DenialsInRow), //nolint:gosec // int columns
		MaxConcurrent: int32(r.MaxConcurrent), StartedAt: timestamppb.New(r.StartedAt), //nolint:gosec // int column
		ExpiresAt: timestamppb.New(r.ExpiresAt), EndedAt: timestamp(r.EndedAt), EndReason: r.EndReason}
}

func timelineToProto(tl Timeline) (*werbotv1.GetRunResponse, error) {
	out := &werbotv1.GetRunResponse{Run: runToProto(tl.Run)}
	for _, c := range tl.ToolCalls {
		st := &structpb.Struct{}
		if err := protojson.Unmarshal(c.Arguments, st); err != nil {
			return nil, fmt.Errorf("agentic: arguments of tool call %s: %w", c.ID, err)
		}
		out.ToolCalls = append(out.ToolCalls, &werbotv1.ToolCall{Id: c.ID.String(), Tool: c.Tool, Arguments: st,
			Traceparent: c.Traceparent, Status: c.Status, StartedAt: timestamppb.New(c.StartedAt), EndedAt: timestamp(c.EndedAt)})
	}
	for _, s := range tl.Sessions {
		out.Sessions = append(out.Sessions, &werbotv1.RunSession{Id: s.ID.String(), ResourceName: s.ResourceName,
			ToolCallId: rpcid.String(s.ToolCallID), Command: s.Command, Classes: s.Classes, ExitCode: s.ExitCode,
			EndReason: s.EndReason, OutputSha256: s.OutputSha256, StartedAt: timestamppb.New(s.StartedAt), EndedAt: timestamp(s.EndedAt)})
	}
	for _, a := range tl.Approvals {
		out.Approvals = append(out.Approvals, &werbotv1.RunApproval{Id: a.ID.String(), ResourceName: a.ResourceName,
			Command: a.Command, Classes: a.Classes, Status: a.Status, DecidedByLogin: a.DecidedByLogin,
			CreatedAt: timestamppb.New(a.CreatedAt), DecidedAt: timestamp(a.DecidedAt)})
	}
	for _, d := range tl.Denials {
		out.Denials = append(out.Denials, &werbotv1.RunDenial{Resource: d.Resource, Command: d.Command, Classes: d.Classes,
			Reasons: d.Reasons, At: timestamppb.New(d.At)})
	}
	return out, nil
}
