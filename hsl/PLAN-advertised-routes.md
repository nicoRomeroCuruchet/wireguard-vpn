# Plan: Home LAN Reachability (Advertised Routes)

## Goal

Allow remote VPN clients to directly reach devices on the hub's home LAN
(e.g. `192.168.1.0/24`), not just the overlay (`10.100.0.0/24`).

## How It Works

The server gets a new `--advertise-routes` flag (repeatable). When set, it:
1. Includes the routes in `/register` and `/peers` responses
2. Clients add those CIDRs to the hub peer's `AllowedIPs`
3. Server auto-configures `iptables MASQUERADE` for LAN reachability
4. Server cleans up iptables rules on graceful shutdown

## Changes by File

### 1. `internal/proto/proto.go` — Add `advertised_routes` to wire types

- `RegisterResponse` gains `AdvertisedRoutes []string \`json:"advertised_routes"\``
- `Peer` gains `AdvertisedRoutes []string \`json:"advertised_routes"\``

### 2. `internal/server/server.go` — Plumb routes through server

- `Config` gains `AdvertisedRoutes []string`
- `registerResponse()` sets `AdvertisedRoutes` from `s.cfg.AdvertisedRoutes`
- `handlePeers()` sets `AdvertisedRoutes` on the hub `Peer` entry
- New method `setupSNAT()` that adds `iptables -t nat -A POSTROUTING -s <overlay> -d <route> -j MASQUERADE` for each advertised route
- New method `teardownSNAT()` that deletes those same rules
- `Run()`: call `setupSNAT()` after `reconcileWG()` (guarded by `skipWG`); call `teardownSNAT()` on shutdown before `srv.Shutdown()`

### 3. `cmd/hsl/main.go` (server) — New `--advertise-routes` flag

- Add repeatable `--advertise-routes` flag
- Pass `AdvertisedRoutes` into `server.Config`

### 4. `internal/client/client.go` — Expand AllowedIPs

- `State` gains `AdvertisedRoutes []string \`json:"advertised_routes"\``
- `Register()` copies `rr.AdvertisedRoutes` into `State`
- `realConfigureWG()` merges `st.OverlayNet` + `st.AdvertisedRoutes` into the hub peer's `AllowedIPs`
- `loadState()` handles missing field gracefully (old state files without it)

### 5. SNAT helper package — `internal/server/snat.go`

- New file for iptables logic (keeps server.go clean)
- `func SetupSNAT(overlayCIDR string, routes []string) error`
- `func TeardownSNAT(overlayCIDR string, routes []string) error`
- Uses `exec.Command("iptables", ...)` — the simplest, most portable approach on the BBB (Debian)
- Both functions are idempotent (safe to call multiple times)

### 6. Tests

- `proto_test.go`: verify `advertised_routes` JSON tag
- `register_test.go`: verify response includes advertised routes
- `peers_test.go`: verify hub peer includes advertised routes
- `store_sqlite_test.go`: unchanged (routes are server config, not per-node)
- `client_test.go`: verify AllowedIPs merges overlay + advertised routes

### 7. README update

- Add `--advertise-routes` to server usage
- Add LAN reachability example to demo section
- Note that `iptables` requires root (already required for `CAP_NET_ADMIN`)
- Note that `ip_forward=1` must be enabled on the hub

## Implementation Order (conventional commits)

1. `feat(proto): add advertised_routes to register and peers responses`
2. `feat(server): advertise routes in /register and /peers responses`
3. `feat(client): merge advertised routes into hub peer AllowedIPs`
4. `feat(server): auto-manage SNAT iptables rules for LAN reachability`
5. `feat(server): add --advertise-routes flag`
6. `test: verify advertised routes in responses and client AllowedIPs`
7. `docs: README with LAN reachability instructions`

## Edge Cases Handled

- **Empty `--advertise-routes`** (default): behavior identical to current MVP
- **Old `node.json` without `advertised_routes`**: Go's `json.Unmarshal` leaves the field as `nil`; client just uses overlay only
- **SNAT with no routes**: `setupSNAT` is a no-op
- **Duplicate routes**: iptables `-A` is idempotent-enough (duplicate rules don't break); teardown uses `-D` which removes one match
- **Server crash without graceful shutdown**: SNAT rules persist but are harmless (NAT rules that don't match are no-ops); next start re-adds them anyway
