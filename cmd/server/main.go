package main

import (
	"flag"
	"log"

	"github.com/gotunnel/pkg/config"
	"github.com/gotunnel/pkg/tunnel"
)

func main() {
	configPath := flag.String("c", "server.yaml", "config file path")
	flag.Parse()

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatalf("Load config error: %v", err)
	}

	server := tunnel.NewServer(cfg)
	log.Fatal(server.Run())
}
