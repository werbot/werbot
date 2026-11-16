package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

func TestAuthenticateChecksSerial(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	serial := parseCert(t, en.CertPEM).SerialNumber.Text(16)
	bg := context.Background()

	inst, err := e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
	require.NoError(t, err)
	assert.Equal(t, en.InstanceID, inst.ID)
	assert.Equal(t, g.ID, inst.GatewayID)
	assert.Equal(t, e.org, inst.OrgID)

	other := e.newPool(t, "other")
	enOther := e.enroll(t, e.token(t, other.ID, 0), "gw-o")
	_, err = e.gw.Authenticate(bg, g.ID, enOther.InstanceID, parseCert(t, enOther.CertPEM).SerialNumber.Text(16))
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated, "instance of another pool")
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, "00")
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)

	renewed, err := e.gw.Renew(bg, en.InstanceID, csr(t))
	require.NoError(t, err)
	newCert := parseCert(t, renewed)
	assert.Equal(t, tlsca.GatewayCN(g.ID, en.InstanceID), newCert.Subject.CommonName)
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, newCert.SerialNumber.Text(16))
	require.NoError(t, err)
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated, "old serial after the new one was used")
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM audit_event WHERE target_id = $1", en.InstanceID.String()),
		"Renew writes no audit")

	_, err = e.gw.Renew(bg, uuid.New(), csr(t))
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
}

// A lost RenewCertificate response must not lock the instance out: the old
// serial stays valid until the new one is first used, across repeated renewals.
func TestRenewKeepsPreviousSerialUntilNewUsed(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	old := parseCert(t, en.CertPEM).SerialNumber.Text(16)
	bg := context.Background()
	auth := func(serial string) error {
		_, err := e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
		return err
	}

	lost, err := e.gw.Renew(bg, en.InstanceID, csr(t)) // response never reached the instance
	require.NoError(t, err)
	require.NoError(t, auth(old))
	require.NoError(t, auth(old), "still valid: the new serial is unused")
	renewed, err := e.gw.Renew(bg, en.InstanceID, csr(t)) // retried with the old certificate
	require.NoError(t, err)
	require.NoError(t, auth(old), "a second renewal keeps the serial the instance holds")
	assert.ErrorIs(t, auth(parseCert(t, lost).SerialNumber.Text(16)), apperr.ErrUnauthenticated, "lost serial")

	current := parseCert(t, renewed).SerialNumber.Text(16)
	require.NoError(t, auth(current))
	assert.ErrorIs(t, auth(old), apperr.ErrUnauthenticated, "first use of the new serial ends the old one")
	require.NoError(t, auth(current))
}

func TestDeleteInstanceRevokesBothSerials(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	renewed, err := e.gw.Renew(context.Background(), en.InstanceID, csr(t))
	require.NoError(t, err)
	require.Equal(t, 1, e.count(t, "SELECT count(*) FROM gateway_instance WHERE id = $1 AND prev_cert_serial IS NOT NULL", en.InstanceID))
	require.NoError(t, e.gw.DeleteInstance(e.ctx, en.InstanceID))
	for _, c := range [][]byte{en.CertPEM, renewed} {
		_, err := e.gw.Authenticate(context.Background(), g.ID, en.InstanceID, parseCert(t, c).SerialNumber.Text(16))
		assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	}
}

// The first use of a renewed serial ends the old one, so the hub must re-check its streams.
func TestFirstNewSerialUseNotifies(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	bg := context.Background()
	conn, err := pgx.ConnectConfig(bg, e.pool.Config().ConnConfig)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(bg) })
	_, err = conn.Exec(bg, "LISTEN werbot_org_changed")
	require.NoError(t, err)
	wait := func(d time.Duration) (*pgconn.Notification, error) {
		ctx, cancel := context.WithTimeout(bg, d)
		defer cancel()
		return conn.WaitForNotification(ctx)
	}

	renewed, err := e.gw.Renew(bg, en.InstanceID, csr(t))
	require.NoError(t, err)
	serial := parseCert(t, renewed).SerialNumber.Text(16)
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
	require.NoError(t, err)
	n, err := wait(5 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, e.org.String(), n.Payload)
	_, err = e.gw.Authenticate(bg, g.ID, en.InstanceID, serial)
	require.NoError(t, err)
	_, err = wait(300 * time.Millisecond)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "one notification: Renew and later uses send none")
}
