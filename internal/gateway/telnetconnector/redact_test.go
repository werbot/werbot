package telnetconnector

import (
	"bytes"
	"testing"
	"time"
)

// The redactor removes the password however the output splits it; within the
// login window it holds a trailing prefix until the echo completes or diverges.
func TestRedactor(t *testing.T) {
	var got bytes.Buffer
	r := newRedactor(&got, "ab#cd")
	for _, p := range []string{"Welcome\r\n$ ", "a", "b#", "c", "d\r\n", "ab#cX", "end ab#cd."} {
		if _, err := r.Write([]byte(p)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if want := "Welcome\r\n$ \r\nab#cXend ."; got.String() != want {
		t.Fatalf("output = %q, want %q", got.String(), want)
	}

	got.Reset()
	r.until = time.Now().Add(-time.Second) // the login window is over
	for _, p := range []string{"a", "x ab#cd!"} {
		if _, err := r.Write([]byte(p)); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if want := "ax !"; got.String() != want {
		t.Fatalf("after the window = %q, want %q (no hold, still stripped)", got.String(), want)
	}

	got.Reset()
	if _, err := newRedactor(&got, "").Write([]byte("ab#cd")); err != nil || got.String() != "ab#cd" {
		t.Fatalf("without a password = %q, %v", got.String(), err)
	}
}
