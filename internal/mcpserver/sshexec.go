package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/platform/sshca"
	"github.com/werbot/werbot/internal/platform/version"
)

// Bounds of ssh_exec.
const (
	DefaultTimeout = 60 * time.Second
	MaxTimeout     = 600 * time.Second
	MaxOutput      = 65536 // bytes of stdout and stderr together
)

// pendingAfter is how long a call waits for an approval; tests shrink it.
var pendingAfter = 50 * time.Second

// ExecInput is the input of ssh_exec.
type ExecInput struct {
	Resource   string `json:"resource" jsonschema:"the resource name from list_resources"`
	Command    string `json:"command" jsonschema:"the command; a relative path starts at the login's home directory"`
	Timeout    int    `json:"timeout,omitempty" jsonschema:"seconds, 1 to 600; 0: 60"`
	ApprovalID string `json:"approval_id,omitempty" jsonschema:"the approval of an earlier pending_approval answer, for the transcript only"`
}

// ExecOutput is the result of ssh_exec: status is ok, denied, error or pending_approval.
type ExecOutput struct {
	Status      string `json:"status"`
	ExitCode    *int   `json:"exit_code,omitempty"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Truncated   bool   `json:"truncated"`
	ApprovalID  string `json:"approval_id,omitempty"`
	ApprovalURL string `json:"approval_url,omitempty"`
	Message     string `json:"message,omitempty"`
}

// execTimeout is the deadline of a call from its timeout in seconds.
func execTimeout(seconds int) (time.Duration, error) {
	d := time.Duration(seconds) * time.Second
	if seconds < 0 || d > MaxTimeout {
		return 0, errors.New("timeout must be between 1 and 600 seconds")
	}
	if d == 0 {
		return DefaultTimeout, nil
	}
	return d, nil
}

// sshExec runs in.Command on in.Resource through its gateway, within the
// timeout from the start of the call.
func (s *server) sshExec(ctx context.Context, c call, in ExecInput) (ExecOutput, string, error) {
	timeout, err := execTimeout(in.Timeout)
	switch {
	case err != nil:
		return ExecOutput{}, "", err
	case in.Resource == "":
		return ExecOutput{}, "", errors.New("resource is required")
	case in.Command == "":
		return ExecOutput{}, "", errors.New("command is required")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	r, err := s.resource(ctx, c.run, in.Resource)
	if err == nil && r == nil {
		err = fmt.Errorf("resource %s not found", in.Resource)
	}
	if err != nil {
		return ExecOutput{}, "", err
	}
	out, err := s.exec(ctx, c, r, in.Command, timeout)
	if err != nil {
		return ExecOutput{}, "", err
	}
	return out, out.Status, nil
}

// exec opens one SSH session to the gateway of r and waits for its end, an
// approval wait longer than pendingAfter or the deadline of ctx; the
// session is closed on return.
func (s *server) exec(ctx context.Context, c call, r *werbotv1.AccessibleResource, command string, timeout time.Duration) (ExecOutput, error) {
	client, err := s.dialSSH(ctx, c.run, r)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return timedOut(timeout), nil
	}
	if err != nil {
		return ExecOutput{}, err
	}
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		return ExecOutput{}, err
	}
	if err := sess.Setenv(sshca.EnvToolCall, c.id); err != nil {
		return ExecOutput{}, err
	}
	out := newCapture()
	sess.Stdout, sess.Stderr = sink{out, &out.stdout}, sink{out, &out.stderr}
	if err := sess.Start(command); err != nil {
		return ExecOutput{}, err
	}
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	return out.wait(ctx, done, timeout), nil
}

// dialSSH connects to the gateway of r as <login>+<resource> with the run
// certificate, trusting only the pool host key the server listed.
func (s *server) dialSSH(ctx context.Context, run *clientauth.AgentProfile, r *werbotv1.AccessibleResource) (*ssh.Client, error) {
	hostKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.GetHostKey()))
	if r.GetSshAddress() == "" || err != nil {
		return nil, fmt.Errorf("resource %s has no gateway address or host key", r.GetName())
	}
	conn, err := s.opts.Dial(ctx, connectcli.Endpoint{SSHAddress: r.GetSshAddress()})
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	conf := &ssh.ClientConfig{
		User: run.Login + "+" + r.GetName(),
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			signer, err := run.Signer(ctx)
			if err != nil {
				return nil, err
			}
			return []ssh.Signer{signer}, nil
		})},
		HostKeyCallback: ssh.FixedHostKey(hostKey),
		ClientVersion:   "SSH-2.0-Werbot_mcp_" + strings.ReplaceAll(version.Version, "-", "_"),
	}
	cc, chans, reqs, err := ssh.NewClientConn(conn, r.GetSshAddress(), conf)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ssh.NewClient(cc, chans, reqs), nil
}

func timedOut(timeout time.Duration) ExecOutput {
	return ExecOutput{Status: "error", Message: fmt.Sprintf("timed out after %s", timeout)}
}

// gatewayRefusals are the gateway's fixed texts for an exec it refused
// (gateway.CommandErrorText, agentRunText) and the status each one means.
var gatewayRefusals = map[string]string{
	"Access denied":                         "denied",
	"Approval timed out":                    "denied",
	"Agent limit exceeded":                  "error",
	"Agent run is not active":               "error",
	"Gateway is not ready, try again later": "error",
}

// gatewayStatus reads an exec that ended with code: the gateway refuses with
// exit 1 and one CRLF line of fixed text as all output; anything else is the
// command's own outcome ("ok").
func gatewayStatus(output string, code int) (status, message string) {
	line, ok := strings.CutSuffix(output, "\r\n")
	if code != 1 || !ok || strings.Contains(line, "\n") {
		return "ok", ""
	}
	if strings.HasPrefix(line, "Command denied: ") {
		return "denied", line
	}
	if status := gatewayRefusals[line]; status != "" {
		return status, line
	}
	return "ok", ""
}
