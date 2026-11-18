package tunnel

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// authCacheTTL is the longest a cached instance check lives without a change
// notification.
// ponytail: backstop for missed or undetected-lost notifications; keep it
// while LISTEN failure detection relies on TCP keepalive (WaitForNotification
// has no deadline, so a silently dead socket is not noticed at once).
const authCacheTTL = 30 * time.Second

// authCache remembers successful mTLS instance checks while the LISTEN
// connection is up, so a unary RPC skips the database. It holds one entry per
// instance (its current serial), so it is bounded by the live instances.
// Every change that refuses a cached check notifies werbot_org_changed:
// instance delete, gateway.gc and pool delete (row triggers), first use of a
// renewed serial (gateway_instance_serial_used). drop and reset bump gen, so a
// check that read the database before a notification is never stored.
type authCache struct {
	mu        sync.Mutex
	listening bool // LISTEN is up: notifications reach drop
	gen       uint64
	now       func() time.Time // nil: time.Now
	m         map[uuid.UUID]authEntry
}

type authEntry struct {
	serial string
	g      Gateway
	exp    time.Time
}

func (c *authCache) clock() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}

// get returns the cached check of the certificate, or the generation to pass
// to put after a database check. A mismatching entry is dropped.
func (c *authCache) get(gatewayID, instanceID uuid.UUID, serial string) (Gateway, uint64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[instanceID]
	if ok && e.serial == serial && e.g.ID == gatewayID && c.clock().Before(e.exp) {
		return e.g, c.gen, true
	}
	delete(c.m, instanceID)
	return Gateway{}, c.gen, false
}

// put stores a successful check made at generation gen. Only the current
// serial is cached: a check under the previous one (prev_cert_serial) always
// goes to the database, which ends it on the first use of the current one.
func (c *authCache) put(gen uint64, serial string, g Gateway) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.listening || gen != c.gen || g.CertSerial != serial {
		return
	}
	if c.m == nil {
		c.m = map[uuid.UUID]authEntry{}
	}
	c.m[g.InstanceID] = authEntry{serial: serial, g: g, exp: c.clock().Add(authCacheTTL)}
}

// drop forgets the checks of org's instances (uuid.Nil: every organization).
func (c *authCache) drop(org uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	for id, e := range c.m {
		if org == uuid.Nil || e.g.OrgID == org {
			delete(c.m, id)
		}
	}
}

// reset forgets every check and records whether LISTEN is up.
func (c *authCache) reset(listening bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	clear(c.m)
	c.listening = listening
}

// ListenLost stops the instance check cache until the next Resync: changes
// made while LISTEN is down would not reach it.
func (h *Hub) ListenLost() { h.auth.reset(false) }
