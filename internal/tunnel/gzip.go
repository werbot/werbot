package tunnel

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"io"
	"slices"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/singleflight"
)

// gzipCacheSize is how many compressed Connect messages gzipCache keeps.
// ponytail: FIFO by count; a "*" refresh of more pools than this at once
// gzips an evicted snapshot again (still correct), size it by bytes if that shows up.
const gzipCacheSize = 16

// gzipCache gzips each distinct Connect message once: the streams of a pool
// send the same snapshot, so every stream after the first gets the cached
// bytes. The output is what the default gzip compressor writes for the same
// input (same level, no header fields), so gateways decode it as before.
type gzipCache struct {
	group   singleflight.Group
	encodes atomic.Int64 // gzip runs; tests read it

	mu    sync.Mutex
	out   map[[sha256.Size]byte][]byte // input hash -> gzip of the input
	order [][sha256.Size]byte          // keys, oldest first
}

// compress returns the gzip of in; concurrent calls with equal input share one run.
func (c *gzipCache) compress(in []byte) ([]byte, error) {
	key := sha256.Sum256(in)
	if out, ok := c.get(key); ok {
		return out, nil
	}
	v, err, _ := c.group.Do(string(key[:]), func() (any, error) {
		if out, ok := c.get(key); ok {
			return out, nil
		}
		c.encodes.Add(1)
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(in); err != nil {
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		c.put(key, buf.Bytes())
		return buf.Bytes(), nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil //nolint:forcetypeassert // the func above returns []byte
}

func (c *gzipCache) get(key [sha256.Size]byte) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out, ok := c.out[key]
	return out, ok
}

func (c *gzipCache) put(key [sha256.Size]byte, out []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.out == nil {
		c.out = map[[sha256.Size]byte][]byte{}
	}
	if len(c.order) == gzipCacheSize {
		delete(c.out, c.order[0])
		c.order = slices.Delete(c.order, 0, 1)
	}
	c.out[key] = out
	c.order = append(c.order, key)
}

// gzipOnce is a connect.Compressor that collects one message and writes its
// gzip from the cache on Close.
type gzipOnce struct {
	c   *gzipCache
	dst io.Writer
	in  []byte
}

func (z *gzipOnce) Write(p []byte) (int, error) {
	z.in = append(z.in, p...)
	return len(p), nil
}

func (z *gzipOnce) Close() error {
	out, err := z.c.compress(z.in)
	if err != nil {
		return err
	}
	_, err = z.dst.Write(out)
	return err
}

func (z *gzipOnce) Reset(dst io.Writer) {
	z.dst, z.in = dst, z.in[:0]
}
