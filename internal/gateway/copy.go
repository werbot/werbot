package gateway

import (
	"io"
	"sync"
)

var bufPool = sync.Pool{New: func() any { b := make([]byte, 32<<10); return &b }}

// CopyBuffer is io.Copy through a pooled 32 KiB buffer. EOF is a nil error.
func CopyBuffer(dst io.Writer, src io.Reader) (int64, error) {
	bp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bp)
	return io.CopyBuffer(dst, src, *bp)
}
