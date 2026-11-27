package pgconnector

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

var (
	_ gateway.ReadonlyChecker = Connector{}
	_ gateway.Warmer          = Connector{}
)

// roDB is a fresh database of the test cluster, owned by the pool's role,
// with table public.t; cfg logs in to it as agent role name.
type roDB struct {
	pool *pgxpool.Pool
	cfg  *postgresv1.Config
}

func newRODB(t *testing.T, agent string) roDB {
	t.Helper()
	pool, url := dbtest.New(t)
	pc, err := pgconn.ParseConfig(url)
	require.NoError(t, err)
	d := roDB{pool: pool, cfg: &postgresv1.Config{Host: pc.Host, Port: uint32(pc.Port), Database: pc.Database, Username: pc.User,
		Sslmode: "disable", AgentUsername: agent, AllowedDatabases: []string{pc.Database}}}
	d.exec(t, "CREATE TABLE public.t (id int)", "GRANT SELECT ON public.t TO "+agent)
	return d
}

func (d roDB) exec(t *testing.T, sql ...string) {
	t.Helper()
	for _, s := range sql {
		_, err := d.pool.Exec(t.Context(), s)
		require.NoError(t, err, s)
	}
}

func dial(ctx context.Context, network, address string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, network, address)
}

// check runs CheckReadonly of cfg as the agent with password pass.
func check(t *testing.T, cfg *postgresv1.Config, pass string) (string, string) {
	t.Helper()
	return Connector{}.CheckReadonly(t.Context(), testResource(t, cfg), gateway.Secret{}, passwordSecret(pass), dial)
}

// A role with only CONNECT and SELECT is ok.
func TestReadonlyCheckerOK(t *testing.T) {
	name, pass := dbtest.Role(t, "")
	d := newRODB(t, name)
	status, detail := check(t, d.cfg, pass)
	require.Equal(t, [2]string{gateway.ReadonlyOK, ""}, [2]string{status, detail})
}

// Each kind of write privilege in reach is has_write with its finding.
func TestReadonlyCheckerFindsWrite(t *testing.T) {
	cases := map[string]struct {
		attrs string
		grant func(t *testing.T, d roDB, role string)
		want  string
	}{
		"insert": {grant: func(t *testing.T, d roDB, role string) { d.exec(t, "GRANT INSERT ON public.t TO "+role) },
			want: "INSERT on <db>.public.t"},
		"column update": {grant: func(t *testing.T, d roDB, role string) { d.exec(t, "GRANT UPDATE (id) ON public.t TO "+role) },
			want: "UPDATE on <db>.public.t"},
		"truncate": {grant: func(t *testing.T, d roDB, role string) { d.exec(t, "GRANT TRUNCATE ON public.t TO "+role) },
			want: "TRUNCATE on <db>.public.t"},
		"sequence": {grant: func(t *testing.T, d roDB, role string) {
			d.exec(t, "CREATE SEQUENCE public.s", "GRANT USAGE ON SEQUENCE public.s TO "+role)
		}, want: "USAGE on sequence <db>.public.s"},
		"schema": {grant: func(t *testing.T, d roDB, role string) { d.exec(t, "GRANT CREATE ON SCHEMA public TO "+role) },
			want: "CREATE on schema <db>.public"},
		"database": {grant: func(t *testing.T, d roDB, role string) {
			d.exec(t, "GRANT CREATE ON DATABASE "+d.cfg.GetDatabase()+" TO "+role)
		},
			want: "CREATE on database <db>"},
		"large object": {grant: func(t *testing.T, d roDB, role string) {
			d.exec(t, "SELECT lo_create(424242)", "GRANT UPDATE ON LARGE OBJECT 424242 TO "+role)
		}, want: "UPDATE on large object <db>.424242"},
		"predefined role": {grant: func(t *testing.T, _ roDB, role string) { dbtest.Exec(t, "GRANT pg_write_all_data TO "+role) },
			want: "member of pg_write_all_data"},
		"trigger": {grant: func(t *testing.T, d roDB, role string) { d.exec(t, "GRANT TRIGGER ON public.t TO "+role) },
			want: "TRIGGER on <db>.public.t"},
		"maintain": {grant: func(t *testing.T, d roDB, role string) { d.exec(t, "GRANT MAINTAIN ON public.t TO "+role) },
			want: "MAINTAIN on <db>.public.t"},
		"owner of table": {grant: func(t *testing.T, d roDB, role string) {
			ownTo(t, d, role, "ALTER TABLE public.t OWNER TO "+role)
		}, want: "owner of <db>.public.t"},
		"owner of function": {grant: func(t *testing.T, d roDB, role string) {
			ownTo(t, d, role, "CREATE FUNCTION public.g() RETURNS int LANGUAGE sql AS 'SELECT 1'", "ALTER FUNCTION public.g() OWNER TO "+role)
		}, want: "owner of function <db>.public.g"},
		"admin option": {grant: func(t *testing.T, _ roDB, role string) {
			other, _ := dbtest.Role(t, "")
			dbtest.Exec(t, "GRANT "+other+" TO "+role+" WITH ADMIN OPTION, INHERIT FALSE")
		}, want: "admin of role r_"},
		"lo_compat_privileges": {grant: func(t *testing.T, d roDB, _ string) {
			dbtest.Exec(t, "ALTER DATABASE "+d.cfg.GetDatabase()+" SET lo_compat_privileges = on")
		}, want: "lo_compat_privileges on in database <db>"},
		"createdb":    {attrs: "CREATEDB", want: "role rolcreatedb"},
		"createrole":  {attrs: "CREATEROLE", want: "role rolcreaterole"},
		"bypassrls":   {attrs: "BYPASSRLS", want: "role rolbypassrls"},
		"replication": {attrs: "REPLICATION", want: "role rolreplication"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			role, pass := dbtest.Role(t, c.attrs)
			d := newRODB(t, role)
			if c.grant != nil {
				c.grant(t, d, role)
			}
			status, detail := check(t, d.cfg, pass)
			require.Equal(t, gateway.ReadonlyHasWrite, status, detail)
			require.Contains(t, detail, strings.ReplaceAll(c.want, "<db>", d.cfg.GetDatabase()))
		})
	}
}

// ownTo runs sql as the database owner made a member of role, with CREATE on
// schema public for role only meanwhile, so it may give role an object.
func ownTo(t *testing.T, d roDB, role string, sql ...string) {
	t.Helper()
	var owner string
	require.NoError(t, d.pool.QueryRow(t.Context(), "SELECT current_user").Scan(&owner))
	dbtest.Exec(t, "GRANT "+role+" TO "+owner)
	t.Cleanup(func() { dbtest.Exec(t, "REVOKE "+role+" FROM "+owner) })
	d.exec(t, "GRANT CREATE ON SCHEMA public TO "+role)
	d.exec(t, sql...)
	d.exec(t, "REVOKE CREATE ON SCHEMA public FROM "+role)
}

// Every allowed database is checked: a write in the second is has_write.
func TestReadonlyCheckerSecondDatabase(t *testing.T) {
	role, pass := dbtest.Role(t, "")
	first, second := newRODB(t, role), newRODB(t, role)
	second.exec(t, "GRANT DELETE ON public.t TO "+role)
	first.cfg.AllowedDatabases = []string{first.cfg.GetDatabase(), second.cfg.GetDatabase()}
	status, detail := check(t, first.cfg, pass)
	require.Equal(t, gateway.ReadonlyHasWrite, status)
	require.Equal(t, "DELETE on "+second.cfg.GetDatabase()+".public.t", detail)
}

// allowed_databases bounds the check: a database outside it is not checked;
// more than 32 matching databases is error.
func TestReadonlyCheckerAllowedDatabases(t *testing.T) {
	role, pass := dbtest.Role(t, "")
	app, other := newRODB(t, role), newRODB(t, role)
	other.exec(t, "GRANT INSERT ON public.t TO "+role)
	status, detail := check(t, app.cfg, pass)
	require.Equal(t, [2]string{gateway.ReadonlyOK, ""}, [2]string{status, detail}, "the other database is not allowed")

	prefix := "ro_" + strings.ToLower(role[2:10]) + "_"
	for i := range 33 {
		db := fmt.Sprintf("%s%02d", prefix, i)
		dbtest.Exec(t, "CREATE DATABASE "+db)
		t.Cleanup(func() { dbtest.Exec(t, "DROP DATABASE IF EXISTS "+db) })
	}
	cfg := proto.CloneOf(app.cfg)
	cfg.AllowedDatabases = []string{prefix + "*"}
	status, detail = check(t, cfg, pass)
	require.Equal(t, [2]string{gateway.ReadonlyError, "too many databases to verify (32)"}, [2]string{status, detail})
}

// A failed login is error; the detail never holds the password.
func TestReadonlyCheckerAuthError(t *testing.T) {
	role, pass := dbtest.Role(t, "")
	d := newRODB(t, role)
	status, detail := check(t, d.cfg, pass+"-wrong")
	require.Equal(t, gateway.ReadonlyError, status)
	require.NotEmpty(t, detail)
	if strings.Contains(detail, pass) {
		t.Fatal("the detail holds the password")
	}
}

// The checks cover every role the credential is a member of, whatever its
// inherit option; more than 32 such roles is error (Ruling O3).
func TestReadonlyCheckerMemberRoles(t *testing.T) {
	role, pass := dbtest.Role(t, "")
	writer, _ := dbtest.Role(t, "")
	d := newRODB(t, role)
	d.exec(t, "GRANT INSERT ON public.t TO "+writer)
	dbtest.Exec(t, "GRANT "+writer+" TO "+role+" WITH INHERIT FALSE")
	status, detail := check(t, d.cfg, pass)
	require.Equal(t, gateway.ReadonlyHasWrite, status)
	require.Equal(t, "INSERT on "+d.cfg.GetDatabase()+".public.t (via "+writer+")", detail)

	dbtest.Exec(t, "REVOKE "+writer+" FROM "+role)
	for range 33 {
		member, _ := dbtest.Role(t, "")
		dbtest.Exec(t, "GRANT "+member+" TO "+role)
	}
	status, detail = check(t, d.cfg, pass)
	require.Equal(t, [2]string{gateway.ReadonlyError, "too many roles to verify (32)"}, [2]string{status, detail})
}

// EXECUTE on a SECURITY DEFINER function of another owner is has_write;
// revoked from PUBLIC, the role is ok (Ruling O3).
func TestReadonlyCheckerSecurityDefiner(t *testing.T) {
	role, pass := dbtest.Role(t, "")
	d := newRODB(t, role)
	d.exec(t, "CREATE FUNCTION public.f() RETURNS void LANGUAGE sql SECURITY DEFINER AS 'DELETE FROM public.t'")
	status, detail := check(t, d.cfg, pass)
	require.Equal(t, gateway.ReadonlyHasWrite, status)
	require.Equal(t, "EXECUTE on security definer function "+d.cfg.GetDatabase()+".public.f", detail)

	d.exec(t, "REVOKE EXECUTE ON FUNCTION public.f() FROM PUBLIC")
	status, detail = check(t, d.cfg, pass)
	require.Equal(t, [2]string{gateway.ReadonlyOK, ""}, [2]string{status, detail})
}

// A resource without agent_username cannot be checked (Ruling G6).
func TestReadonlyCheckerNoAgentUsername(t *testing.T) {
	role, pass := dbtest.Role(t, "")
	d := newRODB(t, role)
	d.cfg.AgentUsername = ""
	status, detail := check(t, d.cfg, pass)
	require.Equal(t, gateway.ReadonlyError, status)
	require.Contains(t, detail, "agent_username is not set")
}

// A superuser is has_write at once, without the member-role bound.
func TestReadonlyCheckerSuperuser(t *testing.T) {
	role, pass := dbtest.Role(t, "SUPERUSER")
	d := newRODB(t, role)
	status, detail := check(t, d.cfg, pass)
	require.Equal(t, [2]string{gateway.ReadonlyHasWrite, "role rolsuper"}, [2]string{status, detail})
}
