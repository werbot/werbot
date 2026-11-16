package sdk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func names(ms []Module) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}

func TestRegisterRejectsDuplicate(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	Register(Module{Name: "a", Init: func(context.Context, Host) error { return nil }})
	assert.PanicsWithValue(t, `sdk: module "a" already registered`, func() {
		Register(Module{Name: "a", Init: func(context.Context, Host) error { return nil }})
	})
}

func TestModulesSorted(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	noop := func(context.Context, Host) error { return nil }
	Register(Module{Name: "b", Init: noop})
	Register(Module{Name: "a", Init: noop})
	assert.Equal(t, []string{"a", "b"}, names(Modules()))
}
