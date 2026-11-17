package recorder

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"
)

// Shared across all recorders (10 §10.2 p. 2): a chunk flush reuses the flate
// state and both buffers instead of allocating hundreds of KiB per chunk.
var (
	gzipWriters = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}
	rawBuffers  = sync.Pool{New: func() any { return bytes.NewBuffer(make([]byte, 0, MaxChunkRaw)) }}
	gzBuffers   = sync.Pool{New: func() any { return new(bytes.Buffer) }}
)
