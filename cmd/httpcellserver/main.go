package main

import (
	"log"

	"github.com/brendoncarroll/webfs/pkg/cells/httpcell"
)

func main() {
	const addr = "127.0.0.1:8080"
	s := httpcell.NewServer()
	log.Println("httpcellserver on", addr)
	if err := s.ListenAndServe(addr); err != nil {
		log.Fatal(err)
	}
}
