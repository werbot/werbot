package gateway

import "github.com/werbot/werbot/internal/gateway/mask"

// maskEnabled reports whether exec output is masked under the org setting
// mask_output for a session of an agent or a human: "all" masks everyone,
// "agents" (and "", the default) only agents, "none" nobody. An unknown mode
// masks, failing closed.
func maskEnabled(mode string, agent bool) bool {
	switch mode {
	case "none":
		return false
	case "agents", "":
		return agent
	}
	return true
}

// maskExec masks the session credential and credential-shaped text in the
// output of an exec when the org masks it; a terminal is never masked. The
// returned func writes the held tails and must run before the exit status is
// sent. The recording keeps the original: connectors record apart from
// Exec.Stdout and Exec.Stderr.
func maskExec(s *Session, pol *Policy) (flush func()) {
	if s.Exec == nil || !maskEnabled(pol.MaskOutput(), s.User.Agent != nil) {
		return func() {}
	}
	secrets := [][]byte{s.Secret.Value}
	out, errOut := mask.NewWriter(s.Exec.Stdout, secrets, true), mask.NewWriter(s.Exec.Stderr, secrets, true)
	s.Exec.Stdout, s.Exec.Stderr = out, errOut
	return func() {
		_ = out.Close() // the client may be gone
		_ = errOut.Close()
	}
}
