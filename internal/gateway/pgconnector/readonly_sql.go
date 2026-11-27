package pgconnector

// clusterSQL reads what the agent_readonly check needs from the shared
// catalogs: the credential's role first, then the roles it is a member of
// (whatever their inherit or set options; LIMIT one over the bound), with the
// roleAttrs flags; the roles any of them may grant (ADMIN OPTION); the
// databases its login may connect to.
const clusterSQL = `SELECT rolname, rolname = current_user, rolsuper, rolcreaterole, rolcreatedb, rolreplication, rolbypassrls
FROM pg_roles WHERE pg_has_role(current_user, oid, 'MEMBER')
ORDER BY rolname <> current_user, rolname LIMIT 34;
SELECT DISTINCT g.rolname FROM pg_auth_members m JOIN pg_roles g ON g.oid = m.roleid
WHERE m.admin_option AND pg_has_role(current_user, m.member, 'MEMBER') ORDER BY 1;
SELECT datname FROM pg_database
WHERE datallowconn AND NOT datistemplate AND has_database_privilege(current_user, oid, 'CONNECT') ORDER BY 1`

// databaseSQL lists, in a stable order, the first five ways the credential's
// role or a role it is a member of (" (via <role>)") can write in the current
// database: CREATE on it or a schema, owning it or an object in it, a write
// privilege on a relation or sequence, UPDATE on a large object, EXECUTE on a
// SECURITY DEFINER function of another owner outside pg_catalog and
// information_schema, lo_compat_privileges on. Temporary schemas are skipped.
const databaseSQL = `WITH r AS (
  SELECT oid, CASE WHEN rolname = current_user THEN '' ELSE ' (via ' || rolname || ')' END AS via
  FROM pg_roles WHERE pg_has_role(current_user, oid, 'MEMBER')),
d AS (SELECT current_database() AS db, current_setting('server_version_num')::int >= 170000 AS v17,
             (SELECT oid FROM pg_roles WHERE rolname = current_user) AS self),
f (k, finding) AS (
  SELECT 1, CASE WHEN g.datdba = r.oid THEN 'owner of database ' ELSE 'CREATE on database ' END || g.datname || r.via
  FROM pg_database g, r WHERE g.datname = current_database() AND (g.datdba = r.oid OR has_database_privilege(r.oid, g.oid, 'CREATE'))
  UNION ALL
  SELECT 2, CASE WHEN n.nspowner = r.oid THEN 'owner of schema ' ELSE 'CREATE on schema ' END || d.db || '.' || n.nspname || r.via
  FROM pg_namespace n, r, d
  WHERE n.nspname !~ '^pg_(toast_)?temp_' AND (n.nspowner = r.oid OR has_schema_privilege(r.oid, n.oid, 'CREATE'))
  UNION ALL
  SELECT 3, 'owner of ' || d.db || '.' || n.nspname || '.' || c.relname || r.via
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace, r, d
  WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f', 'S') AND c.relowner = r.oid AND n.nspname !~ '^pg_(toast_)?temp_'
  UNION ALL
  SELECT 4, p.priv || ' on ' || d.db || '.' || n.nspname || '.' || c.relname || r.via
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace, r, d,
       (VALUES ('INSERT'), ('UPDATE'), ('DELETE'), ('TRUNCATE'), ('TRIGGER'), ('MAINTAIN')) p (priv)
  WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f') AND n.nspname NOT IN ('pg_catalog', 'information_schema')
    AND n.nspname !~ '^pg_(toast_)?temp_'
    AND CASE p.priv
      WHEN 'INSERT' THEN has_any_column_privilege(r.oid, c.oid, 'INSERT')
      WHEN 'UPDATE' THEN has_any_column_privilege(r.oid, c.oid, 'UPDATE')
      WHEN 'MAINTAIN' THEN CASE WHEN d.v17 THEN has_table_privilege(r.oid, c.oid, 'MAINTAIN') ELSE false END
      ELSE has_table_privilege(r.oid, c.oid, p.priv) END
  UNION ALL
  SELECT 5, p.priv || ' on sequence ' || d.db || '.' || n.nspname || '.' || c.relname || r.via
  FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace, r, d, (VALUES ('UPDATE'), ('USAGE')) p (priv)
  WHERE c.relkind = 'S' AND n.nspname !~ '^pg_(toast_)?temp_' AND has_sequence_privilege(r.oid, c.oid, p.priv)
  UNION ALL
  SELECT 6, 'EXECUTE on security definer function ' || d.db || '.' || n.nspname || '.' || p.proname || r.via
  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace, r, d
  WHERE p.prosecdef AND p.proowner <> d.self AND n.nspname NOT IN ('pg_catalog', 'information_schema')
    AND has_function_privilege(r.oid, p.oid, 'EXECUTE')
  UNION ALL
  SELECT 7, 'owner of function ' || d.db || '.' || n.nspname || '.' || p.proname || r.via
  FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace, r, d
  WHERE p.proowner = r.oid AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  UNION ALL
  SELECT 8, CASE WHEN m.lomowner = r.oid THEN 'owner of large object ' ELSE 'UPDATE on large object ' END || d.db || '.' || m.oid || r.via
  FROM pg_largeobject_metadata m, r, d
  WHERE m.lomowner = r.oid
     OR EXISTS (SELECT 1 FROM aclexplode(m.lomacl) a WHERE a.privilege_type = 'UPDATE' AND a.grantee IN (r.oid, 0))
  UNION ALL
  SELECT 9, 'lo_compat_privileges on in database ' || d.db FROM d WHERE current_setting('lo_compat_privileges')::bool
)
SELECT finding FROM f ORDER BY k, finding LIMIT 5`
