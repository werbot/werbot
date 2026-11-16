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
