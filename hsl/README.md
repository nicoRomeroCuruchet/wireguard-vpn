# hsl — headscale-lite

A minimalist WireGuard control plane. One binary, two roles: a hub
(`hsl server`) that assigns overlay IPs and programs the kernel, and nodes
(`hsl client`) that join with a single command. WireGuard (in-kernel) is the
data plane; `hsl` only configures it via netlink — it never edits
`/etc/wireguard/*.conf`.

## Requirements

- Linux ≥ 5.6 with the `wireguard` kernel module (`sudo modprobe wireguard`).
- `CAP_NET_ADMIN` (run with `sudo` for now).
- Go 1.25+ to build (the `modernc.org/sqlite` driver forces the module's `go` directive to 1.25; development uses Go 1.26.4).

## Build

```bash
cd hsl
go build -o hsl ./cmd/hsl

# Cross-compile for the BeagleBone Black (ARMv7):
GOOS=linux GOARCH=arm GOARM=7 go build -o hsl-arm ./cmd/hsl
```

## Run the hub (BeagleBone Black)

```bash
sudo ./hsl-arm server \
  --addr :8080 \
  --endpoint <PUBLIC_IP>:51820 \
  --db /var/lib/hsl/hsl.db \
  --key /var/lib/hsl/identity.key
```

Forward UDP/51820 on the router to the BBB. `--endpoint` is the public
`host:port` clients dial; with a fixed public IP, the literal IP is fine.

## Join a node

```bash
./hsl client register --server http://<PUBLIC_IP>:8080 --hostname my-laptop
sudo ./hsl client run --server http://<PUBLIC_IP>:8080
```

State (`identity.key`, `node.json`) lives in `~/.local/state/hsl/`.

## Demo / acceptance

1. Start the hub; `sudo wg show wg0` shows listen port 51820 and `10.100.0.1/24`.
2. On PC #1: `register` then `run`. `ping 10.100.0.1` works.
3. On PC #2: `register` then `run` (gets `10.100.0.3`).
4. From PC #1: `ping 10.100.0.3` works (traffic transits the hub).
5. Restart the hub: clients keep working (state restored from SQLite).

## Scope

Hub-and-spoke only. No NAT hole-punching, ACLs, MagicDNS, exit nodes, TLS,
or auth beyond the `X-Node-ID` token — see the design spec in
`docs/superpowers/specs/`. Those are deliberately out of MVP scope.
