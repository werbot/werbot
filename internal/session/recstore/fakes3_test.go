package recstore_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeS3 struct {
	*httptest.Server
	mu      sync.Mutex
	objects map[string][]byte
	log     []string
	encs    []string
}

func newFakeS3(t *testing.T, bucket string) *fakeS3 {
	t.Helper()
	f := &fakeS3{objects: map[string][]byte{}}
	mux := http.NewServeMux()
	noBucket := func(w http.ResponseWriter, r *http.Request) bool {
		if r.PathValue("bucket") == bucket {
			return false
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `<Error><Code>NoSuchBucket</Code><Message>The specified bucket does not exist.</Message></Error>`)
		return true
	}
	mux.HandleFunc("PUT /{bucket}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		if noBucket(w, r) {
			return
		}
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.objects[r.PathValue("bucket")+"/"+r.PathValue("key")] = body
		f.record(r)
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /{bucket}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		if noBucket(w, r) {
			return
		}
		f.mu.Lock()
		body, ok := f.objects[r.PathValue("bucket")+"/"+r.PathValue("key")]
		f.record(r)
		f.mu.Unlock()
		if !ok {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(w, `<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message></Error>`)
			return
		}
		_, _ = w.Write(body)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

// record must be called with f.mu held.
func (f *fakeS3) record(r *http.Request) {
	f.log = append(f.log, r.Method+" "+r.URL.Path)
	f.encs = append(f.encs, r.Header.Get("Content-Encoding"))
}

func (f *fakeS3) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.log...)
}

func (f *fakeS3) sawAWSChunked() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.encs {
		if strings.Contains(e, "aws-chunked") {
			return true
		}
	}
	return false
}
