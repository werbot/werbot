// Package dbproxy is the shared hot path of the database connectors: the
// query Guard (Check, then Log into the session's querylog), the querylog
// Entry, the gateway-wide CancelRegistry and the TLS settings to a database.
//
// Guard here judges and logs database operations; gateway.Guard is unrelated:
// it runs the relay goroutines of one session.
package dbproxy

// Op is one database operation a connector asks the Guard about. Params are
// the displayed Bind parameters of an execute (as LogParams keeps them);
// SHA256 is the SHA-256 of Text, zero when the Guard computes it.
type Op struct {
	Kind            string // query, parse, execute, command, request, tool_call, cancel
	Text, Statement string // SQL or command text; prepared statement name
	DB              string
	Params          []string
	SHA256          [32]byte
}

// Verdict is the Guard's answer to an Op. A deny that an approval would lift
// carries Obligations ["approval"]; Approve then asks for it. Message is the
// text of a deny. Callers must not modify the slices: they are shared.
type Verdict struct {
	Allow                                   bool
	Obligations                             []string
	Message                                 string
	Classes, Tables, WriteTables, PolicyIDs []string
	ApprovalID, ApprovalURL                 string
}

// Bounds of the logged text and parameters of one querylog line.
const (
	MaxLogText   = 64 << 10
	MaxLogParams = 64 << 10
)

// OverheadBuckets are the buckets of werbot_gateway_query_overhead_seconds.
var OverheadBuckets = []float64{0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01}
