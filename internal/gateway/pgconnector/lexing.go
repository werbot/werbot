package pgconnector

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// clientOnlyEncodings are the client encodings whose multibyte characters
// can hold the byte of a quote or backslash: the classifier, which lexes
// ASCII-safe text, could split such a text unlike the database.
var clientOnlyEncodings = []string{"SJIS", "SHIFT_JIS_2004", "BIG5", "GBK", "UHC", "GB18030", "JOHAB"}

// lexingParams are the protected settings the gateway drops from a
// person's start or follows as the database reports them.
var lexingParams = []string{"standard_conforming_strings", "client_encoding", "backslash_quote"}

// lexing follows the settings the database lexes the session's text with:
// standard_conforming_strings and client_encoding as it reports them. While
// they differ from the classifier's (standard_conforming_strings on, an
// ASCII-safe encoding), or for the whole of a session whose start set what
// the classifier cannot see (hidden), unsafe is set and every operation is
// [opaque].
type lexing struct {
	scs, enc string // the database side's alone after the start
	hidden   bool
	unsafe   atomic.Bool
	ps       pgproto3.ParameterStatus
}

// start takes the parameters the database reported at login; a missing
// standard_conforming_strings is unsafe.
func (l *lexing) start(params map[string]string, hidden bool) {
	l.scs, l.enc, l.hidden = params["standard_conforming_strings"], params["client_encoding"], hidden
	l.check()
}

// hidesSettings reports whether a person's start parameters set what the
// classifier cannot see: non-empty options (any -c setting) or a protected
// setting other than lexingParams.
func hidesSettings(params map[string]string) bool {
	for k, v := range params {
		if (strings.EqualFold(k, "options") && v != "") || (sqlclass.Protected(k) && !slices.Contains(lexingParams, strings.ToLower(k))) {
			return true
		}
	}
	return false
}

// status applies the body of a ParameterStatus; an unreadable one is unsafe.
func (l *lexing) status(body []byte) {
	if l.ps.Decode(body) != nil {
		l.unsafe.Store(true)
		return
	}
	switch l.ps.Name {
	case "standard_conforming_strings":
		l.scs = l.ps.Value
	case "client_encoding":
		l.enc = l.ps.Value
	default:
		return
	}
	l.check()
}

func (l *lexing) check() {
	l.unsafe.Store(l.hidden || l.scs != "on" || slices.Contains(clientOnlyEncodings, strings.ToUpper(l.enc)))
}

// relexes reports whether an operation of classes may change the lexing
// settings: a privilege one (SET, RESET or set_config of a protected
// setting) or an opaque one (DO, an unknown function).
func relexes(classes []string) bool {
	return slices.Contains(classes, string(shellclass.Privilege)) || slices.Contains(classes, string(shellclass.Opaque))
}
