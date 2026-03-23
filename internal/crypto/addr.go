// Package crypto provides deterministic IPv6 overlay address derivation for
// muscimol.
//
// Address scheme:
//
//	input:  32-byte Curve25519 public key (crypto.KeyPair.Public from toxcore)
//	hash:   SHA-512(pubkey) → 64 bytes
//	addr:   bytes[0:16] of hash, with byte[0] forced to 0xfc
//	result: /128 address in fc00::/8 (cjdns-compatible ULA range)
//
// This scheme is fully deterministic — no coordination server is required.
// Every node independently derives the same overlay address for any given key.
// The fc00::/8 ULA subrange was chosen for cjdns interoperability: cjdns uses
// the same SHA-512 derivation with the first byte forced to 0xfc (fc00::/8).
//
// Key management delegates to toxcore:
//   - crypto.GenerateKeyPair()   — generate a new Curve25519 key pair
//   - crypto.FromSecretKey(sk)   — reconstruct a KeyPair from a secret key
//   - crypto.KeyPair.Public      — 32-byte public key
//   - crypto.KeyPair.Secret      — 32-byte secret key
package crypto

import (
	"crypto/sha512"
	"net"
)

// DeriveIPv6 returns a deterministic /128 overlay address in fc00::/8 from a
// 32-byte Curve25519 public key.
//
// Algorithm:
//  1. Compute h = SHA-512(pubkey).
//  2. Take h[0:16] as the raw 128-bit address.
//  3. Force h[0] = 0xfc to guarantee placement within fc00::/8.
//
// This produces the same address as cjdns for the same key, enabling
// seamless cjdns interoperability without any address translation.
func DeriveIPv6(pubkey [32]byte) net.IP {
	h := sha512.Sum512(pubkey[:])
	addr := make(net.IP, 16)
	copy(addr, h[:16])
	addr[0] = 0xfc
	return addr
}
