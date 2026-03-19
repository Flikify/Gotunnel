package main

import (
	"flag"
	"log"

	"github.com/gotunnel/internal/client/config"
	"github.com/gotunnel/internal/client/tunnel"
	"github.com/gotunnel/pkg/crypto"
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

	if cfg.Server == "" || cfg.Token == "" {
		log.Fatal("Usage: client [-c config.yaml] | [-s <server:port> -t <token>]")
	}

	client := tunnel.NewClient(cfg.Server, cfg.Token)

	// TLS 默认启用，默认跳过证书验证（类似 frp）
	if !cfg.NoTLS {
		client.TLSEnabled = true
		client.TLSConfig = crypto.ClientTLSConfig()
		log.Printf("[Client] TLS enabled")
	}

	client.Run()
}
