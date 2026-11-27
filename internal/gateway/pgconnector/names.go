package pgconnector

// store keeps v under name in m (a statement or a portal) within the limits
// of a connection: maxNames names and maxRetained bytes in all. A name that
// is there already is replaced.
// ponytail: a named portal the database drops at transaction end without a
// Close still counts; drop them on ReadyForQuery 'I' if clients hit a limit.
func store[V any](c *clientSide, m map[string]V, name string, v V, size func(string, V) int) error {
	old, ok := m[name]
	kept := c.kept + size(name, v)
	if ok {
		kept -= size(name, old)
	} else if len(c.stmts)+len(c.portals) >= maxNames {
		return errTooManyNames
	}
	if kept > maxRetained {
		return errRetained
	}
	m[name], c.kept = v, kept
	return nil
}

// drop forgets a closed statement or portal and the bytes it kept.
func drop[V any](c *clientSide, m map[string]V, name string, size func(string, V) int) {
	if old, ok := m[name]; ok {
		c.kept -= size(name, old)
		delete(m, name)
	}
}

// stmtSize is what a prepared statement keeps: its name and logged text.
func stmtSize(name, text string) int { return len(name) + len(text) }

// portalSize is what a portal keeps of its own: its name and logged
// parameters (its text is the statement's).
func portalSize(name string, p portal) int {
	n := len(name)
	for _, s := range p.params {
		n += len(s)
	}
	return n
}
