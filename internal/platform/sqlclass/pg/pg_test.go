package pg

import (
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

const (
	read        = shellclass.Read
	write       = shellclass.Write
	destructive = shellclass.Destructive
	privilege   = shellclass.Privilege
	egress      = shellclass.Egress
	opaque      = shellclass.Opaque
)

func TestMain(m *testing.M) {
	if err := Warmup(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func classify(sql string) sqlclass.Result {
	return sqlclass.Classify(sqlclass.Postgres, sql, sqlclass.Options{})
}

var statementCases = []struct {
	sql        string
	classes    []shellclass.Class
	statements int
}{
	{"SELECT 1", []shellclass.Class{read}, 1},
	{"VALUES (1), (2)", []shellclass.Class{read}, 1},
	{"TABLE t", []shellclass.Class{read}, 1},
	{"SELECT 1 UNION SELECT 2", []shellclass.Class{read}, 1},
	{"SHOW work_mem", []shellclass.Class{read}, 1},
	{"BEGIN; COMMIT", []shellclass.Class{read}, 2},
	{"BEGIN READ ONLY", []shellclass.Class{read}, 1},
	{"BEGIN READ WRITE", []shellclass.Class{privilege}, 1},
	{"SET TRANSACTION READ WRITE", []shellclass.Class{privilege}, 1},
	{"SET SESSION CHARACTERISTICS AS TRANSACTION READ WRITE", []shellclass.Class{privilege}, 1},
	{"COMMIT PREPARED 'x'", []shellclass.Class{write}, 1},
	{"FETCH 1 FROM c; CLOSE c; DEALLOCATE s; LISTEN ch; UNLISTEN ch", []shellclass.Class{read}, 5},
	{"SET work_mem = '64MB'", []shellclass.Class{read}, 1},
	{"SET LOCAL statement_timeout = 0", []shellclass.Class{privilege}, 1},
	{"SET ROLE admin", []shellclass.Class{privilege}, 1},
	{"SET NAMES 'SJIS'", []shellclass.Class{privilege}, 1},
	{"RESET search_path", []shellclass.Class{privilege}, 1},
	{"RESET ALL", []shellclass.Class{privilege}, 1},
	{"DISCARD ALL", []shellclass.Class{privilege}, 1},
	{"INSERT INTO t VALUES (1)", []shellclass.Class{write}, 1},
	{"INSERT INTO t SELECT * FROM s", []shellclass.Class{write}, 1},
	{"INSERT INTO t DEFAULT VALUES", []shellclass.Class{write}, 1},
	{"UPDATE t SET a = 1 WHERE id = 2", []shellclass.Class{write}, 1},
	{"UPDATE t SET a = 1", []shellclass.Class{destructive}, 1},
	{"DELETE FROM t", []shellclass.Class{destructive}, 1},
	{"DELETE FROM t WHERE id = 1", []shellclass.Class{write}, 1},
	{"MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN DELETE", []shellclass.Class{write}, 1},
	{"NOTIFY ch; LOCK t; VACUUM t; CLUSTER t; REINDEX TABLE t; REFRESH MATERIALIZED VIEW v", []shellclass.Class{write}, 6},
	{"ALTER TABLE t RENAME TO u; COMMENT ON TABLE t IS 'x'", []shellclass.Class{write}, 2},
	{"CREATE TABLE t (a int); CREATE INDEX i ON t (a); CREATE SEQUENCE s; CREATE DATABASE d", []shellclass.Class{write}, 4},
	{"CREATE VIEW v AS SELECT 1; CREATE TABLE u AS SELECT 1; SELECT 1 INTO w", []shellclass.Class{write}, 3},
	{"CREATE FUNCTION f() RETURNS int LANGUAGE sql AS 'SELECT 1'", []shellclass.Class{write}, 1},
	{"CREATE TRIGGER g AFTER INSERT ON t EXECUTE FUNCTION f()", []shellclass.Class{write}, 1},
	{"DROP TABLE t", []shellclass.Class{destructive}, 1},
	{"TRUNCATE t", []shellclass.Class{destructive}, 1},
	{"DROP DATABASE d; DROP TABLESPACE s", []shellclass.Class{destructive}, 2},
	{"ALTER TABLE t ADD c int", []shellclass.Class{write}, 1},
	{"ALTER TABLE t DROP COLUMN c", []shellclass.Class{destructive}, 1},
	{"ALTER TABLE t ALTER c DROP NOT NULL", []shellclass.Class{destructive}, 1},
	{"ALTER TABLE t DISABLE ROW LEVEL SECURITY", []shellclass.Class{privilege}, 1},
	{"ALTER TABLE t OWNER TO r", []shellclass.Class{privilege}, 1},
	{"GRANT SELECT ON t TO r", []shellclass.Class{privilege}, 1},
	{"GRANT admin TO r", []shellclass.Class{privilege}, 1},
	{"CREATE ROLE r; ALTER ROLE r SUPERUSER; ALTER ROLE r SET search_path = x", []shellclass.Class{privilege}, 3},
	{"ALTER DATABASE d SET x = 1; ALTER SYSTEM SET x = 1; ALTER DEFAULT PRIVILEGES GRANT SELECT ON TABLES TO r", []shellclass.Class{privilege}, 3},
	{"ALTER FUNCTION f() OWNER TO r; REASSIGN OWNED BY a TO b", []shellclass.Class{privilege}, 2},
	{"CREATE EXTENSION x", []shellclass.Class{privilege}, 1},
	{"ALTER EXTENSION x UPDATE; LOAD 'x'", []shellclass.Class{privilege}, 2},
	{"CREATE POLICY p ON t; ALTER POLICY p ON t TO r; SECURITY LABEL ON TABLE t IS 'x'; CHECKPOINT", []shellclass.Class{privilege}, 4},
	{"DROP ROLE r", []shellclass.Class{destructive, privilege}, 1},
	{"DROP OWNED BY r", []shellclass.Class{destructive, privilege}, 1},
	{"SELECT * FROM t FOR UPDATE", []shellclass.Class{read, write}, 1},
	{"WITH d AS (DELETE FROM t RETURNING *) SELECT * FROM d", []shellclass.Class{destructive, read}, 1},
	{"WITH s AS (SELECT 1) INSERT INTO t SELECT * FROM s", []shellclass.Class{write}, 1},
	{"WITH d AS (DELETE FROM t RETURNING *) INSERT INTO u SELECT * FROM d", []shellclass.Class{destructive, write}, 1},
	{"EXPLAIN DELETE FROM t", []shellclass.Class{read}, 1},
	{"EXPLAIN (ANALYZE false) DELETE FROM t", []shellclass.Class{read}, 1},
	{"EXPLAIN ANALYZE DELETE FROM t", []shellclass.Class{destructive}, 1},
	{"EXPLAIN (ANALYZE maybe) DELETE FROM t", []shellclass.Class{destructive}, 1},
	{"COPY t FROM STDIN", []shellclass.Class{write}, 1},
	{"COPY t TO STDOUT", []shellclass.Class{read}, 1},
	{"COPY (DELETE FROM t RETURNING *) TO STDOUT", []shellclass.Class{destructive, read}, 1},
	{"CREATE SERVER s FOREIGN DATA WRAPPER w; CREATE USER MAPPING FOR r SERVER s; IMPORT FOREIGN SCHEMA x FROM SERVER s INTO y", []shellclass.Class{egress}, 3},
	{"PREPARE p AS DELETE FROM t", []shellclass.Class{destructive}, 1},
	{"DECLARE c CURSOR FOR SELECT 1", []shellclass.Class{read}, 1},
	{"DO $$ BEGIN END $$", []shellclass.Class{opaque}, 0},
	{"CALL p()", []shellclass.Class{opaque}, 0},
	{"EXECUTE s", []shellclass.Class{opaque}, 0},
	{"SELECT 1; CALL p()", []shellclass.Class{opaque, read}, 2},
	{"SELEC 1", []shellclass.Class{opaque}, 0},
	{"", []shellclass.Class{read}, 0},
	{"-- ping", []shellclass.Class{read}, 0},
}

func TestStatementClasses(t *testing.T) {
	for _, tc := range statementCases {
		r := classify(tc.sql)
		if !slices.Equal(r.Classes, tc.classes) || r.Transient {
			t.Errorf("%q: classes %v transient %v, want %v", tc.sql, r.Classes, r.Transient, tc.classes)
		}
		if tc.classes[0] != opaque && r.Statements != tc.statements {
			t.Errorf("%q: statements %d, want %d", tc.sql, r.Statements, tc.statements)
		}
	}
}

func TestCopyProgramAndFileAreEgress(t *testing.T) {
	for _, sql := range []string{
		"COPY t FROM PROGRAM 'cmd'", "COPY t TO PROGRAM 'cmd'",
		"COPY t FROM '/srv/file'", "COPY t TO '/srv/file'", "COPY (SELECT 1) TO '/srv/file'",
	} {
		if r := classify(sql); !slices.Contains(r.Classes, egress) {
			t.Errorf("%q: classes %v, want egress", sql, r.Classes)
		}
	}
	if r := classify("COPY t FROM '/srv/file'"); !slices.Contains(r.Classes, write) {
		t.Errorf("COPY FROM file: classes %v, want write", r.Classes)
	}
}

func TestUnknownNodeIsOpaque(t *testing.T) {
	for _, sql := range []string{
		"CREATE EVENT TRIGGER e ON ddl_command_start EXECUTE FUNCTION f()",
		"CREATE RULE r AS ON INSERT TO t DO INSTEAD NOTHING",
	} {
		if r := classify(sql); !slices.Equal(r.Classes, []shellclass.Class{opaque}) || r.Transient {
			t.Errorf("%q: classes %v transient %v, want deterministic opaque", sql, r.Classes, r.Transient)
		}
	}
}

func TestWarmup(t *testing.T) {
	if err := Warmup(); err != nil {
		t.Fatal(err)
	}
	if err := Warmup(); err != nil {
		t.Fatal(err)
	}
	if Broken() {
		t.Fatal("parser broken after warm-up")
	}
}

func TestConcurrentClassify(t *testing.T) {
	want := make([]sqlclass.Result, len(statementCases))
	for i, tc := range statementCases {
		want[i] = classify(tc.sql)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for range 64 {
		wg.Go(func() {
			for j := range 100 {
				i := j % len(statementCases)
				if got := classify(statementCases[i].sql); !slices.Equal(got.Classes, want[i].Classes) || got.Transient {
					errs <- statementCases[i].sql
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for sql := range errs {
		t.Errorf("%q: concurrent result differs from sequential", sql)
	}
}
