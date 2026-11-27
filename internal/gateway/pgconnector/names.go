package pgconnector

// named is a prepared statement or a portal: its logged text (a portal's is
// its statement's), its statement and logged parameters (a portal's), and
// the bytes it keeps of its own.
type named struct {
	stmt, text string
	params     []string
	size       int
}

// store keeps v under name in m (c.stmts or c.portals) within the limits of
// a connection: maxNames names and maxRetained bytes in all. v.size is what
// v keeps besides its name. A name that is there already is replaced.
// ponytail: a named portal the database drops at transaction end without a
// Close still counts; drop them on ReadyForQuery 'I' if clients hit a limit.
func (c *clientSide) store(m map[string]named, name string, v named) error {
	v.size += len(name)
	old, ok := m[name]
	if !ok && len(c.stmts)+len(c.portals) >= maxNames {
		return errTooManyNames
	}
	kept := c.kept + v.size - old.size
	if kept > maxRetained {
		return errRetained
	}
	m[name], c.kept = v, kept
	return nil
}

// drop forgets a closed statement or portal and the bytes it kept.
func (c *clientSide) drop(m map[string]named, name string) {
	c.kept -= m[name].size
	delete(m, name)
}
