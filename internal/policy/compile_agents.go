package policy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/rpcid"
)

// anyAgent is the scope of a policy on every agent.
const anyAgent = "principal is Werbot::Agent, action, resource"

// agentEntities adds the Cedar entities of agents to entities.
func agentEntities(entities cedar.EntityMap, agents []SnapshotAgent) {
	for _, a := range agents {
		uid := agentUID(a.ID)
		entities[uid] = cedar.Entity{UID: uid,
			Attributes: cedar.NewRecord(cedar.RecordMap{"sponsor": userUID(a.SponsorID), "status": cedar.String(a.Status)})}
	}
}

// writeBaseline emits the built-in agent policies of types: connect for an
// exec, commands of exactly the read class, never a dangerous class, a
// terminal, an sftp or remote_forward action, a frozen agent, and no blocked
// run. A database (databaseConnects, queries) opens to agents only with a
// verified agent_readonly credential (db_connect, db_read, db_readonly):
// agentReadonly guards the attribute with has, so a resource entity without
// it never turns the forbid into an evaluation error that cedar skips. Every
// policy but blocked_runs is written even without agents, since an agent may
// act before the snapshot lists it.
func writeBaseline(b *strings.Builder, types []connector.Type, blockedRuns []uuid.UUID) {
	var connect, exec, sftp, rf []string
	for _, t := range types {
		if a, ok := t.ExecAction(); ok {
			exec = append(exec, actionRef(a))
			if c := t.Name + ".connect"; slices.Contains(t.Actions, c) {
				connect = append(connect, actionRef(c))
			}
		}
		if a := t.Name + ".sftp"; slices.Contains(t.Actions, a) {
			sftp = append(sftp, actionRef(a))
		}
		if a := t.Name + ".remote_forward"; slices.Contains(t.Actions, a) {
			rf = append(rf, actionRef(a))
		}
	}
	baseline(b, "connect", "permit", agentActions(connect), `when { context.kind == "exec" }`, len(connect) > 0)
	baseline(b, "read", "permit", agentActions(exec), `when { ["read"].containsAll(context.classes) }`, len(exec) > 0)
	baseline(b, "dangerous", "forbid", anyAgent, `when { context.classes.containsAny(["destructive", "secrets", "egress"]) }`, true)
	baseline(b, "terminal", "forbid", anyAgent, `when { context.kind == "terminal" }`, true)
	baseline(b, "sftp", "forbid", agentActions(sftp), "", len(sftp) > 0)
	baseline(b, "remote_forward", "forbid", agentActions(rf), "", len(rf) > 0)
	db, query := databaseConnects(types), queryActions(types)
	baseline(b, "db_connect", "permit", agentActions(db), `when { context.kind == "stream" && `+agentReadonly+" }", len(db) > 0)
	baseline(b, "db_read", "permit", agentActions(query), `when { ["read"].containsAll(context.classes) }`, len(query) > 0)
	baseline(b, "db_readonly", "forbid", agentActions(slices.Concat(db, query)), "unless { "+agentReadonly+" }", len(db)+len(query) > 0)
	baseline(b, "frozen", "forbid", anyAgent, `when { principal.status == "frozen" }`, true)
	baseline(b, "blocked_runs", "forbid", "principal, action, resource",
		"when { ["+strings.Join(cedarStrings(rpcid.Strings(blockedRuns)), ", ")+"].contains(context.run_id) }", len(blockedRuns) > 0)
}

// agentActions is the scope of a policy on every agent and actions.
func agentActions(actions []string) string {
	return "principal is Werbot::Agent, action in [" + strings.Join(actions, ", ") + "], resource"
}

// actionRef is the Cedar reference of action a.
func actionRef(a string) string { return fmt.Sprintf("Werbot::Action::%q", a) }

// databaseConnects are the references of "<name>.connect" of the types
// recorded as querylog or with a query action: agents connect to them only
// with a verified agent_readonly credential.
func databaseConnects(types []connector.Type) []string {
	var out []string
	for _, t := range types {
		_, query := t.QueryAction()
		if c := t.Name + ".connect"; (query || t.RecordFormat == connector.RecordQuerylog) && slices.Contains(t.Actions, c) {
			out = append(out, actionRef(c))
		}
	}
	return out
}

// baseline writes the agent:baseline:<name> policy, with its condition
// clause (when or unless) unless it is "", when emit is set.
func baseline(b *strings.Builder, name, effect, scope, cond string, emit bool) {
	if !emit {
		return
	}
	fmt.Fprintf(b, "@id(\"agent:baseline:%s\")\n%s (%s)", name, effect, scope)
	if cond != "" {
		b.WriteString("\n" + cond)
	}
	b.WriteString(";\n\n")
}

// agentLockTerms are the scope and condition prefix of a lock on agents;
// ok is false for any other target and for an agent target without an id.
// The SQL function agent_locked (migration 00161) checks the same targets.
func agentLockTerms(l SnapshotLock) (scope, cond string, ok bool) {
	if l.TargetType == "all_agents" {
		return anyAgent, "", true
	}
	if l.TargetID == nil {
		return "", "", false
	}
	switch l.TargetType {
	case "agent":
		return fmt.Sprintf("principal == Werbot::Agent::%q, action, resource", *l.TargetID), "", true
	case "agent_run":
		return "principal, action, resource", fmt.Sprintf("context.run_id == %q && ", *l.TargetID), true
	case "agents_of_user":
		return anyAgent, fmt.Sprintf("principal.sponsor == Werbot::User::%q && ", *l.TargetID), true
	}
	return "", "", false
}

// agentSubject is the agent principal term of s: every agent, or the agents
// sponsored by its users or members of its teams; "" when s names no agents
// (all_users covers people only).
func agentSubject(s Subjects) string {
	if s.AllAgents {
		return "principal is Werbot::Agent"
	}
	terms := slices.DeleteFunc([]string{sponsorIn("Werbot::Team", s.Teams), sponsorIn("Werbot::User", s.Users)},
		func(t string) bool { return t == "" })
	if len(terms) == 0 {
		return ""
	}
	return "(principal is Werbot::Agent && " + orTerms(terms) + ")"
}

// sponsorIn is "principal.sponsor in [<ids>]"; "" without ids.
func sponsorIn(typ string, ids []uuid.UUID) string {
	if len(ids) == 0 {
		return ""
	}
	return "principal.sponsor in [" + strings.Join(entityRefs(typ, ids), ", ") + "]"
}

// ruleSubject is the principal term of a command or path rule: its people
// or its agents; "" when it has neither.
func ruleSubject(s Subjects) string {
	terms := slices.DeleteFunc([]string{subjectCondition(s), agentSubject(s)}, func(t string) bool { return t == "" })
	return orTerms(terms)
}

// orTerms joins terms with ||, in parentheses when there are several.
func orTerms(terms []string) string {
	if len(terms) > 1 {
		return "(" + strings.Join(terms, " || ") + ")"
	}
	return strings.Join(terms, "")
}

// writeAgentPermit emits the permit of an allow or approval command rule for
// its agents: commands whose classes are all among the rule's. An approval
// rule's permit (":agent") lets the approval lift the agent pass too.
func writeAgentPermit(b *strings.Builder, r SnapshotCommandRule, actions []string) {
	t := target(agentSubject(r.Subjects), r.ResourceIDs)
	if t == "" || len(actions) == 0 {
		return
	}
	id := "command:" + r.ID.String()
	if r.Effect == "approval" {
		id += ":agent"
	}
	fmt.Fprintf(b, "@id(%q)\npermit (principal is Werbot::Agent, action in [%s], resource)\nwhen { %s && [%s].containsAll(context.classes) };\n\n",
		id, strings.Join(actions, ", "), t, strings.Join(cedarStrings(r.Classes), ", "))
}
