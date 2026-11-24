# werbot connect --vnet

`werbot connect --vnet` gives every resource you can open a stable address and a DNS name on your computer. Programs connect to `<resource>.<org>.werbot` on the resource's own ports, with no local port per resource and no `werbot connect <resource>` per session. TCP and UDP both work (Teleport VNet has no UDP).

```sh
sudo werbot connect --vnet
pg.acme.werbot → 100.71.12.40 (tcp)
dns.acme.werbot → 100.98.3.7 (udp)

psql -h pg.acme.werbot
dig @dns.acme.werbot example.com
```

It runs until Ctrl+C (exit code 0) and then removes what it set up.

## What it sets up

- A virtual interface: `werbot0` on Linux, the next free `utunN` on macOS, MTU 1500. Its address is the network + 1 of the prefix (`100.64.0.1` for the default `100.64.0.0/10`), and the whole prefix is routed to it.
- A DNS responder at the network + 2 (`100.64.0.2`), on UDP and TCP port 53.
- An address per resource from the network + 3 up, taken from a hash of the resource id: the same set of resources always gets the same addresses.
- `<org>` is the organization name in lower case with every run of other characters than `a-z` and `0-9` turned into `-` (`Acme Corp` → `acme-corp`), at most 63 characters; a name with none of them uses the first 8 hex digits of the organization id.
- The resource list is fetched again every 60 s. A resource that is gone loses its name (`NXDOMAIN`), new connections to its address are reset and open ones are closed. The list is printed again when it changes.

## Which resources

Only resources you may open now (`ACCESS` `allowed` in `werbot ls`) with the `stream` kind (TCP) or the `datagram` kind (UDP) get an address: `tcp`, `udp`, `telnet` and `ssh` resources. An `ssh` resource without `port_forwarding` gets an address, but the gateway refuses its connections. Resources you can only request get no address until access is granted (at the next refresh).

Every TCP connection opens a `direct-tcpip` channel through the gateway: one gateway session and one access check each, as with [`werbot connect`](connect.md). UDP to a resource port uses one `datagram@werbot.com` channel with a flow per local socket, at most 32; a flow with no traffic for 2 minutes is freed. Each resource gets its own SSH connection to its pool as `<login>+<resource>`, dialed on the first connection over SSH or the pool's WebSocket URL ([connect.md](connect.md#websocket)), with the pool host key from the resource list. An MFA rule asks in the terminal, one prompt at a time; under `sudo` the browser is not opened and the MFA URL is printed.

## DNS

The responder answers only names under `<org>.werbot`, over UDP and TCP (several queries per TCP connection). Names under the suffix without a resource get `NXDOMAIN`; other query types than `A` for a resource (`AAAA` included) get an empty `NOERROR`; names outside the suffix get `REFUSED`, so the OS goes on to its other resolvers. Records live 60 s.

- Linux: `resolvectl dns werbot0 100.64.0.2` and `resolvectl domain werbot0 ~<org>.werbot` (systemd-resolved). Without `resolvectl`, or when it fails, the command prints the reason and `Add "nameserver 100.64.0.2" before other nameservers in /etc/resolv.conf to resolve *.<org>.werbot`; addresses work without names.
- macOS: the file `/etc/resolver/<org>.werbot` with `nameserver 100.64.0.2`; check with `scutil --dns`.

## Requirements and privileges

- Linux: root or `CAP_NET_ADMIN`. Prefer the capability: `sudo setcap cap_net_admin+ep "$(command -v werbot)"`, then run `werbot connect --vnet` as yourself. `ip` and `resolvectl` get the capability as an ambient capability. Without either: `--vnet needs CAP_NET_ADMIN: run with sudo or "sudo setcap cap_net_admin+ep <path>"`.
- macOS: root (`--vnet needs root: run "sudo werbot connect --vnet"`); it uses `ifconfig`, `route` and `/etc/resolver`.
- Under `sudo` the profile comes from the home of `SUDO_USER` (`~/.werbot`) unless `WERBOT_HOME` is set. Root is used only to open the interface, to set the address, route and DNS, and to remove them on exit. The API calls, the certificate renewal and the gateway connections run with the effective user and group of `SUDO_UID`/`SUDO_GID` and the user's groups, so the profile files stay yours. The saved user id stays root, so the process can take root back: on Linux `setcap` avoids root altogether.
- Windows: not supported (`--vnet is not supported on windows`); it is planned for Werbot Desktop.

## Flags

- `--vnet`: takes no resources and no other flags (`--vnet does not take resources`).
- `--vnet-prefix <cidr>`: the IPv4 range of the addresses, at least a `/29` (default `100.64.0.0/10`). Change it when the range is taken: Tailscale uses `100.64.0.0/10` too. Example: `sudo werbot connect --vnet --vnet-prefix 10.233.0.0/16`.

## Security

- Every process and every local user of the computer can reach the resources through the addresses while `--vnet` runs, with your access. Do not run it on shared machines.
- Only packets from the interface address enter: traffic that other hosts or containers route to the prefix is dropped.

## Cleanup

Ctrl+C (or `SIGTERM`) removes the interface with its address, route and Linux DNS settings, and on macOS the file `/etc/resolver/<org>.werbot`. After `kill -9` or a crash on macOS remove it yourself: `sudo rm /etc/resolver/<org>.werbot`. On Linux everything goes with the interface.

## Manual check on macOS

Not covered by CI; check it before a release:

1. `sudo werbot connect --vnet` prints `<resource>.<org>.werbot → 100.…` lines.
2. `scutil --dns | grep -A3 werbot` shows the resolver for `<org>.werbot` with `100.64.0.2`.
3. `dig <resource>.<org>.werbot` answers the printed address; `curl http://<resource>.<org>.werbot` (an HTTP resource) answers.
4. Ctrl+C: `/etc/resolver/<org>.werbot` is gone and `ifconfig` shows no `100.64.0.1`.
5. `ls -l ~/.werbot/*/` shows your user as the owner of every file.
