// Package dht provides the DHT client for muscimol mesh peer discovery.
//
// It wraps the following toxcore types:
//   - dht.RoutingTable  — 256 k-buckets with XOR distance and lookup caching
//   - dht.Maintainer    — periodic pings, lookups, and pruning
//   - dht.GossipBootstrap — seed-list bootstrapping
//   - dht.LANDiscovery  — same-LAN broadcast discovery
//   - dht.GroupStorage  — DHT-stored overlay endpoint records
//   - dht.RelayStorage  — DHT-stored relay metadata
//
// All peer addresses are net.Addr values; no concrete *net.UDPAddr is used.
package dht

import (
	"context"
	"net"
)

// PeerRecord holds the information stored in the DHT for a single mesh peer.
type PeerRecord struct {
	// PublicKey is the peer's 32-byte Curve25519 public key.
	PublicKey [32]byte
	// Addr is the peer's network address (toxnet.ToxAddr at runtime).
	Addr net.Addr
	// OverlayIP is the deterministic IPv6 address derived from PublicKey.
	OverlayIP net.IP
}

// Client is the primary interface for DHT operations in muscimol.
//
// Implementations wrap dht.RoutingTable and dht.Maintainer from
// github.com/opd-ai/toxcore.
type Client interface {
	// Bootstrap seeds the routing table from well-known or user-supplied peers.
	Bootstrap(ctx context.Context, seeds []net.Addr) error

	// FindNode returns up to count PeerRecords closest to targetID using
	// dht.RoutingTable.FindClosestNodes with its built-in lookup cache.
	FindNode(ctx context.Context, targetID [32]byte, count int) ([]PeerRecord, error)

	// Store publishes an overlay endpoint record (e.g. WireGuard public key +
	// current Tox address) into dht.GroupStorage / dht.RelayStorage.
	Store(ctx context.Context, key [32]byte, value []byte) error

	// FindValue retrieves an overlay endpoint record by key.
	FindValue(ctx context.Context, key [32]byte) ([]byte, error)

	// LocalID returns this node's 32-byte Curve25519 public key.
	LocalID() [32]byte

	// Close shuts down the maintainer and releases resources.
	Close() error
}
