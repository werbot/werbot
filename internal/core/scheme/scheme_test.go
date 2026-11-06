package scheme_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/werbot/werbot/internal/core/scheme"
	schemeaccesspb "github.com/werbot/werbot/internal/core/scheme/proto/access"
	schemeauthpb "github.com/werbot/werbot/internal/core/scheme/proto/auth"
	schemepb "github.com/werbot/werbot/internal/core/scheme/proto/scheme"
	"github.com/werbot/werbot/internal/trace"
	"github.com/werbot/werbot/internal/utils/test"
	"github.com/werbot/werbot/internal/utils/test/factory"
	"github.com/werbot/werbot/pkg/crypto"
)

// newSchemeGraph creates one owned SSH scheme with the default access policy.
func newSchemeGraph(t *testing.T) (*scheme.Handler, *test.PostgresService, *factory.Builder, factory.Profile, factory.Project, factory.Scheme) {
	t.Helper()

	pg := test.ServerPostgres(t, "../../../migration")
	builder := factory.New(t, pg.Conn().Conn)

	owner := builder.Profile(factory.ProfileParams{Alias: "admin", Role: 3})
	project := builder.Project(factory.ProjectParams{OwnerID: owner.ID, Title: "project1"})
	ssh := builder.Scheme(factory.SchemeParams{ProjectID: project.ID, Title: "Git server"})

	return &scheme.Handler{DB: pg.Conn()}, pg, builder, owner, project, ssh
}

func fillHours(size int, value int32) []int32 {
	hours := make([]int32, size)
	for i := range hours {
		hours[i] = value
	}
	return hours
}

func weekGrid(size int, value int32) map[string][]int32 {
	days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	week := make(map[string][]int32, len(days))
	for _, day := range days {
		week[day] = fillHours(size, value)
	}
	return week
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
	handler, pg, _, owner, project, ssh := newSchemeGraph(t)
	ctx := context.Background()

	var countryBefore string
	if err := pg.Conn().Conn.QueryRowContext(ctx, `
    SELECT "access_policy"->>'country' FROM "scheme" WHERE "id" = $1
  `, ssh.ID).Scan(&countryBefore); err != nil {
		t.Fatalf("query access_policy before update: %v", err)
	}

	_, err := handler.UpdateSchemeFirewall(ctx, &schemepb.UpdateSchemeFirewall_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  ssh.ID,
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
  `, ssh.ID).Scan(&network, &country); err != nil {
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
	handler, _, builder, owner, project, ssh := newSchemeGraph(t)
	ctx := context.Background()

	// Simulate a row written before the 24 hour fix.
	builder.SchemeActivity(ssh.ID, weekGrid(23, 1))

	// 2024-09-16 is a Monday; 23:00 has no entry in a 23 element grid.
	response, err := handler.SchemeActivity(ctx, &schemepb.SchemeActivity_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  ssh.ID,
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
	handler, pg, builder, owner, project, ssh := newSchemeGraph(t)
	ctx := context.Background()

	// Start from a closed grid so the update is observable.
	builder.SchemeActivity(ssh.ID, weekGrid(24, 0))

	week := weekWithHours(24, 1)
	if _, err := handler.UpdateSchemeActivity(ctx, &schemepb.UpdateSchemeActivity_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  ssh.ID,
		Activity:  week,
	}); err != nil {
		t.Fatalf("UpdateSchemeActivity(24 hours) error = %v", err)
	}

	var storedHour string
	if err := pg.Conn().Conn.QueryRowContext(ctx, `
    SELECT "data"->'mon'->>23 FROM "scheme_activity" WHERE "scheme_id" = $1
  `, ssh.ID).Scan(&storedHour); err != nil {
		t.Fatalf("query scheme_activity: %v", err)
	}
	if storedHour != "1" {
		t.Fatalf("scheme_activity mon[23] = %q, want %q", storedHour, "1")
	}

	response, err := handler.SchemeActivity(ctx, &schemepb.SchemeActivity_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  ssh.ID,
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

const telnetTestAESKey = "3D6A619811A17396E45D514695DCDA3A"

func telnetAccess(password string) *schemeaccesspb.AccessScheme {
	return &schemeaccesspb.AccessScheme{
		Access: &schemeaccesspb.AccessScheme_ServerTelnet{
			ServerTelnet: &schemeaccesspb.AccessScheme_Server_Telnet{
				Address: "10.0.0.1",
				Port:    23,
				Access: &schemeauthpb.Auth_Password{
					Login:    "ubuntu1",
					Password: password,
				},
			},
		},
	}
}

func readTelnetAccess(t *testing.T, pg *test.PostgresService, schemeID string) *schemeaccesspb.AccessScheme_Server_Telnet {
	t.Helper()

	var raw []byte
	if err := pg.Conn().Conn.QueryRow(`SELECT "access" FROM "scheme" WHERE "id" = $1`, schemeID).Scan(&raw); err != nil {
		t.Fatalf("read scheme access: %v", err)
	}

	access := &schemeaccesspb.AccessScheme_Server_Telnet{}
	if err := protojson.Unmarshal(raw, access); err != nil {
		t.Fatalf("unmarshal scheme access: %v", err)
	}
	return access
}

func Test_AddSchemeTelnetEncryptsPassword(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", telnetTestAESKey)

	handler, pg, _, owner, project, _ := newSchemeGraph(t)
	ctx := context.Background()

	response, err := handler.AddScheme(ctx, &schemepb.AddScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		Scheme:    telnetAccess("secret123"),
	})
	if err != nil {
		t.Fatalf("AddScheme() error = %v", err)
	}

	access := readTelnetAccess(t, pg, response.GetSchemeId())
	if access.GetAlias() == "" {
		t.Fatal("AddScheme() left telnet alias empty")
	}
	password := access.GetAccess().GetPassword()
	if password == "secret123" {
		t.Fatal("AddScheme() stored telnet password in plaintext")
	}
	plain, err := crypto.TextDecrypt(password, telnetTestAESKey)
	if err != nil {
		t.Fatalf("TextDecrypt() error = %v", err)
	}
	if plain != "secret123" {
		t.Fatalf("decrypted password = %q, want %q", plain, "secret123")
	}
}

func Test_AddSchemeTelnetRequiresPassword(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", telnetTestAESKey)

	handler, _, _, owner, project, _ := newSchemeGraph(t)
	ctx := context.Background()

	_, err := handler.AddScheme(ctx, &schemepb.AddScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		Scheme:    telnetAccess(""),
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("AddScheme() code = %v, want InvalidArgument", status.Code(err))
	}
	if got := status.Convert(err).Message(); got != "password is required" {
		t.Fatalf("AddScheme() message = %q, want %q", got, "password is required")
	}
}

func Test_UpdateSchemeTelnetKeepsPassword(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", telnetTestAESKey)

	handler, pg, _, owner, project, _ := newSchemeGraph(t)
	ctx := context.Background()

	response, err := handler.AddScheme(ctx, &schemepb.AddScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		Scheme:    telnetAccess("secret123"),
	})
	if err != nil {
		t.Fatalf("AddScheme() error = %v", err)
	}
	before := readTelnetAccess(t, pg, response.GetSchemeId())

	update := telnetAccess("")
	update.GetServerTelnet().Port = 2323
	_, err = handler.UpdateScheme(ctx, &schemepb.UpdateScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  response.GetSchemeId(),
		Setting:   &schemepb.UpdateScheme_Request_Scheme{Scheme: update},
	})
	if err != nil {
		t.Fatalf("UpdateScheme() error = %v", err)
	}

	after := readTelnetAccess(t, pg, response.GetSchemeId())
	if after.GetPort() != 2323 {
		t.Fatalf("port = %d, want 2323", after.GetPort())
	}
	if after.GetAlias() != before.GetAlias() {
		t.Fatalf("alias = %q, want unchanged %q", after.GetAlias(), before.GetAlias())
	}
	if after.GetAccess().GetPassword() != before.GetAccess().GetPassword() {
		t.Fatal("UpdateScheme() changed encrypted password when none was sent")
	}
	plain, err := crypto.TextDecrypt(after.GetAccess().GetPassword(), telnetTestAESKey)
	if err != nil {
		t.Fatalf("TextDecrypt() error = %v", err)
	}
	if plain != "secret123" {
		t.Fatalf("decrypted password = %q, want %q", plain, "secret123")
	}
}

func Test_UpdateSchemeTelnetNewPassword(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", telnetTestAESKey)

	handler, pg, _, owner, project, _ := newSchemeGraph(t)
	ctx := context.Background()

	response, err := handler.AddScheme(ctx, &schemepb.AddScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		Scheme:    telnetAccess("secret123"),
	})
	if err != nil {
		t.Fatalf("AddScheme() error = %v", err)
	}

	update := telnetAccess("newsecret1")
	_, err = handler.UpdateScheme(ctx, &schemepb.UpdateScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  response.GetSchemeId(),
		Setting:   &schemepb.UpdateScheme_Request_Scheme{Scheme: update},
	})
	if err != nil {
		t.Fatalf("UpdateScheme() error = %v", err)
	}

	access := readTelnetAccess(t, pg, response.GetSchemeId())
	plain, err := crypto.TextDecrypt(access.GetAccess().GetPassword(), telnetTestAESKey)
	if err != nil {
		t.Fatalf("TextDecrypt() error = %v", err)
	}
	if plain != "newsecret1" {
		t.Fatalf("decrypted password = %q, want %q", plain, "newsecret1")
	}
}

func Test_UpdateSchemeTelnetTypeDiffers(t *testing.T) {
	t.Setenv("SECURITY_AES_KEY", telnetTestAESKey)

	handler, _, _, owner, project, ssh := newSchemeGraph(t)
	ctx := context.Background()

	_, err := handler.UpdateScheme(ctx, &schemepb.UpdateScheme_Request{
		OwnerId:   owner.ID,
		ProjectId: project.ID,
		SchemeId:  ssh.ID,
		Setting:   &schemepb.UpdateScheme_Request_Scheme{Scheme: telnetAccess("secret123")},
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("UpdateScheme() code = %v, want InvalidArgument", status.Code(err))
	}
	if got := status.Convert(err).Message(); got != trace.MsgSchemeDiffers {
		t.Fatalf("UpdateScheme() message = %q, want %q", got, trace.MsgSchemeDiffers)
	}
}
