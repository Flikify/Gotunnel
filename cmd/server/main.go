package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/gotunnel/internal/server/app"
	"github.com/gotunnel/internal/server/config"
	"github.com/gotunnel/internal/server/db"
	"github.com/gotunnel/internal/server/tunnel"
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

	// 创建隧道服务
	server := tunnel.NewServer(
		clientStore,
		cfg.Server.BindAddr,
		cfg.Server.BindPort,
		cfg.Server.Token,
		cfg.Server.HeartbeatSec,
		cfg.Server.HeartbeatTimeout,
	)

	// 启动 Web 控制台
	if cfg.Web.Enabled {
		ws := app.NewWebServer(clientStore, server, cfg, *configPath)
		addr := fmt.Sprintf("%s:%d", cfg.Web.BindAddr, cfg.Web.BindPort)

		go func() {
			var err error
			if cfg.Web.Username != "" && cfg.Web.Password != "" {
				err = ws.RunWithAuth(addr, cfg.Web.Username, cfg.Web.Password)
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
