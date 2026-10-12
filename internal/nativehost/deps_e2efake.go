//go:build e2efake

package nativehost

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/HyakoV3/librespeed-route/internal/route"
)

// This file is only built with "-tags e2efake", for the end to end test of the extension (it is never in a release): the host resolves every
// name to a public address of its own and "runs" a tool that writes a canned output with internal hops, so that the test needs no network.
type fakeResolver struct{}

func (fakeResolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) {
	return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
}

const canned = `traceroute to 8.8.8.8 (8.8.8.8), 30 hops max, 60 byte packets
 1  192.168.0.1  1.253 ms  1.211 ms  1.201 ms
 2  172.24.74.112  5.234 ms  5.887 ms  5.878 ms
 3  100.65.61.62  5.927 ms  5.125 ms  4.770 ms
 4  72.14.208.80  20.117 ms  18.602 ms  18.884 ms
 5  8.8.8.8  18.624 ms  17.498 ms  16.800 ms
`

func lockDir() string {
	dir, err := os.MkdirTemp("", "librespeed-route-e2e-")
	if err != nil {
		return filepath.Join(os.TempDir(), "librespeed-route-e2e")
	}
	return dir
}

// SystemDeps of the end to end build.
func SystemDeps(version string) Deps {
	return Deps{
		Version:  version,
		GOOS:     runtime.GOOS,
		Resolver: fakeResolver{},
		Exists:   func(string) bool { return true },
		Run: func(ctx context.Context, _ route.Spec, _ route.Options) (route.Output, error) {
			select {
			case <-time.After(300 * time.Millisecond):
				return route.Output{Text: canned}, nil
			case <-ctx.Done():
				return route.Output{}, ctx.Err()
			}
		},
		Now:   time.Now,
		Nonce: func() (string, error) { return "0123456789abcdef", nil },
		// A folder of its own for each process: the test runs one trace after another and the gap between two would refuse the next one
		// (the lock is tested in the Go tests)
		LockDir:   lockDir(),
		ChromeIDs: ChromeIDs,
		GeckoIDs:  GeckoIDs,
		// The test says which protocol the program speaks (this build is never in a release)
		HelloProtocol: protocolFromEnv(),
	}
}

func protocolFromEnv() int {
	n, _ := strconv.Atoi(os.Getenv("LSROUTE_E2E_PROTOCOL"))
	return n
}
