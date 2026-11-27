package tunnel

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

var errUnexpected = errors.New("unexpected call")

// WriteCredential and ProbeCredential complete the Credentials of the base
// fake; credFake records them.
func (f *fake) WriteCredential(context.Context, Gateway, uuid.UUID, uuid.UUID, []byte) (Secret, error) {
	return Secret{}, errUnexpected
}

func (f *fake) ProbeCredential(context.Context, Gateway, uuid.UUID) (Secret, int64, error) {
	return Secret{}, 0, errUnexpected
}

// credCall is one Reveal, WriteCredential or ProbeCredential call.
type credCall struct {
	g            Gateway
	agent        bool
	id, approval uuid.UUID // the session or resource id; the approval of a write
	sha          []byte
	typ          string
}

// credFake answers the credential calls with secret, version and err and records them.
type credFake struct {
	*fake
	mu      sync.Mutex
	secret  Secret
	version int64
	err     error
	reveals []credCall
	writes  []credCall
	probes  []credCall
}

func (c *credFake) Reveal(_ context.Context, _, resourceID uuid.UUID, resourceType string, agent bool) (Secret, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reveals = append(c.reveals, credCall{agent: agent, id: resourceID, typ: resourceType})
	return c.secret, c.err
}

func (c *credFake) WriteCredential(_ context.Context, g Gateway, sessionID, approvalID uuid.UUID, sha []byte) (Secret, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, credCall{g: g, id: sessionID, approval: approvalID, sha: sha})
	return c.secret, c.err
}

func (c *credFake) ProbeCredential(_ context.Context, g Gateway, resourceID uuid.UUID) (Secret, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.probes = append(c.probes, credCall{g: g, id: resourceID})
	return c.secret, c.version, c.err
}
