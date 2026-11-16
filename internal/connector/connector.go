// Package connector is the registry of resource types, shared by server and gateway.
package connector

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
)

// Category groups resource types in the UI.
type Category string

// Resource categories.
const (
	CategoryServers      Category = "servers"
	CategoryDatabases    Category = "databases"
	CategoryApplications Category = "applications"
	CategoryDesktops     Category = "desktops"
	CategoryContainers   Category = "containers"
	CategoryClouds       Category = "clouds"
)

// CredentialKind is a kind of secret a type accepts.
type CredentialKind string

// Credential kinds.
const (
	CredentialPassword   CredentialKind = "password"
	CredentialPrivateKey CredentialKind = "private_key"
)

// Kind is a session kind served by the gateway connector of a type.
type Kind string

// Session kinds.
const (
	KindTerminal Kind = "terminal"
	KindExec     Kind = "exec"
	KindStream   Kind = "stream"
)

// allowedKinds grows together with the gateway entry of a new kind.
var allowedKinds = []Kind{KindTerminal, KindExec, KindStream}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9]{0,31}$`)

// Type describes a resource type.
type Type struct {
	Name        string
	Category    Category
	NewConfig   func() proto.Message
	Validate    func(proto.Message) error // may normalize the message (defaults)
	Credentials []CredentialKind
	Actions     []string
	Level       string // "L1" | "L2" | "L3"
	Kinds       []Kind
}

var (
	mu       sync.RWMutex
	registry map[string]Type
)

// Register adds a type; it panics on a duplicate or an invalid definition.
func Register(t Type) {
	check(t)
	mu.Lock()
	defer mu.Unlock()
	if registry == nil {
		registry = map[string]Type{}
	}
	if _, dup := registry[t.Name]; dup {
		panic(fmt.Sprintf("connector: type %q already registered", t.Name))
	}
	registry[t.Name] = t
}

func check(t Type) {
	bad := func(msg string) { panic(fmt.Sprintf("connector: type %q: %s", t.Name, msg)) }
	switch {
	case !nameRe.MatchString(t.Name):
		bad("invalid name")
	case t.NewConfig == nil || t.Validate == nil:
		bad("NewConfig and Validate are required")
	case len(t.Actions) == 0:
		bad("no actions")
	case len(t.Kinds) == 0:
		bad("no kinds")
	}
	for _, a := range t.Actions {
		if !strings.HasPrefix(a, t.Name+".") {
			bad(fmt.Sprintf("action %q lacks prefix %q", a, t.Name+"."))
		}
	}
	for _, k := range t.Kinds {
		if !slices.Contains(allowedKinds, k) {
			bad(fmt.Sprintf("unknown kind %q", k))
		}
	}
}

// Lookup returns the registered type by name.
func Lookup(name string) (Type, bool) {
	mu.RLock()
	defer mu.RUnlock()
	t, ok := registry[name]
	return t, ok
}

// Types returns a copy sorted by Name.
func Types() []Type {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Type, 0, len(registry))
	for _, t := range registry {
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b Type) int { return strings.Compare(a.Name, b.Name) })
	return out
}
