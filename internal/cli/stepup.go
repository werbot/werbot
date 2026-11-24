package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

// errStepUpHost is a step-up page that is not on the server of the profile.
var errStepUpHost = errors.New("the step-up page is not on the server host")

// mfaRequired reports the server's refusal of an admin action without a fresh second factor.
func mfaRequired(err error) bool {
	ce := new(connect.Error)
	return errors.As(err, &ce) && ce.Code() == connect.CodePermissionDenied && ce.Message() == "mfa required"
}

// withStepUp runs call; when the server wants a fresh second factor it steps
// up (a TOTP typed, or the page confirmed in the browser and Enter) and runs
// call once more. The second answer is returned as it is.
func withStepUp(ctx context.Context, env Env, p *clientauth.Profile, call func() error) error {
	err := call()
	if !mfaRequired(err) {
		return err
	}
	if err := stepUp(ctx, env, p); err != nil {
		return err
	}
	return call()
}

func stepUp(ctx context.Context, env Env, p *clientauth.Profile) error {
	c := werbotv1connect.NewAuthServiceClient(p.HTTPClient(), p.Server)
	res, err := c.BeginStepUp(ctx, connect.NewRequest(&werbotv1.BeginStepUpRequest{}))
	if err != nil {
		return serverText(err)
	}
	prompt, page, err := stepUpPrompt(p.Server, res.Msg)
	if err != nil {
		return err
	}
	if page != "" && env.OpenBrowser != nil {
		_ = env.OpenBrowser(page) // the prompt shows the URL
	}
	if _, err := fmt.Fprint(env.Stderr, prompt); err != nil {
		return err
	}
	line, err := bufio.NewReader(env.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if line = strings.TrimSpace(line); line == "" {
		return nil // confirmed in the browser
	}
	_, err = c.CompleteStepUp(ctx, connect.NewRequest(&werbotv1.CompleteStepUpRequest{
		Proof: &werbotv1.CompleteStepUpRequest_TotpCode{TotpCode: line},
	}))
	return serverText(err)
}

// stepUpPrompt is the line shown before the answer, in the form of the gateway
// prompt, and the page to open: an http(s) page on the host of server. A page
// elsewhere (a separate web UI) is never opened: TOTP is asked alone, and
// without TOTP the step-up fails.
func stepUpPrompt(server string, r *werbotv1.BeginStepUpResponse) (prompt, page string, err error) {
	totp := slices.Contains(r.GetMethods(), "totp")
	switch {
	case r.GetUrl() != "" && onHost(r.GetUrl(), server) && totp:
		return fmt.Sprintf("Open %s (code %s) or enter TOTP: ", r.GetUrl(), r.GetCode()), r.GetUrl(), nil
	case r.GetUrl() != "" && onHost(r.GetUrl(), server):
		return fmt.Sprintf("Open %s (code %s): ", r.GetUrl(), r.GetCode()), r.GetUrl(), nil
	case r.GetUrl() == "" || totp:
		return "Enter TOTP: ", "", nil
	}
	return "", "", fmt.Errorf("%w: %s", errStepUpHost, r.GetUrl())
}

// onHost reports whether page is an http(s) URL on the host of server.
func onHost(page, server string) bool {
	u, err := url.Parse(page)
	srv, serr := url.Parse(server)
	return err == nil && serr == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host == srv.Host
}
