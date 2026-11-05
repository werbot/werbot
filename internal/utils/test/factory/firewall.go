package factory

import (
	"github.com/stretchr/testify/require"
)

// FirewallListParams is the set of optional and required values for a firewall list.
type FirewallListParams struct {
	ID, Name string
	Active   *bool
	Path     int16
}

// FirewallList inserts one firewall list row and returns its ID.
func (b *Builder) FirewallList(l FirewallListParams) string {
	b.t.Helper()

	l.ID = uuidOrNew(l.ID)
	l.Name = stringOr(l.Name, "firewall-list")
	// Firewall lists are created active by default; path defaults to 0.
	active := boolOr(l.Active, true)

	_, err := b.db.Exec(`
		INSERT INTO "firewall_list" ("id", "name", "active", "path")
		VALUES ($1, $2, $3, $4)
	`, l.ID, l.Name, active, l.Path)
	require.NoError(b.t, err, "firewall list: insert")

	return l.ID
}

// FirewallCountry inserts one firewall country row for the list.
func (b *Builder) FirewallCountry(listID, code string) {
	b.t.Helper()

	_, err := b.db.Exec(`
		INSERT INTO "firewall_country" ("firewall_list_id", "country_code")
		VALUES ($1, $2)
	`, listID, code)
	require.NoError(b.t, err, "firewall country: insert")
}

// FirewallNetwork inserts one firewall network row for the list.
func (b *Builder) FirewallNetwork(listID, network string) {
	b.t.Helper()

	_, err := b.db.Exec(`
		INSERT INTO "firewall_network" ("firewall_list_id", "network")
		VALUES ($1, $2)
	`, listID, network)
	require.NoError(b.t, err, "firewall network: insert")
}
