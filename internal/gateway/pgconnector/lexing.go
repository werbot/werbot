package pgconnector

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/werbot/werbot/internal/platform/shellclass"
)

// clientOnlyEncodings are the client encodings whose multibyte characters
// can hold the byte of a quote or backslash: the classifier, which lexes
// ASCII-safe text, could split such a text unlike the database.
var clientOnlyEncodings = []string{"SJIS", "SHIFT_JIS_2004", "BIG5", "GBK", "UHC", "GB18030", "JOHAB"}

// lexing follows the settings the database lexes the session's text with:
// standard_conforming_strings and client_encoding as it reports them. While
// they differ from the classifier's (standard_conforming_strings on, an
// ASCII-safe encoding), unsafe is set and every operation is [opaque].
type lexing struct {
	scs, enc string // the database side's alone after the start
	unsafe   atomic.Bool
	ps       pgproto3.ParameterStatus
}

// start takes the parameters the database reported at login; a missing
// standard_conforming_strings is unsafe.
func (l *lexing) start(params map[string]string) {
	l.scs, l.enc = params["standard_conforming_strings"], params["client_encoding"]
	l.check()
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
	l.unsafe.Store(l.scs != "on" || slices.Contains(clientOnlyEncodings, strings.ToUpper(l.enc)))
}

// relexes reports whether an operation of classes may change the lexing
// settings: a privilege one (SET, RESET or set_config of a protected
// setting) or an opaque one (DO, an unknown function).
func relexes(classes []string) bool {
	return slices.Contains(classes, string(shellclass.Privilege)) || slices.Contains(classes, string(shellclass.Opaque))
}
