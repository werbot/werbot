package shellclass

import (
	"slices"
	"testing"
)

// Input classes "awk or sed program that writes files or runs commands, program from a file or not
// literal, output option of less, curl or wget, git external diff or textconv" are opaque or write;
// each clean twin keeps its old class (owner decision O1).
func TestScriptFormsWriteOrRun(t *testing.T) {
	classifyAll(t, map[string]string{
		// awk: run, getline, output redirection after print/printf, program from a file or option.
		`awk 'BEGIN{system("id")}'`:                     "opaque",
		`awk '{print > "/tmp/x"}' f`:                    "opaque",
		`awk '{printf "%s", $1 >> "/tmp/x"}' f`:         "opaque",
		`awk '{print | "sort"}' f`:                      "opaque",
		`awk '{"date" | getline d; print d}'`:           "opaque",
		`awk '{print $1} $3 > 100' f`:                   "opaque",
		`awk -f prog.awk f`:                             "opaque",
		`awk --file=prog.awk f`:                         "opaque",
		`awk -e '{print}' f`:                            "opaque",
		`awk -E prog.awk f`:                             "opaque",
		`awk "$P" f`:                                    "opaque",
		`awk -F, -v n=1`:                                "opaque",
		`awk '{print $1}' /var/log/syslog`:              "read",
		`awk '$3 > 100 {print $1}' f`:                   "read",
		`awk -F: -v OFS=, '{print $1, $3}' /etc/passwd`: "read",
		`awk -- '{print $1}' f`:                         "read",
		// sed: e, w, W commands and s flags, script from a file, unparsed script.
		`sed 's/a/b/e' f`:                      "opaque",
		`sed -n 'w /tmp/x' f`:                  "opaque",
		`sed '1e id' f`:                        "opaque",
		`sed 's/a/b/w /tmp/x' f`:               "opaque",
		`sed 's/a/b/gw /tmp/x' f`:              "opaque",
		`sed '$W /tmp/x' f`:                    "opaque",
		`sed -e 's/a/b/' -e '1w /tmp/x' f`:     "opaque",
		`sed --expression='1e id' f`:           "opaque",
		`sed -f script.sed f`:                  "opaque",
		`sed --file=script.sed f`:              "opaque",
		`sed "$S" f`:                           "opaque",
		`sed 's/a/b' f`:                        "opaque",
		`sed '1Z' f`:                           "opaque",
		`sed`:                                  "opaque",
		`sed -i 's/a/b/w /tmp/x' f`:            "opaque,write",
		`sed -i 's/a/b/' f`:                    "write",
		`sed -n '1,20p' /etc/nginx/nginx.conf`: "read",
		`sed 's/error/warn/g' f`:               "read",
		`sed 's|/w|/e|2' f`:                    "read",
		`sed -E 's/(a)/\1e/I' f`:               "read",
		`sed '/w/d;/e/!p' f`:                   "read",
		`sed -n '/x/{p;q}' f`:                  "read",
		`sed '1~2d;0,/re/s/a/b/' f`:            "read",
		`sed '$!N;P;D' f`:                      "read",
		`sed 'y/we/ew/' f`:                     "read",
		`sed '1a we write' f`:                  "read",
		`sed '1r /etc/hosts' f`:                "read",
		`sed '/^#/b end; s/a/b/; :end' f`:      "read",
		`sed -n -e '/x/p' -e '/y/=' f`:         "read",
		`sed '1q5' f`:                          "read",
		`sed '# a comment with w and e
p' f`: "read",
		// less, curl, wget: output to a file, quiet values.
		`less -o /tmp/x f`:                                  "write",
		`less -O/tmp/x f`:                                   "write",
		`less --log-file=/tmp/x f`:                          "write",
		`less --LOG-FILE /tmp/x f`:                          "write",
		`less --log-f /tmp/x f`:                             "write",
		`less -o $F f`:                                      "opaque",
		`less -o /dev/null f`:                               "read",
		`less -N +G f`:                                      "read",
		`curl -o /tmp/x http://localhost/`:                  "write",
		`curl -O http://localhost/f`:                        "write",
		`curl --remote-name http://localhost/f`:             "write",
		`curl --output=/tmp/x http://localhost/`:            "write",
		`curl --out /tmp/x http://localhost/`:               "egress,write",
		`curl -o /tmp/x https://203.0.113.5/`:               "egress,write",
		`curl -x http://p:3128 -o /tmp/x http://localhost/`: "egress,write",
		`curl -s -o /dev/null http://localhost/`:            "read",
		`curl -so/dev/null http://localhost/`:               "read",
		`curl -o - http://localhost/`:                       "read",
		`wget -O /tmp/x http://127.0.0.1/`:                  "write",
		`wget -o /tmp/log http://127.0.0.1/`:                "write",
		`wget -P /tmp http://127.0.0.1/f`:                   "write",
		`wget --output-document=/tmp/x http://127.0.0.1/`:   "write",
		`wget --directory-prefix /tmp http://127.0.0.1/f`:   "write",
		`wget -qO- http://127.0.0.1/`:                       "read",
		`wget -O /dev/null http://127.0.0.1/`:               "read",
		`wget -qO- http://203.0.113.5/x`:                    "egress",
		// curl and wget: other files they write; wget saves the download in the cwd by default.
		`curl -D /tmp/h http://localhost/`:                     "write",
		`curl --dump-header /tmp/h http://localhost/`:          "write",
		`curl -c /tmp/j http://localhost/`:                     "write",
		`curl --cookie-jar=/tmp/j http://localhost/`:           "write",
		`curl --trace /tmp/t http://localhost/`:                "write",
		`curl --stderr /tmp/e http://localhost/`:               "write",
		`curl --libcurl /tmp/c.c http://localhost/`:            "write",
		`curl --etag-save /tmp/e http://localhost/`:            "write",
		`curl --hsts /tmp/h https://localhost/`:                "write",
		`curl --alt-svc /tmp/a https://localhost/`:             "write",
		`curl --remote-name-all http://localhost/f`:            "write",
		`curl -sD /tmp/h https://203.0.113.5/`:                 "egress,write",
		`curl -D - http://localhost/`:                          "read",
		`curl --trace-ascii /dev/null http://localhost/`:       "read",
		`curl -b /tmp/j http://localhost/`:                     "read",
		`wget -a /tmp/log -O /dev/null http://127.0.0.1/`:      "write",
		`wget --append-output=/tmp/log -qO- http://127.0.0.1/`: "write",
		`wget http://127.0.0.1/f`:                              "write",
		`wget -q http://203.0.113.5/f`:                         "egress,write",
		`wget -a - -qO- http://127.0.0.1/`:                     "read",
		`wget --spider http://127.0.0.1/`:                      "read",
		`wget -O - http://127.0.0.1/`:                          "read",
		// git: external diff and textconv drivers run programs.
		`git diff --ext-diff`:           "opaque",
		`git log -p --textconv`:         "opaque",
		`git show --ext-diff HEAD`:      "opaque",
		`git -C /srv diff --ext-diff`:   "opaque",
		`git diff --no-ext-diff`:        "read",
		`git diff --text`:               "read",
		`git log --no-textconv -p`:      "read",
		`git diff HEAD -- --ext-diff`:   "read",
		`git log --output=/tmp/log.txt`: "read,write",
	})
}

// Input class "less with a log file": the class is write and the log file is a write path (Ruling G19).
func TestAnalyzeLessOutputIsWritePath(t *testing.T) {
	a := mustNew(t).Analyze("less -o /tmp/x f", corpusHost, "~")
	if join(a.Classes) != "write" || !slices.Contains(a.Paths, PathRef{Path: "/tmp/x", Access: AccessWrite}) {
		t.Fatalf("got %s %v, want write with /tmp/x written", join(a.Classes), a.Paths)
	}
}
