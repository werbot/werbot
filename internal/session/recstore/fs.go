// Package recstore holds the file system and S3 implementations of session.RecordingStore.
package recstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/google/uuid"
)

// Key is the object key shared by the FS and S3 stores.
func Key(orgID, sessionID uuid.UUID, seq int) string {
	return fmt.Sprintf("%s/%s/%08d.gz", orgID, sessionID, seq)
}

func checkKey(orgID, sessionID uuid.UUID, seq int) error {
	if orgID == uuid.Nil || sessionID == uuid.Nil || seq < 0 {
		return fmt.Errorf("recording chunk: invalid key (org %s, session %s, seq %d)", orgID, sessionID, seq)
	}
	return nil
}

// FS stores chunks as files under root; every path is confined with os.Root.
type FS struct{ root string }

// NewFS returns an FS store rooted at root; the directory is created on first Put.
func NewFS(root string) *FS { return &FS{root: root} }

// Put writes the chunk to a temp file, syncs it and renames it over the final name.
func (f *FS) Put(_ context.Context, orgID, sessionID uuid.UUID, seq int, gz []byte) (err error) {
	if err = checkKey(orgID, sessionID, seq); err != nil {
		return err
	}
	if err = os.MkdirAll(f.root, 0o700); err != nil {
		return fmt.Errorf("recordings dir: %w", err)
	}
	r, err := os.OpenRoot(f.root)
	if err != nil {
		return fmt.Errorf("recordings dir: %w", err)
	}
	defer r.Close()

	key := Key(orgID, sessionID, seq)
	dir := path.Dir(key)
	if err = r.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("recording chunk %s: %w", key, err)
	}
	tmp := path.Join(dir, "."+uuid.NewString()+".tmp")
	w, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("recording chunk %s: %w", key, err)
	}
	_, err = w.Write(gz)
	if err == nil {
		err = w.Sync()
	}
	err = errors.Join(err, w.Close())
	if err == nil {
		err = r.Rename(tmp, key)
	}
	if err != nil {
		_ = r.Remove(tmp)
		return fmt.Errorf("recording chunk %s: %w", key, err)
	}
	if err = syncDir(r, dir); err != nil { // makes the rename durable: the gateway drops its copy on success
		return fmt.Errorf("recording chunk %s: %w", key, err)
	}
	return nil
}

func syncDir(r *os.Root, name string) error {
	d, err := r.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}

// Get opens the chunk file; a missing file wraps fs.ErrNotExist.
func (f *FS) Get(_ context.Context, orgID, sessionID uuid.UUID, seq int) (io.ReadCloser, error) {
	if err := checkKey(orgID, sessionID, seq); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(f.root)
	if err != nil {
		return nil, fmt.Errorf("recordings dir: %w", err)
	}
	defer r.Close()
	key := Key(orgID, sessionID, seq)
	file, err := r.Open(key)
	if err != nil {
		return nil, fmt.Errorf("recording chunk %s: %w", key, err)
	}
	return file, nil
}
