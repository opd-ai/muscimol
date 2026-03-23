// Package gateway bridges the muscimol mesh to external overlay networks:
// dn42, cjdns, and Yggdrasil.
//
// BGP sessions (for dn42 and Yggdrasil) are handled by github.com/osrg/gobgp/v3,
// a pure-Go BGP speaker.  Static route injection is also supported for simpler
// cjdns peering (no BGP required on the cjdns side).
//
// Yggdrasil peers share the 0200::/7 prefix range; dn42 uses 172.20.0.0/14 and
// fd00::/8. Local muscimol prefixes (fc00::/8) are announced outbound; learned
// external prefixes are injected into the mesh routing table.
package gateway

import (
	"context"
	"net"
)

// PeerConfig holds the configuration for a single external BGP or static peer.
type PeerConfig struct {
	// Addr is the peer's BGP session endpoint.
	Addr net.Addr
	// ASN is the peer's Autonomous System Number (dn42 / Yggdrasil).
	ASN uint32
	// Static, when true, uses static route injection instead of BGP.
	Static bool
	// StaticPrefixes lists prefixes to inject when Static is true.
	StaticPrefixes []*net.IPNet
}

// Gateway is the interface for external routing interoperability.
//
// Implementations use github.com/osrg/gobgp/v3 for BGP sessions.
type Gateway interface {
	// AddPeer registers an external BGP or static peering session.
	AddPeer(ctx context.Context, cfg PeerConfig) error

	// AdvertisePrefix announces a local prefix to all connected external peers.
	AdvertisePrefix(prefix *net.IPNet) error

	// Withdraw removes a prefix advertisement from all connected external peers.
	Withdraw(prefix *net.IPNet) error

	// Run starts the BGP speaker event loop. It blocks until ctx is cancelled.
	Run(ctx context.Context) error
}
