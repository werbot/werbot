package connector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func valid(name string) Type {
	return Type{
		Name:        name,
		Category:    CategoryServers,
		NewConfig:   func() proto.Message { return &emptypb.Empty{} },
		Validate:    func(proto.Message) error { return nil },
		Credentials: []CredentialKind{CredentialPassword},
		Actions:     []string{name + ".open"},
		Level:       "L1",
		Kinds:       []Kind{KindStream},
	}
}

func TestRegisterRejectsDuplicateAndBadTypes(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	Register(valid("a"))
	assert.PanicsWithValue(t, `connector: type "a" already registered`, func() { Register(valid("a")) })
	assert.Panics(t, func() { Register(valid("Bad")) })
	assert.Panics(t, func() { b := valid("b"); b.Actions = []string{"other.open"}; Register(b) })
	assert.Panics(t, func() { c := valid("c"); c.Kinds = nil; Register(c) })
	assert.Panics(t, func() { d := valid("d"); d.Kinds = []Kind{KindStream, "bogus"}; Register(d) })
	_, ok := Lookup("d")
	assert.False(t, ok)
}

func TestRegisterRejectsMissingParts(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	assert.Panics(t, func() { e := valid("e"); e.NewConfig = nil; Register(e) })
	assert.Panics(t, func() { f := valid("f"); f.Validate = nil; Register(f) })
	assert.Panics(t, func() { g := valid("g"); g.Actions = nil; Register(g) })
}

func TestTypesSorted(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	Register(valid("c"))
	Register(valid("b"))
	var names []string
	for _, ty := range Types() {
		names = append(names, ty.Name)
	}
	assert.Equal(t, []string{"b", "c"}, names)
}
