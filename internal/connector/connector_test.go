package connector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
		Endpoint:    func(proto.Message) Endpoint { return Endpoint{} },
	}
}

func TestRegisterRejectsBadRecordedAndEndpoint(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	assert.Panics(t, func() { a := valid("a"); a.Recorded = []Kind{KindExec}; Register(a) })
	assert.Panics(t, func() { b := valid("b"); b.Endpoint = nil; Register(b) })
	assert.NotPanics(t, func() { c := valid("c"); c.Kinds = []Kind{KindDatagram}; Register(c) })
	assert.Panics(t, func() { e := valid("e"); e.Kinds = []Kind{KindStream, KindStream}; Register(e) })
	assert.Panics(t, func() {
		f := valid("f")
		f.Kinds, f.Recorded = []Kind{KindStream, KindTerminal}, []Kind{KindTerminal, KindTerminal}
		Register(f)
	})
	d := valid("d")
	d.Kinds, d.Recorded, d.Actions = []Kind{KindStream, KindTerminal}, []Kind{KindTerminal}, []string{"d.open", "d.shell"}
	Register(d)
	assert.True(t, d.Records(KindTerminal))
	assert.False(t, d.Records(KindStream))
}

func TestEndpointOf(t *testing.T) {
	ty := valid("e")
	ty.NewConfig = func() proto.Message { return &wrapperspb.StringValue{} }
	ty.Endpoint = func(m proto.Message) Endpoint { return Endpoint{Host: m.(*wrapperspb.StringValue).Value} }
	e, ok, err := ty.EndpointOf([]byte(`"h"`))
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "h", e.Host)
	_, _, err = ty.EndpointOf([]byte(`{`))
	assert.Error(t, err)
	ty.Endpoint = nil
	_, ok, err = ty.EndpointOf([]byte(`"h"`))
	assert.NoError(t, err)
	assert.False(t, ok)
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

func TestExecAction(t *testing.T) {
	ty := valid("x")
	_, ok := ty.ExecAction()
	assert.False(t, ok, "no exec action")
	ty.Actions = []string{"x.connect", "x.exec", "x.other.exec"}
	a, ok := ty.ExecAction()
	assert.True(t, ok)
	assert.Equal(t, "x.exec", a, "the first .exec action")
}

func TestShellAction(t *testing.T) {
	ty := valid("x")
	a, ok := ty.ShellAction()
	assert.False(t, ok, "no shell action")
	assert.Empty(t, a)
	ty.Actions = []string{"x.connect", "x.exec", "x.shell"}
	a, ok = ty.ShellAction()
	assert.True(t, ok)
	assert.Equal(t, "x.shell", a)
}

func TestRegisterRequiresShellForTerminal(t *testing.T) {
	t.Cleanup(func() { registry = nil })
	x := valid("x")
	x.Kinds, x.Actions = []Kind{KindTerminal}, []string{"x.connect"}
	assert.PanicsWithValue(t, `connector: type "x": terminal kind needs action "x.shell"`, func() { Register(x) })
	x.Actions = []string{"x.connect", "x.shell"}
	assert.NotPanics(t, func() { Register(x) })
	assert.NotPanics(t, func() { Register(valid("y")) }, "a type without the terminal kind needs no shell action")
}
