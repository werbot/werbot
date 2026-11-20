package shellclass

import "strings"

// optSpec describes the options of a command.
type optSpec struct {
	short string          // short options without a value
	value string          // short options with a value
	long  map[string]bool // long option names; true when they take a value
	loose bool            // unknown options are accepted as flags without a value
}

// opt is one parsed option.
type opt struct {
	name string // "-x" or "--name"
	val  word
}

func has(opts []opt, names ...string) bool {
	for _, o := range opts {
		for _, n := range names {
			if o.name == n {
				return true
			}
		}
	}
	return false
}

// scanOpts splits args into options and operands. With stop it ends at the
// first operand (wrapper syntax); otherwise options may follow operands.
// ok is false for an unknown option (unless loose), a missing value, or a
// value word that may split into several words.
func scanOpts(args []word, spec optSpec, stop bool) (opts []opt, operands []word, ok bool) {
	for i := 0; i < len(args); i++ {
		t := args[i].text
		var parsed []opt
		used := 0
		switch {
		case t == "--":
			return opts, append(operands, args[i+1:]...), true
		case len(t) < 2 || t[0] != '-':
			if stop {
				return opts, append(operands, args[i:]...), true
			}
			operands = append(operands, args[i])
			continue
		case strings.HasPrefix(t, "--"):
			parsed, used, ok = spec.longOpt(t[2:], args[i+1:])
		default:
			parsed, used, ok = spec.shortOpts(t[1:], args[i+1:])
		}
		if !ok {
			return nil, nil, false
		}
		opts = append(opts, parsed...)
		i += used
	}
	return opts, operands, true
}

// longOpt parses "name[=value]"; used counts the following words taken as the value.
func (s optSpec) longOpt(t string, next []word) ([]opt, int, bool) {
	name, val, eq := strings.Cut(t, "=")
	takes, known := s.long[name]
	switch {
	case !known && !s.loose:
		return nil, 0, false
	case eq || !takes:
		return []opt{{name: "--" + name, val: word{text: val, lit: true}}}, 0, true
	}
	v, ok := nextValue(next)
	return []opt{{name: "--" + name, val: v}}, 1, ok
}

// shortOpts parses a cluster such as "xvf" or "n10".
func (s optSpec) shortOpts(t string, next []word) ([]opt, int, bool) {
	var opts []opt
	for i := 0; i < len(t); i++ {
		name := "-" + t[i:i+1]
		switch {
		case strings.IndexByte(s.value, t[i]) >= 0 && i+1 < len(t):
			return append(opts, opt{name: name, val: word{text: t[i+1:], lit: true}}), 0, true
		case strings.IndexByte(s.value, t[i]) >= 0:
			v, ok := nextValue(next)
			return append(opts, opt{name: name, val: v}), 1, ok
		case strings.IndexByte(s.short, t[i]) >= 0 || s.loose:
			opts = append(opts, opt{name: name})
		default:
			return nil, 0, false
		}
	}
	return opts, 0, true
}

func nextValue(next []word) (word, bool) {
	if len(next) == 0 || next[0].split {
		return word{}, false
	}
	return next[0], true
}
