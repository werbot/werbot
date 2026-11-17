package recstore_test

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/session/recstore"
)

func read(t *testing.T, s session.RecordingStore, org, sess uuid.UUID, seq int) string {
	t.Helper()
	rc, err := s.Get(t.Context(), org, sess, seq)
	require.NoError(t, err)
	defer rc.Close()
	b, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(b)
}

func runSuite(t *testing.T, s session.RecordingStore) (org, sess uuid.UUID) {
	t.Helper()
	ctx := t.Context()
	org, sess = uuid.New(), uuid.New()
	require.NoError(t, s.Put(ctx, org, sess, 0, []byte("a")))
	require.NoError(t, s.Put(ctx, org, sess, 0, []byte("b")))
	assert.Equal(t, "b", read(t, s, org, sess, 0))

	_, err := s.Get(ctx, org, sess, 1)
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = s.Get(ctx, uuid.New(), sess, 0)
	require.ErrorIs(t, err, fs.ErrNotExist)
	return org, sess
}

func TestFS(t *testing.T) {
	root := t.TempDir()
	org, sess := runSuite(t, recstore.NewFS(root))

	p := filepath.Join(root, recstore.Key(org, sess, 0))
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
	di, err := os.Stat(filepath.Dir(p))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), di.Mode().Perm())
	entries, err := os.ReadDir(filepath.Dir(p))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "no temp files left")
}

func TestFSRejectsBadKey(t *testing.T) {
	s := recstore.NewFS(t.TempDir())
	require.Error(t, s.Put(t.Context(), uuid.New(), uuid.New(), -1, []byte("x")))
	require.Error(t, s.Put(t.Context(), uuid.Nil, uuid.New(), 0, []byte("x")))
	_, err := s.Get(t.Context(), uuid.New(), uuid.Nil, 0)
	require.Error(t, err)
}

func TestS3Fake(t *testing.T) {
	f := newFakeS3(t, "werbot-test")
	s, err := recstore.NewS3(t.Context(), config.RecordingsS3{
		Endpoint: f.URL, Bucket: "werbot-test", Region: "us-east-1", AccessKey: "ak", SecretKey: "sk",
	})
	require.NoError(t, err)
	org, sess := runSuite(t, s)
	assert.Contains(t, f.requests(), "PUT /werbot-test/"+recstore.Key(org, sess, 0))
	assert.False(t, f.sawAWSChunked())
}

func TestS3Real(t *testing.T) {
	ep := os.Getenv("WERBOT_TEST_S3_ENDPOINT")
	if ep == "" {
		t.Skip("WERBOT_TEST_S3_ENDPOINT not set")
	}
	s, err := recstore.NewS3(t.Context(), config.RecordingsS3{
		Endpoint:  ep,
		Bucket:    os.Getenv("WERBOT_TEST_S3_BUCKET"),
		Region:    os.Getenv("WERBOT_TEST_S3_REGION"),
		AccessKey: os.Getenv("WERBOT_TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("WERBOT_TEST_S3_SECRET_KEY"),
	})
	require.NoError(t, err)
	runSuite(t, s)
}

func TestOpenDefaultsToFS(t *testing.T) {
	dir := t.TempDir()
	s, kind, err := recstore.Open(t.Context(), config.RecordingsS3{}, dir)
	require.NoError(t, err)
	assert.Equal(t, "fs", kind)
	org, sess := uuid.New(), uuid.New()
	require.NoError(t, s.Put(t.Context(), org, sess, 0, []byte("x")))
	assert.FileExists(t, filepath.Join(dir, "recordings", recstore.Key(org, sess, 0)))
}
