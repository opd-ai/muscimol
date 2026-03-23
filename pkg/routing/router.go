// Package routing implements mesh path selection for muscimol.
//
// It maintains an in-process peer graph and selects next-hops using Dijkstra's
// algorithm with a composite metric:
//
//	score = latency_ms + hop_count × 10
//
// Convergence target: < 2 s for 10 000 nodes (bounded by DHT lookup p95 < 500 ms
// plus one gossip round via dht.GroupStorage).
//
// All net.Addr values flowing through this package are toxnet.ToxAddr.
// No *net.UDPAddr is used.
package routing

import "net"

// PathMetric holds the composite routing metric for a path to a peer.
type PathMetric struct {
	// LatencyMS is the measured round-trip latency to the next hop in milliseconds.
	LatencyMS int64
	// HopCount is the number of hops from this node to the destination.
	HopCount int
	// Score is the pre-computed composite: LatencyMS + HopCount*10.
	Score int64
}

// Router is the interface for overlay path selection.
//
// Implementations query pkg/dht.Client for peer metadata and gossip updated
// metrics via dht.GroupStorage records.
type Router interface {
	// AddPeer registers a peer and its canonical overlay address.
	// addr must be a toxnet.ToxAddr.
	AddPeer(pubkey [32]byte, addr net.Addr) error

	// RemovePeer deregisters a peer and removes it from the path graph.
	RemovePeer(pubkey [32]byte)

	// NextHop returns the best next-hop net.Addr (toxnet.ToxAddr) for a
	// destination IPv6 address. Returns an error if no path is known.
	NextHop(dst net.IP) (net.Addr, error)

	// Metrics returns the current PathMetric for the path to pubkey.
	Metrics(pubkey [32]byte) (PathMetric, error)
}
