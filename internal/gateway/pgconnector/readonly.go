package pgconnector

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/sqlclass/pg"
)

// Bounds of the agent_readonly check (contract §3.2).
const (
	maxCheckedRoles     = 32 // roles the credential is a member of
	maxCheckedDatabases = 32
	maxReadonlyFindings = 5
)

// roleAttrs are the pg_roles flags that let a role write or manage roles,
// in the column order of clusterSQL.
var roleAttrs = []string{"rolsuper", "rolcreaterole", "rolcreatedb", "rolreplication", "rolbypassrls"}

// writeRoles are the predefined roles whose members can write.
var writeRoles = []string{"pg_write_all_data", "pg_execute_server_program", "pg_write_server_files", "pg_maintain"}

// Warmup compiles the SQL parser; a failure is logged by the parser and
// leaves every query opaque.
func (Connector) Warmup() { _ = pg.Warmup() }

// CheckReadonly verifies that the agent_readonly credential of r can write
// nothing it reaches: it logs in as agent_username with agent through dial
// and checks the credential's role and every role it is a member of (at most
// 32) in every database it may connect to within allowed_databases (at most
// 32). Findings are has_write with the first five; any failure is error.
func (Connector) CheckReadonly(ctx context.Context, r gateway.Resource, _, agent gateway.Secret,
	dial func(ctx context.Context, network, address string) (net.Conn, error),
) (status, detail string) {
	findings, err := checkReadonly(ctx, r, agent, dial)
	switch {
	case err != nil:
		return gateway.ReadonlyError, agent.Redact(err.Error())
	case len(findings) > 0:
		return gateway.ReadonlyHasWrite, strings.Join(findings[:min(len(findings), maxReadonlyFindings)], "; ")
	}
	return gateway.ReadonlyOK, ""
}

func checkReadonly(ctx context.Context, r gateway.Resource, agent gateway.Secret, dial pgconn.DialFunc) ([]string, error) {
	cfg, err := resourceConfig(r)
	if err != nil {
		return nil, err
	}
	if cfg.GetAgentUsername() == "" {
		return nil, errNoAgentUsername
	}
	results, err := readonlyQuery(ctx, cfg, agent, dial, cmp.Or(cfg.GetDatabase(), "postgres"), clusterSQL)
	if err != nil {
		return nil, err
	}
	findings, dbs, err := clusterFindings(results, cfg.GetAllowedDatabases())
	for _, db := range dbs {
		if err != nil || len(findings) >= maxReadonlyFindings {
			break
		}
		var rs []*pgconn.Result
		if rs, err = readonlyQuery(ctx, cfg, agent, dial, db, databaseSQL); err == nil {
			findings = append(findings, column(rs[0])...)
		}
	}
	return findings, err
}

// readonlyQuery runs sql (constant statements) in database db as the agent,
// read-only.
func readonlyQuery(ctx context.Context, cfg *postgresv1.Config, agent gateway.Secret, dial pgconn.DialFunc, db, sql string) ([]*pgconn.Result, error) {
	params := map[string]string{"default_transaction_read_only": "on", "application_name": "werbot agent_readonly check"}
	pc, err := dialConfig(cfg, agent, cfg.GetAgentUsername(), params, db, dial)
	if err != nil {
		return nil, err
	}
	conn, err := pgconn.ConnectConfig(ctx, pc)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	return conn.Exec(ctx, sql).ReadAll()
}

// clusterFindings reads the answers of clusterSQL: the role findings, then
// the databases to check. A superuser needs no more; more than
// maxCheckedRoles member roles or maxCheckedDatabases databases is an error.
func clusterFindings(rs []*pgconn.Result, allowed []string) (findings, dbs []string, err error) {
	if len(rs) != 3 || len(rs[0].Rows) == 0 {
		return nil, nil, errors.New("postgres: unexpected answer to the role query")
	}
	roles := rs[0].Rows
	if string(roles[0][2]) == "t" { // the credential's own role is first
		return []string{"role rolsuper"}, nil, nil
	}
	if len(roles) > maxCheckedRoles+1 {
		return nil, nil, fmt.Errorf("too many roles to verify (%d)", maxCheckedRoles)
	}
	for _, row := range roles {
		findings = append(findings, roleFindings(row)...)
	}
	for _, name := range column(rs[1]) {
		findings = append(findings, "admin of role "+name)
	}
	for _, db := range column(rs[2]) {
		if allowedDB(allowed, db) {
			dbs = append(dbs, db)
		}
	}
	if len(dbs) > maxCheckedDatabases {
		return nil, nil, fmt.Errorf("too many databases to verify (%d)", maxCheckedDatabases)
	}
	return findings, dbs, nil
}

// roleFindings are the findings of one row of clusterSQL's first query:
// rolname, whether it is the credential's role, then roleAttrs.
func roleFindings(row [][]byte) []string {
	name, via := string(row[0]), ""
	if string(row[1]) != "t" {
		via = " (via " + name + ")"
	}
	var out []string
	if slices.Contains(writeRoles, name) {
		out = append(out, "member of "+name)
	}
	for i, attr := range roleAttrs {
		if string(row[2+i]) == "t" {
			out = append(out, "role "+attr+via)
		}
	}
	return out
}

// column is the first column of r's rows.
func column(r *pgconn.Result) []string {
	out := make([]string, len(r.Rows))
	for i, row := range r.Rows {
		out[i] = string(row[0])
	}
	return out
}
