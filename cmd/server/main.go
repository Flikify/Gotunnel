package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/gotunnel/pkg/config"
	"github.com/gotunnel/pkg/tunnel"
	"github.com/gotunnel/pkg/webserver"
)

func main() {
	configPath := flag.String("c", "server.yaml", "config file path")
	flag.Parse()

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		log.Fatalf("Load config error: %v", err)
	}

	server := tunnel.NewServer(cfg)

	// 启动 Web 控制台
	if cfg.Web.Enabled {
		ws := webserver.NewWebServer(cfg, *configPath, server)
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
