package dbproxy

import (
	"encoding/json"
	"time"

	"github.com/werbot/werbot/internal/gateway"
)

// Truncated ends a cut text and replaces the parameters past the bound.
const Truncated = "…[truncated]"

// Entry is one querylog line: an operation that reached the database.
type Entry struct {
	T                         time.Duration // since the session start
	Kind, Text, Statement, DB string
	Params                    []string
	Classes, Tables           []string
	Decision                  string
	PolicyIDs                 []string
	Rows, Bytes, DurationUS   int64
	Error                     string
}

// line is the querylog JSON of an Entry, keys in format order.
type line struct {
	T          float64  `json:"t"`
	Kind       string   `json:"kind"`
	SQL        string   `json:"sql,omitempty"`
	Command    string   `json:"command,omitempty"`
	DB         string   `json:"db,omitempty"`
	Params     []string `json:"params,omitempty"`
	Statement  string   `json:"statement,omitempty"`
	Classes    []string `json:"classes,omitempty"`
	Tables     []string `json:"tables,omitempty"`
	Decision   string   `json:"decision,omitempty"`
	PolicyIDs  []string `json:"policy_ids,omitempty"`
	Rows       int64    `json:"rows,omitempty"`
	Bytes      int64    `json:"bytes,omitempty"`
	DurationUS int64    `json:"duration_us,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// MarshalJSON writes the querylog line: t in seconds to the microsecond, the
// text under "command" for command, request and tool_call, else under "sql";
// empty fields other than t and kind are left out.
func (e Entry) MarshalJSON() ([]byte, error) {
	l := line{
		T: float64(e.T.Round(time.Microsecond).Microseconds()) / 1e6, Kind: e.Kind, DB: e.DB, Params: e.Params,
		Statement: e.Statement, Classes: e.Classes, Tables: e.Tables, Decision: e.Decision, PolicyIDs: e.PolicyIDs,
		Rows: e.Rows, Bytes: e.Bytes, DurationUS: e.DurationUS, Error: e.Error,
	}
	switch e.Kind {
	case "command", "request", "tool_call":
		l.Command = e.Text
	default:
		l.SQL = e.Text
	}
	return json.Marshal(l)
}

// LogText is s cut to MaxLogText bytes on a UTF-8 boundary plus "…[truncated]"
// when it is longer; a shorter s is returned as it is.
func LogText(s string) string {
	if len(s) <= MaxLogText {
		return s
	}
	return gateway.TruncateUTF8(s, MaxLogText) + Truncated
}

// LogParams is ps while their total size fits MaxLogParams; otherwise the
// parameters that fit, then "…[truncated]" in place of the rest.
func LogParams(ps []string) []string {
	n := 0
	for i, p := range ps {
		if n += len(p); n > MaxLogParams {
			return append(ps[:i:i], Truncated)
		}
	}
	return ps
}
