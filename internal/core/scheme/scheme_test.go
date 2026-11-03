package scheme_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/core/scheme"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/utils/test"
)

func newDBHandler(t *testing.T) (*scheme.Handler, *test.PostgresService) {
	t.Helper()

	pg := test.ServerPostgres(t, "../../../migration", "../../../fixtures/migration")
	return &scheme.Handler{DB: pg.Conn()}, pg
}

func fillHours(size int, value int32) []int32 {
	hours := make([]int32, size)
	for i := range hours {
		hours[i] = value
	}
	return hours
}

func weekWithHours(size int, value int32) *schemepb.SchemeActivity_Week {
	hours := fillHours(size, value)
	return &schemepb.SchemeActivity_Week{
		Mon: hours,
		Tue: hours,
		Wed: hours,
		Thu: hours,
		Fri: hours,
		Sat: hours,
		Sun: hours,
	}
}

func Test_UpdateSchemeFirewallNetworkStatus(t *testing.T) {
	handler, pg := newDBHandler(t)
	ctx := context.Background()

	var countryBefore string
	if err := pg.Conn().Conn.QueryRowContext(ctx, `
    SELECT "access_policy"->>'country' FROM "scheme" WHERE "id" = $1
  `, test.ConstAdminSchemeSSH1ID).Scan(&countryBefore); err != nil {
		t.Fatalf("query access_policy before update: %v", err)
	}

	_, err := handler.UpdateSchemeFirewall(ctx, &schemepb.UpdateSchemeFirewall_Request{
		OwnerId:   test.ConstAdminID,
		ProjectId: test.ConstAdminProject1ID,
		SchemeId:  test.ConstAdminSchemeSSH1ID,
		Status: &schemepb.UpdateSchemeFirewall_Request_Network{
			Network: true,
		},
	})
	if err != nil {
		t.Fatalf("UpdateSchemeFirewall(network) error = %v", err)
	}

	var network, country string
	if err := pg.Conn().Conn.QueryRowContext(ctx, `
    SELECT "access_policy"->>'network', "access_policy"->>'country'
    FROM "scheme"
    WHERE "id" = $1
  `, test.ConstAdminSchemeSSH1ID).Scan(&network, &country); err != nil {
		t.Fatalf("query access_policy: %v", err)
	}

	if network != "1" {
		t.Fatalf("access_policy->>'network' = %q, want %q", network, "1")
	}
	if country != countryBefore {
		t.Fatalf("access_policy->>'country' = %q, want unchanged %q", country, countryBefore)
	}
}

func Test_SchemeActivityShortGridFailsClosed(t *testing.T) {
	handler, pg := newDBHandler(t)
	ctx := context.Background()

	// Simulate a row written before the 24 hour fix.
	data, err := protojson.Marshal(weekWithHours(23, 1))
	if err != nil {
		t.Fatalf("marshal week: %v", err)
	}
	if _, err := pg.Conn().Conn.ExecContext(ctx, `
    UPDATE "scheme_activity" SET "data" = $1 WHERE "scheme_id" = $2
  `, data, test.ConstAdminSchemeSSH1ID); err != nil {
		t.Fatalf("update scheme_activity: %v", err)
	}

	// 2024-09-16 is a Monday; 23:00 has no entry in a 23 element grid.
	response, err := handler.SchemeActivity(ctx, &schemepb.SchemeActivity_Request{
		OwnerId:   test.ConstAdminID,
		ProjectId: test.ConstAdminProject1ID,
		SchemeId:  test.ConstAdminSchemeSSH1ID,
		Timestamp: timestamppb.New(time.Date(2024, time.September, 16, 23, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("SchemeActivity() error = %v", err)
	}

	hour, ok := response.GetPeriod().(*schemepb.SchemeActivity_Response_Hour)
	if !ok {
		t.Fatalf("SchemeActivity() period = %T, want hour", response.GetPeriod())
	}
	if hour.Hour {
		t.Fatal("SchemeActivity() allowed hour 23 from a 23 element grid, want fail closed")
	}
}

func Test_UpdateSchemeActivity24Hours(t *testing.T) {
	handler, pg := newDBHandler(t)
	ctx := context.Background()

	week := weekWithHours(24, 1)
	if _, err := handler.UpdateSchemeActivity(ctx, &schemepb.UpdateSchemeActivity_Request{
		OwnerId:   test.ConstAdminID,
		ProjectId: test.ConstAdminProject1ID,
		SchemeId:  test.ConstAdminSchemeSSH1ID,
		Activity:  week,
	}); err != nil {
		t.Fatalf("UpdateSchemeActivity(24 hours) error = %v", err)
	}

	var stored []byte
	if err := pg.Conn().Conn.QueryRowContext(ctx, `
    SELECT "data" FROM "scheme_activity" WHERE "scheme_id" = $1
  `, test.ConstAdminSchemeSSH1ID).Scan(&stored); err != nil {
		t.Fatalf("query scheme_activity: %v", err)
	}

	response, err := handler.SchemeActivity(ctx, &schemepb.SchemeActivity_Request{
		OwnerId:   test.ConstAdminID,
		ProjectId: test.ConstAdminProject1ID,
		SchemeId:  test.ConstAdminSchemeSSH1ID,
		Timestamp: timestamppb.New(time.Date(2024, time.September, 16, 23, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("SchemeActivity() error = %v", err)
	}

	hour, ok := response.GetPeriod().(*schemepb.SchemeActivity_Response_Hour)
	if !ok {
		t.Fatalf("SchemeActivity() period = %T, want hour", response.GetPeriod())
	}
	if !hour.Hour {
		t.Fatal("SchemeActivity() denied hour 23 for a 24 element grid")
	}
}
