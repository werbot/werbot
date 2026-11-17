package recorder

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	ss := &session{st: state{Closed: true}, sent: -1}
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
			ss.pending += sizes[seq]
			if ss.sent < 0 || seq < ss.sent {
				ss.sent = seq
			}
		}
	}
	for seq := ss.st.Chunks; sizes[seq] > 0; seq++ {
		hash, err := s.readHash(id, seq)
		if err != nil {
			return err
		}
		ss.st.Chunks, ss.st.Bytes, ss.st.FinalHash, ss.stored = seq+1, ss.st.Bytes+sizes[seq], hex.EncodeToString(hash), true
	}
	if ss.sent < 0 {
		ss.sent = ss.st.Chunks
	}
	s.sessions[id] = ss
	s.total.Add(ss.pending)
	return nil
}

func (s *Spool) readJSON(name string, v any) error {
	b, err := s.root.ReadFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func (s *Spool) readHash(id uuid.UUID, seq int) ([]byte, error) {
	f, err := s.root.Open(chunkName(id, seq))
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read only
	hash := make([]byte, hashLen)
	_, err = io.ReadFull(f, hash)
	return hash, err
}
