package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

var defaultSettings = OrgSettings{CLISessionTTL: 168 * time.Hour, CLISessionMax: 720 * time.Hour,
	GrantDefaultTTL: 8 * time.Hour, GrantMaxTTL: 14 * 24 * time.Hour, RequestPendingTTL: time.Hour,
	CommandApprovalTTL: 5 * time.Minute, LockMode: "best_effort", MaskOutput: "agents"}

func TestSetupCreatesOrgSetting(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	token, err := svc.Setup(ctx, setupInput("owner@example.com", "owner"), "test")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	got, err := svc.Settings(ctx, p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, defaultSettings, got)

	got, err = svc.Settings(ctx, dbtest.Org(t, pool))
	require.NoError(t, err)
	assert.Equal(t, defaultSettings, got, "dbtest.Org writes the row")
}

func TestSettingsMissingRow(t *testing.T) {
	svc, pool, _ := newTestService(t)
	var org uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(), "INSERT INTO organization (name) VALUES ('bare') RETURNING id").Scan(&org))
	_, err := svc.Settings(context.Background(), org)
	assert.Error(t, err, "no code default for a missing row")
}

func TestCookiePrincipalAuthMethod(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	p, err := principal.Require(ctx)
	require.NoError(t, err)
	assert.Equal(t, principal.AuthMethodCookie, p.AuthMethod)
	assert.Equal(t, uuid.Nil, p.CLISessionID)
}

func TestUpdateSettings(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "admin", principal.RoleAdmin)
	seedUser(t, pool, org, "bob", principal.RoleMember)
	adminCtx, _ := loginCtx(t, svc, "admin")
	memberCtx, _ := loginCtx(t, svc, "bob")
	ap, _ := principal.Require(adminCtx)
	mp, _ := principal.Require(memberCtx)

	got, err := svc.UpdateSettings(adminCtx, ap, OrgSettings{CLISessionTTL: 12 * time.Hour}, []string{"cli_session_ttl"})
	require.NoError(t, err)
	want := defaultSettings
	want.CLISessionTTL = 12 * time.Hour
	assert.Equal(t, want, got)
	stored, err := svc.Settings(context.Background(), org)
	require.NoError(t, err)
	assert.Equal(t, got, stored)

	var fields []any
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT data->'fields' FROM audit_event WHERE action = 'settings.update'").Scan(&fields))
	assert.Equal(t, []any{"cli_session_ttl"}, fields)

	_, err = svc.UpdateSettings(memberCtx, mp, OrgSettings{CLISessionTTL: 2 * time.Hour}, []string{"cli_session_ttl"})
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
}

func TestUpdateSettingsValidation(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "owner", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "owner")
	p, _ := principal.Require(ctx)
	h := time.Hour
	for name, tc := range map[string]struct {
		in    OrgSettings
		paths []string
	}{
		"ttl below 1h":         {OrgSettings{CLISessionTTL: 30 * time.Minute}, []string{"cli_session_ttl"}},
		"ttl above 720h":       {OrgSettings{CLISessionTTL: 721 * h}, []string{"cli_session_ttl"}},
		"max below 24h":        {OrgSettings{CLISessionMax: 12 * h}, []string{"cli_session_max"}},
		"max above 2160h":      {OrgSettings{CLISessionMax: 2161 * h}, []string{"cli_session_max"}},
		"ttl above max":        {OrgSettings{CLISessionTTL: 240 * h, CLISessionMax: 168 * h}, []string{"cli_session_ttl", "cli_session_max"}},
		"empty mask":           {OrgSettings{CLISessionTTL: 2 * h}, nil},
		"unknown path":         {OrgSettings{CLISessionTTL: 2 * h}, []string{"foo"}},
		"masked field is zero": {OrgSettings{}, []string{"cli_session_ttl"}},
	} {
		_, err := svc.UpdateSettings(ctx, p, tc.in, tc.paths)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	_, err := svc.UpdateSettings(ctx, p, OrgSettings{CLISessionTTL: 240 * h, CLISessionMax: 168 * h}, []string{"cli_session_ttl", "cli_session_max"})
	assert.ErrorContains(t, err, "cli_session_ttl must not exceed cli_session_max")
	got, err := svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, defaultSettings, got, "failed updates change nothing")
}

func TestSettingsJITFields(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "owner", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "owner")
	p, _ := principal.Require(ctx)
	got, err := svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, defaultSettings, got, "defaults of 00124")

	got, err = svc.UpdateSettings(ctx, p, OrgSettings{LockMode: "strict", GrantMaxTTL: time.Hour}, []string{"lock_mode"})
	require.NoError(t, err)
	want := defaultSettings
	want.LockMode = "strict"
	assert.Equal(t, want, got, "only lock_mode changes")

	got, err = svc.UpdateSettings(ctx, p, OrgSettings{GrantDefaultTTL: 2 * time.Hour, GrantMaxTTL: 3 * time.Hour,
		RequestPendingTTL: 30 * time.Minute, CommandApprovalTTL: time.Minute},
		[]string{"grant_default_ttl", "grant_max_ttl", "request_pending_ttl", "command_approval_ttl"})
	require.NoError(t, err)
	want.GrantDefaultTTL, want.GrantMaxTTL, want.RequestPendingTTL, want.CommandApprovalTTL = 2*time.Hour, 3*time.Hour, 30*time.Minute, time.Minute
	assert.Equal(t, want, got)
	stored, err := svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, want, stored)

	d := 24 * time.Hour
	for name, tc := range map[string]struct {
		in    OrgSettings
		paths []string
	}{
		"default above max": {OrgSettings{GrantDefaultTTL: 20 * d}, []string{"grant_default_ttl"}},
		"default zero":      {OrgSettings{}, []string{"grant_default_ttl"}},
		"max above 14 days": {OrgSettings{GrantMaxTTL: 15 * d}, []string{"grant_max_ttl"}},
		"max below default": {OrgSettings{GrantMaxTTL: time.Hour}, []string{"grant_max_ttl"}},
		"pending below 1m":  {OrgSettings{RequestPendingTTL: 30 * time.Second}, []string{"request_pending_ttl"}},
		"pending above 24h": {OrgSettings{RequestPendingTTL: 25 * time.Hour}, []string{"request_pending_ttl"}},
		"command below 30s": {OrgSettings{CommandApprovalTTL: 10 * time.Second}, []string{"command_approval_ttl"}},
		"command above 1h":  {OrgSettings{CommandApprovalTTL: 2 * time.Hour}, []string{"command_approval_ttl"}},
		"lock mode unknown": {OrgSettings{LockMode: "x"}, []string{"lock_mode"}},
		"lock mode empty":   {OrgSettings{}, []string{"lock_mode"}},
	} {
		_, err := svc.UpdateSettings(ctx, p, tc.in, tc.paths)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	stored, err = svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, want, stored, "failed updates change nothing")
}

func TestSettingsRPC(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "owner", principal.RoleOwner)
	seedUser(t, pool, org, "bob", principal.RoleMember)
	_, ownerTok := loginCtx(t, svc, "owner")
	_, memberTok := loginCtx(t, svc, "bob")

	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewSettingsServiceHandler(NewHandler(svc)))
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)
	client := werbotv1connect.NewSettingsServiceClient(srv.Client(), srv.URL)
	cookie := sessionCookieName(true)

	got, err := client.GetSettings(context.Background(), withCookie(&werbotv1.GetSettingsRequest{}, cookie, ownerTok))
	require.NoError(t, err)
	assert.Equal(t, 168*time.Hour, got.Msg.GetSettings().GetCliSessionTtl().AsDuration())

	upd := &werbotv1.UpdateSettingsRequest{
		Settings:   &werbotv1.OrgSettings{CliSessionTtl: durationpb.New(12 * time.Hour)},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"cli_session_ttl"}},
	}
	res, err := client.UpdateSettings(context.Background(), withCookie(upd, cookie, ownerTok))
	require.NoError(t, err)
	assert.Equal(t, 12*time.Hour, res.Msg.GetSettings().GetCliSessionTtl().AsDuration())
	assert.Equal(t, 720*time.Hour, res.Msg.GetSettings().GetCliSessionMax().AsDuration())

	_, err = client.GetSettings(context.Background(), withCookie(&werbotv1.GetSettingsRequest{}, cookie, memberTok))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = client.UpdateSettings(context.Background(), withCookie(upd, cookie, memberTok))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestUpdateSettingsJIT(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "admin", principal.RoleAdmin)
	ctx, _ := loginCtx(t, svc, "admin")
	h := NewHandler(svc)
	update := func(s *werbotv1.OrgSettings, paths ...string) (*connect.Response[werbotv1.UpdateSettingsResponse], error) {
		return h.UpdateSettings(ctx, connect.NewRequest(&werbotv1.UpdateSettingsRequest{Settings: s, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths}}))
	}

	res, err := update(&werbotv1.OrgSettings{LockMode: "strict"}, "lock_mode")
	require.NoError(t, err)
	got, err := h.GetSettings(ctx, connect.NewRequest(&werbotv1.GetSettingsRequest{}))
	require.NoError(t, err)
	for _, s := range []*werbotv1.OrgSettings{res.Msg.GetSettings(), got.Msg.GetSettings()} {
		assert.Equal(t, "strict", s.GetLockMode())
		assert.Equal(t, 8*time.Hour, s.GetGrantDefaultTtl().AsDuration())
		assert.Equal(t, 14*24*time.Hour, s.GetGrantMaxTtl().AsDuration())
		assert.Equal(t, time.Hour, s.GetRequestPendingTtl().AsDuration())
		assert.Equal(t, 5*time.Minute, s.GetCommandApprovalTtl().AsDuration())
	}

	res, err = update(&werbotv1.OrgSettings{GrantDefaultTtl: durationpb.New(time.Hour), GrantMaxTtl: durationpb.New(2 * time.Hour),
		RequestPendingTtl: durationpb.New(30 * time.Minute), CommandApprovalTtl: durationpb.New(time.Minute)},
		"grant_default_ttl", "grant_max_ttl", "request_pending_ttl", "command_approval_ttl")
	require.NoError(t, err)
	s := res.Msg.GetSettings()
	assert.Equal(t, []time.Duration{time.Hour, 2 * time.Hour, 30 * time.Minute, time.Minute}, []time.Duration{s.GetGrantDefaultTtl().AsDuration(),
		s.GetGrantMaxTtl().AsDuration(), s.GetRequestPendingTtl().AsDuration(), s.GetCommandApprovalTtl().AsDuration()})

	_, err = update(&werbotv1.OrgSettings{GrantDefaultTtl: durationpb.New(20 * 24 * time.Hour)}, "grant_default_ttl")
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	rows, err := pool.Query(context.Background(), "SELECT data->'fields' FROM audit_event WHERE action = 'settings.update' ORDER BY created_at")
	require.NoError(t, err)
	fields, err := pgx.CollectRows(rows, pgx.RowTo[[]string])
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"lock_mode"}, {"grant_default_ttl", "grant_max_ttl", "request_pending_ttl", "command_approval_ttl"}}, fields)
}
