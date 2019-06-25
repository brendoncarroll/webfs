package main

import (
	"fmt"

	"github.com/brendoncarroll/webfs/pkg/webfscmd"
)

func main() {
	if err := webfscmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
