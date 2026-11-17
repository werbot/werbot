package recstore

import (
	"context"
	"path/filepath"

	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/session"
)

// Open picks the store: an empty Bucket means the file system under dataDir/recordings, else S3.
// FS suits only a single server replica or a shared RWX volume; several replicas need S3-compatible storage.
func Open(ctx context.Context, cfg config.RecordingsS3, dataDir string) (store session.RecordingStore, kind string, err error) {
	if cfg.Bucket == "" {
		return NewFS(filepath.Join(dataDir, "recordings")), "fs", nil
	}
	s, err := NewS3(ctx, cfg)
	if err != nil {
		return nil, "", err
	}
	return s, "s3", nil
}
