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
	"github.com/gotunnel/pkg/plugin/builtin"
	"github.com/gotunnel/pkg/plugin/sign"
)

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

	// 初始化插件系统
	registry := plugin.NewRegistry()
	if err := registry.RegisterAllServer(builtin.GetServerPlugins()); err != nil {
		log.Fatalf("[Plugin] Register error: %v", err)
	}
	server.SetPluginRegistry(registry)
	server.SetJSPluginStore(clientStore) // 设置 JS 插件存储，用于客户端重连时恢复插件
	log.Printf("[Plugin] Registered %d plugins", len(builtin.GetServerPlugins()))

	// 加载 JS 插件配置
	if len(cfg.JSPlugins) > 0 {
		jsPlugins := loadJSPlugins(cfg.JSPlugins)
		server.LoadJSPlugins(jsPlugins)
	}

	// 启动 Web 控制台
	if cfg.Web.Enabled {
		// 强制生成 Web 凭据（如果未配置）
		if config.GenerateWebCredentials(cfg) {
			log.Printf("[Web] Auto-generated credentials - Username: %s, Password: %s",
				cfg.Web.Username, cfg.Web.Password)
			log.Printf("[Web] Please save these credentials and update your config file")
			// 保存配置以持久化凭据
			if err := config.SaveServerConfig(*configPath, cfg); err != nil {
				log.Printf("[Web] Warning: failed to save config: %v", err)
			}
		}

		ws := app.NewWebServer(clientStore, server, cfg, *configPath, clientStore)
		addr := fmt.Sprintf("%s:%d", cfg.Web.BindAddr, cfg.Web.BindPort)

		go func() {
			// 始终使用 JWT 认证
			err := ws.RunWithJWT(addr, cfg.Web.Username, cfg.Web.Password, cfg.Server.Token)
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

// loadJSPlugins 加载 JS 插件文件
func loadJSPlugins(configs []config.JSPluginConfig) []tunnel.JSPluginEntry {
	var plugins []tunnel.JSPluginEntry

	for _, cfg := range configs {
		source, err := os.ReadFile(cfg.Path)
		if err != nil {
			log.Printf("[JSPlugin] Failed to load %s: %v", cfg.Path, err)
			continue
		}

		// 加载签名文件
		sigPath := cfg.SigPath
		if sigPath == "" {
			sigPath = cfg.Path + ".sig"
		}
		signature, err := os.ReadFile(sigPath)
		if err != nil {
			log.Printf("[JSPlugin] Failed to load signature for %s: %v", cfg.Name, err)
			continue
		}

		// 服务端也验证签名，防止配置文件被篡改
		if err := verifyPluginSignature(cfg.Name, string(source), string(signature)); err != nil {
			log.Printf("[JSPlugin] Signature verification failed for %s: %v", cfg.Name, err)
			continue
		}

		plugins = append(plugins, tunnel.JSPluginEntry{
			Name:      cfg.Name,
			Source:    string(source),
			Signature: string(signature),
			AutoPush:  cfg.AutoPush,
			Config:    cfg.Config,
			AutoStart: cfg.AutoStart,
		})

		log.Printf("[JSPlugin] Loaded: %s from %s (verified)", cfg.Name, cfg.Path)
	}

	return plugins
}

// verifyPluginSignature 验证插件签名
func verifyPluginSignature(name, source, signature string) error {
	// 解码签名
	signed, err := sign.DecodeSignedPlugin(signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}

	// 获取公钥
	pubKey, err := sign.GetPublicKeyByID(signed.Payload.KeyID)
	if err != nil {
		return err
	}

	// 验证插件名称
	if signed.Payload.Name != name {
		return fmt.Errorf("name mismatch: %s vs %s", signed.Payload.Name, name)
	}

	// 验证签名
	return sign.VerifyPlugin(pubKey, signed, source)
}
