package gateway

import (
	"context"
	"errors"
	"sync"
	"time"
)

// fakeApprovals is the approval side of fakeControl. RequestApproval returns
// the ticket "a1" of status, expiring ttl after the call, or reqErr; GetApproval answers
// getErr or the statuses in turn, the last one repeating. Decisions reach the
// waits through the production router, as the tunnel delivers them.
type fakeApprovals struct {
	approvals
	mu       sync.Mutex
	ttl      time.Duration
	status   string
	reqErr   error
	getErr   error
	statuses []string
	requests []approvalRequest
	gets     []time.Time // when GetApproval was called
}

func (f *fakeControl) RequestApproval(_ context.Context, r approvalRequest) (approvalTicket, error) {
	if f.approvals == nil {
		return approvalTicket{}, errors.New("fakeControl: no approvals")
	}
	return f.approvals.request(r)
}

func (f *fakeControl) GetApproval(_ context.Context, id string) (string, error) {
	return f.approvals.get(id)
}

func (f *fakeControl) SubscribeApproval(id string) (<-chan struct{}, func()) {
	return f.approvals.subscribe(id)
}

func (a *fakeApprovals) request(r approvalRequest) (approvalTicket, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, r)
	if a.reqErr != nil {
		return approvalTicket{}, a.reqErr
	}
	return approvalTicket{ID: "a1", URL: "https://werbot.test/requests/a1", ExpiresAt: time.Now().Add(a.ttl), Status: a.status}, nil
}

func (a *fakeApprovals) get(string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gets = append(a.gets, time.Now())
	if a.getErr != nil {
		return "", a.getErr
	}
	s := a.statuses[0]
	if len(a.statuses) > 1 {
		a.statuses = a.statuses[1:]
	}
	return s, nil
}

// set makes GetApproval answer status from now on.
func (a *fakeApprovals) set(status string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.statuses = []string{status}
}

// decide sets status and sends the decision of "a1" as the tunnel does.
func (a *fakeApprovals) decide(status string) {
	a.set(status)
	a.deliver("a1")
}

// waiting is the number of open subscriptions.
func (a *approvals) waiting() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.subs)
}
