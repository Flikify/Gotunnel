package main

import (
	"flag"
	"log"

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
	configPath := flag.String("c", "", "config file path (required)")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("Error: -c flag is required. Usage: server -c <config-file>")
	}

	log.Fatal(bootstrap.Run(*configPath))
}
