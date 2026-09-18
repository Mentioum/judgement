package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mentioum/judgement/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// An OS stdin read is not always interruptible by Close. Feed an io.Pipe so
	// cancellation can release the CLI even while the input producer stays open.
	stdin, feed := io.Pipe()
	go func() {
		_, err := io.Copy(feed, os.Stdin)
		_ = feed.CloseWithError(err)
	}()
	go func() {
		<-ctx.Done()
		_ = stdin.CloseWithError(ctx.Err())
	}()
	code := cli.Run(ctx, os.Args[1:], stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
