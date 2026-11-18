// Package sdk is the public contract between the werbot core and its modules.
package sdk

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Module is a unit of functionality the core initializes at server start.
type Module struct {
	Name    string                                  // unique, non-empty
	Feature string                                  // entitlement key; "" means an OSS module
	Init    func(ctx context.Context, h Host) error // not nil
}

// Host is the shared infrastructure the core hands to a module in Init.
type Host interface {
	DB() *pgxpool.Pool
	Handle(pattern string, h http.Handler)
	Entitled(feature string) bool
	// Subscribe registers fn for NOTIFY on channel over the server's shared LISTEN connection.
	// Call it only from Init. channel must match ^[a-z][a-z0-9_]{0,62}$.
	// Subscriptions to one channel run in registration order, after the core handler of that channel.
	// fn runs on the LISTEN goroutine and must not block: hand long work to a job.
	// ctx is the process context. Notifications sent while the connection is being re-established are lost.
	// The payload is not authenticated: any database session can NOTIFY this channel, so validate it.
	// Subscribe opens no connections and starts no goroutines.
	Subscribe(channel string, fn func(ctx context.Context, payload string))
}

var (
	mu       sync.Mutex
	registry []Module
)

// Register adds m to the registry, normally from an init func. It panics on an
// empty Name, a nil Init or a Name that is already registered.
func Register(m Module) {
	if m.Name == "" {
		panic("sdk: module with empty name")
	}
	if m.Init == nil {
		panic(`sdk: module "` + m.Name + `" has nil Init`)
	}
	mu.Lock()
	defer mu.Unlock()
	if slices.ContainsFunc(registry, func(r Module) bool { return r.Name == m.Name }) {
		panic(`sdk: module "` + m.Name + `" already registered`)
	}
	registry = append(registry, m)
}

// Modules returns a copy of the registered modules sorted by Name.
func Modules() []Module {
	mu.Lock()
	defer mu.Unlock()
	out := slices.Clone(registry)
	slices.SortFunc(out, func(a, b Module) int { return strings.Compare(a.Name, b.Name) })
	return out
}
