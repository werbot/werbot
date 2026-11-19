package clientauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Profile is one logged-in server: <home>/<server-host>/profile.json (0600).
type Profile struct {
	Server       string    `json:"server"`
	OrgID        uuid.UUID `json:"org_id"`
	UserID       uuid.UUID `json:"user_id"`
	Login        string    `json:"login"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	AccessExpiry time.Time `json:"access_expiry"`

	home string
	mu   sync.Mutex // guards the token fields across goroutines of one process
}

// Home is WERBOT_HOME, else <user home>/.werbot.
func Home(getenv func(string) string) (string, error) {
	if h := getenv(envHome); h != "" {
		return h, nil
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("%s is not set and the home directory is unknown: %w", envHome, err)
	}
	return filepath.Join(user, ".werbot"), nil
}

// Dir is the profile directory of server: its host:port with every byte
// outside [A-Za-z0-9.-] replaced by "_".
func Dir(home, server string) string {
	host := server
	if u, err := url.Parse(server); err == nil && u.Host != "" {
		host = u.Host
	}
	return filepath.Join(home, strings.Map(func(r rune) rune {
		if r == '.' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, host))
}

// Current loads the profile of WERBOT_SERVER, else of the last login (<home>/current).
func Current(home string, getenv func(string) string) (*Profile, error) {
	server := getenv(envServer)
	if server == "" {
		raw, err := os.ReadFile(filepath.Join(home, "current")) //nolint:gosec // the user's own profile home
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotLoggedIn
		}
		if err != nil {
			return nil, err
		}
		server = strings.TrimSpace(string(raw))
	}
	server, err := normalizeServer(server)
	if err != nil {
		return nil, err
	}
	return load(home, server)
}

func load(home, server string) (*Profile, error) {
	raw, err := os.ReadFile(filepath.Join(Dir(home, server), "profile.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	p := &Profile{home: home}
	if err := json.Unmarshal(raw, p); err != nil {
		return nil, fmt.Errorf("read profile of %s: %w", server, err)
	}
	return p, nil
}

// Save writes the profile and makes it the current one.
func (p *Profile) Save(home string) error {
	p.home = home
	if err := p.write(); err != nil {
		return err
	}
	return writeFile(filepath.Join(home, "current"), []byte(p.Server))
}

// write stores profile.json under a 0700 directory.
func (p *Profile) write() error {
	dir := Dir(p.home, p.Server)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(p) //nolint:gosec // profile.json holds the tokens on purpose; written 0600
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "profile.json"), raw)
}

// writeFile replaces path atomically with a 0600 file (Ruling R17: rename
// replaces on Windows too).
func writeFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".tmp-*") // mode 0600
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		_ = os.Remove(f.Name())
	}
	return err
}

// Logout revokes the refresh token and always removes the profile; a revoke
// failure is returned after the removal.
func (p *Profile) Logout(ctx context.Context, home string) error {
	err := postForm(ctx, p.Server+"/oauth/revoke", url.Values{
		"client_id": {clientID}, "token": {p.RefreshToken}, "token_type_hint": {"refresh_token"},
	}, nil)
	if err != nil {
		err = fmt.Errorf("revoke the session on %s: %w", p.Server, err)
	}
	errs := []error{err, os.RemoveAll(Dir(home, p.Server))}
	current := filepath.Join(home, "current")
	if raw, rerr := os.ReadFile(current); rerr == nil && strings.TrimSpace(string(raw)) == p.Server { //nolint:gosec // the user's own profile home
		errs = append(errs, os.Remove(current))
	}
	return errors.Join(errs...)
}
