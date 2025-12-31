package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

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
	skipVerify := flag.Bool("skip-verify", false, "skip TLS certificate verification (insecure)")
	flag.Parse()

	if *server == "" || *token == "" {
		log.Fatal("Usage: client -s <server:port> -t <token> [-id <client_id>] [-no-tls] [-skip-verify]")
	}

	client := tunnel.NewClient(*server, *token, *id)

	// TLS 默认启用，使用 TOFU 验证
	if !*noTLS {
		client.TLSEnabled = true
		// 获取数据目录
		home, _ := os.UserHomeDir()
		dataDir := filepath.Join(home, ".gotunnel")
		client.TLSConfig = crypto.ClientTLSConfigWithTOFU(*server, dataDir, *skipVerify)
		if *skipVerify {
			log.Printf("[Client] TLS enabled (certificate verification DISABLED - insecure)")
		} else {
			log.Printf("[Client] TLS enabled with TOFU certificate verification")
		}
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
