# muscimol

A decentralized mesh-network daemon written in pure Go.

- **Peer discovery** via `github.com/opd-ai/toxcore` DHT (`dht.RoutingTable`, 256 k-buckets, XOR distance)
- **Encrypted transport** via WireGuard-Go userspace (`golang.zx2c4.com/wireguard`) carried over `transport.NoiseTransport` (Noise_IK_25519_ChaChaPoly1305_SHA256)
- **Deterministic IPv6 addressing** — `fc00::/8` from SHA-512(Curve25519 pubkey), cjdns-compatible
- **NAT traversal** via `transport.NATTraversal` / `transport.HolePuncher` / `transport.AdvancedNATTraversal`
- **Interoperability** with dn42, cjdns, and Yggdrasil via a BGP/static-route gateway (`github.com/osrg/gobgp/v3`)
- **No coordination server** — fully decentralized, no DERP, no accounts, targets 10 000+ concurrent peers

See [SPEC.md](SPEC.md) for the full technical specification including repository structure, core component interfaces, performance targets, implementation phases, and feasibility analysis.

## Package layout

```
cmd/muscimold/      daemon entry-point
pkg/dht/            DHT client (wraps dht.RoutingTable, dht.Maintainer)
pkg/transport/      WireGuard ↔ Tox transport binding + fragmentation layer
pkg/routing/        Dijkstra path selection (latency + hop-count metric)
pkg/gateway/        BGP/static bridge for dn42 / cjdns / Yggdrasil
internal/crypto/    Deterministic IPv6 derivation from public key
```

## Quick start

```sh
go build ./cmd/muscimold
./muscimold -config muscimol.toml
```

## License

See [LICENSE](LICENSE).
