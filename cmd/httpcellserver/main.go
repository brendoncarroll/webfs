package main

import (
	"context"
	"log"

	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
)

func main() {
	const addr = "127.0.0.1:8080"
	ctx := context.Background()
	s := httpcell.NewServer()
	log.Println("httpcellserver on", addr)
	if err := s.Serve(ctx, addr); err != nil {
		log.Fatal(err)
	}
}
