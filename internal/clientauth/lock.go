package clientauth

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const (
	lockStale = 30 * time.Second
	lockWait  = 10 * time.Second
	lockStep  = 100 * time.Millisecond
)

// lockProfile takes <dir>/.lock and returns its release.
// ponytail: an O_EXCL lock file instead of flock — one code path on every OS;
// a crashed holder blocks others until the file is lockStale old.
func lockProfile(ctx context.Context, dir string) (func(), error) {
	path := filepath.Join(dir, ".lock")
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // the lock file in the user's own profile directory
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) > lockStale {
			_ = os.Remove(path) // abandoned by a crashed process
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("profile is locked by another werbot process: %s", path)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(lockStep):
		}
	}
}
