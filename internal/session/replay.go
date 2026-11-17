package session

import (
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

// NewReplayHandler serves GET /api/recordings/{session_id}.cast: the recording
// of a session the principal may see, as one asciicast v2 stream. The store is
// read once into a temporary file while the chain is checked, so the bytes sent
// are exactly the ones X-Werbot-Recording-Verified speaks for. An unfinished or
// tampered recording is served as far as it goes with the header "false".
func NewReplayHandler(svc *Service, rec *Recordings) http.Handler {
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
			replayError(w, r, err)
			return
		}
		tmp, err := os.CreateTemp("", "werbot-replay-*")
		if err != nil {
			replayError(w, r, err)
			return
		}
		defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
		v, err := rec.copyChain(ctx, p.OrgID, id, tmp)
		if err == nil && v.Reason == ReasonNoRecording {
			err = fmt.Errorf("session has no recording: %w", apperr.ErrNotFound)
		}
		if err == nil {
			_, err = tmp.Seek(0, io.SeekStart)
		}
		if err != nil {
			replayError(w, r, err)
			return
		}

		h := w.Header()
		h.Set("Content-Type", "application/x-asciicast")
		h.Set("Cache-Control", "no-store")
		h.Set("X-Werbot-Recording-Verified", strconv.FormatBool(v.Verified))
		w.WriteHeader(http.StatusOK)
		zr, err := gzip.NewReader(tmp) // multistream: the gzip members read as one stream
		if errors.Is(err, io.EOF) {
			return // no chunk could be read: an empty recording
		}
		if err == nil {
			_, err = io.Copy(w, zr) //nolint:gosec // streamed, never buffered: the client's read pace bounds the work
		}
		if err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "replay: recording cut short", "session_id", id, "err", err)
		}
	})
}

// replayError answers a failed replay: hidden or absent is 404, a
// recording store outage (apperr.ErrUnavailable) is 503, anything else is 500.
func replayError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		http.NotFound(w, r)
	case errors.Is(err, apperr.ErrUnavailable):
		http.Error(w, "recording store unavailable, try again later", http.StatusServiceUnavailable)
	default:
		slog.ErrorContext(r.Context(), "replay failed", "path", r.URL.Path, "err", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
