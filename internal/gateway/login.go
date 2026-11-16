package gateway

import (
	"errors"
	"regexp"
	"strings"
)

var (
	loginRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,31}$`)
	resourceRE = regexp.MustCompile(`^[a-z0-9-]{1,63}$`)
)

// ParseLogin splits an SSH user name: "<login>" opens the resource menu (PTY
// only), "<login>+<resource>" connects to the resource directly.
func ParseLogin(user string) (login, resource string, err error) {
	login, resource, direct := strings.Cut(user, "+")
	if !loginRE.MatchString(login) || direct && !resourceRE.MatchString(resource) {
		return "", "", errors.New("invalid login: want <login> or <login>+<resource>")
	}
	return login, resource, nil
}
