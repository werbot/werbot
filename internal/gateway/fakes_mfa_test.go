package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// mfaRuleOn is an MFA rule of 60 s with strength for u1 on resource id,
// connected with action "<typ>.connect" (internal/policy/compile_mfa.go).
func mfaRuleOn(id, typ, strength string) string {
	anyFactor := "true"
	if strength == "webauthn" {
		anyFactor = "false"
	}
	return `
@id("mfa:a")
@obligation("mfa")
forbid (principal, action in [Werbot::Action::"` + typ + `.connect"], resource)
when { principal in [Werbot::User::"u1"] && [Werbot::Resource::"` + id + `"].contains(resource) }
unless { context.mfa_age < 60 && (` + anyFactor + ` || context.mfa_method == "webauthn") };`
}

// fakeMFA adds the MFA challenge calls to a fakeControl: every challenge is
// "c1" and expires mfaChallengeTTL after it was created; VerifyMFATOTP and
// GetMFAChallenge answer from verify and status.
type fakeMFA struct {
	*fakeControl
	waits approvals

	mmu     sync.Mutex
	creates []challengeRequest
	codes   []string
	gets    int
	verify  func(code string) (bool, error) // nil: false
	status  func(n int) string              // by GetMFAChallenge call, from 1; nil: "pending"
}

func (f *fakeMFA) CreateMFAChallenge(_ context.Context, r challengeRequest) (mfaTicket, error) {
	f.mmu.Lock()
	defer f.mmu.Unlock()
	f.creates = append(f.creates, r)
	return mfaTicket{ID: "c1", Code: "BCDF-GHJK", URL: "https://ui.werbot.test/mfa/c1", ExpiresAt: time.Now().Add(mfaChallengeTTL)}, nil
}

func (f *fakeMFA) VerifyMFATOTP(_ context.Context, _, code string) (bool, error) {
	f.mmu.Lock()
	defer f.mmu.Unlock()
	f.codes = append(f.codes, code)
	if f.verify == nil {
		return false, nil
	}
	return f.verify(code)
}

func (f *fakeMFA) GetMFAChallenge(context.Context, string) (string, error) {
	f.mmu.Lock()
	defer f.mmu.Unlock()
	f.gets++
	if f.status == nil {
		return "pending", nil
	}
	return f.status(f.gets), nil
}

func (f *fakeMFA) SubscribeMFA(id string) (<-chan struct{}, func()) { return f.waits.subscribe(id) }

func (f *fakeMFA) calls() (creates []challengeRequest, codes []string, gets int) {
	f.mmu.Lock()
	defer f.mmu.Unlock()
	return f.creates, f.codes, f.gets
}

func (f *fakeMFA) set(fn func(*fakeMFA)) {
	f.mmu.Lock()
	defer f.mmu.Unlock()
	fn(f)
}

// confirmedFrom answers "confirmed" from GetMFAChallenge call n on.
func confirmedFrom(n int) func(int) string {
	return func(got int) string {
		if got >= n {
			return "confirmed"
		}
		return "pending"
	}
}

// withMFA makes the pipeline of e answer the MFA calls through a fakeMFA.
func withMFA(p *Pipeline) *fakeMFA {
	f := &fakeMFA{fakeControl: p.ctl.(*fakeControl)}
	p.ctl = f
	return f
}

// mfaPipe is a pipeline env whose resource "blocker" has an MFA rule of strength.
func mfaPipe(t *testing.T, strength string) (*pipeEnv, *fakeMFA) {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.f.policy = compileTest(t, e.m, pipePolicies+mfaRuleOn("rblocker", "blocker", strength), nil,
		res{"rfake", "fake", "fake"}, res{"rblocker", "blocker", "blocker"})
	return e, withMFA(e.p)
}

// askWith is a prompt that records its texts and answers from answers in turn, "" after them.
type askWith struct {
	mu      sync.Mutex
	answers []string
	texts   []string
}

func (a *askWith) ask(_ context.Context, text string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.texts = append(a.texts, text)
	if len(a.answers) == 0 {
		return "", nil
	}
	out := a.answers[0]
	a.answers = a.answers[1:]
	return out, nil
}

// mfaSSH is an SSH env whose resource "db" has an MFA rule of strength any.
func mfaSSH(t *testing.T) (*sshEnv, *fakeMFA) {
	t.Helper()
	var fm *fakeMFA
	e := newSSHEnvWith(t, Config{}, func(p *Pipeline) { fm = withMFA(p) })
	e.setMFARule(t, "rdb", "fake")
	return e, fm
}

// setMFARule puts the MFA rule on resource id of type typ ("" removes it).
func (e *sshEnv) setMFARule(t *testing.T, id, typ string) {
	t.Helper()
	rule := ""
	if id != "" {
		rule = mfaRuleOn(id, typ, "any")
	}
	e.f.setPolicy(compileTest(t, e.m, sshPolicies+rule, nil, res{"rdb", "db", "fake"}, res{"rblocker", "blocker", "blocker"},
		res{"rdenied", "denied", "fake"}))
}

// kiClient answers every keyboard-interactive round from answers in turn ("" after them) and records the rounds.
type kiClient struct {
	mu      sync.Mutex
	answers []string
	rounds  [][]string // name, instruction, questions...
	echos   [][]bool
}

func (k *kiClient) challenge(name, instruction string, questions []string, echos []bool) ([]string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.rounds = append(k.rounds, append([]string{name, instruction}, questions...))
	k.echos = append(k.echos, echos)
	out := make([]string, len(questions))
	if len(questions) > 0 && len(k.answers) > 0 {
		out[0], k.answers = k.answers[0], k.answers[1:]
	}
	return out, nil
}

// dialMFA logs in as user with alice's key and, when ki is set, keyboard-interactive; banners collects the auth banners.
func (e *sshEnv) dialMFA(user string, ki *kiClient, banners *syncBuffer) (*gossh.Client, error) {
	auth := []gossh.AuthMethod{gossh.PublicKeys(e.keys["alice"])}
	if ki != nil {
		auth = append(auth, gossh.KeyboardInteractive(ki.challenge))
	}
	return gossh.Dial("tcp", e.addr, &gossh.ClientConfig{User: user, Auth: auth, HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()),
		Timeout: 5 * time.Second, BannerCallback: func(m string) error { _, err := banners.Write([]byte(m)); return err }})
}

func requireNoChallenge(t *testing.T, f *fakeMFA) {
	t.Helper()
	creates, _, _ := f.calls()
	require.Empty(t, creates, "no challenge is created")
}
