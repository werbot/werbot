package policy

import (
	"slices"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// agentReadonly is the Cedar condition of a resource with a verified
// agent_readonly credential; has makes a missing attribute false, not an
// evaluation error.
const agentReadonly = "resource has agent_readonly && resource.agent_readonly"

// queryActions are the references of the query actions of types, by type name.
func queryActions(types []connector.Type) []string {
	var out []string
	for _, t := range types {
		if a, ok := t.QueryAction(); ok {
			out = append(out, actionRef(a))
		}
	}
	return out
}

// operationActions are the actions that run one operation, which command
// rules cover: the exec actions, then the query actions of the registered types.
func operationActions() []string {
	return slices.Concat(execActions(), queryActions(connector.Types()))
}

// operationAction is the action of t that runs one operation and the kind of
// its session: exec, else query on a stream.
func operationAction(t connector.Type) (string, connector.Kind, bool) {
	if a, ok := t.ExecAction(); ok {
		return a, connector.KindExec, true
	}
	a, ok := t.QueryAction()
	return a, connector.KindStream, ok
}

// testQuery classifies the SQL text command with the dialect of the type and
// the secret tables of the resource, and decides it in the resource database:
// connect first, then action. A query has no paths and no cwd.
func (c check) testQuery(action, command string) CommandResult {
	r := sqlclass.Classify(sqlclass.Dialect(c.typ.Name), command, sqlclass.Options{SecretTables: c.res.SecretTables})
	out := CommandResult{Classes: make([]string, len(r.Classes))}
	for i, cl := range r.Classes {
		out.Classes[i] = string(cl)
	}
	c.req.Kind = string(connector.KindStream)
	if out.Decision = c.ps.Authorize(c.req); !out.Allow {
		return out
	}
	c.req.Action, c.req.Command, c.req.Classes = action, command, out.Classes
	c.req.DBName, c.req.Tables, c.req.WriteTables = c.res.Database, r.Tables, r.WriteTables
	out.Decision = c.ps.Authorize(c.req)
	return out
}
