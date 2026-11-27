package pgconnector

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// written is the client side of a PreSession: what the connector answered.
type written struct{ bytes.Buffer }

func (*written) Read([]byte) (int, error) { return 0, io.EOF }
func (*written) Close() error             { return nil }

// preSession runs PreSession on a stream that holds in and then ends.
func preSession(t *testing.T, cfg *postgresv1.Config, in ...[]byte) (gateway.PreSessionResult, []byte, error) {
	t.Helper()
	br := bufio.NewReaderSize(bytes.NewReader(bytes.Join(in, nil)), gateway.PreSessionBuffer)
	var w written
	out, err := Connector{}.PreSession(context.Background(), testResource(t, cfg), br.Peek, &w)
	return out, w.Bytes(), err
}

func encode(t *testing.T, m interface{ Encode([]byte) ([]byte, error) }) []byte {
	t.Helper()
	b, err := m.Encode(nil)
	require.NoError(t, err)
	return b
}

func startupMsg(t *testing.T, params map[string]string) []byte {
	return encode(t, &pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30, Parameters: params})
}

func sslRequest(t *testing.T) []byte { return encode(t, &pgproto3.SSLRequest{}) }

func baseConfig() *postgresv1.Config {
	return &postgresv1.Config{Host: "db.example", Database: "main", Username: "app"}
}

// fatal decodes the ErrorResponse a refused start wrote.
func fatal(t *testing.T, b []byte) *pgproto3.ErrorResponse {
	t.Helper()
	require.Greater(t, len(b), 5)
	require.Equal(t, byte('E'), b[0])
	var e pgproto3.ErrorResponse
	require.NoError(t, e.Decode(b[5:]))
	return &e
}

func TestPreSessionSSLThenStartup(t *testing.T) {
	out, w, err := preSession(t, baseConfig(), sslRequest(t), startupMsg(t, map[string]string{"user": "x"}))
	require.NoError(t, err)
	assert.Equal(t, []byte("N"), w)
	assert.Equal(t, gateway.PreSessionResult{DBName: "main"}, out)

	gss := encode(t, &pgproto3.GSSEncRequest{})
	out, w, err = preSession(t, baseConfig(), gss, sslRequest(t), startupMsg(t, map[string]string{"user": "x", "database": "other"}))
	require.NoError(t, err)
	assert.Equal(t, []byte("NN"), w)
	assert.Equal(t, "other", out.DBName)
}

func TestPreSessionReplicationDenied(t *testing.T) {
	for _, v := range []string{"database", "true", "on", "yes", "1", "TRUE"} {
		out, w, err := preSession(t, baseConfig(), startupMsg(t, map[string]string{"user": "x", "replication": v}))
		require.ErrorIs(t, err, gateway.ErrDenied, v)
		assert.False(t, out.Handled)
		e := fatal(t, w)
		assert.Equal(t, "FATAL", e.Severity)
		assert.Equal(t, "42501", e.Code)
		assert.Equal(t, "werbot: replication connections are not allowed", e.Message)
	}
	for _, v := range []string{"false", "off", "no", "0", "OFF"} {
		_, w, err := preSession(t, baseConfig(), startupMsg(t, map[string]string{"user": "x", "replication": v}))
		require.NoError(t, err, v)
		assert.Empty(t, w)
	}
}

func TestPreSessionDatabaseNotAllowed(t *testing.T) {
	cfg := baseConfig()
	cfg.AllowedDatabases = []string{"app_*"}
	out, _, err := preSession(t, cfg, startupMsg(t, map[string]string{"user": "x", "database": "app_1"}))
	require.NoError(t, err)
	assert.Equal(t, "app_1", out.DBName)

	_, w, err := preSession(t, cfg, startupMsg(t, map[string]string{"user": "x", "database": "other"}))
	require.ErrorIs(t, err, gateway.ErrDenied)
	e := fatal(t, w)
	assert.Equal(t, "42501", e.Code)
	assert.Equal(t, `werbot: database "other" is not allowed`, e.Message)

	cfg.AllowedDatabases = []string{"main"}
	out, _, err = preSession(t, cfg, startupMsg(t, map[string]string{"user": "x"}))
	require.NoError(t, err)
	assert.Equal(t, "main", out.DBName, "no dbname is the resource database")
}

func TestPreSessionGarbage(t *testing.T) {
	hello := []byte{0x16, 0x03, 0x01, 0x00, 0xa5, 0x01, 0x00, 0x00, 0xa1, 0x03, 0x03}
	big := make([]byte, 20000)
	binary.BigEndian.PutUint32(big, 20000)
	binary.BigEndian.PutUint32(big[4:], pgproto3.ProtocolVersion30)
	short := []byte{0, 0, 0, 4, 0, 3, 0, 0}
	unknown := []byte{0, 0, 0, 8, 0, 0, 0x12, 0x34}
	v2 := []byte{0, 0, 0, 9, 0, 2, 0, 0, 0}
	noParams := []byte{0, 0, 0, 9, 0, 3, 0, 0, 0}
	for name, in := range map[string][]byte{"tls hello": hello, "over limit": big, "short": short, "unknown code": unknown, "protocol 2": v2, "no params": noParams, "truncated": startupMsg(t, map[string]string{"user": "x"})[:12]} {
		out, w, err := preSession(t, baseConfig(), in)
		require.Error(t, err, name)
		assert.False(t, out.Handled, name)
		assert.Empty(t, w, name)
	}
	_, _, err := preSession(t, baseConfig(), hello)
	require.ErrorIs(t, err, gateway.ErrDenied)

	_, w, err := preSession(t, baseConfig(), sslRequest(t), sslRequest(t), sslRequest(t), startupMsg(t, map[string]string{"user": "x"}))
	require.ErrorIs(t, err, gateway.ErrDenied)
	assert.Equal(t, []byte("NN"), w, "the third encryption request gets no answer")
}

func cancelPacket(t *testing.T, key uint64) []byte {
	k := binary.BigEndian.AppendUint64(nil, key)
	return encode(t, &pgproto3.CancelRequest{ProcessID: binary.BigEndian.Uint32(k), SecretKey: k[4:]})
}

func TestPreSessionCancelRequest(t *testing.T) {
	res := testResource(t, baseConfig())
	var cancelled, logged int
	key, release := cancels.Register(dbproxy.CancelTarget{
		ResourceID: res.ID,
		Cancel:     func(context.Context) error { cancelled++; return nil },
		OnCancel:   func() { logged++ },
	})
	defer release()
	run := func(r gateway.Resource, in ...[]byte) (gateway.PreSessionResult, []byte, error) {
		br := bufio.NewReader(bytes.NewReader(bytes.Join(in, nil)))
		var w written
		out, err := Connector{}.PreSession(context.Background(), r, br.Peek, &w)
		return out, w.Bytes(), err
	}

	out, w, err := run(res, sslRequest(t), cancelPacket(t, key))
	require.NoError(t, err)
	assert.True(t, out.Handled)
	assert.Equal(t, []byte("N"), w)
	assert.Equal(t, 1, cancelled)
	assert.Equal(t, 1, logged)

	other := res
	other.ID = uuid.New()
	out, w, err = run(other, cancelPacket(t, key))
	require.NoError(t, err)
	assert.True(t, out.Handled)
	assert.Empty(t, w, "a cancel request gets no answer")
	assert.Equal(t, 1, cancelled, "a key acts only within its own resource")

	out, _, err = run(res, cancelPacket(t, key^1))
	require.NoError(t, err)
	assert.True(t, out.Handled)
	assert.Equal(t, 1, cancelled, "an unknown key cancels nothing")
}

func TestPreSessionBadConfig(t *testing.T) {
	r := gateway.Resource{ID: uuid.New(), Type: "postgres", Config: []byte(`{"host":""}`)}
	br := bufio.NewReader(bytes.NewReader(startupMsg(t, map[string]string{"user": "x"})))
	_, err := Connector{}.PreSession(context.Background(), r, br.Peek, &written{})
	require.Error(t, err)
	assert.False(t, errors.Is(err, gateway.ErrHandled))
}
