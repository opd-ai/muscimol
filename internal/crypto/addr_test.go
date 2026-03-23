package crypto

import (
	"net"
	"testing"
)

func TestDeriveIPv6_Deterministic(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i)
	}

	ip1 := DeriveIPv6(key)
	ip2 := DeriveIPv6(key)

	if !ip1.Equal(ip2) {
		t.Fatalf("DeriveIPv6 is not deterministic: got %s and %s", ip1, ip2)
	}
}

func TestDeriveIPv6_Prefix(t *testing.T) {
	var key [32]byte
	ip := DeriveIPv6(key)

	if ip[0] != 0xfc {
		t.Fatalf("expected first byte 0xfc (fc00::/8 ULA prefix), got 0x%02x", ip[0])
	}

	_, fc00, _ := net.ParseCIDR("fc00::/8")
	if !fc00.Contains(ip) {
		t.Fatalf("derived address %s is not in fc00::/8", ip)
	}
}

func TestDeriveIPv6_Unique(t *testing.T) {
	var key1, key2 [32]byte
	key1[0] = 1
	key2[0] = 2

	ip1 := DeriveIPv6(key1)
	ip2 := DeriveIPv6(key2)

	if ip1.Equal(ip2) {
		t.Fatal("different keys produced the same overlay address")
	}
}

func TestDeriveIPv6_Length(t *testing.T) {
	var key [32]byte
	ip := DeriveIPv6(key)

	if len(ip) != 16 {
		t.Fatalf("expected 16-byte IPv6 address, got %d bytes", len(ip))
	}
}
