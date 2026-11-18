//go:build cluster

package cluster

import (
	"context"
	"net/netip"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/policy/geoiptest"
)

// createBinding binds alice to the named resource through api and returns the binding id.
func (s *Stand) createBinding(t testing.TB, api API, resource string) string {
	t.Helper()
	res, err := api.Access.CreateBinding(context.Background(), connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: s.AliceID.String(), ResourceId: s.Resources[resource].String(),
	}))
	require.NoError(t, err)
	return res.Msg.GetBinding().GetId()
}

func (s *Stand) deleteBinding(t testing.TB, id string) {
	t.Helper()
	_, err := s.Edge.Access.DeleteBinding(context.Background(), connect.NewRequest(&werbotv1.DeleteBindingRequest{Id: id}))
	require.NoError(t, err)
}

// publishGeoIP stores a database mapping prefix to country and notifies the
// replicas in one transaction (monthsAgo moves the stored month back: replicas skip a row of the month they already loaded), as geoip.refresh does (contract §3.2).
func (s *Stand) publishGeoIP(t testing.TB, prefix, country string, monthsAgo int) {
	t.Helper()
	data := geoiptest.MMDB(netip.MustParsePrefix(prefix), country)
	err := pgx.BeginFunc(context.Background(), s.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(context.Background(), `
INSERT INTO geoip_db (id, month, data, updated_at)
VALUES (1, (date_trunc('month', now() AT TIME ZONE 'UTC') - make_interval(months => $2))::date, $1, now())
ON CONFLICT (id) DO UPDATE SET month = EXCLUDED.month, data = EXCLUDED.data, updated_at = now()`, data, monthsAgo); err != nil {
			return err
		}
		_, err := tx.Exec(context.Background(), `SELECT pg_notify('werbot_geoip_changed', '')`)
		return err
	})
	require.NoError(t, err)
}
