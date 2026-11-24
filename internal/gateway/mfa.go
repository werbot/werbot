package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
)

const (
	// MFAPollInterval is how often an MFA wait reads the challenge status: the
	// fallback for a confirmation the stream did not bring.
	MFAPollInterval = 2 * time.Second
	// mfaChallengeTTL bounds an MFA wait, as the server bounds the challenge.
	mfaChallengeTTL = 60 * time.Second
)

// MFAPrompt shows text to the user and returns one line of their answer.
type MFAPrompt func(ctx context.Context, text string) (answer string, err error)

// MFAStep is the second factor of an OpenRequest. Method is the factor the
// connection passed ("totp" | "webauthn"; "": none), which counts as fresh.
// Prompt asks the user; nil: the front cannot ask and an mfa obligation is
// ErrMFARequired. Session is the SSH session id a challenge is bound to.
type MFAStep struct {
	Method  string
	Prompt  MFAPrompt
	Session []byte
}

// mfaControl is the MFA challenge part of the control plane; *tunnelClient
// implements it, and a control plane without it answers ErrMFARequired.
type mfaControl interface {
	CreateMFAChallenge(ctx context.Context, r challengeRequest) (mfaTicket, error)
	VerifyMFATOTP(ctx context.Context, id, code string) (ok bool, err error)
	GetMFAChallenge(ctx context.Context, id string) (status string, err error)
	SubscribeMFA(id string) (confirmed <-chan struct{}, unsubscribe func())
}

// challengeRequest asks the user to confirm a login to a resource with a second
// factor; Binding is the verified key or certificate the factor is recorded
// for, Strength "any" or "webauthn".
type challengeRequest struct {
	UserID, ResourceID uuid.UUID
	SSHSessionID       []byte
	ClientIP           netip.Addr
	Binding, Strength  string
}

// mfaTicket is a challenge: its confirmation page, the code shown on both
// sides and when it expires.
type mfaTicket struct {
	ID, Code, URL string
	ExpiresAt     time.Time
}

// totpCode is the form of a TOTP answer; anything else asks again.
var totpCode = regexp.MustCompile(`^[0-9]{6}$`)

// connect decides the connect request req of r with the user's last second
// factor, or the connection's. A person's deny that only a fresh factor lifts
// asks for one through r.MFA.Prompt and decides again; an agent never meets
// it (a plain deny). req keeps the factor: the exec, shell and console steps
// and the sweeper decide with it.
func (p *Pipeline) connect(ctx context.Context, pol *Policy, req *AccessRequest, r OpenRequest) error {
	req.MFAAgeSeconds, req.MFAMethod = req.User.MFAAgeSeconds, req.User.MFAMethod
	if r.MFA.Method != "" {
		req.MFAAgeSeconds, req.MFAMethod = 0, r.MFA.Method
	}
	d := pol.Decide(*req)
	switch {
	case d.Allow:
		return nil
	case req.User.Agent != nil || !slices.Equal(d.Obligations, []string{ObligationMFA}):
		return accessDenied{d.Reasons}
	case r.MFA.Prompt == nil:
		return ErrMFARequired
	}
	method, err := p.awaitMFA(ctx, challengeRequest{UserID: req.User.ID, ResourceID: req.Resource.ID, SSHSessionID: r.MFA.Session,
		ClientIP: r.ClientIP, Binding: mfaBinding(r.Identity), Strength: pol.MFAStrength(*req)}, r.MFA.Prompt)
	if err != nil {
		return err
	}
	req.MFAAgeSeconds, req.MFAMethod = 0, method
	if d := pol.Decide(*req); !d.Allow {
		return accessDenied{d.Reasons}
	}
	return nil
}

// mfaBinding is what a second factor of id is recorded for: the key or the
// certificate fingerprint.
func mfaBinding(id Identity) string {
	if id.Method == "ssh-cert" {
		return "cert:" + id.Value
	}
	return "key:" + id.Value
}

// awaitMFA creates a challenge for r and asks until it is met: a TOTP code
// (strength any) or an empty answer, then the browser confirmation. A wrong
// or malformed code asks again; it returns the method that met it.
func (p *Pipeline) awaitMFA(ctx context.Context, r challengeRequest, ask MFAPrompt) (string, error) {
	mc, ok := p.ctl.(mfaControl)
	if !ok {
		return "", ErrMFARequired
	}
	end := p.now().Add(mfaChallengeTTL)
	t, err := mc.CreateMFAChallenge(ctx, r)
	if err != nil {
		return "", controlErr(err)
	}
	if t.ExpiresAt.Before(end) { // a zero ExpiresAt is in the past: the challenge is over
		end = t.ExpiresAt
	}
	confirmed, unsubscribe := mc.SubscribeMFA(t.ID)
	defer unsubscribe()
	text := promptText(t.URL, t.Code, r.Strength != "webauthn")
	actx, cancel := context.WithDeadline(ctx, end)
	defer cancel()
	for {
		answer, err := ask(actx, text)
		if err != nil {
			return "", askErr(actx, err)
		}
		answer = strings.TrimSpace(answer)
		if answer == "" {
			return p.mfaConfirmation(actx, mc, t.ID, confirmed)
		}
		if method, err := p.mfaCode(actx, mc, t.ID, answer, r.Strength); method != "" || err != nil {
			return method, err
		}
	}
}

// askErr is the error of a prompt that failed: past the deadline the wait
// timed out, otherwise the user did not answer.
func askErr(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrMFATimeout
	}
	return fmt.Errorf("%w: %w", ErrMFARequired, err)
}

// mfaCode checks a typed answer: "totp" once a code is accepted, "" to ask
// again (a wrong or malformed code, or any code when only a passkey meets
// the rule).
func (p *Pipeline) mfaCode(ctx context.Context, mc mfaControl, id, answer, strength string) (string, error) {
	if ctx.Err() != nil {
		return "", ErrMFATimeout
	}
	if strength == "webauthn" || !totpCode.MatchString(answer) {
		return "", nil
	}
	ok, err := mc.VerifyMFATOTP(ctx, id, answer)
	switch {
	case connect.CodeOf(err) == connect.CodeResourceExhausted:
		return "", ErrMFATooManyAttempts
	case ctx.Err() != nil:
		return "", ErrMFATimeout
	case err != nil:
		return "", controlErr(err)
	case ok:
		return "totp", nil
	}
	return "", nil
}

// mfaOutcomes end an MFA wait by the challenge status; "pending" waits on.
var mfaOutcomes = map[string]error{
	"confirmed": nil,
	"denied":    ErrMFATooManyAttempts,
	"expired":   ErrMFATimeout,
}

// mfaConfirmation waits for the browser confirmation of challenge id: a
// hint on the stream or every MFAPollInterval it reads the status; at the
// deadline of ctx it reads it a last time. The hint names one challenge, so
// another's never wakes it, and it is only a hint: the status decides.
func (p *Pipeline) mfaConfirmation(ctx context.Context, mc mfaControl, id string, confirmed <-chan struct{}) (string, error) {
	poll := time.NewTicker(MFAPollInterval)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) && p.mfaStatus(context.WithoutCancel(ctx), mc, id) == "confirmed" {
				return "webauthn", nil
			}
			return "", askErr(ctx, ctx.Err())
		case <-p.active.drained:
			return "", ErrNotReady
		case <-confirmed:
		case <-poll.C:
		}
		if err, done := mfaOutcomes[p.mfaStatus(ctx, mc, id)]; done {
			return confirmedBy(err)
		}
	}
}

// confirmedBy is the result of a wait ended with err: the browser's passkey
// confirmed it when err is nil.
func confirmedBy(err error) (string, error) {
	if err != nil {
		return "", err
	}
	return "webauthn", nil
}

// mfaStatus reads the status of challenge id; "" when the read fails.
func (p *Pipeline) mfaStatus(ctx context.Context, mc mfaControl, id string) string {
	status, err := mc.GetMFAChallenge(ctx, id)
	if err != nil {
		p.log.Warn("mfa: status read failed, still waiting", "error", err)
	}
	return status
}
