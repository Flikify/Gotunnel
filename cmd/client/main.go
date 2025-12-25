package main

import (
	"flag"
	"log"

	"github.com/gotunnel/pkg/tunnel"
)

func main() {
	server := flag.String("s", "", "server address (ip:port)")
	token := flag.String("t", "", "auth token")
	id := flag.String("id", "", "client id (optional)")
	flag.Parse()

	if *server == "" || *token == "" {
		log.Fatal("Usage: client -s <server:port> -t <token> [-id <client_id>]")
	}

	client := tunnel.NewClient(*server, *token, *id)
	client.Run()
}
