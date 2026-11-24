package oidclogin

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// linked signs bob in through the stand's IdP (a cookie session and a link
// with a refresh token), gives him a CLI session and ages the link by age.
func (s *stand) linked(age time.Duration) uuid.UUID {
	s.t.Helper()
	bob := s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})
	_, ok := s.session(s.login("/", nil))
	require.True(s.t, ok)
	s.exec(`INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, expires_at, absolute_expires_at)
		VALUES ($1, $2, 'werbot-cli', '\x01', now() + interval '1 day', now() + interval '1 day')`, s.org, bob)
	s.age(age)
	return bob
}

func (s *stand) exec(sql string, args ...any) {
	s.t.Helper()
	_, err := s.pool.Exec(context.Background(), sql, args...)
	require.NoError(s.t, err)
}

// age sets the last check of bob's link to age ago.
func (s *stand) age(age time.Duration) {
	s.exec("UPDATE user_idp_link SET last_checked_at = now() - make_interval(secs => $1)", age.Seconds())
}

// linkState is bob's link: the opened refresh token ("" without one) and its last check.
func (s *stand) linkState() (refresh string, checked time.Time) {
	s.t.Helper()
	var sealed []byte
	var keyID *string
	require.NoError(s.t, s.pool.QueryRow(context.Background(),
		"SELECT refresh_ciphertext, key_id, last_checked_at FROM user_idp_link WHERE subject = 'sub-bob'").Scan(&sealed, &keyID, &checked))
	if sealed == nil {
		return "", checked
	}
	plain, err := s.box.Open(sealed, *keyID)
	require.NoError(s.t, err)
	return string(plain), checked
}

// sessions counts bob's cookie sessions and reads his CLI session's revoke reason.
func (s *stand) sessions(bob uuid.UUID) (cookies int, cliReason string) {
	s.t.Helper()
	ctx := context.Background()
	require.NoError(s.t, s.pool.QueryRow(ctx, "SELECT count(*) FROM user_session WHERE user_id = $1", bob).Scan(&cookies))
	require.NoError(s.t, s.pool.QueryRow(ctx, "SELECT coalesce(revoke_reason, '') FROM cli_session WHERE user_id = $1", bob).Scan(&cliReason))
	return cookies, cliReason
}

func TestRecheckRevokesOnInvalidGrant(t *testing.T) {
	s := newStand(t)
	bob := s.linked(2 * time.Hour)
	s.op.Disable("sub-bob")

	require.NoError(t, Recheck(context.Background(), s.store, s.svc, s.cfg))
	cookies, reason := s.sessions(bob)
	assert.Zero(t, cookies, "cookie sessions deleted")
	assert.Equal(t, "idp", reason)
	refresh, _ := s.linkState()
	assert.Empty(t, refresh, "the refused refresh token is dropped")

	var actor, target string
	var data map[string]any
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT actor_kind, target_id, data FROM audit_event WHERE action = 'idp.recheck.revoke' AND org_id = $1", s.org).Scan(&actor, &target, &data))
	assert.Equal(t, "system", actor)
	assert.Equal(t, bob.String(), target)
	assert.Equal(t, map[string]any{"idp_id": s.idp.ID.String(), "subject": "sub-bob"}, data)
}

// rtFunc is an http.RoundTripper.
type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRecheckKeepsSessionsWhenIdPDown(t *testing.T) {
	opTransport := func(s *stand) http.RoundTripper { return s.op.Client().Transport }
	cases := map[string]func(s *stand){
		"503 everywhere": func(s *stand) { s.op.SetDown(true) },
		"5xx at the token endpoint": func(s *stand) {
			s.cfg.HTTPClient = &http.Client{Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					return &http.Response{StatusCode: http.StatusBadGateway, Body: io.NopCloser(strings.NewReader("bad gateway")), Request: r}, nil
				}
				return opTransport(s).RoundTrip(r)
			})}
		},
		"timeout": func(s *stand) {
			s.cfg.HTTPClient = &http.Client{Timeout: 50 * time.Millisecond, Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
				<-r.Context().Done()
				return nil, r.Context().Err()
			})}
		},
		"network": func(s *stand) { s.op.Close() },
	}
	for name, down := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStand(t)
			bob := s.linked(2 * time.Hour)
			s.op.Disable("sub-bob") // only an explicit refusal could revoke
			before, checked := s.linkState()
			down(s)

			require.NoError(t, Recheck(context.Background(), s.store, s.svc, s.cfg))
			cookies, reason := s.sessions(bob)
			assert.Equal(t, 1, cookies, "cookie session alive")
			assert.Empty(t, reason, "CLI session alive")
			after, checkedAfter := s.linkState()
			assert.Equal(t, before, after)
			assert.True(t, checked.Equal(checkedAfter), "last_checked_at unchanged")
		})
	}
}

func TestRecheckRotatesRefresh(t *testing.T) {
	s := newStand(t)
	bob := s.linked(2 * time.Hour)
	before, _ := s.linkState()

	require.NoError(t, Recheck(context.Background(), s.store, s.svc, s.cfg))
	after, checked := s.linkState()
	require.NotEmpty(t, after)
	assert.NotEqual(t, before, after, "the IdP's new refresh token is stored")
	assert.WithinDuration(t, time.Now(), checked, 5*time.Second)
	var sealed []byte
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT refresh_ciphertext FROM user_idp_link WHERE subject = 'sub-bob'").Scan(&sealed))
	assert.False(t, bytes.Contains(sealed, []byte(after)), "stored sealed")
	cookies, reason := s.sessions(bob)
	assert.Equal(t, 1, cookies)
	assert.Empty(t, reason)
}

func TestRecheckSelectsStaleOnly(t *testing.T) {
	s := newStand(t)
	bob := s.linked(30 * time.Minute)
	s.op.Disable("sub-bob")

	require.NoError(t, Recheck(context.Background(), s.store, s.svc, s.cfg))
	cookies, _ := s.sessions(bob)
	assert.Equal(t, 1, cookies, "checked 30 minutes ago: not asked")

	s.exec("UPDATE user_idp_link SET refresh_ciphertext = NULL, key_id = NULL")
	s.age(2 * time.Hour)
	require.NoError(t, Recheck(context.Background(), s.store, s.svc, s.cfg))
	cookies, _ = s.sessions(bob)
	assert.Equal(t, 1, cookies, "no refresh token: nothing to ask")
}

func TestRecheckUser(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	bob := s.linked(30 * time.Minute)
	other := dbtest.User(t, s.pool, s.org, "member")
	s.op.Disable("sub-bob")

	assert.NoError(t, RecheckUser(ctx, s.store, s.svc, s.cfg, s.org, bob), "fresh link: not asked")
	s.age(2 * time.Hour)
	assert.NoError(t, RecheckUser(ctx, s.store, s.svc, s.cfg, s.org, other), "another user's link is not asked")
	cookies, _ := s.sessions(bob)
	assert.Equal(t, 1, cookies)

	s.op.SetDown(true)
	assert.NoError(t, RecheckUser(ctx, s.store, s.svc, s.cfg, s.org, bob), "an unreachable IdP does not refuse")
	s.op.SetDown(false)
	assert.ErrorIs(t, RecheckUser(ctx, s.store, s.svc, s.cfg, s.org, bob), ErrIdPRejected)
	cookies, reason := s.sessions(bob)
	assert.Zero(t, cookies)
	assert.Equal(t, "idp", reason)
}

func TestRecheckJob(t *testing.T) {
	assert.Equal(t, "idp.recheck", RecheckArgs{}.Kind())
	assert.Equal(t, jobs.QueueIdP, RecheckArgs{}.InsertOpts().Queue)
	args, err := json.Marshal(RecheckArgs{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(args), "job args carry no secrets")
	assert.Equal(t, 10*time.Minute, RecheckInterval)
	assert.Equal(t, 50*time.Minute, RecheckAfter, "worst case 50 + 10 minutes")

	s := newStand(t)
	workers := river.NewWorkers()
	periodic := RegisterJobs(workers, s.store, s.svc, s.cfg)
	require.Len(t, periodic, 1)
	c, err := jobs.NewClient(s.pool, workers, periodic, nil)
	require.NoError(t, err)
	assert.True(t, c.PeriodicJobs().RemoveByID("idp.recheck"))
}
