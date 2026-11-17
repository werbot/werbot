// Package hashchain is the recording chunk hash chain shared by gateway and server.
package hashchain

import "crypto/sha256"

// Genesis returns the chain start: 32 zero bytes (a new slice).
func Genesis() []byte { return make([]byte, sha256.Size) }

// Next returns SHA-256(prev || gz).
func Next(prev, gz []byte) []byte {
	h := sha256.New()
	h.Write(prev)
	h.Write(gz)
	return h.Sum(nil)
}
