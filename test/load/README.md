# Load tests

Manual, not part of CI. Criteria: design doc 10 §8.3 (L1–L5) and §10.1 (budgets).

## Run

Linux host, Docker with Compose v2, Go 1.27.4, `ulimit -n 65536`.
`make load` runs L1–L5, each on a fresh stand (`test/cluster/docker-compose.yaml`), removed after the test.
One test: `make load LOAD_TESTS=L2`. Smaller run: `LOAD_ARGS='-load.sessions=100 -load.duration=1m'`.
Flags (defaults are the design-doc profile): `-load.sessions=5000`, `-load.rate=100` (new SSH sessions per second), `-load.duration=10m` (L1/L3), `-load.idle=5m` (L4 hold), `-load.tcp=500`, `-load.udp=100` (L1 mix), `-load.instances=200`, `-load.resources=10000` (L2), `-load.agent-runs=20`, `-load.agent-rate=1000` (ssh_exec calls per minute in total), `-load.agent-duration=10m` (at most 1h), `-load.relays=500`, `-load.relay-hold=2m` (L5).
The SSH target and the echo targets run in the test process and listen on the Docker bridge address, so allow traffic from the Docker networks to the host. The harness dials nginx on its bridge IP (`10.231.0.10`), not the docker-proxy port.
Every criterion prints a `RESULT | <load> | <metric> | <criterion> | <result> | PASS/FAIL` line.

## How it is measured

- RSS: `VmRSS` from `/proc/<pid>/status`, pid from `docker inspect -f '{{.State.Pid}}'`, every 5 s.
  `docker stats` is not used: cgroup memory includes page cache (gateway spool files).
- Budgets use decimal units: gateway 200 MB + 256 kB × active sessions; server 300 MB + 64 kB × (tunnel streams + relay streams).
  Each sample uses the counters from the same moment (`werbot_gateway_sessions_active`, `werbot_tunnel_streams`, `werbot_relay_streams`).
  The result row shows the sample with the highest RSS and its budget; PASS needs no sample over its own budget.
- Connect latency: from the start of the TCP dial to the `ready` line printed by the target shell. SSH sessions only (the L1 mix is not counted). p99 is nearest-rank.
- L2 latency: from just before the binding change request to the first snapshot with a version other than the previous one on each instance. Every instance must receive it within 30 s.
- Postgres connections: `pg_stat_activity` grouped by replica address, ≤ `pool_max_conns` (20) + 2 = 22. Postgres runs with `log_min_duration_statement=250`; the "slow statements" rows are references.

## L5: agents and relays

`make load LOAD_TESTS=L5`. One stand, two subtests.

- `AgentExec`: `-load.agent-runs` `werbot mcp` processes (agents `load-<i>`, limits `max_actions 1000`, `max_duration 1h`, `max_concurrent 32`) send `-load.agent-rate` `ssh_exec` calls (`echo load`, a read) a minute in total for `-load.agent-duration` through the pool front. One run takes at most 1 000 tool calls (`MaxToolCallsPerRun`, `list_resources` included): the test stops before the start when rate ÷ runs × minutes + 1 is over 1 000. Criteria: 0 failed calls, every call answered, one `session` row per call (`agent_run_id`), RSS of gateways and replicas within budget in every sample.
- `Relays`: `gw2` is stopped (the stand is restored after the test), `-load.relays` web terminals of server2 reach the host load target through gw1, so each is a browser stream on server2 and a bridge on server1; each writes one line every 5 s for `-load.relay-hold`. Criteria: `werbot_relay_streams` equals the terminal count on server2 and on server1, no terminal breaks, `werbot_relay_rejected_total{reason="limit"}` is 0, RSS within budget (tunnel + relay streams). `relay.MaxStreams` is 1 000 a replica, so 500 fit.
- Not run on a stand yet: it needs a Linux host (the default profile is not run on the development machine). Smaller run: `LOAD_ARGS='-load.agent-rate=200 -load.agent-duration=1m -load.relays=50'`.

### Exec decision at 10 000 bindings (known limit)

The classifier and policy decision of an `ssh_exec` (analysis, path rules, `Decide`) has no stand test. `TestExecDecisionP99` (`internal/gateway`, run alone) holds p99 ≤ 5 ms at the budget snapshot: 1 000 resources and bindings, 100 command rules, 100 path rules of 4 globs, 10 custom policies. `BenchmarkExecDecisionAtL2Scale` measures the L2 scale (10 000 resources and bindings, 200 command rules, half with an approval, 100 path rules of 4 globs, 5 custom policies, 50 grants, 10 locks, 100 agents). It reports p50 and p99 and never fails on latency. `go test ./...` and CI do not run it. Run it by hand (about 35 s):

```sh
go test -run '^$' -bench '^BenchmarkExecDecisionAtL2Scale$' -benchtime 1x ./internal/gateway/
```

The cost grows with the number of bindings, so at 10 000 bindings the 5 ms p99 budget does not hold. Development machine (macOS, 12 cores, other load running), 2 000 runs after 100 warm-up runs:

| Request | p50 at 1 000 bindings | p99 at 1 000 bindings | p50 at 10 000 bindings | p99 at 10 000 bindings |
|---|---|---|---|---|
| person, `benchCommand` | 0.43 ms | 0.8–1.4 ms | 3.0–3.7 ms | 8.5–8.9 ms |
| person, 100-path command | 0.69 ms | 1.1–1.2 ms | 3.5–4.3 ms | 8.7–9.5 ms |
| agent (two passes), `benchCommand` | 0.58 ms | 1.0–1.1 ms | 3.7–4.3 ms | 8.8–10.4 ms |
| agent (two passes), 100-path command | 0.91 ms | 1.4 ms | 4.2 ms | 10.1–11.4 ms |

Owner decision O1: the gate stays at 1 000 bindings. Splitting the policies per resource is phase 3. [docs/scaling.md](../../docs/scaling.md#resources) lists it as a known limit.

## Stand

| Field | Value |
|---|---|
| Date | 2026-11-18 (run started 18:10 UTC) |
| Werbot commit | 69e69c8 |
| Host | 2 vCPU Intel Xeon (Skylake, IBRS, no TSX), 3.7 GiB RAM, 38 GB disk (4.6 GB free) |
| OS, kernel | Ubuntu 22.04.5 LTS, 5.15.0-160-generic |
| Docker, Compose, Go | 29.8.0, 5.5.1, 1.27.4 |
| Shared host | other services run on it; the harness runs on the same 2 vCPU as the stand |
| Command | `make load LOAD_ARGS='-load.sessions=300 -load.rate=75 -load.duration=5m -load.tcp=150 -load.udp=30 -load.instances=200 -load.resources=2000 -load.idle=5m'` |

## Results

Host-scaled profile (see Notes). All rows PASS, exit code 0.

| Load | Metric | Criterion | Result | Pass |
|---|---|---|---|---|
| L1 | mix ramp (TCP and UDP echoed once) | reference | 2s | PASS |
| L1 | errors (300 SSH sessions) | 0 | 0 | PASS |
| L1 | TCP via werbot connect: errors | 0 | 0 of 150, 0 silent, 47.0 MB echoed | PASS |
| L1 | UDP via werbot connect: errors | 0 | 0 of 30, 0 silent, 0.6 MB echoed | PASS |
| L1 | connect p99 | ≤ 1 s | 329ms | PASS |
| L1 | spool drained, sessions ended, recordings finished | ≤ 5m, 300 each | 5s, 300 ended, 300 finished | PASS |
| L1 | recordings verified | all 300 | 300 | PASS |
| L1 | RSS gw1 | ≤ budget | max 138.6 MB, budget 280.6 MB at 315 active; 0 of 63 samples over | PASS |
| L1 | RSS gw2 | ≤ budget | max 109.5 MB, budget 203.8 MB at 15 active; 0 of 63 samples over | PASS |
| L1 | RSS server1 | ≤ budget | max 167.2 MB, budget 300.1 MB at 1 active; 0 of 63 samples over | PASS |
| L1 | RSS server2 | ≤ budget | max 50.5 MB, budget 300.1 MB at 1 active; 0 of 63 samples over | PASS |
| L1 | Postgres connections server1 / server2 | ≤ 22 | max 14 / 10 | PASS |
| L1 | slow statements (≥ 250 ms) | reference | 0 | PASS |
| L2 | first snapshot (200 instances) | all 200 ≤ 2m | 200 of 200 | PASS |
| L2 | first snapshot p99 | reference | 1.08s | PASS |
| L2 | werbot_tunnel_streams server1 + server2 | 202 | 102 + 100 | PASS |
| L2 | new version received | all 200 ≤ 30s | 200 of 200 | PASS |
| L2 | new version p99 (200 instances, 2 000 resources) | ≤ 5 s | 2.731s | PASS |
| L2 | version max | reference | 2.736s | PASS |
| L2 | snapshot size | reference | 1.0 MB | PASS |
| L2 | RSS server1 | reference | max 194.5 MB, budget 306.5 MB at 102 active; 0 of 3 samples over | PASS |
| L2 | RSS server2 | reference | max 219.5 MB, budget 306.4 MB at 100 active; 0 of 3 samples over | PASS |
| L2 | slow statements (≥ 250 ms) | reference | 37 | PASS |
| L3 | broken sessions | 0 | 0 | PASS |
| L3 | kill server1: werbot_tunnel_streams(server2) = 2 | reference | 700ms | PASS |
| L3 | recordings verified | reference | 300 of 300 | PASS |
| L4 | gw1 / gw2 peak sessions_active | 150 each | 150 / 150 | PASS |
| L4 | RSS gw1 (150 idle sessions) | ≤ budget | max 68.5 MB, budget 238.4 MB at 150 active; 0 of 61 samples over | PASS |
| L4 | RSS gw2 (150 idle sessions) | ≤ budget | max 67.7 MB, budget 238.4 MB at 150 active; 0 of 61 samples over | PASS |
| L4 | errors | reference | 0 | PASS |
| L4 | gateway listeners gw1 / gw2 | 2222, 9090 only | 2222 9090 / 2222 9090 | PASS |

L4 ran 150 idle sessions per gateway (300 in total), not 2 500 each; the budget grows with the session count (§10.1: 840 MB at 2 500).
L1/L3 ran 300 sessions at 75/s for 5 minutes, with a mix of 150 TCP connections and 30 UDP bindings. L2 ran 200 instances with 2 000 resources, not 10 000.

## Notes

- Host-scaled profile (owner decision O4): the default profile (5 000 sessions at 100/s, 500 TCP + 100 UDP, 200 instances, 10 000 resources) does not fit this host. The run uses the command in the Stand table. The criteria are unchanged: budgets and thresholds are not weakened.
- Default profile hardware: roughly 16 cores (steady-state estimate of about 3 ms CPU per session-second). Run it on reference-class hardware, or put the harness on a second machine.
- Harness co-location: the harness runs on the same 2 vCPU as the stand, about 25–30 % of burst CPU, so these numbers are conservative for the product.
- Connect knee on this host (L1 connect p99 ≤ 1 s, no mix): before the stage 7a optimizations (a024993) 30–35 connects/s (50/s gave p99 2.584 s, FAIL). After Tasks 14–17: 50/s gave 79–116 ms; 75/s with 200 sessions gave 141 ms (PASS); 100/s with 300 sessions gave 2.666 s (FAIL; 1.815 s FAIL with the 150/30 mix). The knee is now 75–100/s.
- CPU per connect: about 50–60 ms before, about 27 ms after (gateway 9, harness 8, server 4, Postgres 4, nginx 2).
- L2 before/after (200 instances, 2 000 resources, 1.0 MB snapshot): version p99 5.065 s (FAIL) before Task 18, 2.6–2.96 s (PASS) after. First snapshot p99 21.1 s before, about 1.1–1.3 s after. Changes: JIT off, org parts built once per refresh, concurrent pool builds, gzip once per snapshot, Connect shares an in-flight build.
- Mix before ramp: the L1 mix reaches steady state (every TCP connection and UDP binding echoed once) before the measured SSH ramp, and runs the whole L1. A timeout is a FAIL row.
- Bridge dial: on Linux the harness dials nginx on its bridge address, which skips docker-proxy.
- Load tests are Linux only; `make load` stops elsewhere.
