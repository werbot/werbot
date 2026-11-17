package recorder

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
)

func (s *Spool) writeState(id uuid.UUID, st state) error {
	b, _ := json.Marshal(st)
	return s.write(path.Join(id.String(), stateFile), b)
}

// probeFile is the write Writable tries while the spool is failing.
const probeFile = ".probe"

// probeData is as large as a chunk file: an empty file still fits on a full disk.
var probeData = make([]byte, hashLen+MaxChunkRaw)

// Writable reports whether the last disk write succeeded. While it failed, each
// call first tries a chunk-sized write, so a disk with room again admits sessions.
func (s *Spool) Writable() bool {
	if !s.writeFailed.Load() {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.write(probeFile, probeData) == nil {
		_ = s.root.Remove(probeFile)
	}
	return !s.writeFailed.Load()
}

// write replaces name with parts: a synced temp file renamed over it.
func (s *Spool) write(name string, parts ...[]byte) (err error) {
	defer func() { s.writeFailed.Store(err != nil) }()
	tmp := name + tmpExt
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	for _, p := range parts {
		if _, err = f.Write(p); err != nil {
			break
		}
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = s.root.Rename(tmp, name)
	}
	if err != nil {
		_ = s.root.Remove(tmp)
		return err
	}
	return s.syncDir(path.Dir(name)) // makes the rename durable
}

// mkdir creates a session directory durably.
func (s *Spool) mkdir(id uuid.UUID) (err error) {
	defer func() { s.writeFailed.Store(err != nil) }()
	if err := s.root.MkdirAll(id.String(), 0o700); err != nil {
		return err
	}
	return s.syncDir(".")
}

func (s *Spool) syncDir(name string) error {
	d, err := s.root.Open(name)
	if err != nil {
		return err
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	return err
}

func chunkName(id uuid.UUID, seq int) string {
	return path.Join(id.String(), fmt.Sprintf("%08d%s", seq, chunkExt))
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
