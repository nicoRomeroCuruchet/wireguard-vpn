# hsl — headscale-lite

A minimalist WireGuard control plane. One binary, two roles:

- **Hub** (`hsl server`) — runs on a permanently online Linux host (e.g. a
  BeagleBone Black at home). It assigns overlay IPs, tracks peers, and
  configures the kernel WireGuard interface.
- **Client** (`hsl client`) — runs on every node that wants to join the
  overlay. It registers once, then polls the hub and keeps `wg0` in sync.

WireGuard (in-kernel) is the data plane. `hsl` only configures it via netlink
— it never edits `/etc/wireguard/*.conf`.

---

## Table of contents

1. [What you need](#what-you-need)
2. [Install dependencies](#install-dependencies)
3. [Build `hsl`](#build-hsl)
4. [Deploy the hub](#deploy-the-hub)
5. [Deploy a client](#deploy-a-client)
6. [Verification](#verification)
7. [Troubleshooting](#troubleshooting)

---

## What you need

### Software (all hosts)

- Linux ≥ 5.6 with the in-kernel `wireguard` module.
- `iptables` (for SNAT / LAN reachability).
- `curl` / `wget` and standard POSIX tools.
- Go **1.25+** if you build from source.

### Hardware / topology

- One always-on Linux host as the **hub**. The reference deployment uses a
  BeagleBone Black (ARMv7) running Debian, but any Linux host works.
- One or more Linux **clients** (laptops, desktops, servers, phones via a
  Linux userspace, etc.).
- The hub must have a publicly reachable IP or port-forwarded UDP/51820.

### Privileges

`hsl` manipulates network interfaces, routes and iptables. Run it as `root`
or with `CAP_NET_ADMIN`.

---

## Install dependencies

### Debian / Ubuntu / Raspberry Pi OS

```bash
sudo apt update
sudo apt install -y wireguard iptables curl
sudo modprobe wireguard
```

Make WireGuard load on boot:

```bash
echo 'wireguard' | sudo tee /etc/modules-load.d/wireguard.conf
```

### Fedora / RHEL / CentOS Stream

```bash
sudo dnf install -y wireguard-tools iptables curl
sudo modprobe wireguard
```

### Arch Linux

```bash
sudo pacman -S wireguard-tools iptables curl
sudo modprobe wireguard
```

### Verify

```bash
sudo modprobe wireguard
lsmod | grep wireguard
```

You should see the `wireguard` module loaded.

---

## Build `hsl`

### Build natively on the build machine

```bash
cd hsl
go build -o hsl ./cmd/hsl
```

### Cross-compile for the BeagleBone Black (ARMv7)

Build on an x86_64 workstation and copy the binary to the BBB:

```bash
cd hsl
GOOS=linux GOARCH=arm GOARM=7 go build -o hsl-arm ./cmd/hsl
scp hsl-arm beaglebone.local:/tmp/hsl
```

### Requirements note

The project uses `modernc.org/sqlite`, which forces the module's `go`
directive to **1.25**. Development is validated with Go **1.26.4** installed at
`~/.local/go`.

---

## Deploy the hub

### 1. Prepare directories and binary

On the hub (as root or with sudo):

```bash
sudo mkdir -p /var/lib/hsl /usr/local/bin
sudo cp /tmp/hsl /usr/local/bin/hsl
sudo chmod 755 /usr/local/bin/hsl
```

### 2. Enable IP forwarding

Required for any routing beyond the hub itself (overlay-to-overlay and
overlay-to-LAN):

```bash
sudo sysctl -w net.ipv4.ip_forward=1
echo 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-hsl-forward.conf
```

### 3. Run manually

```bash
sudo hsl server \
  --addr :8080 \
  --endpoint <PUBLIC_IP>:51820 \
  --db /var/lib/hsl/hsl.db \
  --key /var/lib/hsl/identity.key \
  --advertise-routes 192.168.1.0/24
```

Flags:

| Flag | Meaning |
|------|---------|
| `--addr` | HTTP listen address for the control plane. |
| `--endpoint` | Public `host:port` clients dial over WireGuard. |
| `--db` | SQLite database path (node state). |
| `--key` | Hub WireGuard private key path. |
| `--advertise-routes` | Repeatable. LAN CIDRs remote clients may reach through the hub. |

Forward **UDP/51820** on your home router to the hub's internal IP.

### 4. Run as a systemd service

Create `/etc/systemd/system/hsl-server.service`:

```ini
[Unit]
Description=hsl server (headscale-lite hub)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/hsl server \
  --addr :8080 \
  --endpoint PUBLIC_IP:51820 \
  --db /var/lib/hsl/hsl.db \
  --key /var/lib/hsl/identity.key \
  --advertise-routes 192.168.1.0/24
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now hsl-server
sudo systemctl status hsl-server
sudo journalctl -u hsl-server -f
```

### 5. Advertised routes / LAN reachability

If you want remote clients to reach devices on the hub's home LAN (e.g.
`192.168.1.0/24`), add:

```bash
--advertise-routes 192.168.1.0/24
```

`hsl` will automatically insert the matching iptables rule:

```text
iptables -t nat -A POSTROUTING -s 10.100.0.0/24 -d 192.168.1.0/24 -j MASQUERADE
```

The rule is removed again when the service shuts down cleanly.

---

## Deploy a client

### 1. Install binary

```bash
sudo mkdir -p /usr/local/bin
sudo cp /tmp/hsl /usr/local/bin/hsl
sudo chmod 755 /usr/local/bin/hsl
```

### 2. Register once

```bash
hsl client register \
  --server http://<PUBLIC_IP>:8080 \
  --hostname my-laptop
```

This creates:

- `~/.local/state/hsl/identity.key` — the node's WireGuard private key.
- `~/.local/state/hsl/node.json` — node ID, overlay IP, server key, etc.

### 3. Run the tunnel

```bash
sudo hsl client run --server http://<PUBLIC_IP>:8080
```

The client will create `wg0`, configure the hub as its only peer, and poll
`/peers` every 10 seconds.

### 4. Run as a systemd service (user registration + root tunnel)

The registration step runs as the user (so state lands in the user's home
directory). The tunnel step must run as root. A common pattern is a user
service for registration and a system service for the tunnel, or simply run
registration manually once and then use a root service that points at the
user's state directory.

Create `/etc/systemd/system/hsl-client.service`:

```ini
[Unit]
Description=hsl client tunnel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Environment="STATE_DIR=/home/YOUR_USER/.local/state/hsl"
ExecStart=/usr/local/bin/hsl client run \
  --server http://PUBLIC_IP:8080 \
  --state-dir ${STATE_DIR}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Adjust `YOUR_USER` to the user who ran `hsl client register`.

> **NetworkManager note:** On desktop Linux distributions, NetworkManager may
> detect `wg0` and try to manage it, which can remove the address and routes
> that `hsl` sets. If the tunnel works after starting but breaks after a
> restart, tell NetworkManager to ignore `wg0`:
>
> ```bash
> sudo mkdir -p /etc/NetworkManager/conf.d
> cat <<'EOF' | sudo tee /etc/NetworkManager/conf.d/99-unmanaged-wg0.conf
> [keyfile]
> unmanaged-devices=interface-name:wg0
> EOF
> sudo systemctl reload NetworkManager
> sudo nmcli connection delete wg0 2>/dev/null || true
> sudo systemctl restart hsl-client
> ```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now hsl-client
sudo systemctl status hsl-client
sudo journalctl -u hsl-client -f
```

---

## Verification

On the hub:

```bash
sudo wg show wg0
sudo iptables -t nat -L POSTROUTING -n -v
```

On a client:

```bash
ip addr show wg0
sudo wg show wg0
ping 10.100.0.1                  # hub overlay IP
ping 10.100.0.3                  # another client
ping 192.168.1.42                # LAN device (if --advertise-routes set)
```

Expected demo:

1. Start the hub; `sudo wg show wg0` shows listen port 51820 and `10.100.0.1/24`.
2. On PC #1: `register` then `run`. `ping 10.100.0.1` works.
3. On PC #2: `register` then `run` (gets `10.100.0.3`).
4. From PC #1: `ping 10.100.0.3` works (traffic transits the hub).
5. With `--advertise-routes 192.168.1.0/24` on the hub: from PC #1,
   `ping 192.168.1.42` reaches a device on the hub's home LAN.
6. Restart the hub: clients keep working (state restored from SQLite).

---

## Troubleshooting

### `hsl server` fails to create `wg0`

- Ensure the `wireguard` module is loaded: `sudo modprobe wireguard`.
- Run as root or with `CAP_NET_ADMIN`.

### Clients cannot register

- Check that UDP/51820 and TCP/8080 (or your chosen `--addr`) are reachable
  from the client.
- Verify `--endpoint` matches the public IP clients see.

### Overlay ping works but LAN ping does not

- Confirm `net.ipv4.ip_forward=1` on the hub.
- Confirm `--advertise-routes` includes the correct LAN CIDR.
- Check the iptables rule: `sudo iptables -t nat -L POSTROUTING -n -v`.
- Ensure the LAN device does not have a firewall blocking the hub.

### Service fails after reboot

- Verify the service is enabled: `sudo systemctl is-enabled hsl-server`.
- Ensure `/var/lib/hsl` exists and is writable.

### Tunnel works once but breaks after `systemctl restart hsl-client`

NetworkManager may have taken over `wg0`. Check with:

```bash
nmcli device show wg0
```

If it shows a NetworkManager connection, apply the unmanaged config from the
client deployment section, delete the `wg0` connection, and restart the
service:

```bash
sudo nmcli connection delete wg0
sudo systemctl restart hsl-client
```

---

## Scope

Hub-and-spoke only. No NAT hole-punching, ACLs, MagicDNS, TLS, or auth
beyond the `X-Node-ID` token. Those are deliberately out of MVP scope.
