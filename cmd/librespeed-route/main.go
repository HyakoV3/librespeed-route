// Command librespeed-route traces the route from this computer to a host with the tool that the system already has. See internal/cli.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/HyakoV3/librespeed-route/internal/cli"
)

// version is set by the build (-ldflags "-X main.version=1.2.3").
var version = "0.1.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Main(ctx, os.Args[1:], os.Stdout, os.Stderr, cli.SystemEnv(version)))
}
