package shellclass

import "strings"

// Arguments of sed commands by letter; any other letter does not parse.
const (
	sedRun   = "ewW"              // write a file or run a command
	sedText  = "aic"              // text to the end of the line; "\" continues it
	sedLine  = "#rR"              // a comment or a file name to the end of the line
	sedWord  = ":btTvqQlL"        // a label or a number up to a blank, ";" or newline
	sedPlain = "{}=dDgGhHnNpPxzF" // nothing
)

// sedParser walks a sed script command by command as GNU sed reads it.
type sedParser struct {
	s string
	i int
}

// sedRuns reports a script with an e, w or W command or an e or w flag of
// s, or one that does not parse.
func sedRuns(script string) bool {
	p := &sedParser{s: script}
	for p.skip(" \t\n;"); p.i < len(p.s); p.skip(" \t\n;") {
		if runs, ok := p.command(); runs || !ok {
			return true
		}
	}
	return false
}

// command passes over one command and its addresses.
func (p *sedParser) command() (runs, ok bool) {
	if !p.address() {
		return false, false
	}
	switch c := p.next(); {
	case strings.IndexByte(sedRun, c) >= 0:
		return true, true
	case c == 's':
		return p.substitute()
	case c == 'y':
		return false, p.parts()
	case strings.IndexByte(sedText, c) >= 0:
		p.line(true)
	case strings.IndexByte(sedLine, c) >= 0:
		p.line(false)
	case strings.IndexByte(sedWord, c) >= 0:
		p.skip(" \t")
		p.word()
	default:
		return false, strings.IndexByte(sedPlain, c) >= 0
	}
	return false, true
}

// address passes over "[addr1[,addr2]][!]".
func (p *sedParser) address() bool {
	if !p.addr() {
		return false
	}
	if p.i < len(p.s) && p.s[p.i] == ',' {
		p.i++
		p.skip(" \t")
		if !p.addr() {
			return false
		}
	}
	p.skip(" \t!")
	return true
}

// addr passes over one optional address: a line number, "first~step",
// "+N", "~N", "$", or a regex with its I and M flags.
func (p *sedParser) addr() bool {
	if p.i < len(p.s) && (p.s[p.i] == '/' || p.s[p.i] == '\\') {
		d := p.next()
		if d == '\\' {
			d = p.next()
		}
		if !p.delimited(d) {
			return false
		}
		p.skip("IM")
		return true
	}
	p.skip("0123456789$+~")
	return true
}

// substitute passes over "s/re/repl/flags"; an e or w flag runs or writes.
func (p *sedParser) substitute() (runs, ok bool) {
	if !p.parts() {
		return false, false
	}
	for ; p.i < len(p.s); p.i++ {
		switch c := p.s[p.i]; {
		case c == 'e' || c == 'w':
			return true, true
		case strings.IndexByte(" \t\n;}#", c) >= 0:
			return false, true
		case strings.IndexByte("gpiImM0123456789", c) < 0:
			return false, false
		}
	}
	return false, true
}

// parts passes over the delimiter and the two delimited parts of s or y.
func (p *sedParser) parts() bool {
	d := p.next()
	if d == 0 || d == '\\' || d == '\n' {
		return false
	}
	first := p.delimited(d)
	return first && p.delimited(d)
}

// delimited passes over text up to an unescaped d; false at a newline or the end.
func (p *sedParser) delimited(d byte) bool {
	for p.i < len(p.s) {
		switch c := p.next(); c {
		case '\\':
			p.i++
		case d:
			return true
		case '\n':
			return false
		}
	}
	return false
}

// next returns the next byte and moves past it; 0 at the end.
func (p *sedParser) next() byte {
	if p.i >= len(p.s) {
		return 0
	}
	p.i++
	return p.s[p.i-1]
}

func (p *sedParser) skip(chars string) {
	for p.i < len(p.s) && strings.IndexByte(chars, p.s[p.i]) >= 0 {
		p.i++
	}
}

// word passes over bytes up to a blank, ";" or newline.
func (p *sedParser) word() {
	for p.i < len(p.s) && strings.IndexByte(" \t\n;", p.s[p.i]) < 0 {
		p.i++
	}
}

// line passes over the rest of the line; with cont "\" escapes the next
// byte, so a backslash-newline continues the line.
func (p *sedParser) line(cont bool) {
	for p.i < len(p.s) && p.s[p.i] != '\n' {
		if cont && p.s[p.i] == '\\' {
			p.i++
		}
		p.i++
	}
}
