package oauth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func jwtPayload(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

// sign signs claims as a JWT with key under alg and kid.
func sign(t *testing.T, alg jose.SignatureAlgorithm, key any, kid string, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid))
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	jws, err := signer.Sign(payload)
	require.NoError(t, err)
	out, err := jws.CompactSerialize()
	require.NoError(t, err)
	return out
}

// TestVerifyAccessToken: ES256 by the signing key, our issuer, unexpired,
// aud ∋ werbot-api and UUID sub, jti and werbot_org — nothing else passes.
func TestVerifyAccessToken(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	code, verifier := login(t, s, s.fx.MemberID)
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	var sessID uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT id FROM cli_session").Scan(&sessID))

	c, err := s.p.VerifyAccessToken(ctx, tr.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.fx.MemberID, c.UserID)
	assert.Equal(t, s.fx.OrgID, c.OrgID)
	assert.Equal(t, sessID, c.CLISessionID)
	assert.Nil(t, c.AgentID)
	assert.Nil(t, c.RunID)
	assert.WithinDuration(t, time.Now().Add(oauth.AccessTokenTTL), c.Expiry, 5*time.Second)

	priv, ok := s.key.Key().(*ecdsa.PrivateKey)
	require.True(t, ok)
	good := func() map[string]any {
		return map[string]any{
			"iss": s.srv.URL, "sub": uuid.NewString(), "aud": []string{oauth.Audience}, "jti": uuid.NewString(),
			"werbot_org": uuid.NewString(), "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(),
		}
	}
	with := func(k string, v any) map[string]any {
		m := good()
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
		return m
	}
	_, err = s.p.VerifyAccessToken(ctx, sign(t, jose.ES256, priv, s.key.ID(), good()))
	require.NoError(t, err, "a hand-made token with the right key and claims passes")

	otherEC, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	otherRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	bad := map[string]string{
		"other key":          sign(t, jose.ES256, otherEC, s.key.ID(), good()),
		"RS256":              sign(t, jose.RS256, otherRSA, s.key.ID(), good()),
		"HS256":              sign(t, jose.HS256, []byte(strings.Repeat("k", 32)), s.key.ID(), good()),
		"expired":            sign(t, jose.ES256, priv, s.key.ID(), with("exp", time.Now().Add(-time.Minute).Unix())),
		"other iss":          sign(t, jose.ES256, priv, s.key.ID(), with("iss", "https://evil.example")),
		"aud without api":    sign(t, jose.ES256, priv, s.key.ID(), with("aud", []string{oauth.ClientCLI})),
		"no aud":             sign(t, jose.ES256, priv, s.key.ID(), with("aud", nil)),
		"sub not uuid":       sign(t, jose.ES256, priv, s.key.ID(), with("sub", "alice")),
		"no jti":             sign(t, jose.ES256, priv, s.key.ID(), with("jti", nil)),
		"jti not uuid":       sign(t, jose.ES256, priv, s.key.ID(), with("jti", "x")),
		"no werbot_org":      sign(t, jose.ES256, priv, s.key.ID(), with("werbot_org", nil)),
		"werbot_org not str": sign(t, jose.ES256, priv, s.key.ID(), with("werbot_org", 7)),
		"nil org":            sign(t, jose.ES256, priv, s.key.ID(), with("werbot_org", uuid.Nil.String())),
		"garbage":            "not.a.jwt",
		"empty":              "",
	}
	for name, token := range bad {
		_, err := s.p.VerifyAccessToken(ctx, token)
		assert.ErrorIs(t, err, apperr.ErrUnauthenticated, name)
	}
}

// TestVerifyRejectsIDToken: the ID token of the same exchange shares aud
// werbot-api but has no jti and no werbot_org, so it is never a Bearer token.
func TestVerifyRejectsIDToken(t *testing.T) {
	s := newStand(t)
	code, verifier := login(t, s, s.fx.MemberID)
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	require.NotEmpty(t, tr.IDToken)
	id := jwtPayload(t, tr.IDToken)
	assert.Contains(t, id["aud"], oauth.Audience)
	assert.Equal(t, s.fx.MemberID.String(), id["sub"], "same sub: only jti and werbot_org tell it apart")
	_, err := s.p.VerifyAccessToken(context.Background(), tr.IDToken)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
}
