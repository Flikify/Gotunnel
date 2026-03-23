package main

//go:generate go run github.com/swaggo/swag/cmd/swag init -g main.go -d .,../../internal/server/http/handler,../../internal/server/http/dto,../../internal/server/storage/sqlite,../../pkg/protocol -o ../../docs --parseDependency --parseInternal

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
	"log"

	_ "github.com/gotunnel/docs" // Swagger docs

	"github.com/gotunnel/internal/server/bootstrap"
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
	configPath := flag.String("c", "server.yaml", "config file path")
	flag.Parse()

	log.Fatal(bootstrap.Run(*configPath))
}
