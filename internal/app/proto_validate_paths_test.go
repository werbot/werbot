package app

import (
	"strconv"
	"strings"
	"testing"

	"buf.build/go/protovalidate"
	"github.com/stretchr/testify/assert"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// TestRequestValidationStage11b covers the bounds of the path rule, terminal
// rule and TestCommand requests (stage 11b).
func TestRequestValidationStage11b(t *testing.T) {
	id := "0190a000-0000-7000-8000-000000000001"
	subj := &werbotv1.Subjects{AllUsers: true}
	sel := &werbotv1.ResourceSelector{Labels: map[string]string{"env": "prod"}}
	paths := func(n, size int) []string {
		out := make([]string, n)
		for i := range out {
			suffix := strconv.Itoa(i)
			out[i] = "/" + strings.Repeat("a", size-1-len(suffix)) + suffix
		}
		return out
	}

	create := func(mut func(*werbotv1.CreatePathRuleRequest)) *werbotv1.CreatePathRuleRequest {
		r := &werbotv1.CreatePathRuleRequest{Name: "app", Subjects: subj, ResourceSelector: sel, Paths: []string{"~/app/**"}, Access: "any", Effect: "only"}
		mut(r)
		return r
	}
	assert.NoError(t, protovalidate.Validate(create(func(*werbotv1.CreatePathRuleRequest) {})))
	assert.NoError(t, protovalidate.Validate(create(func(r *werbotv1.CreatePathRuleRequest) { r.Paths = paths(32, 1024) })))
	assert.NoError(t, protovalidate.Validate(create(func(r *werbotv1.CreatePathRuleRequest) { r.Message = strings.Repeat("я", 256) })))
	for name, mut := range map[string]func(*werbotv1.CreatePathRuleRequest){
		"empty name":     func(r *werbotv1.CreatePathRuleRequest) { r.Name = "" },
		"long name":      func(r *werbotv1.CreatePathRuleRequest) { r.Name = strings.Repeat("a", 64) },
		"no paths":       func(r *werbotv1.CreatePathRuleRequest) { r.Paths = nil },
		"33 paths":       func(r *werbotv1.CreatePathRuleRequest) { r.Paths = paths(33, 10) },
		"repeated path":  func(r *werbotv1.CreatePathRuleRequest) { r.Paths = []string{"/a", "/a"} },
		"empty path":     func(r *werbotv1.CreatePathRuleRequest) { r.Paths = []string{""} },
		"long path":      func(r *werbotv1.CreatePathRuleRequest) { r.Paths = paths(1, 1025) },
		"unknown access": func(r *werbotv1.CreatePathRuleRequest) { r.Access = "exec" },
		"unknown effect": func(r *werbotv1.CreatePathRuleRequest) { r.Effect = "allow" },
		"long message":   func(r *werbotv1.CreatePathRuleRequest) { r.Message = strings.Repeat("a", 257) },
	} {
		assert.Error(t, protovalidate.Validate(create(mut)), name)
	}

	update := &werbotv1.UpdatePathRuleRequest{Id: id, Name: "app", Subjects: subj, ResourceSelector: sel, Paths: []string{"/srv/**"}, Access: "read", Effect: "deny"}
	assert.NoError(t, protovalidate.Validate(update))
	update.Id = "x"
	assert.Error(t, protovalidate.Validate(update))
	update.Id, update.Effect = id, "maybe"
	assert.Error(t, protovalidate.Validate(update))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeletePathRuleRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeletePathRuleRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListPathRulesRequest{}))

	term := &werbotv1.CreateTerminalRuleRequest{Name: "console", Subjects: subj, ResourceSelector: sel, Mode: "console"}
	assert.NoError(t, protovalidate.Validate(term))
	term.Mode = "shell"
	assert.Error(t, protovalidate.Validate(term))
	term.Mode, term.Name = "deny", ""
	assert.Error(t, protovalidate.Validate(term))
	assert.NoError(t, protovalidate.Validate(&werbotv1.DeleteTerminalRuleRequest{Id: id}))
	assert.Error(t, protovalidate.Validate(&werbotv1.DeleteTerminalRuleRequest{Id: "x"}))
	assert.NoError(t, protovalidate.Validate(&werbotv1.ListTerminalRulesRequest{}))

	probe := func(cwd string) *werbotv1.TestCommandRequest {
		return &werbotv1.TestCommandRequest{Subject: &werbotv1.TestCommandRequest_UserId{UserId: id}, ResourceId: id, Command: "ls", Cwd: cwd}
	}
	assert.NoError(t, protovalidate.Validate(probe("")))
	assert.NoError(t, protovalidate.Validate(probe(paths(1, 1024)[0])))
	assert.Error(t, protovalidate.Validate(probe(paths(1, 1025)[0])))
}
