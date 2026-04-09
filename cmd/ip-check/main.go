package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	ipcheck "github.com/lovegitgit/ip-check-go"
)

func main() {
	if err := ipcheck.RunIPCheck(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		if errors.Is(err, ipcheck.ErrUsage) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
