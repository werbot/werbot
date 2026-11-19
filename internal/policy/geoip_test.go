package policy

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/policy/geoiptest"
)

func TestOpenGeoIP(t *testing.T) {
	g, err := OpenGeoIP(geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE"))
	require.NoError(t, err)
	for ip, want := range map[string]string{
		"10.1.2.3":        "DE",
		"::ffff:10.1.2.3": "DE",
		"192.0.2.1":       "",
		"2001:db8::1":     "",
	} {
		assert.Equal(t, want, g.Country(netip.MustParseAddr(ip)), ip)
	}
}

func TestGeoIPCountryFields(t *testing.T) {
	p, ip := netip.MustParsePrefix("10.0.0.0/8"), netip.MustParseAddr("10.1.2.3")
	for name, c := range map[string]struct {
		data []byte
		want string
	}{
		"country.iso_code":   {geoiptest.MMDB(p, "DE"), "DE"},
		"country_code":       {geoiptest.CountryCodeMMDB(p, "FR"), "FR"},
		"lower case":         {geoiptest.CountryCodeMMDB(p, "fr"), "FR"},
		"alpha-3":            {geoiptest.MMDB(p, "DEU"), ""},
		"not letters":        {geoiptest.CountryCodeMMDB(p, "1F"), ""},
		"empty country_code": {geoiptest.CountryCodeMMDB(p, ""), ""},
	} {
		g, err := OpenGeoIP(c.data)
		require.NoError(t, err, name)
		assert.Equal(t, c.want, g.Country(ip), name)
	}
}

func TestOpenGeoIPErrors(t *testing.T) {
	random := make([]byte, 4096)
	_, _ = rand.Read(random)
	for name, data := range map[string][]byte{"nil": nil, "text": []byte("not a mmdb"), "random": random} {
		g, err := OpenGeoIP(data)
		assert.Error(t, err, name)
		assert.Nil(t, g, name)
	}
}

func TestReloadGeoIPFromDB(t *testing.T) {
	// A replica that never ran geoip.refresh: the row is written by SQL and the
	// service is created after it.
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	ip := netip.MustParseAddr("10.1.2.3")

	svc := New(Deps{Pool: pool, Logger: slog.New(slog.DiscardHandler)})
	assert.Empty(t, svc.Country(ip))
	_, err := pool.Exec(ctx, "INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-10-01', $1)", de)
	require.NoError(t, err)
	require.NoError(t, svc.ReloadGeoIP(ctx))
	assert.Equal(t, "DE", svc.Country(ip))

	_, err = pool.Exec(ctx, `UPDATE geoip_db SET data = '\x00', month = '2026-11-01'`)
	require.NoError(t, err)
	assert.Error(t, svc.ReloadGeoIP(ctx))
	assert.Equal(t, "DE", svc.Country(ip), "a broken database keeps the previous one")

	_, err = pool.Exec(ctx, "DELETE FROM geoip_db")
	require.NoError(t, err)
	require.NoError(t, svc.ReloadGeoIP(ctx))
	assert.Equal(t, "DE", svc.Country(ip), "no row keeps the current database")
}

func TestReloadGeoIPSkipsUnchangedMonth(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	ip := netip.MustParseAddr("10.1.2.3")
	_, err := pool.Exec(ctx, "INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-10-01', $1)", de)
	require.NoError(t, err)
	svc := New(Deps{Pool: pool, Logger: slog.New(slog.DiscardHandler)})
	require.NoError(t, svc.ReloadGeoIP(ctx))

	_, err = pool.Exec(ctx, `UPDATE geoip_db SET data = '\x00'`)
	require.NoError(t, err)
	require.NoError(t, svc.ReloadGeoIP(ctx), "same month: blob not read")
	assert.Equal(t, "DE", svc.Country(ip))

	_, err = pool.Exec(ctx, `UPDATE geoip_db SET month = '2026-09-01'`)
	require.NoError(t, err)
	assert.Error(t, svc.ReloadGeoIP(ctx), "another month: blob read and rejected")
	assert.Equal(t, "DE", svc.Country(ip))
}

func TestReloadGeoIPEmptyBlobIsAnError(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	ip := netip.MustParseAddr("10.1.2.3")
	_, err := pool.Exec(ctx, "INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-10-01', $1)", de)
	require.NoError(t, err)
	svc := New(Deps{Pool: pool, Logger: slog.New(slog.DiscardHandler)})
	require.NoError(t, svc.ReloadGeoIP(ctx))

	// A new month with an empty blob must fail, not look like "month unchanged".
	_, err = pool.Exec(ctx, `UPDATE geoip_db SET data = '\x', month = '2026-11-01'`)
	require.NoError(t, err)
	assert.Error(t, svc.ReloadGeoIP(ctx))
	assert.Equal(t, "DE", svc.Country(ip))
}
