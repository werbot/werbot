package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// wsReadLimit bounds one browser message; a larger one closes the stream
// with 1009.
const wsReadLimit = 64 << 10

// Close codes of browser WebSockets by stream close reason.
var closeCodes = map[string]websocket.StatusCode{
	ReasonClosed:   websocket.StatusNormalClosure,
	ReasonDenied:   4003,
	ReasonNotFound: 4004,
	ReasonLimit:    4008,
	"mfa_required": 4001,
}

// CloseCode is the WebSocket close code of a stream close reason; 1011 for
// error and any reason it does not know.
func CloseCode(reason string) websocket.StatusCode {
	if c, ok := closeCodes[reason]; ok {
		return c
	}
	return websocket.StatusInternalError
}

// errorCode is the close code and reason of a stream that failed to open.
// An unknown resource of a terminal looks like a denied one.
func errorCode(err error, terminal bool) (websocket.StatusCode, string) {
	switch {
	case errors.Is(err, ErrLimit): // wraps ErrUnavailable
		return CloseCode(ReasonLimit), ReasonLimit
	case errors.Is(err, ErrAwaitTimeout): // wraps ErrUnavailable
		return websocket.StatusInternalError, ReasonError
	case errors.Is(err, apperr.ErrUnavailable):
		return websocket.StatusInternalError, "Gateway is unavailable"
	case errors.Is(err, apperr.ErrNotFound) && !terminal:
		return CloseCode(ReasonNotFound), ReasonNotFound
	case errors.Is(err, apperr.ErrNotFound), errors.Is(err, apperr.ErrPermissionDenied):
		return CloseCode(ReasonDenied), ReasonDenied
	}
	return websocket.StatusInternalError, ReasonError
}

// WebSocketHandler serves the browser streams GET /api/terminal
// (?resource=&cols=&rows=) and GET /api/sessions/{id}/live. It refuses a
// request whose Origin is not the origin of Config.WebURL (403), without a
// principal (401) or of an agent (403) before the upgrade.
func (r *Relay) WebSocketHandler() http.Handler {
	want := origin(r.cfg.WebURL)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/terminal", r.serveTerminal)
	mux.HandleFunc("GET /api/sessions/{id}/live", r.serveLive)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if want == "" || origin(req.Header.Get("Origin")) != want {
			r.reject("origin")
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		p, err := principal.Require(req.Context())
		if err != nil {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		if p.IsAgent() { // agents never get an interactive shell or a live view
			http.Error(w, "permission denied", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, req)
	})
}

// origin is scheme://host[:port] of an http(s) URL in lower case without
// the scheme's default port; "" for anything else.
func origin(s string) string {
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	if p := u.Port(); (u.Scheme == "https" && p == "443") || (u.Scheme == "http" && p == "80") {
		host = strings.TrimSuffix(host, ":"+p)
	}
	return u.Scheme + "://" + host
}

func (r *Relay) serveTerminal(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	cols, okC := dimension(q, "cols", 80, maxCols)
	rows, okR := dimension(q, "rows", 24, maxRows)
	if !okC || !okR {
		http.Error(w, "cols must be 1-500 and rows 1-200", http.StatusBadRequest)
		return
	}
	p, _ := principal.From(req.Context())
	r.serveWS(w, req, true, func(ctx context.Context) (Pipe, error) {
		return r.Terminal(ctx, p, q.Get("resource"), cols, rows)
	})
}

// dimension is the window size in query key, def when absent; false when
// it is not an integer in 1..limit.
func dimension(q url.Values, key string, def, limit int) (int, bool) {
	if !q.Has(key) {
		return def, true
	}
	n, err := strconv.Atoi(q.Get(key))
	return n, err == nil && n >= 1 && n <= limit
}

func (r *Relay) serveLive(w http.ResponseWriter, req *http.Request) {
	p, _ := principal.From(req.Context())
	r.serveWS(w, req, false, func(ctx context.Context) (Pipe, error) {
		id, err := uuid.Parse(req.PathValue("id"))
		if err != nil {
			return nil, apperr.ErrNotFound
		}
		return r.Observe(ctx, p, id)
	})
}

// serveWS upgrades the request, opens its stream and pumps both ways until
// either side ends; a stream that fails to open closes with errorCode.
func (r *Relay) serveWS(w http.ResponseWriter, req *http.Request, terminal bool, open func(context.Context) (Pipe, error)) {
	conn, err := websocket.Accept(w, req, &websocket.AcceptOptions{InsecureSkipVerify: true}) // Origin checked by WebSocketHandler
	if err != nil {
		return // Accept answered the request
	}
	conn.SetReadLimit(wsReadLimit)
	ctx := req.Context()
	pipe, err := open(ctx)
	if err != nil {
		code, reason := errorCode(err, terminal)
		if reason == ReasonError && !errors.Is(err, ErrAwaitTimeout) {
			r.log.Warn("relay: stream not opened", "error", err)
		}
		_ = conn.Close(code, reason)
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Go(func() { toBrowser(ctx, conn, pipe, !terminal) })
	wg.Go(func() { r.pingBrowser(ctx, conn, pipe) })
	fromBrowser(ctx, conn, pipe, terminal)
	_ = pipe.Close(ReasonClosed)
	cancel() // ends pingBrowser; toBrowser ends with the stream
	wg.Wait()
}

// resizeMsg is a window change: from the terminal's browser, to a live view.
type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// toBrowser copies the stream to the browser until the stream ends, then
// closes the WebSocket with the stream's reason. A failed write closes the
// stream; it is still read to its end.
func toBrowser(ctx context.Context, conn *websocket.Conn, pipe Pipe, live bool) {
	for {
		d, cols, rows, err := pipe.Read()
		if err != nil {
			reason := reasonOf(err)
			_ = conn.Close(CloseCode(reason), reason)
			return
		}
		switch {
		case cols > 0 && live:
			b, _ := json.Marshal(resizeMsg{Type: "resize", Cols: cols, Rows: rows}) // ints only: never fails
			err = conn.Write(ctx, websocket.MessageText, b)
		case cols > 0: // a terminal's browser sets its own window
		default:
			err = conn.Write(ctx, websocket.MessageBinary, d)
		}
		if err != nil {
			_ = pipe.Close(ReasonClosed)
		}
	}
}

// fromBrowser copies the terminal's keystrokes and in-bounds resizes to the
// stream until the browser goes; a live view drops all browser input, and
// malformed text is skipped.
func fromBrowser(ctx context.Context, conn *websocket.Conn, pipe Pipe, terminal bool) {
	for {
		typ, b, err := conn.Read(ctx)
		if err != nil {
			return
		}
		switch {
		case !terminal:
		case typ == websocket.MessageBinary:
			err = pipe.Write(b)
		default:
			if cols, rows, ok := parseResize(b); ok {
				err = pipe.Resize(cols, rows)
			}
		}
		if err != nil {
			return
		}
	}
}

// parseResize reads a resize message with an in-bounds window.
func parseResize(b []byte) (cols, rows int, ok bool) {
	var m resizeMsg
	if json.Unmarshal(b, &m) != nil || m.Type != "resize" {
		return 0, 0, false
	}
	return m.Cols, m.Rows, m.Cols >= 1 && m.Cols <= maxCols && m.Rows >= 1 && m.Rows <= maxRows
}

// pingBrowser pings the browser every r.ping until ctx ends; an unanswered
// ping closes the stream and drops the connection.
func (r *Relay) pingBrowser(ctx context.Context, conn *websocket.Conn, pipe Pipe) {
	t := time.NewTicker(r.ping)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		pctx, cancel := context.WithTimeout(ctx, r.ping)
		err := conn.Ping(pctx)
		cancel()
		if err != nil && ctx.Err() == nil {
			_ = pipe.Close(ReasonClosed)
			_ = conn.CloseNow()
			return
		}
	}
}
