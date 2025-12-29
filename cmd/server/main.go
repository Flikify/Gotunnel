package main

import (
	"flag"
	"fmt"
	"log"
	"os"

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
	log.Printf("[Plugin] Registered %d plugins", len(builtin.GetServerPlugins()))

	// 加载 JS 插件配置
	if len(cfg.JSPlugins) > 0 {
		jsPlugins := loadJSPlugins(cfg.JSPlugins)
		server.LoadJSPlugins(jsPlugins)
	}

	// 启动 Web 控制台
	if cfg.Web.Enabled {
		ws := app.NewWebServer(clientStore, server, cfg, *configPath, clientStore)
		addr := fmt.Sprintf("%s:%d", cfg.Web.BindAddr, cfg.Web.BindPort)

		go func() {
			var err error
			if cfg.Web.Username != "" && cfg.Web.Password != "" {
				err = ws.RunWithJWT(addr, cfg.Web.Username, cfg.Web.Password, cfg.Server.Token)
			} else {
				err = ws.Run(addr)
			}
			if err != nil {
				log.Printf("[Web] Server error: %v", err)
			}
		}()
	}

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

	// 检查插件是否被撤销
	if revoked, reason := sign.IsPluginRevoked(name, signed.Payload.Version); revoked {
		return fmt.Errorf("plugin revoked: %s", reason)
	}

	// 检查密钥是否已吊销
	if sign.IsKeyRevoked(signed.Payload.KeyID) {
		return fmt.Errorf("signing key revoked: %s", signed.Payload.KeyID)
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
