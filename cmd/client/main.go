package main

import (
	"flag"
	"log"

	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/plugin/builtin"
)

func main() {
	server := flag.String("s", "", "server address (ip:port)")
	token := flag.String("t", "", "auth token")
	id := flag.String("id", "", "client id (optional, auto-assigned if empty)")
	noTLS := flag.Bool("no-tls", false, "disable TLS")
	flag.Parse()

	if *server == "" || *token == "" {
		log.Fatal("Usage: client -s <server:port> -t <token> [-id <client_id>] [-no-tls]")
	}

	client := tunnel.NewClient(*server, *token, *id)

	// TLS 默认启用，默认跳过证书验证（类似 frp）
	if !*noTLS {
		client.TLSEnabled = true
		client.TLSConfig = crypto.ClientTLSConfig()
		log.Printf("[Client] TLS enabled")
	}

	// 初始化插件系统
	registry := plugin.NewRegistry()
	for _, h := range builtin.GetClientPlugins() {
		if err := registry.RegisterClient(h); err != nil {
			log.Fatalf("[Plugin] Register error: %v", err)
		}
	}
	client.SetPluginRegistry(registry)
	log.Printf("[Plugin] Registered %d plugins", len(builtin.GetClientPlugins()))

	client.Run()
}
