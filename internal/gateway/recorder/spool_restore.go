package recorder

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// restore loads one session directory. Chunk files the state does not count yet
// (a crash between the two writes of Append) are added to it.
func (s *Spool) restore(id uuid.UUID) error {
	dir := id.String()
	files, err := fs.ReadDir(s.root.FS(), dir)
	if err != nil {
		return err
	}
	ss := &session{st: state{Closed: true}}
	sizes := map[int]int64{}
	for _, f := range files {
		name := path.Join(dir, f.Name())
		switch n := f.Name(); {
		case strings.HasSuffix(n, tmpExt):
			_ = s.root.Remove(name) // torn write
		case n == stateFile:
			if err := s.readJSON(name, &ss.st); err != nil {
				return err
			}
			ss.st.Closed, ss.stored = true, true
		case n == endFile:
			var e struct{ Reason string }
			if err := s.readJSON(name, &e); err != nil {
				return err
			}
			s.ends[id] = e.Reason
		case strings.HasSuffix(n, chunkExt):
			seq, err := strconv.Atoi(strings.TrimSuffix(n, chunkExt))
			info, ierr := f.Info()
			if err != nil || ierr != nil || info.Size() <= hashLen {
				return fmt.Errorf("bad chunk file %s", n)
			}
			sizes[seq] = info.Size() - hashLen
		}
	}
	for seq := ss.st.Chunks; sizes[seq] > 0; seq++ {
		hash, err := s.readHash(id, seq)
		if err != nil {
			return err
		}
		ss.st.Chunks, ss.st.Bytes, ss.st.FinalHash, ss.stored = seq+1, ss.st.Bytes+sizes[seq], hex.EncodeToString(hash), true
	}
	// Uploads run in order and remove their file, so the unsent chunks are the run
	// that ends at the last one; files below a gap were sent but not removed.
	ss.sent = ss.st.Chunks
	for sizes[ss.sent-1] > 0 {
		ss.sent--
	}
	for seq, n := range sizes {
		if seq < ss.sent {
			_ = s.root.Remove(chunkName(id, seq))
		} else {
			ss.pending += n
		}
	}
	s.sessions[id] = ss
	s.total.Add(ss.pending)
	return nil
}
