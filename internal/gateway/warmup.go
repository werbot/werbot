package gateway

// Warmer is an optional Connector interface of a type whose sessions need a
// costly process-wide preparation (the SQL parser of a database type). The
// gateway calls Warmup in the background when a snapshot holds a resource of
// the type, so gateways without such resources never pay for it. Warmup is
// idempotent and logs its own failure.
type Warmer interface {
	Warmup()
}

// startWarmups starts the Warmup of each resource type of p that has one.
func startWarmups(p *Policy) {
	seen := map[string]bool{}
	for _, r := range p.resources {
		if seen[r.Type] {
			continue
		}
		seen[r.Type] = true
		c, _ := LookupConnector(r.Type)
		if w, ok := c.(Warmer); ok {
			go w.Warmup()
		}
	}
}
