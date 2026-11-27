package mask

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testSecret = "S3cr3t-Value!"

// maskAll writes each chunk to a new writer, closes it and returns the output.
func maskAll(t testing.TB, secrets []string, patterns bool, chunks ...string) string {
	t.Helper()
	var out bytes.Buffer
	w := NewWriter(&out, toBytes(secrets), patterns)
	for _, c := range chunks {
		n, err := w.Write([]byte(c))
		require.NoError(t, err)
		require.Equal(t, len(c), n)
	}
	require.NoError(t, w.Close())
	return out.String()
}

func toBytes(ss []string) [][]byte {
	var out [][]byte
	for _, s := range ss {
		out = append(out, []byte(s))
	}
	return out
}

func TestMaskLiteral(t *testing.T) {
	in := "pw=" + testSecret + " again " + testSecret
	require.Equal(t, "pw=[masked] again [masked]", maskAll(t, []string{testSecret}, false, in))
	require.Equal(t, in, maskAll(t, nil, false, in), "no secrets: output unchanged")
}

// A secret split at any position, or written one byte at a time, never shows
// a prefix of the occurrence in progress; the output equals one write.
func TestMaskSplitAcrossWrites(t *testing.T) {
	in := "pw=" + testSecret + " again " + testSecret
	want := maskAll(t, []string{testSecret}, false, in)
	for i := range len(in) + 1 {
		require.Equal(t, want, maskAll(t, []string{testSecret}, false, in[:i], in[i:]), "cut at %d", i)
	}

	var out bytes.Buffer
	w := NewWriter(&out, [][]byte{[]byte(testSecret)}, false)
	for i := range len(in) {
		_, err := w.Write([]byte{in[i]})
		require.NoError(t, err)
		for k := 1; k < len(testSecret); k++ {
			require.False(t, strings.HasSuffix(out.String(), testSecret[:k]), "after byte %d the output ends with a secret prefix", i)
		}
	}
	require.NoError(t, w.Close())
	require.Equal(t, want, out.String())
}

// Random chunkings of a long text give the same output as one replace pass,
// and no full secret leaves the writer.
func TestMaskRandomChunking(t *testing.T) {
	secrets := []string{testSecret, "A9-ZZ-TOKEN-77", "Q1W2"}
	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed, not a secret
	var sb strings.Builder
	for sb.Len() < 64<<10 {
		if r.IntN(50) == 0 {
			sb.WriteString(secrets[r.IntN(len(secrets))])
			continue
		}
		sb.WriteByte("abcdefghij \n"[r.IntN(12)])
	}
	in := sb.String()
	want := in
	for _, s := range secrets { // longest first, as the writer does
		want = strings.ReplaceAll(want, s, string(marker))
	}
	for range 1000 {
		var chunks []string
		for rest := in; rest != ""; {
			n := min(len(rest), 1+r.IntN(4096))
			chunks, rest = append(chunks, rest[:n]), rest[n:]
		}
		got := maskAll(t, secrets, false, chunks...)
		require.Equal(t, want, got)
		for _, s := range secrets {
			require.NotContains(t, got, s)
		}
	}
}

// Output that ends without a secret prefix reaches w at once.
func TestMaskHoldsOnlyPrefixes(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, [][]byte{[]byte(testSecret)}, false)
	_, err := w.Write([]byte("$ "))
	require.NoError(t, err)
	require.Equal(t, "$ ", out.String())
}

func TestMaskFlushOnClose(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, [][]byte{[]byte(testSecret)}, false)
	_, err := w.Write([]byte("out S3c"))
	require.NoError(t, err)
	require.Equal(t, "out ", out.String(), "a secret prefix is held")
	require.NoError(t, w.Close())
	require.Equal(t, "out S3c", out.String(), "Close writes the held tail")
}

func TestMaskShortSecretIgnored(t *testing.T) {
	require.Equal(t, "abc ab", maskAll(t, []string{"ab"}, false, "abc ab"))
}

func TestMaskPatterns(t *testing.T) {
	gh := "ghp_" + strings.Repeat("aB3", 12)
	pat := "github_pat_" + strings.Repeat("x_9", 27) + "z"
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl"
	cases := []struct {
		name   string
		chunks []string
		want   string
	}{
		{"aws key id", []string{"id AKIAIOSFODNN7EXAMPLE end\n"}, "id [masked] end\n"},
		{"github token", []string{gh + "\n"}, "[masked]\n"},
		{"github fine-grained token", []string{"t=" + pat + "\n"}, "t=[masked]\n"},
		{"jwt", []string{"Bearer " + jwt + "\n"}, "Bearer [masked]\n"},
		{"private key block across writes", []string{
			"a\n-----BEGIN RSA PRIVATE KEY-----\nMIIEow", "IBAAKCAQEA\nmore\n-----END RSA PRI", "VATE KEY-----\nb\n",
		}, "a\n[masked]\nb\n"},
		{"high-entropy assigned value", []string{"token=9f8a7c6b5e4d3c2b\n"}, "token=[masked]\n"},
		{"json assigned value", []string{`"api_key": "Zq8XvT2pLm9KwR4s"` + "\n"}, `"api_key": [masked]` + "\n"},
		{"short assigned value", []string{"token=hello\n"}, "token=hello\n"},
		{"one-letter password", []string{"password: x\n"}, "password: x\n"},
		{"low-entropy assigned value", []string{"secret=aaaaaaaaaaaa\n"}, "secret=aaaaaaaaaaaa\n"},
		{"plain text", []string{"total 8\ndrwxr-xr-x 2 root root\n"}, "total 8\ndrwxr-xr-x 2 root root\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, maskAll(t, nil, true, tc.chunks...))
		})
	}
	t.Run("patterns off", func(t *testing.T) {
		in := "id AKIAIOSFODNN7EXAMPLE token=9f8a7c6b5e4d3c2b\n" //nolint:gosec // AWS documentation example key: the masking input
		require.Equal(t, in, maskAll(t, nil, false, in))
	})
}

// With patterns an unfinished line is held up to patternWindow; a longer one
// is written as is.
func TestMaskPatternsHoldUnfinishedLine(t *testing.T) {
	var out bytes.Buffer
	w := NewWriter(&out, nil, true)
	_, err := w.Write([]byte("line\n$ "))
	require.NoError(t, err)
	require.Equal(t, "line\n", out.String())
	long := strings.Repeat("x", patternWindow+1)
	_, err = w.Write([]byte(long))
	require.NoError(t, err)
	require.Equal(t, "line\n$ "+long, out.String())
	require.NoError(t, w.Close())
}

type failWriter struct{}

var errBroken = errors.New("broken pipe")

func (failWriter) Write([]byte) (int, error) { return 0, errBroken }

func TestMaskWriteErrorPropagates(t *testing.T) {
	w := NewWriter(failWriter{}, [][]byte{[]byte(testSecret)}, false)
	_, err := w.Write([]byte("out\n"))
	require.ErrorIs(t, err, errBroken)

	w = NewWriter(failWriter{}, [][]byte{[]byte(testSecret)}, false)
	_, err = w.Write([]byte("S3c"))
	require.NoError(t, err, "nothing to write yet")
	require.ErrorIs(t, w.Close(), errBroken)
}

func BenchmarkMask(b *testing.B) {
	pem := "-----BEGIN OPENSSH PRIVATE KEY-----\n" + strings.Repeat(strings.Repeat("b3BlbnNzaC1rZXktdjEAAAAA", 3)+"\n", 42) + "-----END OPENSSH PRIVATE KEY-----"
	require.Greater(b, len(pem), 3<<10)
	var sb strings.Builder
	for i := 0; sb.Len() < 32<<10; i++ {
		sb.WriteString("-rw-r--r-- 1 deploy deploy  4096 Nov 20 10:00 build-artifact-")
		sb.WriteByte(byte('a' + i%26))
		sb.WriteString(".log\n")
	}
	chunk := []byte(sb.String()[:32<<10])
	w := NewWriter(discard{}, [][]byte{[]byte(pem)}, true)
	b.SetBytes(int64(len(chunk)))
	b.ReportAllocs()
	for b.Loop() {
		_, _ = w.Write(chunk)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestMaskBytes(t *testing.T) {
	secrets := toBytes([]string{testSecret})
	require.Equal(t, "pw=[masked]", string(Bytes([]byte("pw="+testSecret), secrets, false)))
	require.Equal(t, "tail "+testSecret[:5], string(Bytes([]byte("tail "+testSecret[:5]), secrets, false)), "a held prefix is written")
	key := "AKIA" + strings.Repeat("A", 16)
	require.Equal(t, "id [masked] no newline", string(Bytes([]byte("id "+key+" no newline"), nil, true)), "patterns on a cell without a newline")
	require.Equal(t, "id "+key, string(Bytes([]byte("id "+key), nil, false)))
	require.Empty(t, Bytes(nil, secrets, true))
}
