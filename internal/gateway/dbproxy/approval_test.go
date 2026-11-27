package dbproxy_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// approveWith answers every approval with err after telling notify about ticket "a1".
func approveWith(err error, timeout time.Duration) func(context.Context, gateway.ApprovalInput, func(string, string, time.Duration) error) error {
	return func(_ context.Context, _ gateway.ApprovalInput, notify func(id, url string, timeout time.Duration) error) error {
		if nerr := notify("a1", "https://werbot.test/requests/a1", timeout); nerr != nil {
			return nerr
		}
		return err
	}
}

// writeEnv asks for an approval of write operations.
func writeEnv(t *testing.T) (*env, *dbproxy.Guard) {
	t.Helper()
	e := newEnv(rules)
	e.dialect.res["update t set a = 1"] = result(cWrite, []string{"public.t"}, []string{"public.t"})
	e.dialect.res["update t set a = $1"] = result(cWrite, []string{"public.t"}, []string{"public.t"})
	e.pdp.rule = denyClasses("ask ops", "approval", "write")
	e.approve = approveWith(nil, 5*time.Minute)
	return e, e.guard(t)
}

// approve checks op, which needs an approval, and approves it.
func approve(t *testing.T, g *dbproxy.Guard, op dbproxy.Op, notify func(id, url string, timeout time.Duration) error) (dbproxy.Verdict, error) {
	t.Helper()
	v := check(t, g, op)
	require.Equal(t, []string{"approval"}, v.Obligations)
	return g.Approve(t.Context(), op, v, notify)
}

func TestApproveHoldFlow(t *testing.T) {
	e, g := writeEnv(t)
	var notices []string
	notify := func(id, url string, timeout time.Duration) error {
		notices = append(notices, id+" "+url+" "+gateway.FormatWait(timeout))
		return nil
	}
	v, err := approve(t, g, query("update t set a = 1"), notify)
	require.NoError(t, err)
	assert.Equal(t, dbproxy.Verdict{Allow: true, Classes: []string{"write"}, Tables: []string{"public.t"}, WriteTables: []string{"public.t"},
		PolicyIDs: []string{"binding:b"}, ApprovalID: "a1", ApprovalURL: "https://werbot.test/requests/a1"}, v)
	assert.Equal(t, []string{"a1 https://werbot.test/requests/a1 5m"}, notices)
	assert.Equal(t, []gateway.ApprovalInput{{Command: "update t set a = 1", Classes: []string{"write"}, Reasons: []string{"command:r1"}}}, e.approved)
	ds := e.pdp.decisions()
	assert.True(t, ds[len(ds)-1].ApprovalGranted, "decided again with the approval")

	e.approve, notices = approveWith(nil, 0), nil
	v, err = approve(t, g, query("update t set a = 1"), notify)
	require.NoError(t, err)
	assert.Equal(t, "a1", v.ApprovalID, "an approval granted before carries its id")
	assert.Empty(t, notices, "and is not waited for")
}

func TestApproveDeniedAndTimeout(t *testing.T) {
	e, g := writeEnv(t)
	notify := func(string, string, time.Duration) error { return nil }
	for err, msg := range map[error]string{gateway.ErrApprovalDenied: "approval denied", gateway.ErrApprovalTimeout: "approval timed out"} {
		e.approve = approveWith(err, time.Minute)
		v, aerr := approve(t, g, query("update t set a = 1"), notify)
		require.NoError(t, aerr)
		assert.False(t, v.Allow)
		assert.Equal(t, msg, v.Message)
	}
	for _, want := range []error{gateway.ErrNotReady, context.Canceled} {
		e.approve = approveWith(want, time.Minute)
		_, err := approve(t, g, query("update t set a = 1"), notify)
		require.ErrorIs(t, err, want)
	}
	e.approve = approveWith(nil, time.Minute)
	failed := errors.New("client gone")
	_, err := approve(t, g, query("update t set a = 1"), func(string, string, time.Duration) error { return failed })
	require.ErrorIs(t, err, failed, "a failed notice ends the wait")
}

func TestApprovalCoversBindParams(t *testing.T) {
	e, g := writeEnv(t)
	parse := dbproxy.Op{Kind: "parse", Text: "update t set a = $1", Statement: "s1", DB: "app"}
	assert.True(t, check(t, g, parse).Obligations != nil)
	exec := dbproxy.Op{Kind: "execute", Text: "update t set a = $1", Statement: "s1", DB: "app", Params: []string{"42", `\N`}}
	v, err := approve(t, g, exec, func(string, string, time.Duration) error { return nil })
	require.NoError(t, err)
	require.True(t, v.Allow)
	assert.Equal(t, "update t set a = $1\n-- params: [\"42\",\"\\\\N\"]", e.approved[0].Command, "the approvers read the parameters")
	assert.Equal(t, e.approved[0].Command, dbproxy.ApprovalText(exec))
	assert.True(t, check(t, g, exec).Allow, "an execute with the same parameters reuses the approval")
	assert.Len(t, e.approved, 1)
}

func TestApprovedParseDoesNotCoverExecuteOtherParams(t *testing.T) {
	e, g := writeEnv(t)
	parse := dbproxy.Op{Kind: "parse", Text: "update t set a = $1", Statement: "s1", DB: "app"}
	_, err := approve(t, g, parse, func(string, string, time.Duration) error { return nil })
	require.NoError(t, err)
	assert.True(t, check(t, g, dbproxy.Op{Kind: "execute", Text: parse.Text, Statement: "s1", DB: "app"}).Allow,
		"an execute without parameters is the approved text")
	v := check(t, g, dbproxy.Op{Kind: "execute", Text: parse.Text, Statement: "s1", DB: "app", Params: []string{"1"}})
	assert.Equal(t, []string{"approval"}, v.Obligations, "other parameters need their own approval")
	e.pdp.set(denyClasses("frozen", "", "write"))
	v = check(t, g, dbproxy.Op{Kind: "execute", Text: parse.Text, Statement: "s1", DB: "app"})
	assert.False(t, v.Allow, "a newer deny rule wins over the approval")
	assert.Equal(t, "frozen", v.Message)
}

func TestApprovedButDeniedByNewRule(t *testing.T) {
	e, g := writeEnv(t)
	e.approve = func(_ context.Context, _ gateway.ApprovalInput, notify func(string, string, time.Duration) error) error {
		e.pdp.set(denyClasses("change freeze", "", "write")) // a new snapshot during the wait
		return notify("a1", "u", time.Minute)
	}
	v, err := approve(t, g, query("update t set a = 1"), func(string, string, time.Duration) error { return nil })
	require.NoError(t, err)
	assert.False(t, v.Allow)
	assert.Equal(t, "change freeze", v.Message)
	assert.Len(t, e.denials, 1)
}

func TestApproveTooLongText(t *testing.T) {
	e, g := writeEnv(t)
	long := "update t set a = '" + strings.Repeat("x", 16384) + "'"
	e.dialect.res[long] = result(cWrite, []string{"public.t"}, []string{"public.t"})
	v, err := approve(t, g, query(long), func(string, string, time.Duration) error { return nil })
	require.NoError(t, err)
	assert.False(t, v.Allow)
	assert.Equal(t, "query too long to approve (16384 bytes)", v.Message)
	assert.Empty(t, e.approved, "no approval request")

	exec := dbproxy.Op{Kind: "execute", Text: "update t set a = $1", Statement: "s1", Params: []string{strings.Repeat("y", 16384)}}
	check(t, g, dbproxy.Op{Kind: "parse", Text: exec.Text, Statement: "s1"})
	v, err = approve(t, g, exec, func(string, string, time.Duration) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, "query too long to approve (16384 bytes)", v.Message, "the bound covers the parameters")
	assert.Empty(t, e.approved)
}

func TestApproveCutParamsRefused(t *testing.T) {
	e, g := writeEnv(t)
	check(t, g, dbproxy.Op{Kind: "parse", Text: "update t set a = $1", Statement: "s1"})
	for _, params := range [][]string{
		dbproxy.LogParams([]string{strings.Repeat("y", dbproxy.MaxLogParams+1)}), // one value past the bound: ["…[truncated]"]
		{"1", dbproxy.Truncated},
	} {
		exec := dbproxy.Op{Kind: "execute", Text: "update t set a = $1", Statement: "s1", Params: params}
		v, err := approve(t, g, exec, func(string, string, time.Duration) error { return nil })
		require.NoError(t, err)
		assert.False(t, v.Allow)
		assert.Equal(t, "query too long to approve (16384 bytes)", v.Message, "a cut display is never approved")
	}
	assert.Empty(t, e.approved, "no approval request")
}
