package policy

import (
	"fmt"
	"strings"

	"github.com/werbot/werbot/internal/connector"
)

const schemaHead = `namespace Werbot {
  entity Team;
  entity User in [Team];
  entity Resource { name: String, type: String } tags String;
  type Context = { ip: ipaddr, country: String, weekday: Long, hour: Long, ip_blocked: Bool };
`

// Schema returns the Cedar schema with one action per action of every registered resource type.
func Schema() string {
	var b strings.Builder
	b.WriteString(schemaHead)
	for _, t := range connector.Types() {
		for _, a := range t.Actions {
			fmt.Fprintf(&b, "  action %q appliesTo { principal: [User], resource: [Resource], context: Context };\n", a)
		}
	}
	b.WriteString("}\n")
	return b.String()
}
