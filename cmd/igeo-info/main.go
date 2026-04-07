package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	ipcheck "github.com/lovegitgit/ip-check-go"
)

func main() {
	if err := ipcheck.RunGeoInfo(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
