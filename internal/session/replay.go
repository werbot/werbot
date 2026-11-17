package session

import (
	"bufio"
	"cmp"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// maxReplayChunk caps the decompressed bytes served per chunk: a gateway
// writes at most 64 KiB raw per chunk, so only a forged (bomb) chunk reaches it.
const maxReplayChunk = 1 << 20

// errChunkTooLarge is a chunk that decompresses past maxReplayChunk.
var errChunkTooLarge = fmt.Errorf("recording chunk decompresses past %d bytes", maxReplayChunk)

// NewReplayHandler serves GET /api/recordings/{session_id}.cast: the recording
// of a session the principal may see, as one asciicast v2 stream. The store is
// read once into a temporary file while the chain is checked, so the bytes sent
// are exactly the ones X-Werbot-Recording-Verified speaks for. An unfinished or
// tampered recording is served as far as it goes with the header "false".
// logger nil is slog.Default().
func NewReplayHandler(svc *Service, rec *Recordings, logger *slog.Logger) http.Handler {
	logger = cmp.Or(logger, slog.Default())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, err := principal.Require(ctx)
		if err != nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		raw, ok := strings.CutSuffix(r.PathValue("file"), ".cast")
		id, err := uuid.Parse(raw)
		if !ok || err != nil {
			http.NotFound(w, r)
			return
		}
		if _, err := svc.GetSession(ctx, p, id); err != nil {
			replayError(w, r, logger, err)
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
		h.Set("Content-Type", "application/x-asciicast")
		h.Set("Content-Disposition", `attachment; filename="`+id.String()+`.cast"`)
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

// gunzipChunks writes the decompressed gzip members of src to w, at most
// maxReplayChunk bytes of each; it stops at the first member over the cap, so
// what was written is a prefix of the recording.
func gunzipChunks(w io.Writer, src io.Reader) error {
	br := bufio.NewReader(src) // Reset must keep reading from the same buffer
	zr, err := gzip.NewReader(br)
	for err == nil {
		zr.Multistream(false)
		var n int64
		if n, err = io.Copy(w, io.LimitReader(zr, maxReplayChunk+1)); err == nil && n > maxReplayChunk {
			err = errChunkTooLarge
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
