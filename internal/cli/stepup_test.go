package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// fakeStepUp is the step-up side of the fake API: Approve answers "mfa required" until it is fresh.
type fakeStepUp struct {
	begin     *werbotv1.BeginStepUpResponse
	confirmed bool     // the browser confirmation arrives with BeginStepUp
	fresh     bool     // an approval passes
	begun     int      // BeginStepUp calls
	totp      []string // CompleteStepUp codes
	approves  int
}

func (h *harness) BeginStepUp(context.Context, *connect.Request[werbotv1.BeginStepUpRequest]) (*connect.Response[werbotv1.BeginStepUpResponse], error) {
	h.su.begun++
	h.su.fresh = h.su.confirmed
	return connect.NewResponse(h.su.begin), nil
}

func (h *harness) CompleteStepUp(_ context.Context, r *connect.Request[werbotv1.CompleteStepUpRequest]) (*connect.Response[werbotv1.CompleteStepUpResponse], error) {
	h.su.totp = append(h.su.totp, r.Msg.GetTotpCode())
	h.su.fresh = true
	return connect.NewResponse(&werbotv1.CompleteStepUpResponse{}), nil
}

// stepUpHarness is a requestHarness whose Approve needs a fresh step-up, with stdin in and a recording browser.
func stepUpHarness(t *testing.T, in string, begin *werbotv1.BeginStepUpResponse) (*harness, *fakeRequests) {
	t.Helper()
	h, f := requestHarness(t)
	h.su = &fakeStepUp{begin: begin}
	f.approve = func(*werbotv1.ApproveRequest) error {
		h.su.approves++
		if !h.su.fresh {
			return connect.NewError(connect.CodePermissionDenied, errors.New("mfa required"))
		}
		return nil
	}
	h.env.Stdin = strings.NewReader(in)
	h.env.OpenBrowser = func(u string) error { h.opened = u; return nil }
	return h, f
}

func TestApproveStepUp(t *testing.T) {
	both := func(h *harness) *werbotv1.BeginStepUpResponse {
		return &werbotv1.BeginStepUpResponse{Methods: []string{"webauthn", "totp"}, Url: h.srv.URL + "/mfa/1", Code: "BCDF-GHJK"}
	}
	t.Run("totp", func(t *testing.T) {
		h, _ := stepUpHarness(t, "123456\n", nil)
		h.su.begin = both(h)
		require.NoError(t, h.run("approve", "r1"))
		assert.Equal(t, "Open "+h.srv.URL+"/mfa/1 (code BCDF-GHJK) or enter TOTP: ", h.errOut.String())
		assert.Equal(t, h.srv.URL+"/mfa/1", h.opened)
		assert.Equal(t, []string{"123456"}, h.su.totp)
		assert.Equal(t, 2, h.su.approves, "retried once")
		assert.NotContains(t, h.out.String()+h.errOut.String(), "123456", "a TOTP is never echoed or logged")
	})
	t.Run("browser confirmation", func(t *testing.T) {
		h, _ := stepUpHarness(t, "\n", nil)
		h.su.begin, h.su.confirmed = both(h), true
		require.NoError(t, h.run("approve", "r1"))
		assert.Empty(t, h.su.totp, "an empty line completes nothing")
		assert.Equal(t, 2, h.su.approves)
	})
	t.Run("denied again", func(t *testing.T) {
		h, _ := stepUpHarness(t, "\n", nil)
		h.su.begin = both(h)
		assert.EqualError(t, h.run("approve", "r1"), "mfa required")
		assert.Equal(t, 2, h.su.approves, "only one retry")
	})
	t.Run("totp only", func(t *testing.T) {
		h, _ := stepUpHarness(t, "654321\n", &werbotv1.BeginStepUpResponse{Methods: []string{"totp"}})
		require.NoError(t, h.run("approve", "r1"))
		assert.Equal(t, "Enter TOTP: ", h.errOut.String())
		assert.Empty(t, h.opened)
	})
	t.Run("passkey only", func(t *testing.T) {
		h, _ := stepUpHarness(t, "\n", nil)
		h.su.begin, h.su.confirmed = both(h), true
		h.su.begin.Methods = []string{"webauthn"}
		require.NoError(t, h.run("approve", "r1"))
		assert.Equal(t, "Open "+h.srv.URL+"/mfa/1 (code BCDF-GHJK): ", h.errOut.String())
	})
}

// Another refusal is not a step-up; a page on a foreign host is never opened.
func TestStepUpGuards(t *testing.T) {
	h, f := stepUpHarness(t, "123456\n", &werbotv1.BeginStepUpResponse{Methods: []string{"totp"}})
	f.approve = func(*werbotv1.ApproveRequest) error {
		return connect.NewError(connect.CodePermissionDenied, errors.New("you cannot decide this request"))
	}
	assert.EqualError(t, h.run("approve", "r1"), "you cannot decide this request")
	assert.Zero(t, h.su.begun)

	h, _ = stepUpHarness(t, "123456\n", &werbotv1.BeginStepUpResponse{Methods: []string{"webauthn"}, Url: "https://evil.example/mfa/1", Code: "BCDF-GHJK"})
	assert.ErrorContains(t, h.run("approve", "r1"), "evil.example")
	assert.Empty(t, h.opened)
	assert.Equal(t, 1, h.su.approves, "no retry after a refused step-up")

	// a separate web UI: the page is on another host, TOTP still works
	h, _ = stepUpHarness(t, "123456\n", &werbotv1.BeginStepUpResponse{Methods: []string{"webauthn", "totp"}, Url: "https://ui.example/mfa/1", Code: "BCDF-GHJK"})
	require.NoError(t, h.run("approve", "r1"))
	assert.Equal(t, "Enter TOTP: ", h.errOut.String())
	assert.Empty(t, h.opened, "a page on another host is never opened")
	assert.Equal(t, []string{"123456"}, h.su.totp)
}

func TestBreakGlassStepUp(t *testing.T) {
	h, f := stepUpHarness(t, "123456\n", &werbotv1.BeginStepUpResponse{Methods: []string{"totp"}})
	calls := 0
	f.create = func(*werbotv1.CreateRequestRequest) (*werbotv1.AccessRequest, error) {
		if calls++; !h.su.fresh {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("mfa required"))
		}
		return &werbotv1.AccessRequest{Id: "r1", Status: "approved"}, nil
	}
	require.NoError(t, h.run("request", "db", "--break-glass", "--reason", "fire"))
	assert.Equal(t, 2, calls)
}
