# muscimol — Technical Specification

`github.com/opd-ai/muscimol` is a decentralized mesh-network daemon written in pure Go.
It uses `github.com/opd-ai/toxcore` for DHT peer discovery and encrypted transport, and
`golang.zx2c4.com/wireguard` (userspace WireGuard-Go) for the data plane.

---

## Repository Structure

```
github.com/opd-ai/muscimol
├── cmd/
│   └── muscimold/          # daemon entry-point; CLI flags, config loading, signal handling
│       └── main.go
├── pkg/
│   ├── dht/                # DHT client — wraps dht.RoutingTable + dht.Maintainer
│   │   └── client.go
│   ├── transport/          # WireGuard-Go tun + Tox transport binding
│   │   └── wgtox.go
│   ├── routing/            # mesh path selection; metric calculation
│   │   └── router.go
│   └── gateway/            # BGP/static bridge for dn42 / cjdns / Yggdrasil peering
│       └── gateway.go
├── internal/
│   └── crypto/             # deterministic IPv6 from Curve25519 public key
│       └── addr.go
├── go.mod
├── go.sum
├── SPEC.md
└── README.md
```

### Required pure-Go dependencies

| Dependency | Role | Pure Go? |
|---|---|---|
| `github.com/opd-ai/toxcore` | DHT, Noise-IK transport, NAT traversal, net.Conn wrappers | ✅ |
| `golang.zx2c4.com/wireguard` | Userspace WireGuard data-plane | ✅ |
| `golang.zx2c4.com/wireguard/tun/netstack` | gVisor-backed in-process TUN device (no kernel module) | ✅ |
| `golang.org/x/net` | IPv6, ICMPv6, DNS helpers | ✅ |
| `golang.org/x/crypto` | SHA-512, Curve25519 supplementary ops | ✅ |
| `github.com/osrg/gobgp/v3` | BGP speaker for dn42/Yggdrasil route exchange | ✅ |

### Package composition of toxcore types

- `pkg/dht` — holds a `dht.RoutingTable` (256 k-buckets, XOR distance), a `dht.Maintainer` for periodic pings/lookups/pruning, and a `dht.GossipBootstrap` seeder. Uses `dht.GroupStorage` / `dht.RelayStorage` to store overlay endpoint records. `dht.LANDiscovery` handles LAN segment bootstrapping.
- `pkg/transport` — owns a `transport.UDPTransport` (or `transport.MultiTransport` for .onion/.i2p/.nym) as the packet carrier; wraps it in `transport.NoiseTransport` (Noise_IK_25519_ChaChaPoly1305_SHA256 via `noise.IKHandshake`). Exposes a `toxnet.ToxListener` / `toxnet.ToxConn` pair so the WireGuard netstack TUN can exchange packets over a `net.Conn`-compatible surface. `transport.NATTraversal` and `transport.HolePuncher` are invoked before adding a peer session.
- `pkg/routing` — queries `pkg/dht` for peer metadata; uses `toxnet.ToxAddr` as the canonical `net.Addr` for all peer addresses, never `*net.UDPAddr`.
- `internal/crypto` — uses `crypto.KeyPair` / `crypto.GenerateKeyPair()` from toxcore; derives the IPv6 overlay address without any external coordination.

---

## Core Components

### DHT Client (`pkg/dht/`)

```go
type Client interface {
    // Bootstrap seeds the routing table from well-known or user-supplied peers.
    Bootstrap(ctx context.Context, seeds []net.Addr) error
    // FindNode returns up to count peers closest to targetID.
    FindNode(ctx context.Context, targetID [32]byte, count int) ([]PeerRecord, error)
    // Store publishes an overlay endpoint record into DHT-stored metadata.
    Store(ctx context.Context, key [32]byte, value []byte) error
    // FindValue retrieves an overlay endpoint record by key.
    FindValue(ctx context.Context, key [32]byte) ([]byte, error)
    // LocalID returns this node's 32-byte Curve25519 public key.
    LocalID() [32]byte
}
```

Wraps: `dht.RoutingTable`, `dht.Maintainer`, `dht.GossipBootstrap`, `dht.GroupStorage`, `dht.RelayStorage`, `dht.LANDiscovery`.
`FindNode` delegates to `dht.RoutingTable.FindClosestNodes(targetID, count)` with its built-in lookup cache.

---

### Transport (`pkg/transport/`)

```go
type MeshTransport interface {
    // DialPeer opens a WireGuard session to peer identified by its public key.
    // Internally resolves the peer's Tox address via DHT, then calls
    // transport.NATTraversal.PunchHole before establishing the session.
    DialPeer(ctx context.Context, pubkey [32]byte) (net.Conn, error)
    // ListenPeer accepts inbound WireGuard session requests.
    ListenPeer(ctx context.Context) (net.Conn, error)
    // LocalAddr returns the node's canonical net.Addr (toxnet.ToxAddr).
    LocalAddr() net.Addr
    // Close shuts down all sessions and releases the TUN device.
    Close() error
}
```

Wraps: `transport.UDPTransport`, `transport.NoiseTransport` (`noise.IKHandshake`), `transport.NATTraversal`, `transport.HolePuncher`, `transport.AdvancedNATTraversal`, `toxnet.ToxListener`, `toxnet.ToxConn`, `toxnet.ToxPacketListener`.

WireGuard data path: WireGuard-Go netstack TUN (`tun/netstack`) ↔ `toxnet.ToxConn` ↔ `transport.NoiseTransport` ↔ `transport.UDPTransport` (UDP/IP).

**MTU chain**: WireGuard default MTU 1420 B → WireGuard adds 60 B overhead → ciphertext 1480 B. Tox lossless message maximum is **1372 bytes**, so a single WireGuard packet **does not fit in one Tox message**. Fragmentation into ≤1372-byte chunks with a 4-byte sequence header (fragment index + total count) is mandatory. See §Feasibility & Risks.

Connection pool: a `sync.Map` keyed by peer public key stores live `toxnet.ToxConn` sessions, bounded by max-peers config. LRU eviction when idle > 5 min.

---

### Addressing (`internal/crypto/`)

```go
// DeriveIPv6 returns a deterministic /128 address in fc00::/8
// by computing SHA-512(pubkey) and using bytes [0:16) (16 bytes) as the host part.
// Byte 0 is forced to 0xfc to stay within the fc00::/7 ULA range.
// This matches cjdns's addressing scheme, enabling interoperability.
func DeriveIPv6(pubkey [32]byte) net.IP
```

Wraps: `crypto.KeyPair`, `crypto.GenerateKeyPair()`, `crypto.FromSecretKey()`.
No coordination server required — every node independently derives the same address for a given key.

---

### Routing (`pkg/routing/`)

```go
type Router interface {
    // AddPeer registers a peer and its overlay address.
    AddPeer(pubkey [32]byte, addr net.Addr) error
    // RemovePeer deregisters a peer.
    RemovePeer(pubkey [32]byte)
    // NextHop returns the best next-hop net.Addr for a destination IPv6 prefix.
    NextHop(dst net.IP) (net.Addr, error)
    // Metrics returns the current path metric (latency ms + hop count * weight).
    Metrics(pubkey [32]byte) PathMetric
}
```

Metric: `score = latency_ms + hop_count * 10`. Path selection uses Dijkstra over the in-memory peer graph. Convergence target: < 2 s for 10 K nodes (bounded by DHT lookup p95 < 500 ms + one gossip round).
All `net.Addr` values are `toxnet.ToxAddr` — never `*net.UDPAddr`.

---

### Gateway (`pkg/gateway/`)

```go
type Gateway interface {
    // AddPeer registers an external BGP or static peering session.
    AddPeer(ctx context.Context, cfg PeerConfig) error
    // AdvertisePrefix announces the local fc00::/8 allocation into the external peer.
    AdvertisePrefix(prefix *net.IPNet) error
    // Withdraw removes a prefix advertisement.
    Withdraw(prefix *net.IPNet) error
    // Run starts the BGP speaker event loop (blocks until ctx is cancelled).
    Run(ctx context.Context) error
}
```

Uses `github.com/osrg/gobgp/v3` for BGP sessions with dn42 / Yggdrasil border routers.
Static route injection is also supported for simpler cjdns peering.

---

## Performance Targets

| Metric | Target | Notes |
|--------|--------|-------|
| Max concurrent peers | 10,000+ | Bounded by DHT k-bucket depth; `dht.RoutingTable` supports 256 buckets |
| DHT lookup latency (p95) | < 500 ms | `FindClosestNodes` with lookup cache; iterative with parallelism = 3 |
| Memory per peer | < 1 MB | Includes WG session state, routing entry, DHT record |
| Control-plane bandwidth overhead | < 5% | DHT pings + gossip amortised over data-plane sessions |
| WireGuard-over-Tox effective MTU | **1308 B** | 1372 B Tox max − 4 B frag header − 60 B WG overhead = 1308 B payload |
| Noise-IK handshake latency | < 100 ms | Single round-trip; session reused for lifetime of Tox connection |

---

## Implementation Phases

### Phase 1 — DHT core + WireGuard tunnel (Complexity: M)

**Success criterion**: 100-node encrypted mesh test with direct WireGuard sessions.

- Implement `pkg/dht.Client` backed by `dht.RoutingTable` + `dht.Maintainer`.
- Bootstrap via `dht.GossipBootstrap` from user-supplied seed list.
- Implement `internal/crypto.DeriveIPv6` and key generation via `crypto.GenerateKeyPair()`.
- Bind WireGuard-Go netstack TUN to a `toxnet.ToxConn` over `transport.UDPTransport`.
- Implement 1372-byte Tox fragmentation layer in `pkg/transport`.
- Implement `transport.NoiseTransport` session establishment using `noise.IKHandshake`.
- Daemon skeleton in `cmd/muscimold/main.go` with config file and graceful shutdown.

Toxcore packages: `dht`, `transport` (`UDPTransport`, `NoiseTransport`), `noise`, `crypto`, `toxnet`.

---

### Phase 2 — Mesh routing + NAT traversal (Complexity: L)

**Success criterion**: Two peers behind separate NAT can exchange WireGuard traffic.

- Integrate `transport.NATTraversal.DetectNATType()` on startup; cache result.
- Use `transport.HolePuncher` for UDP hole punching before every new peer connection.
- Fall back to `transport.AdvancedNATTraversal` relay path if hole-punch fails within 3 s.
- Implement `pkg/routing.Router` with Dijkstra path selection and `PathMetric`.
- Gossip routing metrics via `dht.GroupStorage` records.
- Add connection pool with LRU eviction in `pkg/transport`.

Toxcore packages: `transport` (`NATTraversal`, `HolePuncher`, `AdvancedNATTraversal`, `MultiTransport`), `dht` (`GroupStorage`, `RelayStorage`).

---

### Phase 3 — IPv6 addressing + DNS + app integration (Complexity: M)

**Success criterion**: A standard Go `net.Conn` application connects over the mesh without modification.

- Assign per-node fc00::/8 address derived by `internal/crypto.DeriveIPv6`.
- Configure WireGuard-Go netstack to serve the derived /128 address on the TUN.
- Expose `toxnet.ToxListener` so callers can `net.Listen("tcp", meshAddr)` transparently.
- Add lightweight DNS resolver mapping `<pubkey-hex>.mesh.local` → overlay IPv6.
- Validate `toxnet.ToxAddr` propagation: ensure no `*net.UDPAddr` leaks.
- `dht.LANDiscovery` broadcast for same-LAN peer auto-discovery.

Toxcore packages: `toxnet` (`ToxConn`, `ToxListener`, `ToxAddr`, `ToxPacketListener`), `dht` (`LANDiscovery`).

---

### Phase 4 — BGP gateway + dn42 / Yggdrasil interop (Complexity: L)

**Success criterion**: Bidirectional route exchange with at least one live dn42 peer.

- Implement `pkg/gateway.Gateway` using `gobgp/v3` BGP speaker.
- Announce local fc00::/8 allocation to external BGP peers; import dn42 prefixes into mesh routing table.
- Static peer config for cjdns (no BGP required — route injection only).
- Yggdrasil peering via shared-prefix routing (`0200::/7`).
- Integration test: exchange test prefix with a local GoBGP instance.

Toxcore packages: none (data plane only; gateway speaks native BGP to external peers).

---

## Feasibility & Risks

### Risk 1 — WireGuard MTU vs Tox 1372-byte message limit ⚠️ SHOWSTOPPER

WireGuard's default MTU is 1420 bytes; after WireGuard's 60-byte overhead the ciphertext is up to 1480 bytes. Tox lossless messages are capped at **1372 bytes**, so every WireGuard packet must be split across multiple Tox messages.

**Mitigation**: Implement a framing layer in `pkg/transport` that prepends a 4-byte header `[seq:2][total:1][idx:1]` to each fragment. The receiver reassembles before passing to the WireGuard TUN. Effective payload MTU becomes `1372 − 4 = 1368` bytes per fragment; a 1480-byte packet requires 2 fragments. Advertise a clamped WireGuard MTU of **1308 B** to avoid 3-fragment paths under normal conditions.

---

### Risk 2 — Double encryption overhead (WireGuard + Noise-IK + Tox framing)

Data traverses WireGuard ChaCha20-Poly1305, then `transport.NoiseTransport` Noise_IK_25519_ChaChaPoly1305_SHA256, then Tox UDP framing. Combined overhead ≈ 120 B per packet plus two symmetric-key operations per packet.

**Mitigation**: Noise-IK provides session reuse via `noise.IKHandshake`; the per-packet cost is one AEAD pass on the Tox layer. On modern hardware (AES-NI or ChaCha20 SIMD), this adds < 5 µs per packet. Consider allowing WireGuard's built-in encryption to be replaced with a pass-through mode when Noise-IK already guarantees confidentiality (Phase 2 optimisation).

---

### Risk 3 — DHT lookup latency for real-time endpoint resolution

Each new peer connection requires a DHT lookup (up to p95 500 ms) before a WireGuard session can be established. At 10 K nodes this may cause noticeable connection setup delay.

**Mitigation**: Cache resolved `toxnet.ToxAddr` entries in-process with a 5-minute TTL. Pre-emptively refresh cached entries via `dht.Maintainer`'s background lookup pass. Use `dht.RoutingTable.FindClosestNodes` lookup cache to avoid redundant network round-trips.

---

### Risk 4 — Correct `net.Addr` interface usage (toxcore convention)

toxcore requires all peer addresses to flow through `net.Addr` / `net.Conn` / `net.Listener` interfaces. Any use of concrete types such as `*net.UDPAddr` is a bug that breaks multi-transport routing (`.onion`, `.i2p`, `.nym`, `.loki`).

**Mitigation**: All peer address fields in `pkg/routing` and `pkg/transport` are typed as `net.Addr`; only `toxnet.ToxAddr` is used as the concrete value. Enforced via interface assertions in unit tests (Phase 1).

---

### Advantages from toxcore

- **Multi-network transport**: `transport.MultiTransport` transparently routes over UDP, Tor, I2P, Nym, or Loki — muscimol inherits censorship-resistance with no extra code.
- **Existing NAT traversal**: `transport.HolePuncher` and `transport.AdvancedNATTraversal` cover the common NAT traversal cases that Tailscale's DERP infrastructure solves, but without any coordination server.
- **Noise-IK session reuse**: `noise.IKHandshake` is a single round-trip; re-keying is handled by `transport.NoiseTransport` in the background, so connection establishment is fast for repeat peers.
