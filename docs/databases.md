# Databases

Werbot reaches databases through connectors of the `stream` kind. This page covers the `postgres` type. Later releases add more database types to it.

## PostgreSQL

A resource of type `postgres` lets a user open a normal PostgreSQL connection through a gateway. The gateway holds the database credential: the client never sees it and sends no password. Every operation that reaches the database is written to a query log that is stored with the session recording and shown in the web UI.

### Resource fields

| Field | Meaning |
| --- | --- |
| `host` | Database host (a name or an address). |
| `port` | Database port; `0` or empty means `5432`. |
| `database` | Default database, up to 63 bytes. Empty means `postgres`. |
| `allowed_databases` | Up to 64 globs (`path.Match` syntax, each 1 to 63 bytes) that limit the databases a client may ask for. Empty allows any. |
| `sslmode` | TLS between the gateway and the database: `verify-full` (default), `verify-ca` or `disable`. |
| `ca_pem` | PEM CA certificates that verify the database, up to 65 536 bytes. Required for `verify-ca`; for `verify-full` empty uses the system roots. |
| `record_params` | Records the values of bind parameters in the query log. On when unset. |
| `username` | Database role the gateway logs in as, 1 to 63 bytes. |
| `secret_tables` | Up to 64 globs over `schema.table` (`path.Match` syntax, each up to 255 bytes). A query that reads a matching table gets the class `secrets`. |
| `agent_username` | Database role AI agents log in as, up to 63 bytes, with the [agent credential](#agent-credential). Empty: agents get no access. |

### TLS to the database

- `verify-full` checks the certificate chain and the host name. Use it for every database you can.
- `verify-ca` checks the certificate chain against `ca_pem` but not the host name. Use it when the gateway reaches the database by an IP address or through NAT, so the name in the certificate does not match `host`. `ca_pem` is required: a resource with `verify-ca` and no `ca_pem` is invalid.
- `disable` sends queries, results and the password exchange in plain text between the gateway and the database. Use it only on a network you trust, and run the gateway next to the database. The web form shows a warning for this choice.

The client to gateway leg is the SSH channel of `werbot connect`, so the client connects to its local port with `sslmode=disable`.

### Credentials

Set the credential on the resource's Credentials tab (or with `SetCredential`). Kinds:

- `password`: the role's password.
- `client_cert`: a PEM client certificate followed by its private key in one value, up to 16 384 bytes. It needs TLS: with `sslmode=disable` a connect fails with `client certificate requires TLS`.

The web UI sets the [agent credential](#agent-credential) as a password only; set a `client_cert` agent credential with `SetCredential`.

The gateway reads the credential for each session; it is never cached on the gateway and never appears in errors, logs, audit data or the query log. The gateway ignores the `PG*` environment variables, a password file and a `pg_service` file of its own process: only the resource decides the connection. An unreadable `PGSERVICE` in the gateway environment makes every PostgreSQL connect fail closed.

### Connect

```sh
werbot connect --gateway gw.example.com --login alice pg
psql "postgres://127.0.0.1:5432/app?sslmode=disable"
```

`werbot connect` ([connect.md](connect.md)) opens a local port for the resource. Any PostgreSQL client works: `psql`, drivers and GUI tools. Access is decided by the Werbot policy for `postgres.connect`; the gateway logs in to the database with the resource's `username` and credential.

The client's other start parameters (`options`, `application_name`, `client_encoding`, `search_path` and other settings) are passed to the database. They take effect as the resource's database role, the same as a `SET` from that role; a few settings can be changed only at connection start, so grant the role only what its users may set. A session whose start parameters carry a non-empty `options` or a protected setting (the list under `privilege` in [Query checks](#query-checks)) has every operation classified `opaque`: query rules decide on it, and a person without query rules takes the fast path as before. A database name longer than 63 bytes is cut to 63 bytes, as PostgreSQL does (on the byte, even inside a multibyte character), before the `allowed_databases` check.

The gateway refuses before it creates a session, so no session row exists for these cases:

| Client sends | Answer |
| --- | --- |
| A replication connection | `FATAL 42501 werbot: replication connections are not allowed` |
| A database outside `allowed_databases` | `FATAL 42501 werbot: database "<db>" is not allowed` |
| Any other startup packet | The connection closes with `unsupported startup packet`. |

An `SSLRequest` or `GSSENCRequest` is answered `N` (at most twice); the client continues in plain text on its local port. The startup packet is 8 to 10 000 bytes and the protocol is 3.0 or 3.2. For a 3.2 client the gateway answers with `NegotiateProtocolVersion` and speaks 3.0 on both legs.

If the database cannot be reached the client gets `08006 werbot: connection to <resource> failed`. A database error is passed on with its own severity and SQLSTATE as `werbot: connection to <resource> failed: <message>`. The address and the secret are never in the text.

### Cancel

A client cancels a query with a new connection that sends a `CancelRequest`. The gateway hands every session a random 64-bit key and maps it to the database connection of that session on the same gateway instance. An unknown key does nothing and gets no answer; a cancel never opens a session. A key cancels only a session of the same resource: a cancel request that arrives through another resource cancels nothing. The cancel is written to the query log of the session it cancels as a `cancel` line.

### Messages, COPY and limits

- `COPY` data passes through; the log holds the command and the byte count.
- `DataRow` and `CopyData` are copied through, never buffered whole.
- `Query`, `Parse`, `Function call`, `Execute` and `Close` messages may be up to 16 MiB; a larger one ends the connection with `FATAL 54000 werbot: message exceeds 16 MiB`. A larger `Bind` is streamed and its parameters are logged as one `…[truncated]` value.
- At most 10 000 prepared statements and portals per connection (`FATAL 54000 werbot: too many prepared statements`).
- At most 65 536 client messages per connection wait for a reply from the database (`FATAL 54000 werbot: too many pending messages`).
- Prepared statements and portals keep at most 16 MiB per connection: their names, logged text and parameters (`FATAL 54000 werbot: prepared statements and portals exceed 16 MiB`). A `Close` gives the bytes back.
- A malformed protocol message closes the connection without an answer.
- Logged text is cut at 64 KiB on a UTF-8 boundary and ends with `…[truncated]`. Bind parameters are logged up to 64 KiB in total: the values past that bound are replaced by one `…[truncated]` value. Binary values are logged as `\x<hex>`, NULL as `\N`, and a text value that starts with a backslash gets one more in front, so no two values look the same.

### Query checks

Every operation that would reach the database (a simple `Query`, an `Execute` of a prepared statement, a fast-path function call) is decided by the policy for `postgres.query` before it is sent. The gateway follows prepared statements as the database does: a `Parse` to a name that is still in use replaces nothing (the database refuses it), and after an SQL `PREPARE`, `DEALLOCATE` or `DISCARD` every named statement is forgotten. An `Execute` of a statement or portal the database has not confirmed, or that the gateway no longer knows, is `opaque` and is never approved. The request carries the operation's `classes` and, in the Cedar context, `db_name`, `tables` (every table it reads or writes, as `schema.table`; an unqualified name is `public`, or `pg_catalog` for `pg_*`) and `write_tables`. The classifier does not know the session's `search_path` (the default `"$user", public` included): an unqualified name may reach a table of another schema, and a `secret_tables` glob on that schema then misses it. Cover such a table with a glob such as `*.<table>`. [Command rules](policies.md#command-rules) apply to queries with the same classes as commands.

| Class | Queries |
| --- | --- |
| `read` | `SELECT`, `SHOW`, `EXPLAIN` without `ANALYZE` (the functions it names keep their classes: planning can run them), `FETCH`, `COPY … TO STDOUT`, `BEGIN`/`COMMIT`/`ROLLBACK`; an empty text. |
| `write` | `INSERT`, `UPDATE` and `DELETE` with `WHERE`, `MERGE`, `CREATE`, most `ALTER TABLE` changes, `COPY … FROM STDIN`, `VACUUM`, `LOCK`, `NOTIFY`, prepared transactions. |
| `destructive` | `DROP`, `TRUNCATE`, `UPDATE` or `DELETE` without `WHERE`, `ALTER TABLE … DROP …`. |
| `privilege` | `GRANT`, roles, owners, row security, `ALTER SYSTEM`, extensions, `SET`/`RESET`/`set_config` of a protected setting (`statement_timeout`, `default_transaction_read_only`, `transaction_read_only`, `role`, `session_authorization`, `search_path`, `standard_conforming_strings`, `client_encoding`, `backslash_quote`), `BEGIN READ WRITE`. |
| `secrets` | A read of `pg_authid`, `pg_shadow`, `pg_user_mapping(s)` or a table in `secret_tables`. |
| `egress` | `COPY` to or from a server file or program, `lo_import`, `lo_export`, `dblink`, foreign servers and user mappings. |
| `opaque` | Text the classifier cannot judge (see below). |

The classes of a text with several statements are the union of all of them. Built-in functions of `pg_catalog` have classes of their own (`pg_read_file` is `secrets`, `pg_terminate_backend` is `privilege`, …); a call of any other function is `opaque`, and so is attribute notation `(expr).name`, which PostgreSQL may run as the call `name(expr)`. A two-part name `t.name` is judged a column (see the limits below).

A person whose policy has no rule for queries on the resource takes the fast path: the operation goes to the database without waiting for the classifier, and its querylog line gets its classes afterwards. Classifications are kept in a cache of 4096 texts per type on each gateway, decisions in a cache of 1024 per session for 60 seconds; a new policy snapshot clears the decisions.

### Limits of the classifier

The classifier parses with libpg_query (the PostgreSQL 17 grammar, compiled to WebAssembly). It never guesses: the result is `opaque` for a parse error, syntax that only PostgreSQL 18 knows, `DO`, `CALL`, SQL `EXECUTE`, an unknown function, a text over 1 MiB, more than 2000 operators in one text, a parser that is busy for more than 100 ms or not warmed up yet, and a parser that failed (`werbot_gateway_sql_parser_broken{type}` is 1 until the gateway restarts).

- `opaque` is a class of its own: a rule on `destructive` or `write` does not match a text that does not parse. List `opaque` in the rule to cover it.
- Unqualified function names are judged as the `pg_catalog` functions of that name: the classifier assumes `pg_catalog` comes first in `search_path`. A role or database default `search_path` (or the default `"$user"` schema) can change that.
- Custom operators and casts are not checked: an operator or a cast can run a function.
- Attribute notation is not checked for `t.name`: PostgreSQL runs it as `name(t)` when the row of `t` has no column `name` (a computed field). Without the catalog the classifier cannot tell that from a column, so `t.name` is a column read; `(expr).name` is `opaque`.
- While the session's `standard_conforming_strings` is not `on`, or its `client_encoding` is `SJIS`, `SHIFT_JIS_2004`, `BIG5`, `GBK`, `UHC`, `GB18030` or `JOHAB`, every operation is `opaque`: the database could split such text unlike the classifier. The gateway drops the client's start parameters `standard_conforming_strings` and `backslash_quote` and sends `standard_conforming_strings=on`.

For people, classes are what policy rules work with. For agents, the guarantee is the rights of the [agent credential](#agent-credential): the classifier only decides what is tried.

### Denials

A refused operation gets `ERROR 42501 werbot: query denied: <message>` (the rule's message, or `not allowed (<classes>)`) in the order of the replies, and the session goes on. A simple query gets the error and `ReadyForQuery`; in the extended protocol the gateway drops the messages up to the next `Sync`, as the database does after an error. The querylog has a line with `decision` `deny` for it. Two differences from an error of the database itself:

- A denial inside an explicit transaction does not abort that transaction.
- A denial inside an extended batch lets the statements of the batch before it commit at `Sync`.

### Approvals

A query under an `approval` rule waits for a person. The client gets `NOTICE 00000 werbot: Waiting for approval <id>, timeout <duration>: <url>` and every 30 seconds `werbot: Still waiting for approval <id>`; the replies to earlier messages still arrive, later messages wait. The gateway drops every notice of the database whose message starts with `werbot:`, so a notice with that prefix always comes from the gateway.

The approval holds the exact text after a first line `-- database: <db>`, so an approval in one database never covers the same text in another. For an `Execute` it holds the text plus its parameter values as the querylog shows them (a text value that is not valid UTF-8 shown as `\X<hex>`, and a valid one that starts with `\X` with one more backslash in front), so an approval for one set of parameters never covers another. The approver sees, and the server stores with the request, these values even when `record_params` is off. The parameter types of `Parse` are not part of it. A text over 16 384 bytes is refused with `query too long to approve (16384 bytes)`; a refusal is `approval denied`, `approval timed out` or `approval unavailable, try again later`.

### Agent credential

AI agents reach a database only with a second credential (slot `agent_readonly`, the **Agent credential** block of the Credentials tab, or `SetCredential` with `slot`) for the role `agent_username`. Grant that role reads only:

```sql
CREATE ROLE werbot_agent LOGIN PASSWORD '…';
GRANT CONNECT ON DATABASE app TO werbot_agent;
GRANT USAGE ON SCHEMA public TO werbot_agent;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO werbot_agent;
```

A gateway of the pool checks the credential when it is set and every 10 minutes (30 seconds per check). It logs in as `agent_username`, read-only, and looks at the role and every role it is a member of (at most 32): the attributes `SUPERUSER`, `CREATEROLE`, `CREATEDB`, `REPLICATION` and `BYPASSRLS`, membership of `pg_write_all_data`, `pg_execute_server_program`, `pg_write_server_files` and `pg_maintain`, and roles it may grant. Then, in every database within `allowed_databases` it may connect to (at most 32): `CREATE` on the database or a schema, ownership of any object of the database or of a shared object (a database, a tablespace; default privileges set for the role are not a finding), `lo_compat_privileges` on, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `TRIGGER` or `MAINTAIN` on a table, `UPDATE` or `USAGE` on a sequence, `UPDATE` on a large object, `EXECUTE` on a `SECURITY DEFINER` function of another owner outside `pg_catalog` and `information_schema`.

| Status | Meaning |
| --- | --- |
| `none` | No agent credential. |
| `pending` | Set or changed (a change of the resource too), not checked yet. |
| `ok` | Nothing found: agents may connect. |
| `has_write` | Up to five findings in the detail, for example `INSERT on app.public.orders`. |
| `error` | The check failed (login, more than 32 roles or databases, `agent_username is not set`). |

Only `ok` lets agents in; every other status refuses them, even under a custom `permit`. `werbot_gateway_agent_readonly_checks_total{result}` counts the checks by `ok`, `has_write` and `error`; a change of status is audited as `resource.agent_readonly.check`.

### Agent sessions

An agent session logs in as `agent_username` with `default_transaction_read_only=on`, the agent's `statement_timeout` and only the start parameters `application_name`, `client_encoding`, `DateStyle`, `TimeZone`, `IntervalStyle` and `extra_float_digits`. The agent baseline allows `read` queries; anything else is denied unless a command rule for agents allows it or asks for an approval. Its next operation starts only after the previous one is answered.

An approved `write` of an agent runs on a connection of its own as `username` with the primary credential, one statement, outside a transaction (`approved writes run one statement at a time`, `approved writes run outside a transaction`). The agent's limits ([agents.md](agents.md#limits)) apply per session: past `max_queries` an operation gets `query limit <n> reached`. A result past `max_rows` or `max_result_bytes` is cut: the gateway cancels the query, drops the rest and sends `WARNING 01000 werbot: result truncated at <n> rows` (or `bytes`) and `SELECT <n>`; a cut `COPY … TO STDOUT` ends with the database's `57014`. A cut inside an explicit transaction leaves it failed (`ReadyForQuery` `E`): roll it back. An approved write is limited the same way: a cut cancels it, so the write is rolled back and the agent gets the database's `57014`. With `mask_output` ([policies.md](policies.md#output-masking)) the gateway masks the cells of the rows and the message, detail and hint of errors and notices, notification payloads and fast-path results.

### Health

A health probe connects and sends a startup message without a password. Any answer from the server (an authentication request or an error of class `28`) counts as a live database. The probe times out after 5 seconds and uses the same guarded dialer as sessions.

### Query log

A `postgres` session is recorded as a query log (`recording.format` `querylog`) instead of a terminal recording. Each line is one operation that reached the database, written in order. A full log buffer (1024 entries) slows the session down; lines are never dropped. The hash chain and the integrity check are the same as for terminal recordings.

Replay: `GET /api/recordings/<session_id>.querylog` (`application/x-ndjson`; the header `X-Werbot-Recording-Verified` is `true` or `false`). The `.cast` extension on a query log answers 404, and the other way round. The same access rules as for `.cast` apply.

A line is a JSON object; empty fields are left out except `t` and `kind`:

| Key | Meaning |
| --- | --- |
| `t` | Seconds since the gateway logged in to the database for the session, to the microsecond. |
| `kind` | `query`, `parse`, `execute`, `cancel`, `command`, `request` or `tool_call`. |
| `sql`, `command` | The text. `command` holds it for the kinds `command`, `request` and `tool_call`, `sql` for the rest. |
| `db` | Database name. |
| `params` | Bind parameter values (only with `record_params`; an approval holds them regardless, see [Approvals](#approvals)). |
| `statement` | Prepared statement name for `execute` and `parse` (empty for the unnamed statement); the text is in `sql`. |
| `classes`, `tables`, `decision`, `policy_ids` | Result of the policy check: `decision` is `allow` or `deny` (a denied operation never reached the database). |
| `rows`, `bytes` | Rows and data bytes the database returned (`COPY FROM`: bytes sent). |
| `duration_us` | Time until the answer, in microseconds. |
| `error` | `<SQLSTATE>: <message>` of the database error when the operation failed, for example `22012: division by zero`. |

In the extended protocol there is one line per statement that reached the database: a successful `Parse` writes none, a failed `Parse` writes a `parse` line, and a failed `Bind` writes an `execute` line. After an error the database skips every message up to the next `Sync`, so the statements it skipped get no line. An error at `Sync` (for example a deferred constraint at the implicit commit) writes an `execute` line with no text and that error. A `Sync` sent during `COPY FROM STDIN` is ignored by the database and gets no line. A fast-path function call is a `query` line with the text `fastpath function <oid>`.

The query log stores the text the way the client sent it. With a `client_encoding` other than `UTF8`, text that is not valid UTF-8 appears in the query log with replacement characters, so it can differ from the text the database ran.

`session.queries` counts the accepted lines (cancel lines included). The web UI session page shows the table with filters for kind, decision, text and errors, the first 1000 lines and a button for the rest, and the integrity banner. A query log can hold bind parameter values: turn off `record_params` for resources where parameters carry secrets.

### Metrics

| Metric | Meaning |
| --- | --- |
| `werbot_gateway_query_overhead_seconds{type}` | Time the gateway adds to one operation (histogram). |
| `werbot_gateway_querylog_blocked_total{type}` | Times a session waited for a full query log buffer. |

Budgets and the benchmark: [scaling.md](scaling.md#database-connections).
