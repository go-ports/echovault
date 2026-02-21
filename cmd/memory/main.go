package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	rootcmd "github.com/go-ports/echovault/cmd/memory/root"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return rootcmd.New().ExecuteContext(ctx)
}
