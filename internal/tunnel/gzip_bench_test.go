package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"google.golang.org/protobuf/proto"
)

// BenchmarkFanoutEncode sends one ~1 MB snapshot to the 100 streams of a pool:
// "default" gzips it per stream as connect's default compressor does, "once"
// goes through gzipCache. One op is the whole fan-out.
func BenchmarkFanoutEncode(b *testing.B) {
	snap := bigSnapshot("bench")
	seed := sha256.Sum256(nil)
	for _, r := range snap.GetResources() { // ~1 MB of hex that gzips about 2:1, as the L2 snapshot
		for range 7 {
			seed = sha256.Sum256(seed[:])
			r.ConfigJson = hex.AppendEncode(r.ConfigJson, seed[:])
		}
	}
	b.Logf("snapshot %d bytes", proto.Size(snap))
	const streams = 100
	marshal := func(b *testing.B) []byte {
		in, err := proto.Marshal(snap)
		if err != nil {
			b.Fatal(err)
		}
		return in
	}
	b.Run("default", func(b *testing.B) {
		for b.Loop() {
			for range streams {
				stdGzip(b, marshal(b))
			}
		}
	})
	b.Run("once", func(b *testing.B) {
		for b.Loop() {
			c := &gzipCache{} // a new snapshot version per fan-out
			for range streams {
				if _, err := gzipOnceRun(c, marshal(b)); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
