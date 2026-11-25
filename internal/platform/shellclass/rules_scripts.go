package shellclass

import (
	"slices"
	"strings"
)

// Programs that read by default but can write files or run commands through
// their script or an output option (owner decision O1). A script that does
// not parse is opaque.

var (
	lessOutputs = []string{"-o", "-O", "--log-file", "--LOG-FILE"}
	curlOutputs = []string{"-o", "-O", "--output", "--remote-name", "--remote-name-all", "-D", "--dump-header",
		"-c", "--cookie-jar", "--trace", "--trace-ascii", "--stderr", "--libcurl", "--etag-save", "--hsts", "--alt-svc"}
	wgetOutputs = []string{"-O", "-o", "-a", "-P", "--output-document", "--output-file", "--append-output",
		"--directory-prefix"}
	// wgetNoSave are the options that keep wget from saving the download in
	// the working directory: -O with "-" or /dev/null, and --spider.
	wgetNoSave = []string{"-O", "--output-document", "--spider"}
	// awkSafe are the only awk options that neither load a program nor write a file.
	awkSafe = []string{"-F", "-v", "--field-separator", "--assign"}
	gitRule = rule{pre: map[string]bool{"-C": true}, flags: keyed(map[Class]string{Write: "--output"}),
		subs: keyed(map[Class]string{
			Read:  "status|log|diff|show",
			Write: "pull|fetch|checkout|commit|push",
		})}
)

func addScripts(add func(rule, string)) {
	add(rule{check: awk}, "awk")
	add(rule{check: sed}, "sed")
	add(rule{check: less}, "less")
	add(rule{check: git}, "git")
}

// quietOutput reports an output value that saves no file, as "> /dev/null".
func quietOutput(v word) bool { return v.lit && (v.text == "-" || v.text == "/dev/null") }

// saves reports an option of names (or its long abbreviation) that saves
// output to a file.
func saves(opts []opt, names []string) bool {
	return slices.ContainsFunc(opts, func(o opt) bool {
		return slices.ContainsFunc(names, o.matches) && !quietOutput(o.val)
	})
}

// saving wraps a network client check: a file written by the client adds
// write, which replaces read.
func saving(cl client, saved func(opts []opt, operands []word) bool) check {
	return func(c *Classifier, args []word, host string, depth int) set {
		s := cl.check(c, args, host, depth)
		if opts, operands, _ := scanRole(args[1:], cl.spec); saved(opts, operands) {
			return s&^of(Read) | of(Write)
		}
		return s
	}
}

func curlSaves(opts []opt, _ []word) bool { return saves(opts, curlOutputs) }

// wgetSaves: wget also saves each download in the working directory by default.
func wgetSaves(opts []opt, operands []word) bool {
	return saves(opts, wgetOutputs) || (len(operands) > 0 && !hasOpt(opts, wgetNoSave))
}

// less is write with a log file, as its path role (Ruling G19).
func less(_ *Classifier, args []word, _ string, _ int) set {
	opts, _, ok := scanRole(args[1:], pathRoles["less"].spec)
	switch {
	case !ok || dynamicHead(args[1:]):
		return opaque
	case saves(opts, lessOutputs):
		return of(Write)
	}
	return of(Read)
}

// git is opaque with an external diff or textconv driver: they run
// programs. Git takes these options only spelled out in full.
func git(c *Classifier, args []word, host string, depth int) set {
	for _, a := range args[1:] {
		if a.text == "--" {
			break
		}
		if a.text == "--ext-diff" || a.text == "--textconv" {
			return opaque
		}
	}
	return gitRule.eval(c, args, host, depth)
}

// awk is read only for a literal program given as the first operand, with
// only -F and -v options, that neither runs commands nor redirects output.
func awk(_ *Classifier, args []word, _ string, _ int) set {
	opts, operands, ok := scanRole(args[1:], pathRoles["awk"].spec)
	if !ok || dynamicHead(args[1:]) || len(operands) == 0 || !operands[0].lit {
		return opaque
	}
	for _, o := range opts {
		if !slices.ContainsFunc(awkSafe, o.matches) {
			return opaque
		}
	}
	if awkRuns(operands[0].text) {
		return opaque
	}
	return of(Read)
}

// awkRuns reports a program that runs commands or reads their output, holds
// a gawk directive (@include, @load, @namespace can pull in code that writes),
// or redirects output: awk writes to a file or command only after
// print/printf, so a comparison before the first print stays read.
func awkRuns(prog string) bool {
	if strings.Contains(prog, "system") || strings.Contains(prog, "getline") || strings.Contains(prog, "@") {
		return true
	}
	i := strings.Index(prog, "print")
	return i >= 0 && strings.ContainsAny(prog[i:], "|>")
}

// sed is opaque for a script from a file, a script that is not literal or
// does not parse, and one that writes files or runs commands; -i adds write.
func sed(_ *Classifier, args []word, _ string, _ int) set {
	opts, operands, ok := scanRole(args[1:], pathRoles["sed"].spec)
	if !ok || dynamicHead(args[1:]) || hasOpt(opts, []string{"-f", "--file"}) {
		return opaque
	}
	s := of(Read)
	if hasOpt(opts, []string{"-i", "--in-place"}) {
		s = of(Write)
	}
	if script, ok := sedScript(opts, operands); !ok || sedRuns(script) {
		return s&^of(Read) | opaque
	}
	return s
}

// sedScript joins the -e scripts, or takes the first operand without them.
func sedScript(opts []opt, operands []word) (string, bool) {
	var parts []string
	for _, o := range opts {
		if o.name != "-e" && o.name != "--expression" {
			continue
		}
		if !o.val.lit {
			return "", false
		}
		parts = append(parts, o.val.text)
	}
	if parts != nil {
		return strings.Join(parts, "\n"), true
	}
	if len(operands) == 0 || !operands[0].lit {
		return "", false
	}
	return operands[0].text, true
}
