package policy

import (
	"fmt"
	"net/netip"

	"github.com/oschwald/maxminddb-golang/v2"
)

// GeoIP resolves the country of an address.
type GeoIP interface {
	// Country returns the ISO 3166-1 alpha-2 code of ip, or "" when unknown.
	Country(ip netip.Addr) string
}

type mmdb struct{ r *maxminddb.Reader }

// OpenGeoIP opens a DB-IP Lite country database held in memory; it reads no files.
func OpenGeoIP(data []byte) (GeoIP, error) {
	r, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, fmt.Errorf("policy: open geoip: %w", err)
	}
	return mmdb{r}, nil
}

func (m mmdb) Country(ip netip.Addr) string {
	var code string
	if err := m.r.Lookup(ip.Unmap()).DecodePath(&code, "country", "iso_code"); err != nil {
		return ""
	}
	return code
}
