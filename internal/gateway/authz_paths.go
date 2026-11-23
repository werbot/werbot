package gateway

import "github.com/werbot/werbot/internal/platform/shellclass"

// Analyze classifies command like the server's TestCommand, with the snapshot
// class rules and the host of the target resource, and lists the paths it
// touches from cwd ("~", "~/…" or absolute). No snapshot: opaque and unresolved.
func (p *Policy) Analyze(command, resourceHost, cwd string) shellclass.Analysis {
	if p == nil {
		return shellclass.Analysis{Classes: []shellclass.Class{shellclass.Opaque}, Unresolved: true}
	}
	return p.classifier.Analyze(command, resourceHost, cwd)
}

// PathRules are the snapshot path rules, by ID.
func (p *Policy) PathRules() []shellclass.PathRule {
	if p == nil {
		return nil
	}
	return p.pathRules
}

// pathSet is the compiled path rules of p, with "~" globs resolved from home
// when it is set.
func (p *Policy) pathSet(home string) shellclass.PathSet {
	switch {
	case p == nil:
		return shellclass.PathSet{}
	case home == "":
		return p.paths
	}
	return shellclass.CompilePaths(shellclass.ExpandHome(p.pathRules, home))
}

// MaskOutput is whose exec output the gateway masks: "agents", "all" or
// "none"; an older server sends none, which means "agents".
func (p *Policy) MaskOutput() string {
	if p == nil || p.maskOutput == "" {
		return "agents"
	}
	return p.maskOutput
}
