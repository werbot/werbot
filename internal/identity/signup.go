package identity

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// signupLogin derives the login of a user an IdP provisions from the local
// part of email (users.login CHECK ^[a-z0-9][a-z0-9_-]{1,31}$): lowercased,
// other characters as "-", leading non-alphanumerics dropped, cut to 28
// (room for a "-NNN" suffix); shorter than 2 gives "user".
func signupLogin(email string) string {
	local := strings.ToLower(email[:max(strings.LastIndexByte(email, '@'), 0)])
	login := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, local)
	login = strings.TrimLeft(login, "_-")
	if len(login) > 28 {
		login = login[:28]
	}
	if len(login) < 2 {
		return "user"
	}
	return login
}

// maxLoginSuffix bounds the "-2", "-3", … tries of uniqueLogin.
const maxLoginSuffix = 100

// uniqueLogin is base, or base with the first free "-N" suffix.
func uniqueLogin(ctx context.Context, st Store, base string) (string, error) {
	login := base
	for n := 2; n <= maxLoginSuffix; n++ {
		taken, err := st.LoginTaken(ctx, login)
		if err != nil || !taken {
			return login, err
		}
		login = base + "-" + strconv.Itoa(n)
	}
	return "", fmt.Errorf("no free login for %s: %w", base, apperr.ErrAlreadyExists)
}
