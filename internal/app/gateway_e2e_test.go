package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
)

// The server type "echo" is registered in echo_type_test.go; this is its gateway connector.
func init() { gateway.RegisterConnector(connectortest.Echo{}) }

const settle = 10 * time.Second // a new resource or binding reaches the gateway snapshot

// readUntil reads r until the output contains want or 5s pass.
func readUntil(t *testing.T, r io.Reader, want string) {
	t.Helper()
	found := make(chan struct{})
	go func() {
		var got []byte
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			got = append(got, buf[:n]...)
			if bytes.Contains(got, []byte(want)) {
				close(found)
				_, _ = io.Copy(io.Discard, r)
				return
			}
			if err != nil {
				return
			}
		}
	}()
	select {
	case <-found:
	case <-time.After(5 * time.Second):
		t.Fatalf("output lacks %q after 5s", want)
	}
}

// echoPing sends "ping" through conn and expects it back.
func echoPing(t *testing.T, conn io.ReadWriter) {
	t.Helper()
	_, err := io.WriteString(conn, "ping")
	require.NoError(t, err)
	got := make([]byte, 4)
	_, err = io.ReadFull(conn, got)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(got))
}

func TestE2EShellExecForward(t *testing.T) {
	s := startStack(t)
	s.addResource(t, "srv", "ssh")
	require.Eventually(t, func() bool {
		out, code := s.exec(t, s.gwSSH, "alice+srv", "echo hello")
		return out == "hello\n" && code == 0
	}, settle, 100*time.Millisecond)

	c, _, err := s.dial(s.gwSSH, "alice+srv", s.key)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	sess, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, sess.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	stdin, err := sess.StdinPipe()
	require.NoError(t, err)
	stdout, err := sess.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, sess.Shell())
	_, err = io.WriteString(stdin, "echo hi\r")
	require.NoError(t, err)
	readUntil(t, stdout, "hi")
	require.NoError(t, sess.Close())

	conn, err := c.Dial("tcp", s.target.EchoAddr)
	require.NoError(t, err)
	echoPing(t, conn)
	require.NoError(t, conn.Close())
	require.NoError(t, c.Close())

	_, instanceID, _ := instanceIdentity(t, s.gwData)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var total, closed int
		require.NoError(c, s.pool.QueryRow(context.Background(), `SELECT count(*), count(*) FILTER (WHERE end_reason = 'closed'
			AND gateway_id = $1 AND gateway_instance_id = $2) FROM session`, s.gatewayID, instanceID).Scan(&total, &closed))
		assert.Equal(c, 3, total)
		assert.Equal(c, 3, closed, "exec, shell and forward ended as closed on this pool and instance")
	}, 10*time.Second, 100*time.Millisecond)
}

func TestE2EDenies(t *testing.T) {
	s := startStack(t)
	id := s.addResource(t, "srv", "ssh")
	ctx := context.Background()
	allowed := func() bool {
		out, code := s.exec(t, s.gwSSH, "alice+srv", "echo ok")
		return out == "ok\n" && code == 0
	}
	denied := func() bool {
		out, code := s.exec(t, s.gwSSH, "alice+srv", "echo ok")
		return strings.TrimSpace(out) == "Access denied" && code == 1
	}
	require.Eventually(t, allowed, settle, 100*time.Millisecond)

	setRules := func(t *testing.T, r *werbotv1.SetResourceRulesRequest) {
		r.ResourceId = id.String()
		_, err := s.api.pol.SetResourceRules(ctx, connect.NewRequest(r))
		require.NoError(t, err)
	}
	other := int32((int(time.Now().UTC().Weekday()) + 3) % 7)
	var schedule []*werbotv1.ScheduleSlot
	for h := range int32(24) {
		schedule = append(schedule, &werbotv1.ScheduleSlot{Weekday: other, Hour: h})
	}
	noRules := func(t *testing.T) { setRules(t, &werbotv1.SetResourceRulesRequest{}) }
	exec := func(t *testing.T, sql string) {
		_, err := s.pool.Exec(ctx, sql)
		require.NoError(t, err)
	}
	for _, tc := range []struct {
		name         string
		apply, reset func(t *testing.T)
	}{
		{"cidr", func(t *testing.T) {
			setRules(t, &werbotv1.SetResourceRulesRequest{Networks: []*werbotv1.NetworkRule{{Cidr: "127.0.0.0/8", Effect: "deny"}}})
		}, noRules},
		// no GeoIP database: the client country is "", which a country rule always denies
		{"country", func(t *testing.T) {
			setRules(t, &werbotv1.SetResourceRulesRequest{Countries: []*werbotv1.CountryRule{{Country: "NL", Effect: "allow"}}})
		}, noRules},
		{"schedule", func(t *testing.T) { setRules(t, &werbotv1.SetResourceRulesRequest{Schedule: schedule}) }, noRules},
		{"blocklist", func(t *testing.T) { exec(t, "INSERT INTO ip_blocklist (cidr, source) VALUES ('127.0.0.0/8', 'test')") },
			func(t *testing.T) { exec(t, "DELETE FROM ip_blocklist") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 5s: the change reaches the gateway by NOTIFY, the debounce and a snapshot push
			tc.apply(t)
			require.Eventually(t, denied, 5*time.Second, 100*time.Millisecond)
			tc.reset(t)
			require.Eventually(t, allowed, 5*time.Second, 100*time.Millisecond)
		})
	}
}

func TestE2EUnknownKeyRejected(t *testing.T) {
	s := startStack(t)
	s.addResource(t, "srv", "ssh")
	require.Eventually(t, func() bool { // positive control: the gateway serves alice's key
		out, code := s.exec(t, s.gwSSH, "alice+srv", "echo ok")
		return out == "ok\n" && code == 0
	}, settle, 100*time.Millisecond)
	_, _, err := s.dial(s.gwSSH, "alice+srv", newSigner(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to authenticate")
}

func TestE2EHostKeyMismatch(t *testing.T) {
	s := startStack(t)
	id := s.addResource(t, "srv", "ssh")
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	foreign, err := gossh.NewPublicKey(pub)
	require.NoError(t, err)
	_, err = s.pool.Exec(context.Background(), `INSERT INTO resource_host_key (resource_id, org_id, key_type, public_key, fingerprint_sha256)
		SELECT id, org_id, $2, $3, $4 FROM resource WHERE id = $1`,
		id, foreign.Type(), string(gossh.MarshalAuthorizedKey(foreign)), gossh.FingerprintSHA256(foreign))
	require.NoError(t, err)

	var out string
	var code int
	require.Eventually(t, func() bool {
		out, code = s.exec(t, s.gwSSH, "alice+srv", "echo hello")
		return code != -1 && strings.TrimSpace(out) != "Access denied" // the resource reached the snapshot
	}, settle, 100*time.Millisecond)
	assert.Contains(t, out, "SSH host key mismatch")
	assert.Equal(t, 1, code)
	var pinned string
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT fingerprint_sha256 FROM resource_host_key WHERE resource_id = $1", id).Scan(&pinned))
	assert.Equal(t, gossh.FingerprintSHA256(foreign), pinned, "the pinned key is kept")
}

func TestE2EEchoViaForward(t *testing.T) {
	s := startStack(t)
	s.addResource(t, "app", "echo")
	c, _, err := s.dial(s.gwSSH, "alice+app", s.key)
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client

	var conn io.ReadWriteCloser
	require.Eventually(t, func() bool {
		var err error
		conn, err = c.Dial("tcp", "echo.invalid:7") // the echo connector ignores the target
		return err == nil
	}, settle, 100*time.Millisecond)
	echoPing(t, conn)
	require.NoError(t, conn.Close())

	sess, err := c.NewSession()
	require.NoError(t, err)
	var out bytes.Buffer
	sess.Stdout = &out
	require.NoError(t, sess.Shell())
	var ee *gossh.ExitError
	require.True(t, errors.As(sess.Wait(), &ee))
	assert.Equal(t, 1, ee.ExitStatus())
	assert.Equal(t, "Resource app does not support terminal sessions", strings.TrimSpace(out.String()))

	out2, code := s.exec(t, s.gwSSH, "alice+app", "echo hello")
	assert.Equal(t, "Resource app does not support exec sessions", strings.TrimSpace(out2))
	assert.Equal(t, 1, code)
}
