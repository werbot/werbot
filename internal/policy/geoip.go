package policy

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoIP resolves the country of an address.
type GeoIP interface {
	// Country returns the ISO 3166-1 alpha-2 code of ip, or "" when unknown.
	Country(ip netip.Addr) string
}

type mmdb struct{ r *maxminddb.Reader }

// OpenGeoIP opens a country database held in memory; it reads no files.
func OpenGeoIP(data []byte) (GeoIP, error) {
	r, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("policy: open geoip: %w", err)
	}
	if err := r.Verify(); err != nil {
		return nil, fmt.Errorf("policy: open geoip: %w", err)
	}
	return mmdb{r}, nil
}

// Country reads country.iso_code (DB-IP, MaxMind), else the top-level
// country_code (IPinfo Lite); only an alpha-2 code counts.
func (m mmdb) Country(ip netip.Addr) string {
	res := m.r.Lookup(ip.Unmap())
	var code string
	if res.DecodePath(&code, "country", "iso_code") != nil || code == "" {
		code = ""
		_ = res.DecodePath(&code, "country_code")
	}
	code = strings.ToUpper(code)
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	return code
}
