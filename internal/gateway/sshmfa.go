package gateway

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// mfaMethodKey holds the factor ("totp" | "webauthn") the connection passed
// at keyboard-interactive; every channel of it counts the factor as fresh.
type mfaMethodKey struct{}

// verified is the ServerConfigCallback: once a key signature is verified it
// decides the direct login for the user of that key. A person's deny that
// only a fresh second factor lifts answers with a partial success that asks
// for keyboard-interactive (mfaChallenge), and the handshake limit becomes
// the challenge's. Anything else accepts the key as before: the timer stops
// for a user and every channel decides and tells. A connection the control
// plane could not check (no user) keeps the timer: every channel gets only
// the not ready text, so the limit closes it.
func (h *sshHandler) verified(ctx ssh.Context) *gossh.ServerConfig {
	return &gossh.ServerConfig{VerifiedPublicKeyCallback: func(_ gossh.ConnMetadata, key gossh.PublicKey, perms *gossh.Permissions, _ string) (*gossh.Permissions, error) {
		ctx.SetValue(verifiedKey{}, true)
		if !h.verifiedUser(ctx, key) {
			return nil, errors.New("permission denied")
		}
		t, _ := ctx.Value(handshakeKey{}).(*time.Timer)
		if h.needsMFA(ctx) {
			if t != nil {
				t.Reset(mfaChallengeTTL + 10*time.Second)
			}
			return nil, &gossh.PartialSuccessError{Next: gossh.ServerAuthCallbacks{KeyboardInteractiveCallback: h.mfaChallenge(ctx)}}
		}
		if t != nil && handshakeUser(ctx) != nil {
			t.Stop()
		}
		return perms, nil
	}}
}

// verifiedUser makes the handshake user the one of the verified key: x/crypto
// asks auth for every key offered and caches the answers, so the last answer
// may be another key's. Then the verified key is looked up again.
func (h *sshHandler) verifiedUser(ctx ssh.Context, key gossh.PublicKey) bool {
	if a, _ := ctx.Value(userKey{}).(*authed); a == nil || a.id.Value == gossh.FingerprintSHA256(key) {
		return true
	}
	ctx.SetValue(ssh.ContextKeyPublicKey, key) // request() takes the signing key from it
	return h.auth(ctx, key)
}

// needsMFA reports whether the direct login of ctx is a person's that only a
// fresh second factor lets in. A menu login, an unready gateway and any
// other decision pass: the channels decide.
func (h *sshHandler) needsMFA(ctx ssh.Context) bool {
	_, resource, _ := ParseLogin(ctx.User())
	if resource == "" || handshakeUser(ctx) == nil {
		return false
	}
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, _, _, err := h.p.decide(dctx, h.request(ctx, resource, "", Window{}, ""))
	return errors.Is(err, ErrMFARequired)
}

// mfaChallenge is the keyboard-interactive step after the partial success:
// the connect decision asks in prompt rounds (kiPrompt) until a factor meets
// it. Success stops the handshake timer and keeps the factor for the
// connection's channels. A failure is refused with its text as an auth
// banner and leaves the timer running.
func (h *sshHandler) mfaChallenge(ctx ssh.Context) func(gossh.ConnMetadata, gossh.KeyboardInteractiveChallenge) (*gossh.Permissions, error) {
	return func(_ gossh.ConnMetadata, client gossh.KeyboardInteractiveChallenge) (*gossh.Permissions, error) {
		_, resource, _ := ParseLogin(ctx.User())
		r := h.request(ctx, resource, "", Window{}, "")
		r.MFA.Prompt = kiPrompt(client)
		_, req, _, err := h.p.decide(ctx, r)
		if err != nil {
			return nil, &gossh.BannerError{Err: err, Message: openText(err, resource, "", h.p.ctl.Policy()) + "\r\n"}
		}
		if req.MFAMethod != "" && req.MFAAgeSeconds == 0 {
			ctx.SetValue(mfaMethodKey{}, req.MFAMethod)
		}
		if t, ok := ctx.Value(handshakeKey{}).(*time.Timer); ok {
			t.Stop()
		}
		return ctx.Permissions().Permissions, nil
	}
}

// kiPrompt asks one keyboard-interactive round: name and instruction empty,
// text the only question, not echoed. x/crypto checks the answer count.
func kiPrompt(client gossh.KeyboardInteractiveChallenge) MFAPrompt {
	return func(_ context.Context, text string) (string, error) {
		answers, err := client("", "", []string{text}, []bool{false})
		if err != nil {
			return "", err
		}
		return answers[0], nil
	}
}

// terminalPrompt asks on the terminal of s without echo. The read stops
// waiting with ctx; its goroutine ends with the next input or the channel.
func terminalPrompt(s ssh.Session) MFAPrompt {
	return func(ctx context.Context, text string) (string, error) {
		type line struct {
			text string
			err  error
		}
		got := make(chan line, 1)
		go func() {
			l, err := term.NewTerminal(byteReader{s, func() {}}, "").ReadPassword(text)
			got <- line{l, err}
		}()
		select {
		case l := <-got:
			return l.text, l.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// noticePrompt tells text on w, where the user cannot answer, and answers
// "": the wait is for the confirmation in the browser.
func noticePrompt(w io.Writer) MFAPrompt {
	return func(_ context.Context, text string) (string, error) {
		say(w, text)
		return "", nil
	}
}
