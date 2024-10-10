package logtunnel

import (
	"encoding/binary"
	"io"
	"syscall"
	"time"
)

func writeHeader(fd io.Writer, length int) {
	t := time.Now()
	tv := syscall.NsecToTimeval(t.UnixNano())

	if err := binary.Write(fd, binary.LittleEndian, int32(tv.Sec)); err != nil {
		log.Error(err).Msg("Failed to write log header")
	}
	if err := binary.Write(fd, binary.LittleEndian, tv.Usec); err != nil {
		log.Error(err).Msg("Failed to write log header")
	}
	if err := binary.Write(fd, binary.LittleEndian, int32(length)); err != nil {
		log.Error(err).Msg("Failed to write log header")
	}
}
