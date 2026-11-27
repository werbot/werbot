// Package dbproxy is the shared hot path of the database connectors: the
// query Guard (Check, then Log into the session's querylog), the querylog
// Entry, the gateway-wide CancelRegistry and the TLS settings to a database.
//
// Guard here judges and logs database operations; gateway.Guard is unrelated:
// it runs the relay goroutines of one session.
package dbproxy

// Op is one database operation a connector asks the Guard about.
type Op struct {
	Kind            string // query, parse, execute, command, request, tool_call, cancel
	Text, Statement string // SQL or command text; prepared statement name
	DB              string
	Params          []string
}

// Verdict is the Guard's answer to an Op.
type Verdict struct {
	Allow                      bool
	Classes, Tables, PolicyIDs []string
}

// Bounds of the logged text and parameters of one querylog line.
const (
	MaxLogText   = 64 << 10
	MaxLogParams = 64 << 10
)

// OverheadBuckets are the buckets of werbot_gateway_query_overhead_seconds.
var OverheadBuckets = []float64{0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01}
