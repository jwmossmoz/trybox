package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jwmossmoz/trybox/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, os.Args[1:]); err != nil {
		cli.Fatal(err)
	}
}
