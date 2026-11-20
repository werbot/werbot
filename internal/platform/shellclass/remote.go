package shellclass

import "strings"

// Network clients with their own rules: ssh, scp, rsync, socat.

// ssh reaches the host operand; a command for a local host runs there and is opaque.
func ssh(_ *Classifier, args []word, host string, _ int) set {
	opts, operands, ok := scanOpts(args[1:], sshSpec, true)
	if !ok || len(operands) == 0 {
		return of(Egress)
	}
	s := optionRisk(opts, nil, true)
	if s&of(Egress) != 0 {
		return s
	}
	s |= hostClass(firstHost(opts, operands), host)
	if s&of(Read) != 0 && len(operands) > 1 {
		s |= opaque
	}
	return s
}

var scpSpec = optSpec{value: "cFiJloPS", loose: true}

// scp in server mode (-t sink, -f source) writes or reads; otherwise it copies files.
func scp(_ *Classifier, args []word, host string, _ int) set {
	opts, operands, ok := scanOpts(args[1:], scpSpec, false)
	switch {
	case !ok:
		return of(Egress)
	case has(opts, "-t"):
		return of(Write)
	case has(opts, "-f"):
		return of(Read)
	}
	s := optionRisk(opts, nil, true)
	if s&of(Egress) != 0 {
		return s
	}
	return s | copyClass(operands, jumpHosts(opts), host)
}

var rsyncSpec = optSpec{value: "eBfMT", loose: true, long: longs("rsh exclude include exclude-from include-from " +
	"files-from filter rsync-path password-file log-file backup-dir temp-dir compare-dest link-dest copy-dest " +
	"chown chmod port timeout bwlimit max-size min-size suffix out-format partial-dir")}

// rsync in server mode reads (--sender) or writes; otherwise it copies files.
func rsync(_ *Classifier, args []word, host string, _ int) set {
	opts, operands, ok := scanOpts(args[1:], rsyncSpec, false)
	switch {
	case !ok:
		return of(Egress)
	case has(opts, "--server") && has(opts, "--sender"):
		return of(Read)
	case has(opts, "--server"):
		return of(Write)
	case has(opts, "-e", "--rsh", "--rsync-path"):
		return opaque | copyClass(operands, nil, host)
	}
	return copyClass(operands, nil, host)
}

// copyClass: a copy writes files; a remote operand off the loopback and resource host is egress.
func copyClass(operands []word, hosts []string, resourceHost string) set {
	for _, o := range operands {
		if !o.lit {
			return of(Egress)
		}
		if h, ok := remoteHost(o.text); ok {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) > 0 && hostClass(hosts, resourceHost) == of(Egress) {
		return of(Egress)
	}
	return of(Write)
}

// socatNet are socat address types that connect to a host.
var socatNet = []string{"TCP", "UDP", "SCTP", "DCCP", "OPENSSL", "SSL", "PROXY", "SOCKS", "IP"}

// socat is opaque for program addresses, otherwise classified by its network hosts.
func socat(_ *Classifier, args []word, host string, _ int) set {
	_, operands, ok := scanOpts(args[1:], optSpec{loose: true}, false)
	if !ok {
		return of(Egress)
	}
	var hosts []string
	for _, o := range operands {
		kind, params, _ := strings.Cut(o.text, ":")
		kind = strings.ToUpper(kind)
		switch {
		case !o.lit:
			return of(Egress)
		case strings.HasPrefix(kind, "EXEC") || strings.HasPrefix(kind, "SYSTEM"):
			return opaque
		case hasAnyPrefix(kind, socatNet):
			addr, _, _ := strings.Cut(params, ",")
			hosts = append(hosts, hostOf(addr))
		}
	}
	return hostClass(hosts, host)
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
