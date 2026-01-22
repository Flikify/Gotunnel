package main

// @title GoTunnel API
// @version 1.0
// @description GoTunnel 内网穿透服务器 API
// @host localhost:7500
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description JWT Bearer token

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/gotunnel/docs" // Swagger docs

	"github.com/gotunnel/internal/server/app"
	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/tunnel"
	"github.com/gotunnel/pkg/crypto"
	"github.com/gotunnel/pkg/plugin"
	"github.com/gotunnel/pkg/version"
)

// 版本信息（通过 ldflags 注入）
var Version string

func init() {
	version.SetVersion(Version)
}

func main() {
	configPath := flag.String("c", "server.yaml", "config file path")
	flag.Parse()

	// 加载 YAML 配置
	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatalf("Load config error: %v", err)
	}

	// 初始化数据库
	clientStore, err := db.NewSQLiteStore(cfg.Server.DBPath)
	if err != nil {
		log.Fatalf("Init database error: %v", err)
	}
	defer clientStore.Close()

	// 打印 token（便于客户端连接）
	log.Printf("[Server] Token: %s", cfg.Server.Token)

	// 创建隧道服务
	server := tunnel.NewServer(
		clientStore,
		cfg.Server.BindAddr,
		cfg.Server.BindPort,
		cfg.Server.Token,
		cfg.Server.HeartbeatSec,
		cfg.Server.HeartbeatTimeout,
	)

	// 配置 TLS（默认启用）
	if !cfg.Server.TLSDisabled {
		tlsConfig, err := crypto.GenerateTLSConfig()
		if err != nil {
			log.Fatalf("Generate TLS config error: %v", err)
		}
		server.SetTLSConfig(tlsConfig)
		log.Printf("[Server] TLS enabled")
	}

	// 初始化插件系统（用于客户端 JS 插件管理）
	registry := plugin.NewRegistry()
	server.SetPluginRegistry(registry)
	server.SetJSPluginStore(clientStore) // 设置 JS 插件存储，用于客户端重连时恢复插件

	// 启动 Web 控制台
	if cfg.Server.Web.Enabled {
		// 强制生成 Web 凭据（如果未配置）
		if config.GenerateWebCredentials(cfg) {
			log.Printf("[Web] Auto-generated credentials - Username: %s, Password: %s",
				cfg.Server.Web.Username, cfg.Server.Web.Password)
			log.Printf("[Web] Please save these credentials and update your config file")
			// 保存配置以持久化凭据
			if err := config.SaveServerConfig(*configPath, cfg); err != nil {
				log.Printf("[Web] Warning: failed to save config: %v", err)
			}
		}

		ws := app.NewWebServer(clientStore, server, cfg, *configPath, clientStore)
		addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Web.BindPort)

		go func() {
			// 始终使用 JWT 认证
			err := ws.RunWithJWT(addr, cfg.Server.Web.Username, cfg.Server.Web.Password, cfg.Server.Token)
			if err != nil {
				log.Printf("[Web] Server error: %v", err)
			}
		}()
		log.Printf("[Web] Console running at http://%s (authentication required)", addr)
	}

	// 优雅关闭信号处理
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Printf("[Server] Received shutdown signal")
		server.Shutdown(30 * time.Second)
		os.Exit(0)
	}()

	log.Fatal(server.Run())
}
