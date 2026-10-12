// Command librespeed-route traces the route from this computer to a host with the tool that the system already has. See internal/cli.
// Started by a browser (with the arguments that a browser gives to a native messaging host) it is that host instead: see internal/nativehost.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/HyakoV3/librespeed-route/internal/cli"
	"github.com/HyakoV3/librespeed-route/internal/nativehost"
)

// version is set by the build (-ldflags "-X main.version=1.2.3").
var version = "0.1.0"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	args := os.Args[1:]
	// The arguments of a browser are looked at before any option: the host is not started with "--native-host" (the manifest cannot carry
	// arguments), and what does not come from one of our extensions ends at once, reading and writing nothing
	if nativehost.LooksLikeABrowserCall(args, nativehost.GeckoIDs) {
		os.Exit(nativehost.Main(ctx, args, os.Stdin, os.Stdout, nativehost.SystemDeps(version)))
	}
	os.Exit(cli.Main(ctx, args, os.Stdout, os.Stderr, cli.SystemEnv(version)))
}
