package httpserver

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

// inlineScript matches attribute-less <script> elements: the SvelteKit bootstrap in index.html.
var inlineScript = regexp.MustCompile(`(?s)<script>(.*?)</script>`)

// SPA serves the built web UI from fsys. An existing file is served as is; any
// other GET/HEAD that accepts text/html gets index.html (503 when the UI is not
// built); the rest is 404. Every response carries the CSP and security headers.
func SPA(fsys fs.FS) http.Handler {
	index, indexErr := fs.ReadFile(fsys, "index.html")
	csp := "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'"
	for _, m := range inlineScript.FindAllSubmatch(index, -1) {
		sum := sha256.Sum256(m[1])
		csp += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	csp += "; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'"
	files := http.FileServerFS(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if fi, err := fs.Stat(fsys, name); err == nil && !fi.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && strings.Contains(r.Header.Get("Accept"), "text/html") {
			if indexErr != nil {
				http.Error(w, "web UI is not built: run make web", http.StatusServiceUnavailable)
				return
			}
			http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
			return
		}
		http.NotFound(w, r)
	})
}
