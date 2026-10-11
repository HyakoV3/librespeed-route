package route

import (
	"context"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Runs the real tool of the system against the loopback (the one hop that every machine can reach), to see that the flags are accepted
// and that the output is read. Not run unless LSROUTE_LIVE=1 (the CI sets it on Linux, macOS and Windows).
func TestTheRealToolOnTheLoopback(t *testing.T) {
	if os.Getenv("LSROUTE_LIVE") != "1" {
		t.Skip("LSROUTE_LIVE is not 1")
	}
	for _, ip := range []string{"127.0.0.1"} {
		spec, err := CommandFor(runtime.GOOS, netip.MustParseAddr(ip), FileExists, os.Getenv("SystemRoot"))
		if err == ErrNoTool {
			t.Skipf("no tool on this system (%s)", runtime.GOOS)
		}
		if err != nil {
			t.Fatal(err)
		}
		out, rerr := Run(context.Background(), spec, Options{Timeout: 60 * time.Second})
		if rerr != nil {
			t.Fatalf("%s %v: %v\n%s", spec.Path, spec.Args, rerr, out.Text)
		}
		res, perr := Parse(out.Text)
		if perr != nil {
			t.Fatalf("%s: %v\n%s", spec.Tool, perr, out.Text)
		}
		if res.Tool != spec.Tool || len(res.Hops) < 1 || res.Hops[0].IP == nil || *res.Hops[0].IP != ip {
			t.Fatalf("%s: %+v\n%s", spec.Tool, res, out.Text)
		}
		t.Logf("%s read %d hop(s): %s", spec.Tool, len(res.Hops), *res.Hops[0].IP)
	}
}

// On Linux without traceroute the command uses tracepath: the same hop, and tracepath says that it was reached
func TestTracepathOnTheLoopback(t *testing.T) {
	if os.Getenv("LSROUTE_LIVE") != "1" || runtime.GOOS != "linux" {
		t.Skip("only on Linux with LSROUTE_LIVE=1")
	}
	onlyTracepath := func(p string) bool { return strings.HasSuffix(p, "/tracepath") && FileExists(p) }
	spec, err := CommandFor("linux", netip.MustParseAddr("127.0.0.1"), onlyTracepath, "")
	if err == ErrNoTool {
		t.Skip("no tracepath")
	}
	if err != nil || spec.Tool != "tracepath" {
		t.Fatalf("%+v %v", spec, err)
	}
	out, rerr := Run(context.Background(), spec, Options{Timeout: 60 * time.Second})
	if rerr != nil {
		t.Fatalf("%v\n%s", rerr, out.Text)
	}
	res, perr := Parse(out.Text)
	if perr != nil || res.Tool != "tracepath" || len(res.Hops) != 1 || res.Reached == nil || !*res.Reached {
		t.Fatalf("%v %+v\n%s", perr, res, out.Text)
	}
}
