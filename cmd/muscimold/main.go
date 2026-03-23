// Package main is the entry-point for the muscimol mesh daemon (muscimold).
//
// muscimold wires together the DHT client, encrypted transport, IPv6 addressing,
// and (optionally) the BGP gateway into a single long-running process.
//
// Usage:
//
//	muscimold [-config /etc/muscimol/config.toml]
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfgPath := flag.String("config", "muscimol.toml", "path to configuration file")
	flag.Parse()

	log.Printf("muscimold starting (config: %s)", *cfgPath)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// TODO(Phase 1): load config, initialise dht.Client, transport.MeshTransport,
	// internal/crypto.DeriveIPv6, and start the daemon event loop.

	<-ctx.Done()
	log.Println("muscimold shutting down")
}
