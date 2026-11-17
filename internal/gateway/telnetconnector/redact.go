package telnetconnector

import (
	"bytes"
	"io"
	"slices"
	"time"
)

// heldPrefix is the length of the longest suffix of b that is a non-empty
// prefix of secret (0 if none).
func heldPrefix(b, secret []byte) int {
	for k := min(len(b), len(secret)); k > 0; k-- {
		if bytes.HasSuffix(b, secret[:k]) {
			return k
		}
	}
	return 0
}

// redactor removes the password from the target output before it reaches the
// recording and the user. Until the end of the login window it also holds back
// a trailing prefix of the password, so an echo split across reads or arriving
// after Login returned is removed whole.
// ponytail: bytes held when the window ends show with the next output, not by a
// timer; add a flush timer if a silent target after login ever matters.
type redactor struct {
	w       io.Writer
	secret  []byte // empty: no autologin, output passes as is
	until   time.Time
	pending []byte
}

func newRedactor(w io.Writer, password string) *redactor {
	return &redactor{w: w, secret: []byte(password), until: time.Now().Add(loginTimeout)}
}

func (r *redactor) Write(p []byte) (int, error) {
	if len(r.secret) == 0 {
		return r.w.Write(p)
	}
	buf := slices.Concat(r.pending, p)
	if bytes.Contains(buf, r.secret) {
		buf = bytes.ReplaceAll(buf, r.secret, nil)
	}
	keep := 0
	if time.Now().Before(r.until) {
		keep = heldPrefix(buf, r.secret)
	}
	r.pending = append([]byte(nil), buf[len(buf)-keep:]...)
	if out := buf[:len(buf)-keep]; len(out) > 0 {
		if _, err := r.w.Write(out); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}
