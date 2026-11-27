package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

// ResourceCacheTTL is how long list_resources, ssh_exec and db_query reuse the resource list.
const ResourceCacheTTL = 60 * time.Second

// maxTraceparent is the longest traceparent RecordToolCall accepts.
const maxTraceparent = 55

// call is one recorded tool call of the run.
type call struct {
	run *clientauth.AgentProfile
	id  string
}

// toolFunc does the work of a tool; status is what FinishToolCall records.
type toolFunc[In, Out any] func(ctx context.Context, c call, in In) (out Out, status string, err error)

func (s *server) addTools() {
	addTool(s, &mcp.Tool{Name: "list_resources",
		Description: "List the resources this agent may use, with the command classes allowed and those that need an approval."}, s.listResources)
	addTool(s, &mcp.Tool{Name: "ssh_exec",
		Description: "Run one command on a resource over SSH, without a terminal. Output is capped at 64 KiB. " +
			"A command that needs an approval returns pending_approval: call again with the same resource and command once it is approved."}, s.sshExec)
	addTool(s, &mcp.Tool{Name: "db_query", Annotations: &mcp.ToolAnnotations{DestructiveHint: new(true)},
		Description: "Run one statement on a database resource and return its rows, each value as text or null. " +
			"A statement that needs an approval returns pending_approval: call again with the same resource and query once it is approved."}, s.dbQuery)
	addTool(s, &mcp.Tool{Name: "request_access",
		Description: "Ask for access to a resource for a time (Go duration such as 2h; empty: the organization default)."}, s.requestAccess)
	addTool(s, &mcp.Tool{Name: "run_status",
		Description: "Show this run: status, actions used and left, expiry and the approvals it waits for."}, s.runStatus)
}

// addTool registers h as t: a call starts the run if needed, is recorded with
// its masked arguments, runs h and is finished with h's status. A refused
// exchange or RecordToolCall is a tool error and runs nothing.
func addTool[In, Out any](s *server, t *mcp.Tool, h toolFunc[In, Out]) {
	mcp.AddTool(s.mcp, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var out Out
		run, err := s.agent(ctx, req)
		if err != nil {
			return nil, out, err
		}
		id, err := record(ctx, run, req.Params)
		if err != nil {
			return nil, out, err
		}
		out, status, err := h(ctx, call{run: run, id: id}, in)
		if err != nil {
			status = "error"
		}
		finish(ctx, run, id, status)
		return nil, out, err
	})
}

func agents(run *clientauth.AgentProfile) werbotv1connect.AgentServiceClient {
	return werbotv1connect.NewAgentServiceClient(run.HTTPClient(), run.Server)
}

func requests(run *clientauth.AgentProfile) werbotv1connect.AccessRequestServiceClient {
	return werbotv1connect.NewAccessRequestServiceClient(run.HTTPClient(), run.Server)
}

// record reports the call with its arguments masked by key and the
// traceparent from _meta.
func record(ctx context.Context, run *clientauth.AgentProfile, p *mcp.CallToolParamsRaw) (string, error) {
	var args map[string]any
	_ = json.Unmarshal(p.Arguments, &args) // the SDK checked them against the input schema
	st, err := structpb.NewStruct(maskArguments(args))
	if err != nil {
		return "", err
	}
	tp, _ := p.Meta["traceparent"].(string)
	if len(tp) > maxTraceparent {
		tp = ""
	}
	res, err := agents(run).RecordToolCall(ctx, connect.NewRequest(&werbotv1.RecordToolCallRequest{Tool: p.Name, Arguments: st, Traceparent: tp}))
	if err != nil {
		return "", err
	}
	return res.Msg.GetId(), nil
}

// finish reports the outcome of call id, even when the client went away.
func finish(ctx context.Context, run *clientauth.AgentProfile, id, status string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), endTimeout)
	defer cancel()
	if _, err := agents(run).FinishToolCall(ctx, connect.NewRequest(&werbotv1.FinishToolCallRequest{Id: id, Status: status})); err != nil {
		slog.Warn("mcp: finish a tool call", "tool_call_id", id, "error", err)
	}
}

// resourceCache holds the run's resource list for ResourceCacheTTL.
type resourceCache struct {
	mu   sync.Mutex
	at   time.Time
	list []*werbotv1.AccessibleResource
}

// resources lists the run's resources; fresh skips the cache.
func (s *server) resources(ctx context.Context, run *clientauth.AgentProfile, fresh bool) ([]*werbotv1.AccessibleResource, error) {
	c := &s.cache
	c.mu.Lock()
	defer c.mu.Unlock()
	if !fresh && !c.at.IsZero() && time.Since(c.at) < ResourceCacheTTL {
		return c.list, nil
	}
	res, err := werbotv1connect.NewResourceServiceClient(run.HTTPClient(), run.Server).
		ListAccessibleResources(ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	if err != nil {
		return nil, err
	}
	c.list, c.at = res.Msg.GetResources(), time.Now()
	return c.list, nil
}

// resource finds name in the cache, asking again on a miss; nil: unknown.
func (s *server) resource(ctx context.Context, run *clientauth.AgentProfile, name string) (*werbotv1.AccessibleResource, error) {
	for _, fresh := range []bool{false, true} {
		rs, err := s.resources(ctx, run, fresh)
		if err != nil {
			return nil, err
		}
		if i := slices.IndexFunc(rs, func(r *werbotv1.AccessibleResource) bool { return r.GetName() == name }); i >= 0 {
			return rs[i], nil
		}
	}
	return nil, nil
}

type noInput struct{}

type resourceInfo struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Kinds           []string          `json:"kinds,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	AllowedClasses  []string          `json:"allowed_classes,omitempty"`
	ApprovalClasses []string          `json:"approval_classes,omitempty"`
	Requestable     bool              `json:"requestable"`
}

type resourcesOutput struct {
	Resources []resourceInfo `json:"resources"`
}

func (s *server) listResources(ctx context.Context, c call, _ noInput) (resourcesOutput, string, error) {
	rs, err := s.resources(ctx, c.run, false)
	if err != nil {
		return resourcesOutput{}, "", err
	}
	out := resourcesOutput{Resources: []resourceInfo{}}
	for _, r := range rs {
		out.Resources = append(out.Resources, resourceInfo{Name: r.GetName(), Type: r.GetType(), Kinds: r.GetKinds(), Labels: r.GetLabels(),
			AllowedClasses: r.GetAllowedClasses(), ApprovalClasses: r.GetApprovalClasses(), Requestable: r.GetRequestable()})
	}
	return out, "ok", nil
}

type requestInput struct {
	Resource string `json:"resource" jsonschema:"the resource name"`
	Duration string `json:"duration,omitempty" jsonschema:"how long the access lasts, a Go duration such as 2h; empty: the organization default"`
	Reason   string `json:"reason" jsonschema:"why the access is needed"`
}

type requestOutput struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	URL       string `json:"url"`
}

func (s *server) requestAccess(ctx context.Context, c call, in requestInput) (requestOutput, string, error) {
	req := &werbotv1.CreateRequestRequest{Reason: in.Reason}
	if in.Duration != "" {
		d, err := time.ParseDuration(in.Duration)
		if err != nil {
			return requestOutput{}, "", err
		}
		req.Duration = durationpb.New(d)
	}
	r, err := s.resource(ctx, c.run, in.Resource)
	if err != nil {
		return requestOutput{}, "", err
	}
	if r == nil || !r.GetRequestable() {
		return requestOutput{}, "", errors.New("resource not found or not requestable")
	}
	req.ResourceId = r.GetId()
	res, err := requests(c.run).CreateRequest(ctx, connect.NewRequest(req))
	if err != nil {
		return requestOutput{}, "", err
	}
	a := res.Msg.GetRequest()
	return requestOutput{RequestID: a.GetId(), Status: a.GetStatus(), URL: a.GetUrl()}, "ok", nil
}

type pendingApproval struct {
	ID       string `json:"id"`
	Resource string `json:"resource"`
	Command  string `json:"command,omitempty"`
	URL      string `json:"url"`
}

type statusOutput struct {
	RunID            string            `json:"run_id"`
	Status           string            `json:"status"`
	Actions          int32             `json:"actions"`
	MaxActions       int32             `json:"max_actions"`
	RemainingActions int32             `json:"remaining_actions"`
	ExpiresAt        string            `json:"expires_at"`
	PendingApprovals []pendingApproval `json:"pending_approvals"`
}

// runStatus reads the run and its pending requests (an agent token lists
// only those of its run).
func (s *server) runStatus(ctx context.Context, c call, _ noInput) (statusOutput, string, error) {
	res, err := agents(c.run).GetRun(ctx, connect.NewRequest(&werbotv1.GetRunRequest{Id: c.run.RunID.String()}))
	if err != nil {
		return statusOutput{}, "", err
	}
	pending, err := requests(c.run).ListRequests(ctx, connect.NewRequest(&werbotv1.ListRequestsRequest{Status: "pending"}))
	if err != nil {
		return statusOutput{}, "", fmt.Errorf("list pending approvals: %w", err)
	}
	r := res.Msg.GetRun()
	out := statusOutput{RunID: r.GetId(), Status: r.GetStatus(), Actions: r.GetActions(), MaxActions: r.GetMaxActions(),
		RemainingActions: max(r.GetMaxActions()-r.GetActions(), 0), ExpiresAt: r.GetExpiresAt().AsTime().UTC().Format(time.RFC3339),
		PendingApprovals: []pendingApproval{}}
	for _, a := range pending.Msg.GetRequests() {
		out.PendingApprovals = append(out.PendingApprovals, pendingApproval{ID: a.GetId(), Resource: a.GetResourceName(), Command: a.GetCommand(), URL: a.GetUrl()})
	}
	return out, "ok", nil
}
