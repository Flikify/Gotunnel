package main

import (
	"flag"
	"log"

	"github.com/gotunnel/internal/client/config"
	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/version"
)

// 版本信息（通过 ldflags 注入）
var Version string
var BuildTime string
var GitCommit string

func init() {
	version.SetVersion(Version)
	version.SetBuildInfo(GitCommit, BuildTime)
}

func main() {
	server := flag.String("s", "", "server address (ip:port)")
	token := flag.String("t", "", "auth token")
	id := flag.String("id", "", "client id (optional, auto-assigned if empty)")
	noTLS := flag.Bool("no-tls", false, "disable TLS")
	configPath := flag.String("c", "", "config file path")
	flag.Parse()

	// 优先加载配置文件
	var cfg *config.ClientConfig
	if *configPath != "" {
		var err error
		cfg, err = config.LoadClientConfig(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		cfg = &config.ClientConfig{}
	}

	// 命令行参数覆盖配置文件
	if *server != "" {
		cfg.Server = *server
	}
	if *token != "" {
		cfg.Token = *token
	}
	if *id != "" {
		cfg.ID = *id
	}
	if *noTLS {
		cfg.NoTLS = *noTLS
	}

	if cfg.Server == "" || cfg.Token == "" {
		log.Fatal("Usage: client [-c config.yaml] | [-s <server:port> -t <token> [-id <client_id>] [-no-tls]]")
	}

	client := tunnel.NewClient(cfg.Server, cfg.Token, cfg.ID)

	// TLS 默认启用，默认跳过证书验证（类似 frp）
	if !cfg.NoTLS {
		client.TLSEnabled = true
		client.TLSConfig = crypto.ClientTLSConfig()
		log.Printf("[Client] TLS enabled")
	}

	// 初始化插件注册表（用于 JS 插件）
	registry := plugin.NewRegistry()
	client.SetPluginRegistry(registry)

	// 初始化版本存储
	if err := client.InitVersionStore(); err != nil {
		log.Printf("[Client] Warning: failed to init version store: %v", err)
	}

	client.Run()
}
