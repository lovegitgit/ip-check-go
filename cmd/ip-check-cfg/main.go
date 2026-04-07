package main

import (
	"fmt"
	"os"

	ipcheck "github.com/lovegitgit/ip-check-go"
)

func main() {
	if err := ipcheck.RunIPCheckCfg(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
