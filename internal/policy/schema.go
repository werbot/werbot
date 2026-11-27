package policy

import (
	"fmt"
	"strings"

	"github.com/werbot/werbot/internal/connector"
)

const schemaHead = `namespace Werbot {
  entity Team;
  entity User in [Team];
  entity Agent { sponsor: User, status: String };
  entity Resource { name: String, type: String, agent_readonly: Bool } tags String;
  type Context = { ip: ipaddr, country: String, weekday: Long, hour: Long, ip_blocked: Bool, port: Long, now: Long, cli_session_id: String, command: String, classes: Set<String>, approval_granted: Bool, paths: Set<String>, path_hits: Set<String>, path_outside: Set<String>, console: Bool, run_id: String, kind: String, mfa_age: Long, mfa_method: String, db_name: String, tables: Set<String>, write_tables: Set<String> };
`

// Schema returns the Cedar schema with one action per action of every
// registered resource type; people and agents are principals of each.
func Schema() string {
	var b strings.Builder
	b.WriteString(schemaHead)
	for _, t := range connector.Types() {
		for _, a := range t.Actions {
			fmt.Fprintf(&b, "  action %q appliesTo { principal: [User, Agent], resource: [Resource], context: Context };\n", a)
		}
	}
	b.WriteString("}\n")
	return b.String()
}
