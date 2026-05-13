package main

import (
	"context"
	"os"

	"github.com/jwmossmoz/trybox/internal/cli"
)

func main() {
	ctx := context.Background()
	if err := cli.Run(ctx, os.Args[1:]); err != nil {
		cli.Fatal(err)
	}
}
