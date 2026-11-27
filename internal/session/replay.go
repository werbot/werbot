package session

import (
	"bufio"
	"cmp"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// maxReplayChunk caps the decompressed bytes served per chunk: a gateway
// writes at most 64 KiB raw per chunk, so only a forged (bomb) chunk reaches it.
const maxReplayChunk = 1 << 20

// errChunkTooLarge is a chunk that decompresses past maxReplayChunk.
var errChunkTooLarge = fmt.Errorf("recording chunk decompresses past %d bytes", maxReplayChunk)

// maxReplays caps the replays one handler runs at once: each copies the whole
// chain into a temporary file.
// ponytail: one cap per replica; per-principal limits if one user can starve the others.
const maxReplays = 4

// replayExt maps a replay file extension to the recording format it serves.
var replayExt = map[string]string{".cast": connector.RecordAsciicast, ".querylog": connector.RecordQuerylog}

// replayType is the Content-Type of each recording format.
var replayType = map[string]string{connector.RecordAsciicast: "application/x-asciicast", connector.RecordQuerylog: "application/x-ndjson"}

// replayFile parses "<session_id>.cast" or "<session_id>.querylog" into the
// session id and the recording format; ok is false for any other name.
func replayFile(name string) (id uuid.UUID, format string, ok bool) {
	ext := path.Ext(name)
	format, ok = replayExt[ext]
	id, err := uuid.Parse(strings.TrimSuffix(name, ext))
	return id, format, ok && err == nil
}

// NewReplayHandler serves GET /api/recordings/{session_id}.cast and
// {session_id}.querylog: the recording of a session the principal may see, as
// one asciicast v2 stream or one NDJSON querylog; an extension that is not the
// format of the session's recording is not found. The store is
// read once into a temporary file while the chain is checked, so the bytes sent
// are exactly the ones X-Werbot-Recording-Verified speaks for. An unfinished or
// tampered recording is served as far as it goes with the header "false".
// At most maxReplays run at once; the next gets 503 with Retry-After.
// logger nil is slog.Default().
func NewReplayHandler(svc *Service, rec *Recordings, logger *slog.Logger) http.Handler {
	logger = cmp.Or(logger, slog.Default())
	slots := make(chan struct{}, maxReplays)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, err := principal.Require(ctx)
		if err != nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		id, format, ok := replayFile(r.PathValue("file"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		if err := recordedAs(ctx, svc, p, id, format); err != nil {
			replayError(w, r, logger, err)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			w.Header().Set("Retry-After", "5")
			http.Error(w, "replay busy, try again", http.StatusServiceUnavailable)
			return
		}
		tmp, err := os.CreateTemp("", "werbot-replay-*")
		if err != nil {
			replayError(w, r, logger, err)
			return
		}
		defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
		v, err := rec.copyChain(ctx, p, id, tmp)
		if err == nil && v.Reason == ReasonNoRecording {
			err = fmt.Errorf("session has no recording: %w", apperr.ErrNotFound)
		}
		if err == nil {
			_, err = tmp.Seek(0, io.SeekStart)
		}
		if err != nil {
			replayError(w, r, logger, err)
			return
		}

		h := w.Header()
		h.Set("Content-Type", replayType[format])
		h.Set("Content-Disposition", `attachment; filename="`+id.String()+path.Ext(r.PathValue("file"))+`"`)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Werbot-Recording-Verified", strconv.FormatBool(v.Verified))
		w.WriteHeader(http.StatusOK)
		cw := &clientWriter{w: w}
		if err := gunzipChunks(cw, tmp); err != nil && cw.err == nil && ctx.Err() == nil {
			logger.WarnContext(ctx, "replay: recording cut short", "session_id", id, "err", err)
		}
	})
}

// recordedAs is nil when the principal may see the session and its recording
// has format; otherwise it is apperr.ErrNotFound or the lookup error.
func recordedAs(ctx context.Context, svc *Service, p principal.Principal, id uuid.UUID, format string) error {
	s, err := svc.GetSession(ctx, p, id)
	if err != nil {
		return err
	}
	if s.RecordFormat != format {
		return fmt.Errorf("session has no %s recording: %w", format, apperr.ErrNotFound)
	}
	return nil
}

// gunzipChunks writes the decompressed gzip members of src to w, at most
// maxReplayChunk bytes of each; it stops at the first member over the cap, so
// what was written is a prefix of the recording.
func gunzipChunks(w io.Writer, src io.Reader) error {
	br := bufio.NewReader(src) // Reset must keep reading from the same buffer
	zr, err := gzip.NewReader(br)
	for err == nil {
		zr.Multistream(false)
		var n int64
		if n, err = io.Copy(w, io.LimitReader(zr, maxReplayChunk)); err == nil && n == maxReplayChunk {
			err = endOfMember(zr)
		}
		if err == nil {
			err = zr.Reset(br)
		}
	}
	if errors.Is(err, io.EOF) { // no more members (or none: no chunk could be read)
		return nil
	}
	return err
}

// endOfMember is nil when zr has no byte left in its member (it reads one past
// the cap without serving it), errChunkTooLarge when it has.
func endOfMember(zr io.Reader) error {
	_, err := io.ReadFull(zr, make([]byte, 1))
	switch {
	case err == nil:
		return errChunkTooLarge
	case errors.Is(err, io.EOF):
		return nil
	}
	return err
}

// clientWriter keeps the first write error: the client went away, nothing to log.
type clientWriter struct {
	w   io.Writer
	err error
}

func (c *clientWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if err != nil && c.err == nil {
		c.err = err
	}
	return n, err
}

// replayError answers a failed replay: hidden or absent is 404, a recording
// store outage (apperr.ErrUnavailable) is 503, anything else is 500 and logged
// unless the client went away.
func replayError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, apperr.ErrUnavailable):
		http.Error(w, "recording store unavailable, try again later", http.StatusServiceUnavailable)
	default:
		if r.Context().Err() == nil {
			logger.ErrorContext(r.Context(), "replay failed", "path", r.URL.Path, "err", err)
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
