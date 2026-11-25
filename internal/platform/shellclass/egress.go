package shellclass

import (
	"net/url"
	"slices"
	"strings"
)

// Network clients are read when every host they reach is the loopback or the
// resource host, and egress otherwise or when no host is found.

var localHosts = []string{"localhost", "127.0.0.1", "::1"}

func hostClass(hosts []string, resourceHost string) set {
	if len(hosts) == 0 {
		return of(Egress)
	}
	for _, h := range hosts {
		h = strings.ToLower(strings.Trim(h, "[]"))
		if h == "" || (!slices.Contains(localHosts, h) && !strings.EqualFold(h, resourceHost)) {
			return of(Egress)
		}
	}
	return of(Read)
}

// client is a network client with generic option handling.
type client struct {
	spec  optSpec
	risk  map[string]set // options that force classes (a proxy, a local program)
	ssh   bool           // ssh-style -o/-F/-S options
	hosts func(opts []opt, operands []word) []string
}

func (cl client) check(_ *Classifier, args []word, host string, _ int) set {
	opts, operands, ok := scanOpts(args[1:], cl.spec, false)
	if !ok {
		return of(Egress)
	}
	s := optionRisk(opts, cl.risk, cl.ssh)
	if s&of(Egress) != 0 {
		return s
	}
	return s | hostClass(cl.hosts(opts, operands), host)
}

func optionRisk(opts []opt, risk map[string]set, ssh bool) set {
	var s set
	for _, o := range opts {
		s |= risk[o.name]
		if ssh {
			s |= sshOption(o)
		}
	}
	return s
}

// sshOption: -o may change the destination (egress) or run a local command
// (opaque); -F reads such options from a file; -S names a program or socket.
func sshOption(o opt) set {
	switch o.name {
	case "-o":
		if strings.Contains(strings.ToLower(o.val.text), "command") {
			return of(Egress, Opaque)
		}
		return of(Egress)
	case "-F", "-S":
		return opaque
	}
	return 0
}

func urlHost(s string) string {
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// hostOf returns the host of "[user@]host[:port]", "[v6]:port" or a URL.
func hostOf(s string) string {
	if strings.Contains(s, "://") {
		return urlHost(s)
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if rest, ok := strings.CutPrefix(s, "["); ok {
		h, _, _ := strings.Cut(rest, "]")
		return h
	}
	if strings.Count(s, ":") == 1 {
		s, _, _ = strings.Cut(s, ":")
	}
	return s
}

// remoteHost returns the host of a remote copy operand "[user@]host:path" or a URL.
func remoteHost(s string) (string, bool) {
	if strings.Contains(s, "://") {
		return urlHost(s), true
	}
	if at := strings.Index(s, "@"); at >= 0 && !strings.Contains(s[:at], "/") {
		s = s[at+1:]
	}
	if rest, ok := strings.CutPrefix(s, "["); ok {
		h, tail, ok := strings.Cut(rest, "]")
		return h, ok && strings.HasPrefix(tail, ":")
	}
	h, _, ok := strings.Cut(s, ":")
	return h, ok && !strings.Contains(h, "/")
}

func urlHosts(opts []opt, operands []word) []string {
	var hosts []string
	for _, o := range opts {
		if o.name == "--url" {
			hosts = append(hosts, urlHost(o.val.text))
		}
	}
	for _, w := range operands {
		hosts = append(hosts, urlHost(w.text))
	}
	return hosts
}

// firstHost takes the host from the first operand and from -J/-W values.
func firstHost(opts []opt, operands []word) []string {
	hosts := jumpHosts(opts)
	if len(operands) > 0 {
		hosts = append(hosts, hostOf(operands[0].text))
	}
	return hosts
}

func jumpHosts(opts []opt) []string {
	var hosts []string
	for _, o := range opts {
		if o.name != "-J" && o.name != "-W" {
			continue
		}
		for h := range strings.SplitSeq(o.val.text, ",") {
			hosts = append(hosts, hostOf(h))
		}
	}
	return hosts
}

var (
	curlEgress = "-x|--proxy|--preproxy|-K|--config|--connect-to|--resolve|--socks4|--socks4a|--socks5|--socks5-hostname"
	curlLongs  = "data data-binary data-raw data-urlencode data-ascii header request user output upload-file " +
		"form cookie proxy preproxy config url connect-to resolve socks4 socks4a socks5 socks5-hostname " +
		"dump-header cookie-jar trace trace-ascii stderr libcurl etag-save hsts alt-svc"
	wgetLongs = "output-document output-file execute input-file user-agent directory-prefix tries timeout " +
		"header post-data post-file user password base append-output"
	sshSpec = optSpec{value: "BbcDEeFIiJLlmOoPpQRSWw", loose: true}
)

func longs(names string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(names) {
		m[n] = true
	}
	return m
}

func addClients(add func(rule, string)) {
	add(rule{check: saving(client{spec: optSpec{value: "odDHXuAeTFbcxK", long: longs(curlLongs), loose: true},
		risk: keyed(map[Class]string{Egress: curlEgress}), hosts: urlHosts}, curlSaves)}, "curl")
	add(rule{check: saving(client{spec: optSpec{value: "OoeUPaitTwQlDBIXAR", long: longs(wgetLongs), loose: true},
		risk: keyed(map[Class]string{Egress: "-e|--execute|-i|--input-file|-B|--base"}), hosts: urlHosts}, wgetSaves)}, "wget")
	add(rule{check: client{spec: optSpec{value: "IiOpqsTVwXxPec", long: longs("exec sh-exec lua-exec proxy proxy-type source"), loose: true},
		risk:  keyed(map[Class]string{Opaque: "-e|-c|--exec|--sh-exec|--lua-exec", Egress: "-x|--proxy"}),
		hosts: firstHost}.check}, "nc ncat netcat")
	add(rule{check: client{spec: optSpec{value: "beln", loose: true}, hosts: firstHost}.check}, "telnet ftp")
	add(rule{check: client{spec: optSpec{value: "BbcDFiJloPRSs", loose: true}, ssh: true,
		risk: keyed(map[Class]string{Opaque: "-D"}), hosts: firstHost}.check}, "sftp")
	add(rule{check: ssh}, "ssh")
	add(rule{check: scp}, "scp")
	add(rule{check: rsync}, "rsync")
	add(rule{check: socat}, "socat")
}

// netDevice returns the host of a bash /dev/tcp/<host>/<port> or /dev/udp/… path.
func netDevice(s string) (string, bool) {
	for _, p := range []string{"/dev/tcp/", "/dev/udp/"} {
		if rest, ok := strings.CutPrefix(s, p); ok {
			h, _, _ := strings.Cut(rest, "/")
			return h, true
		}
	}
	return "", false
}
