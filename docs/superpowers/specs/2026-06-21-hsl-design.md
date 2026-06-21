# hsl (headscale-lite) — Diseño / Spec

**Fecha:** 2026-06-21
**Estado:** Aprobado, listo para plan de implementación
**Autor:** Nico + Claude Code

---

## 1. Resumen

`hsl` es un control plane minimalista para WireGuard, inspirado en
Tailscale/Headscale. Automatiza la gestión de una red WireGuard
hub-and-spoke para que los nodos se sumen con un solo comando, sin editar
archivos de configuración de WireGuard a mano. Proyecto de aprendizaje +
utilidad real.

WireGuard sigue siendo el **data plane** (cifrado, transporte UDP, handshake
Noise_IK — todo lo hace el módulo del kernel ya presente en Linux mainline).
**No se reimplementa nada de WireGuard.** `hsl` es solo el **plano de
control**: decide quién es peer de quién y configura los kernels vía netlink.

## 2. Topología y deploy

Hub-and-spoke estricto:

- **Hub = BeagleBone Black** en la red doméstica del autor.
  - IPv4 **pública fija** + **port-forward UDP/51820** en el router (sin CGNAT).
  - `ListenPort = 51820`, `net.ipv4.ip_forward = 1`, rutea entre clientes.
  - Hardware: TI AM3358, ARM Cortex-A8 (ARMv7-A), 512 MB RAM.
- **Clientes** = nodos detrás de NAT (no abren puertos). Cada cliente conoce
  solo al hub como peer directo; el tráfico cliente↔cliente pasa siempre por
  el hub.
- Caso de uso principal: **acceso remoto a la casa desde internet**.

Agregar un nodo nuevo = un solo comando en ese nodo, sin tocar archivos en
ningún otro nodo.

### Consideraciones de hardware (BBB como hub)

- **Cross-compile** desde la PC: `GOARCH=arm GOARM=7`. Todo el stack soporta
  `linux/arm` (`modernc.org/sqlite` pure-Go, `wgctrl`, `vishvananda/netlink`).
  Se compila en la PC y se copia el binario por `scp`.
- **Kernel WireGuard**: requiere Linux ≥5.6 con módulo `wireguard`. Verificar
  en la placa antes de deploy (ver Fase 0).
- **Throughput**: el hub cifra/descifra todo el tráfico cliente↔cliente (doble
  paso). En Cortex-A8 el techo de ChaCha20 ronda decenas de Mbps. Aceptable
  para uso doméstico; es un límite conocido, no un bug.

## 3. Stack

- **Go 1.25+** para compilar. El código solo usa features de Go 1.22 (routing
  method+path), pero `modernc.org/sqlite` fuerza la directiva `go` del módulo a
  1.25. Desarrollo con Go 1.26.4; cross-compile a la BBB con el mismo toolchain.
- **Stdlib first**: `net/http` (router enriquecido de Go 1.22), `log/slog`,
  `encoding/json`, `context`, `os/signal`. Sin Gin/Echo/Chi/Cobra.
- `golang.zx2c4.com/wireguard/wgctrl` — configura WireGuard vía netlink
  (llaves, peers, `ListenPort`, `AllowedIPs`, `PersistentKeepalive`).
- `github.com/vishvananda/netlink` — crea/levanta la interfaz `wg0`, asigna
  `Address` y `MTU`. **Necesario porque `wgctrl` NO crea la interfaz ni
  configura IP/MTU.**
- `modernc.org/sqlite` — SQLite pure-Go (sin CGO) para persistencia del server.
- Sin Docker, sin Kubernetes, sin frameworks de DI.

## 4. Estructura del repo

Monorepo, un solo binario con subcomandos (patrón consul/nomad):

```
hsl/
├── go.mod
├── README.md
├── .gitignore
├── cmd/hsl/main.go              # entry point, rutea subcomandos
└── internal/
    ├── proto/                   # structs JSON compartidas
    ├── server/                  # control plane
    ├── client/                  # agente de nodo
    └── wgmgr/                   # wrapper sobre netlink + wgctrl-go (compartido)
```

### Subcomandos

- `hsl server [--addr :8080] [--db /var/lib/hsl/hsl.db] --endpoint <host:51820>`
- `hsl client register --server URL [--hostname NAME] [--state-dir PATH]`
- `hsl client run --state-dir PATH`
- `hsl version`

## 5. Protocolo HTTP

JSON sobre HTTP. `snake_case` en JSON, `PascalCase` en Go (struct tags). Sin
TLS al principio (asunción: red de confianza durante desarrollo; agregable vía
reverse proxy más adelante).

| Método | Path        | Request                          | Response |
|--------|-------------|----------------------------------|----------|
| POST   | `/register` | `{public_key, hostname}`         | `{node_id, overlay_ip, server_key, server_endpoint, overlay_net}` |
| GET    | `/peers`    | header `X-Node-ID`               | `{peers: [{id, public_key, overlay_ip, hostname, last_seen}]}` |
| POST   | `/heartbeat`| header `X-Node-ID`               | `{ok: true}` |
| GET    | `/healthz`  | —                                | `ok` |

**Auth (MVP):** el `node_id` devuelto en `/register` actúa como bearer token
(header `X-Node-ID: <uuid>`). `/register` **no tiene auth**: cualquiera que
alcance el endpoint obtiene IP y entra a la red. No es seguro contra atacantes
activos; alcanza para iterar. Reemplazable por preauth keys + tokens en fase 2.

## 6. Comportamiento del server

Al arrancar:

1. Abre/crea SQLite en `--db`.
2. Si no tiene su propio par de llaves WireGuard, lo genera y persiste.
3. Crea/levanta `wg0` vía `netlink` (`ip link add wg0 type wireguard`, asigna
   `Address = 10.100.0.1/24`, `MTU 1420`, `up`).
4. Configura `wg0` vía `wgctrl`: privada del hub, `ListenPort = 51820`.
5. Carga peers existentes desde DB y los aplica al kernel
   (cada peer con `AllowedIPs = <overlay_ip>/32`).
6. Asume `net.ipv4.ip_forward = 1` (lo documenta; no lo fuerza en MVP).
7. Levanta el HTTP server.

`POST /register`:

- Valida que `public_key` sea 32 bytes en base64.
- Si la pubkey ya existe en DB → devuelve el mismo `node_id` y `overlay_ip`
  (idempotente).
- Si es nueva → asigna la siguiente IP libre del overlay (skip `.0` red y `.1`
  hub), genera UUID v4 como `node_id`, persiste, agrega el peer al kernel con
  `AllowedIPs = <overlay_ip>/32`.
- Responde con `{node_id, overlay_ip, server_key, server_endpoint, overlay_net}`.
  `server_endpoint` = el valor del flag `--endpoint`.

`GET /peers`:

- Devuelve la lista completa de nodos registrados (incluyendo al hub como
  `10.100.0.1`).
- Actualiza `last_seen` del nodo que pregunta.

Shutdown limpio con `signal.NotifyContext` (SIGINT, SIGTERM) y
`srv.Shutdown(ctx)` con timeout de 10s.

## 7. Comportamiento del cliente

`hsl client register`:

1. Lee/crea `--state-dir` (default `~/.local/state/hsl/`).
2. Si no existe `identity.key`, genera par Curve25519 con `wgctrl` y persiste
   la privada (permisos `0600`).
3. `POST /register` con la pubkey.
4. Guarda la respuesta en `node.json`
   (`node_id, overlay_ip, server_key, server_endpoint, overlay_net`).

`hsl client run`:

1. Lee el state local.
2. Crea/levanta `wg0` vía `netlink` (`Address = overlay_ip` con el **prefijo del
   overlay**, ej. `10.100.0.2/24` — NO `/32`, MTU 1420). El prefijo del overlay
   instala la ruta conectada a toda la subred vía `wg0`; un `/32` solo instala la
   ruta del propio host y el cliente no tendría ruta de kernel hacia el resto del
   overlay (AllowedIPs por sí solo es cryptokey routing, no instala ruta).
3. Configura `wg0` vía `wgctrl`:
   - Privada del nodo.
   - Peer = servidor (`server_key`, `server_endpoint`), con
     `AllowedIPs = overlay_net` (toda la subred vía hub).
   - `PersistentKeepalive = 25`.
4. Loop: cada 10s `GET /peers` + `POST /heartbeat`. Si la lista cambió,
   reconcilia `wg0`. (En hub-and-spoke este loop importa más para el server; el
   cliente principalmente reafirma su sesión.)

## 8. Reglas sobre WireGuard

- **No editar `/etc/wireguard/*.conf` nunca.** Todo vía `netlink` + `wgctrl`.
- El estado del programa (`identity.key`, `node.json`) va en `--state-dir`, no
  en `/etc/wireguard/`.
- Asumir módulo `wireguard` del kernel disponible (Linux ≥5.6). Si no → error
  claro al usuario.
- El hub (y `client run`) necesitan `CAP_NET_ADMIN`. Documentar ejecución con
  `sudo` o systemd unit con esa capability.

## 9. Decisiones tomadas

- Overlay subnet: `10.100.0.0/24`. Hub siempre en `.1`. Clientes desde `.2`.
- Asignación de IPs: indexada por pubkey, persistente. Misma pubkey → misma IP.
- Topología: hub-and-spoke estricto. No mesh, no hole punching, no DERP.
- Single binary dual-mode: `hsl server` y `hsl client` mismo binario.
- Sin frameworks web: `http.ServeMux` (Go 1.22+).
- Sin TLS al principio: HTTP plano.
- Logging: `log/slog` con handler de texto a stderr.
- Interfaz de red: `vishvananda/netlink` (no shellear a `ip`), para mantener
  todo in-process y testeable.
- `server_endpoint`: configurado por flag `--endpoint`, no auto-detectado.

## 10. Fuera de scope (MVP)

Cualquiera de estos puede venir en fase 2; **no agregar sin confirmación**:

- Hole punching / NAT traversal directo entre clientes (mesh real).
- ACLs / policy engine.
- DNS automático tipo MagicDNS.
- Subnet routers / exit nodes.
- OAuth / SSO / preauth keys persistentes.
- Métricas Prometheus, tracing, observabilidad avanzada.
- UI web.
- Multi-tenant.
- Clientes Windows/macOS/móvil. Solo Linux por ahora.
- DDNS (innecesario: IP pública fija; opcional para nombre cómodo).

## 11. Plan de fases (Conventional Commits)

Cada commit debe compilar y pasar `go vet ./...`.

0. **Fase 0 — verificación en hardware** (no es código):
   - `uname -r` (kernel ≥5.6) y `sudo modprobe wireguard` en la BBB.
   - `curl -4 ifconfig.me` vs IP WAN del router → confirmar que **no** hay CGNAT
     (ya confirmado: IP pública fija disponible).
   - Confirmar port-forward UDP/51820 cuando se pruebe remoto.
1. `feat: project skeleton with subcommand routing`
2. `feat: http server with /healthz`
3. `feat: proto messages and node store in memory`
4. `feat: POST /register endpoint`
5. `feat: wg interface bring-up via netlink`
6. `feat: wireguard peer config via wgctrl-go on server`
7. `feat: client register subcommand`
8. `feat: GET /peers and client reconciliation loop`
9. `feat: sqlite persistence`
10. `feat: graceful shutdown and signal handling`
11. `docs: README with setup and demo instructions`

Tests unitarios al menos para el store y la asignación de IPs por pubkey.

## 12. Criterios de "MVP terminado"

1. `hsl server` en la BBB configura su `wg0` y expone HTTP.
2. `hsl client register` en una PC Linux genera llaves, se registra, recibe IP.
3. `hsl client run` configura `wg0` localmente sin tocar archivos de WireGuard.
4. Desde esa PC: `ping 10.100.0.1` funciona.
5. Segunda PC registrada: `ping 10.100.0.3` desde la primera PC funciona (vía hub).
6. Reiniciar el server: al volver, todos los clientes recuperan estado y siguen
   funcionando.
7. Test de integración end-to-end automatizado deseable pero no bloqueante.

## 13. Entorno de prueba

- **Hub:** BeagleBone Black (Debian/ARMv7) en la LAN doméstica, IP pública fija,
  port-forward UDP/51820.
- **Clientes:** dos PCs Linux (Ubuntu 24.04 + Debian 13).
- **Overlay esperada:** hub `10.100.0.1`, clientes `10.100.0.2` y `10.100.0.3`.
- Validación inicial en LAN; luego un cliente remoto vía internet.
