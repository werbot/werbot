// Package geoiptest builds tiny MaxMind DB files for tests, in process and in
// the cluster harness.
package geoiptest

import (
	"bytes"
	"encoding/binary"
	"net/netip"
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

// MMDB builds an IPv4 DB-IP Lite-style database that maps prefix to
// country; every other address has no record. 1 <= prefix.Bits() <= 32.
func MMDB(prefix netip.Prefix, country string) []byte {
	bits := prefix.Bits()
	if !prefix.Addr().Is4() || bits < 1 || bits > 32 {
		panic("geoiptest: IPv4 prefix with 1..32 bits, got " + prefix.String())
	}
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
