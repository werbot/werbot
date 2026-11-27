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

### TLS to the database

- `verify-full` checks the certificate chain and the host name. Use it for every database you can.
- `verify-ca` checks the certificate chain against `ca_pem` but not the host name. Use it when the gateway reaches the database by an IP address or through NAT, so the name in the certificate does not match `host`. `ca_pem` is required: a resource with `verify-ca` and no `ca_pem` is invalid.
- `disable` sends queries, results and the password exchange in plain text between the gateway and the database. Use it only on a network you trust, and run the gateway next to the database. The web form shows a warning for this choice.

The client to gateway leg is the SSH channel of `werbot connect`, so the client connects to its local port with `sslmode=disable`.

### Credentials

Set the credential on the resource's Credentials tab (or with `SetCredential`). Kinds:

- `password`: the role's password.
- `client_cert`: a PEM client certificate followed by its private key in one value, up to 16 384 bytes. It needs TLS: with `sslmode=disable` a connect fails with `client certificate requires TLS`.

The gateway reads the credential for each session; it is never cached on the gateway and never appears in errors, logs, audit data or the query log. The gateway ignores the `PG*` environment variables, a password file and a `pg_service` file of its own process: only the resource decides the connection. An unreadable `PGSERVICE` in the gateway environment makes every PostgreSQL connect fail closed.

### Connect

```sh
werbot connect --gateway gw.example.com --login alice pg
psql "postgres://127.0.0.1:5432/app?sslmode=disable"
```

`werbot connect` ([connect.md](connect.md)) opens a local port for the resource. Any PostgreSQL client works: `psql`, drivers and GUI tools. Access is decided by the Werbot policy for `postgres.connect`; the gateway logs in to the database with the resource's `username` and credential.

The client's other start parameters (`options`, `application_name`, `client_encoding`, `search_path` and other settings) are passed to the database. They take effect as the resource's database role, the same as a `SET` from that role; a few settings can be changed only at connection start, so grant the role only what its users may set. A database name longer than 63 bytes is cut to 63 bytes, as PostgreSQL does (on the byte, even inside a multibyte character), before the `allowed_databases` check.

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
- `Query`, `Parse`, `Function call`, `Execute` and `Close` messages may be up to 16 MiB; a larger one ends the connection with `FATAL 54000 werbot: message exceeds 16 MiB`. A larger `Bind` is streamed and its parameters are not logged.
- At most 10 000 prepared statements and portals per connection (`FATAL 54000 werbot: too many prepared statements`).
- At most 65 536 client messages per connection wait for a reply from the database (`FATAL 54000 werbot: too many pending messages`).
- Prepared statements and portals keep at most 16 MiB per connection: their names, logged text and logged parameters (`FATAL 54000 werbot: prepared statements and portals exceed 16 MiB`). A `Close` gives the bytes back.
- A malformed protocol message closes the connection without an answer.
- Logged text is cut at 64 KiB on a UTF-8 boundary and ends with `…[truncated]`. Bind parameters are logged up to 64 KiB in total: the values past that bound are replaced by one `…[truncated]` value. Binary values are logged as `\x<hex>`, NULL as `\N`.

Policy decisions on the SQL text, masking of results and agent access to databases are not part of this release: the lines of `Query`, `Parse`, `Execute` and function calls carry `decision` `allow` (a `cancel` line and the `execute` line of a failed `Bind` carry none), and `classes`, `tables` and `policy_ids` stay empty until a later release decides statements. Agents never reach `postgres.connect`: the baseline forbids it.

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
| `params` | Bind parameter values (only with `record_params`). |
| `statement` | Prepared statement name for `execute` and `parse` (empty for the unnamed statement); the text is in `sql`. |
| `classes`, `tables`, `decision`, `policy_ids` | Result of the policy check. |
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
