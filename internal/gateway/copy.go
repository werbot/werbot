package gateway

import (
	"errors"
	"io"
	"net"
	"sync"
)

var bufPool = sync.Pool{New: func() any { b := make([]byte, 32<<10); return &b }}

// CopyBuffer is io.Copy through a pooled 32 KiB buffer. EOF is a nil error.
func CopyBuffer(dst io.Writer, src io.Reader) (int64, error) {
	bp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bp)
	return io.CopyBuffer(dst, src, *bp)
}

// IsClosed reports an end caused by a closed connection, pipe or channel
// (an SSH channel write after the remote close is io.EOF).
func IsClosed(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF)
}
