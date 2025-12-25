package main

import (
	"flag"
	"log"

	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
)

func main() {
	server := flag.String("s", "", "server address (ip:port)")
	token := flag.String("t", "", "auth token")
	id := flag.String("id", "", "client id (optional)")
	noTLS := flag.Bool("no-tls", false, "disable TLS")
	flag.Parse()

	if *server == "" || *token == "" {
		log.Fatal("Usage: client -s <server:port> -t <token> [-id <client_id>] [-no-tls]")
	}

	client := tunnel.NewClient(*server, *token, *id)

	// TLS 默认启用
	if !*noTLS {
		client.TLSEnabled = true
		client.TLSConfig = crypto.ClientTLSConfig()
		log.Printf("[Client] TLS enabled")
	}

	client.Run()
}
