package oauth

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// Token exchange (RFC 8693): a member's CLI access token and scope
// agent:<name> give an access token of one new run of the member's agent.

const (
	// AgentScopePrefix starts the exchange scope agent:<name> and the act.sub
	// agent:<agent id> of an agent token.
	AgentScopePrefix = "agent:"
	// AgentTokenMaxTTL caps an agent token; it never outlives its run.
	AgentTokenMaxTTL = time.Hour

	claimRun  = "werbot_run" // private claim of an agent token: its run
	maxClient = 128          // runes of werbot_client
	maxReason = 1024         // runes of werbot_reason
)

// agentName is the name of an agent (contract §3.4).
var agentName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// Agents starts agent runs for the token exchange (*agentic.Service through an
// adapter in internal/app). StartAgentRun creates the sponsor's agent name on
// first use and starts one run of it; client and reason describe the run.
// apperr.ErrPermissionDenied (agent frozen or locked) answers invalid_grant,
// any other error is a server error; either way no token is issued.
type Agents interface {
	StartAgentRun(ctx context.Context, orgID, userID uuid.UUID, name, client, reason string) (agentID, runID uuid.UUID, expiresAt time.Time, err error)
}

// agentGrant is an exchange in flight on reqInfo: its subject and parameters
// (ValidateTokenExchangeRequest), then the run Agents started (Create…).
type agentGrant struct {
	orgID, userID, sessionID uuid.UUID
	name, client, reason     string
	agentID, runID           uuid.UUID
	expiresAt                time.Time
}

var _ op.TokenExchangeStorage = (*storage)(nil)

// ValidateTokenExchangeRequest implements op.TokenExchangeStorage. op has
// checked the client and the subject token's signature, issuer and expiry.
// Here: access tokens only, no actor, audience exactly werbot-api, one scope
// agent:<name>, werbot_client and werbot_reason within bounds, and a
// non-delegated werbot-api subject whose cli_session is live and whose user is
// still a member. It writes nothing.
func (s *storage) ValidateTokenExchangeRequest(ctx context.Context, req op.TokenExchangeRequest) error {
	if s.agents == nil {
		return oidc.ErrUnsupportedGrantType().WithDescription("token exchange is not served")
	}
	if err := checkExchangeTypes(req); err != nil {
		return err
	}
	name, err := agentScope(req.GetScopes())
	if err != nil {
		return err
	}
	ri := requestInfo(ctx)
	client, reason, err := exchangeParams(ri)
	if err != nil {
		return err
	}
	g, err := subjectOf(req)
	if err != nil {
		return err
	}
	if err := s.liveSubject(ctx, g); err != nil {
		return err
	}
	g.name, g.client, g.reason = name, client, reason
	req.SetCurrentScopes([]string{AgentScopePrefix + name})
	ri.grant = g
	return nil
}

// CreateTokenExchangeRequest implements op.TokenExchangeStorage: it starts
// the run; the token step reads it from reqInfo.
func (s *storage) CreateTokenExchangeRequest(ctx context.Context, _ op.TokenExchangeRequest) error {
	g := requestInfo(ctx).grant
	if g == nil {
		return errNoFlow
	}
	agentID, runID, expiresAt, err := s.agents.StartAgentRun(ctx, g.orgID, g.userID, g.name, g.client, g.reason)
	if errors.Is(err, apperr.ErrPermissionDenied) {
		return oidc.ErrInvalidGrant().WithDescription("agent is frozen or locked")
	}
	if err != nil {
		return err
	}
	g.agentID, g.runID, g.expiresAt = agentID, runID, expiresAt
	return nil
}

// GetPrivateClaimsFromTokenExchangeRequest implements
// op.TokenExchangeStorage: the organization, the agent as actor and the run.
func (*storage) GetPrivateClaimsFromTokenExchangeRequest(ctx context.Context, _ op.TokenExchangeRequest) (map[string]any, error) {
	g := requestInfo(ctx).grant
	if g == nil || g.runID == uuid.Nil {
		return nil, errNoFlow
	}
	return map[string]any{
		claimOrg: g.orgID.String(),
		"act":    map[string]any{"sub": AgentScopePrefix + g.agentID.String()},
		claimRun: g.runID.String(),
	}, nil
}

// SetUserinfoFromTokenExchangeRequest implements op.TokenExchangeStorage:
// only sub (the exchange never issues ID tokens).
func (*storage) SetUserinfoFromTokenExchangeRequest(_ context.Context, info *oidc.UserInfo, req op.TokenExchangeRequest) error {
	info.Subject = req.GetSubject()
	return nil
}

// exchangeToken is the access token of a started exchange: its id is the
// sponsor's cli_session and it ends with the run, at most AgentTokenMaxTTL
// from now. ok is false outside an exchange.
func exchangeToken(ctx context.Context, request op.TokenRequest) (id string, exp time.Time, ok bool) {
	g := requestInfo(ctx).grant
	if _, isExchange := request.(op.TokenExchangeRequest); !isExchange || g == nil || g.runID == uuid.Nil {
		return "", time.Time{}, false
	}
	exp = time.Now().Add(AgentTokenMaxTTL)
	if g.expiresAt.Before(exp) {
		exp = g.expiresAt
	}
	return g.sessionID.String(), exp, true
}

// checkExchangeTypes: an access token in, an access token out (an empty
// requested_token_type means one), no actor, audience exactly werbot-api.
func checkExchangeTypes(req op.TokenExchangeRequest) error {
	switch req.GetRequestedTokenType() {
	case "":
		req.SetRequestedTokenType(oidc.AccessTokenType)
	case oidc.AccessTokenType:
	default:
		return oidc.ErrInvalidRequest().WithDescription("requested_token_type must be an access token")
	}
	if req.GetExchangeSubjectTokenType() != oidc.AccessTokenType {
		return oidc.ErrInvalidRequest().WithDescription("subject_token must be an access token")
	}
	if req.GetExchangeActorTokenType() != "" || req.GetExchangeActorTokenIDOrToken() != "" {
		return oidc.ErrInvalidRequest().WithDescription("actor_token is not supported")
	}
	if !slices.Equal(req.GetAudience(), []string{Audience}) {
		return oidc.ErrInvalidTarget().WithDescription("audience must be " + Audience)
	}
	return nil
}

// agentScope returns the name of the only scope agent:<name>.
func agentScope(scopes []string) (string, error) {
	if len(scopes) == 1 {
		if name, ok := strings.CutPrefix(scopes[0], AgentScopePrefix); ok && agentName.MatchString(name) {
			return name, nil
		}
	}
	return "", oidc.ErrInvalidScope().WithDescription("scope must be one agent:<name>")
}

// exchangeParams: werbot_client of 1–128 runes (none is "unknown") and
// werbot_reason of up to 1024; control characters are already dropped.
func exchangeParams(ri *reqInfo) (client, reason string, err error) {
	client = ri.client
	if client == "" {
		client = "unknown"
	}
	if utf8.RuneCountInString(client) > maxClient || utf8.RuneCountInString(ri.reason) > maxReason {
		return "", "", oidc.ErrInvalidRequest().WithDescription("werbot_client or werbot_reason is too long")
	}
	return client, ri.reason, nil
}

// subjectOf reads the subject token's claims as VerifyAccessToken does
// (aud ∋ werbot-api, UUID sub, jti and werbot_org) and refuses an agent token.
func subjectOf(req op.TokenExchangeRequest) (*agentGrant, error) {
	c := req.GetExchangeSubjectTokenClaims()
	_, act := c["act"]
	_, run := c[claimRun]
	if act || run {
		return nil, oidc.ErrInvalidGrant().WithDescription("subject token is already delegated")
	}
	org, _ := c[claimOrg].(string)
	user, err1 := parseID(req.GetExchangeSubject())
	sess, err2 := parseID(req.GetExchangeSubjectTokenIDOrToken())
	orgID, err3 := parseID(org)
	if !hasAudience(c["aud"]) || errors.Join(err1, err2, err3) != nil {
		return nil, oidc.ErrInvalidGrant().WithDescription("subject token is not a " + Audience + " access token")
	}
	return &agentGrant{orgID: orgID, userID: user, sessionID: sess}, nil
}

// hasAudience: the raw aud claim (a string or a list) holds werbot-api.
func hasAudience(aud any) bool {
	switch a := aud.(type) {
	case string:
		return a == Audience
	case []any:
		return slices.Contains(a, any(Audience))
	}
	return false
}

// liveSubject: the subject's cli_session is live and its user still a member
// (as the Bearer middleware checks); otherwise invalid_grant.
func (s *storage) liveSubject(ctx context.Context, g *agentGrant) error {
	err := dbx.ReadInOrg(s.pool, g.orgID, g.userID, func(rd *dbx.Read) error {
		_, err := dbq.New(rd).LiveCLISessionMember(ctx, dbq.LiveCLISessionMemberParams{ID: g.sessionID, OrgID: g.orgID, UserID: g.userID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return oidc.ErrInvalidGrant().WithDescription("cli session ended")
	}
	return err
}
