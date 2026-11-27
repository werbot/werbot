package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// The embedded gateway of `server --with-gateway` refuses a postgres resource
// on the server's own database host before StartSession: the channel is
// refused, no session exists and the denial is target_not_allowed.
func TestEmbeddedGatewayRefusesPostgresOnServerDB(t *testing.T) {
	pool, url := dbtest.New(t)
	s := embeddedStack(t, pool, url)
	pc, err := pgconn.ParseConfig(url)
	require.NoError(t, err)
	local := localPool(t, s.api.gws, 1, 15*time.Second)
	cfg := &postgresv1.Config{Host: pc.Host, Port: uint32(pc.Port), Database: pc.Database, Username: pc.User, Sslmode: "disable"}
	res := s.createResource(t, uuid.MustParse(local.GetId()), "pg", "postgres", cfg, pc.Password, true)
	s.resourceReady(t, "pg")

	c, _, err := s.dial(s.gwSSH, "alice+pg", s.key)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	_, err = c.Dial("tcp", "pg:"+strconv.Itoa(int(pc.Port)))
	oe, ok := errors.AsType[*gossh.OpenChannelError](err)
	require.True(t, ok, "not a channel refusal: %v", err)
	assert.Equal(t, "Access denied", oe.Message)
	assert.Zero(t, sessionCount(t, s, res), "refused before StartSession")

	// denials reach the server with the next heartbeat (every 15s); the
	// readiness probes before the snapshot had the resource are plain denials
	require.Eventually(t, func() bool {
		var n int
		err := s.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_event WHERE action = 'session.deny'
			AND target_id = $1 AND data->>'kind' = 'stream' AND data->>'reason' = 'target_not_allowed'`, res.String()).Scan(&n)
		return err == nil && n == 1
	}, 25*time.Second, 200*time.Millisecond, "a target_not_allowed denial of the stream")
}

// embeddedStack runs `server --with-gateway` on pool (database url) with the
// owner alice and her SSH key; gwSSH is the embedded gateway's SSH address.
func embeddedStack(t *testing.T, pool *pgxpool.Pool, url string) *stack {
	t.Helper()
	httpAddr, sshAddr := freeAddr(t), freeAddr(t)
	r := startRun(t, []string{"server", "--with-gateway"}, env(map[string]string{
		"WERBOT_DATABASE_URL": url, "WERBOT_HTTP_ADDR": httpAddr, "WERBOT_PUBLIC_URL": "http://" + httpAddr,
		"WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t), "WERBOT_DATA_DIR": t.TempDir(),
		"WERBOT_ENV": "development", "WERBOT_GATEWAY_SSH_ADDR": sshAddr,
	})).healthy(t)
	ctx := context.Background()
	hc, _ := jarClient(t, http.DefaultTransport)
	auth := werbotv1connect.NewAuthServiceClient(hc, r.api)
	setup := proto.CloneOf(ownerSetup)
	setup.Login, setup.Email = "alice", "alice@example.com"
	_, err := auth.Setup(ctx, connect.NewRequest(setup))
	require.NoError(t, err)
	me, err := auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	s := &stack{pool: pool, url: r.api, gwSSH: sshAddr, alice: me.Msg.GetUser().GetId(), api: apiClients{
		keys: werbotv1connect.NewSSHKeyServiceClient(hc, r.api), res: werbotv1connect.NewResourceServiceClient(hc, r.api),
		acc: werbotv1connect.NewAccessServiceClient(hc, r.api), gws: werbotv1connect.NewGatewayServiceClient(hc, r.api),
	}}
	s.key = s.addKey(t)
	return s
}
