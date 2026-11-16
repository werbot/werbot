package policy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// MaxMind DB 2.0 data section encoding: the type sits in the top 3 bits of the
// control byte, the size (< 29 here) in the low 5.
const (
	mmdbTypeString = 2
	mmdbTypeUint16 = 5
	mmdbTypeUint32 = 6
	mmdbTypeMap    = 7
)

func mmdbField(typ byte, payload []byte) []byte {
	return append([]byte{typ<<5 | byte(len(payload))}, payload...) //nolint:gosec // test sizes stay below 29
}

func mmdbString(s string) []byte { return mmdbField(mmdbTypeString, []byte(s)) }

// mmdbUint encodes v as uint16 or uint32 in the fewest big-endian bytes.
func mmdbUint(typ byte, v uint32) []byte {
	return mmdbField(typ, bytes.TrimLeft(binary.BigEndian.AppendUint32(nil, v), "\x00"))
}

func mmdbMap(pairs ...[]byte) []byte {
	out := []byte{mmdbTypeMap<<5 | byte(len(pairs)/2)} //nolint:gosec // test maps have under 29 entries
	for _, p := range pairs {
		out = append(out, p...)
	}
	return out
}

// testMMDB builds an IPv4 DB-IP Lite-style database that maps prefix to
// country; every other address has no record. 1 <= prefix.Bits() <= 32.
func testMMDB(t testing.TB, prefix netip.Prefix, country string) []byte {
	t.Helper()
	bits := prefix.Bits()
	require.True(t, prefix.Addr().Is4() && bits >= 1 && bits <= 32, "testMMDB: IPv4 prefix with 1..32 bits")
	addr := prefix.Masked().Addr().As4()
	nodes := uint32(bits) //nolint:gosec // 1..32, checked above
	// record encodes a 24-bit search tree record.
	record := func(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v)[1:] }

	var db []byte
	for i := range bits {
		bit := addr[i/8] >> (7 - i%8) & 1
		next := uint32(i) + 1 // the next node on the path
		if i == bits-1 {
			next = nodes + 16 // data at offset 0
		}
		recs := [2][]byte{record(nodes), record(nodes)} // off the path: no data
		recs[bit] = record(next)
		db = append(db, recs[0]...)
		db = append(db, recs[1]...)
	}
	db = append(db, make([]byte, 16)...)
	db = append(db, mmdbMap(mmdbString("country"), mmdbMap(mmdbString("iso_code"), mmdbString(country)))...)
	db = append(db, "\xAB\xCD\xEFMaxMind.com"...)
	return append(db, mmdbMap(
		mmdbString("binary_format_major_version"), mmdbUint(mmdbTypeUint16, 2),
		mmdbString("binary_format_minor_version"), mmdbUint(mmdbTypeUint16, 0),
		mmdbString("database_type"), mmdbString("DBIP-Country-Lite"),
		mmdbString("description"), mmdbMap(mmdbString("en"), mmdbString("test")),
		mmdbString("ip_version"), mmdbUint(mmdbTypeUint16, 4),
		mmdbString("node_count"), mmdbUint(mmdbTypeUint32, nodes),
		mmdbString("record_size"), mmdbUint(mmdbTypeUint16, 24),
	)...)
}

func TestOpenGeoIP(t *testing.T) {
	g, err := OpenGeoIP(testMMDB(t, netip.MustParsePrefix("10.0.0.0/8"), "DE"))
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
	de := testMMDB(t, netip.MustParsePrefix("10.0.0.0/8"), "DE")
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
	de := testMMDB(t, netip.MustParsePrefix("10.0.0.0/8"), "DE")
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
