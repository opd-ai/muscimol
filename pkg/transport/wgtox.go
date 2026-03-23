// Package transport binds WireGuard-Go userspace tunnels to the toxcore
// transport layer.
//
// It composes the following toxcore types:
//   - transport.UDPTransport      — primary UDP packet carrier
//   - transport.MultiTransport    — multi-network routing (.onion/.i2p/.nym/.loki)
//   - transport.NoiseTransport    — per-peer Noise_IK_25519_ChaChaPoly1305_SHA256 sessions
//   - noise.IKHandshake           — single-round-trip Noise-IK handshake
//   - transport.NATTraversal      — NAT type detection and public address discovery
//   - transport.HolePuncher       — UDP hole punching
//   - transport.AdvancedNATTraversal — relay fallback when hole punching fails
//   - toxnet.ToxListener          — net.Listener over the Tox overlay
//   - toxnet.ToxConn              — net.Conn used as the WireGuard packet carrier
//   - toxnet.ToxPacketListener    — net.PacketConn-style API
//
// MTU contract:
//
//	WireGuard default MTU:   1420 B
//	WireGuard overhead:        60 B  → ciphertext up to 1480 B
//	Tox lossless message max: 1372 B  → fragmentation required
//	Fragment header:             4 B  [seq:2][total:1][idx:1]
//	Effective payload/fragment: 1368 B
//	Fragments for 1480 B pkt:      2
//	Advertised WireGuard MTU: 1308 B  (conservative, avoids 3-fragment paths)
//
// All net.Addr values are toxnet.ToxAddr; no *net.UDPAddr is used anywhere.
package transport

import "net"

// MaxToxMessage is the maximum number of bytes in a single Tox lossless message.
const MaxToxMessage = 1372

// FragmentHeaderSize is the size of the framing header prepended to each
// Tox fragment: [seq uint16][total uint8][idx uint8].
const FragmentHeaderSize = 4

// EffectiveFragmentPayload is the usable payload per Tox message after the
// fragment header is subtracted.
const EffectiveFragmentPayload = MaxToxMessage - FragmentHeaderSize // 1368

// AdvertisedWireGuardMTU is the MTU muscimol advertises to WireGuard-Go.
// It is conservative enough to avoid 3-fragment paths under normal conditions.
const AdvertisedWireGuardMTU = 1308

// MeshTransport is the primary interface for encrypted peer-to-peer sessions
// over the Tox transport layer.
//
// Implementations hold a connection pool (sync.Map keyed by peer public key)
// and perform NAT traversal before establishing each new session.
type MeshTransport interface {
	// DialPeer opens a WireGuard session to the peer identified by pubkey.
	//
	// Internally it:
	//  1. Resolves the peer's toxnet.ToxAddr via pkg/dht.Client.FindValue.
	//  2. Calls transport.NATTraversal.PunchHole (with HolePuncher fallback).
	//  3. Completes the noise.IKHandshake.
	//  4. Returns the resulting toxnet.ToxConn wrapped as net.Conn.
	DialPeer(ctx interface{ Done() <-chan struct{} }, pubkey [32]byte) (net.Conn, error)

	// ListenPeer accepts inbound WireGuard session requests.
	ListenPeer(ctx interface{ Done() <-chan struct{} }) (net.Conn, error)

	// LocalAddr returns this node's canonical address (toxnet.ToxAddr).
	LocalAddr() net.Addr

	// Close shuts down all sessions and releases the TUN device.
	Close() error
}
