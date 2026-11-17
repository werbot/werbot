package gateway

import (
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{
		{"", 4, ""},
		{"abc", 4, "abc"},
		{"abcd", 4, "abcd"},
		{"abcde", 4, "abcd"},
		{"a😀b", 4, "a"},     // the 4-byte character does not fit after "a"
		{"a😀b", 5, "a😀"},    // it fits exactly
		{"ж😀", 3, "ж"},      // 2 + 4 bytes
		{"a\xffb", 3, "ab"}, // invalid bytes are dropped
		{"abc", 0, ""},
	} {
		got := TruncateUTF8(c.in, c.n)
		if got != c.want || len(got) > c.n || !utf8.ValidString(got) {
			t.Errorf("TruncateUTF8(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
