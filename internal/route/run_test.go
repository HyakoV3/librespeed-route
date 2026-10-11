package route

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// A program that writes what the test says and then ends, or never ends: the tests start this test binary again as that program.
//
//	LSROUTE_HELPER=<lines separated by "|"> LSROUTE_HELPER_MODE=exit|hang|exit3
func TestHelperProcess(t *testing.T) {
	spec := os.Getenv("LSROUTE_HELPER")
	mode := os.Getenv("LSROUTE_HELPER_MODE")
	if mode == "" {
		return
	}
	if spec == "BIG" {
		for i := 0; i < 4000; i++ {
			fmt.Printf("%2d  198.51.100.%d  1.0 ms  1.0 ms  1.0 ms\n", 1+i%30, i%250)
		}
	} else if spec != "" {
		fmt.Print(strings.ReplaceAll(strings.ReplaceAll(spec, "|", "\n"), `\x82`, "\x82") + "\n")
	}
	switch mode {
	case "hang":
		time.Sleep(time.Hour)
	case "exit3":
		os.Exit(3)
	}
	os.Exit(0)
}

func helper(t *testing.T, mode, lines string) Spec {
	t.Helper()
	t.Setenv("LSROUTE_HELPER", lines)
	t.Setenv("LSROUTE_HELPER_MODE", mode)
	return Spec{Tool: "traceroute", Path: os.Args[0], Args: []string{"-test.run=TestHelperProcess"}}
}

func TestRunReadsWhatTheToolWrites(t *testing.T) {
	spec := helper(t, "exit", "traceroute to 8.8.8.8 (8.8.8.8), 30 hops max|  1  192.168.0.1  1.2 ms  1.1 ms  1.0 ms|  2  8.8.8.8  9.1 ms  9.2 ms  9.3 ms")
	out, err := Run(context.Background(), spec, Options{Timeout: 10 * time.Second})
	if err != nil || out.Stopped != "" {
		t.Fatalf("%v %q", err, out.Stopped)
	}
	res, perr := Parse(out.Text)
	if perr != nil || len(res.Hops) != 2 {
		t.Fatalf("%v %+v", perr, res)
	}
}

func TestRunKillsAToolThatNeverEnds(t *testing.T) {
	spec := helper(t, "hang", "  1  192.168.0.1  1.2 ms  1.1 ms  1.0 ms|  2  10.0.0.1  2 ms  2 ms  2 ms")
	started := time.Now()
	out, err := Run(context.Background(), spec, Options{Timeout: 1500 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if out.Stopped != "timeout" || time.Since(started) > 8*time.Second {
		t.Fatalf("stopped %q after %v", out.Stopped, time.Since(started))
	}
	if !strings.Contains(out.Text, "192.168.0.1") {
		t.Fatalf("the text was lost: %q", out.Text)
	}
}

func TestRunStopsAfterSilentHops(t *testing.T) {
	lines := "  1  192.168.0.1  1.2 ms  1.1 ms  1.0 ms"
	for i := 2; i <= 9; i++ {
		lines += fmt.Sprintf("|%2d  * * *", i)
	}
	started := time.Now()
	out, err := Run(context.Background(), helper(t, "hang", lines), Options{Timeout: time.Minute, SilentStop: 5})
	if err != nil || out.Stopped != "silent" || time.Since(started) > 20*time.Second {
		t.Fatalf("%v %q after %v", err, out.Stopped, time.Since(started))
	}
	// Four silent hops and then an answer: not stopped
	lines = "  1  192.168.0.1  1 ms  1 ms  1 ms|  2  * * *|  3  * * *|  4  * * *|  5  * * *|  6  8.8.8.8  9 ms  9 ms  9 ms"
	out, err = Run(context.Background(), helper(t, "exit", lines), Options{Timeout: 10 * time.Second, SilentStop: 5})
	if err != nil || out.Stopped != "" {
		t.Fatalf("%v %q", err, out.Stopped)
	}
	// tracert writes the words after the stars, tracepath "no reply"
	for _, line := range []string{" 5     *        *        *     Esgotado o tempo limite do pedido.", " 3   * * *", " 4:  no reply", "12  *"} {
		if !SilentLine(line) {
			t.Errorf("%q is a silent hop", line)
		}
	}
	for _, line := range []string{" 1  192.168.0.1  1 ms", " 6  * 8.8.8.8  9 ms", "traceroute to x", ""} {
		if SilentLine(line) {
			t.Errorf("%q is not a silent hop", line)
		}
	}
}

func TestRunKeepsTheBytesOfAnyCodePageAndLimitsTheText(t *testing.T) {
	out, err := Run(context.Background(), helper(t, "exit", `com no m\x82ximo 30 saltos:|  1    <1 ms    <1 ms    <1 ms  192.168.0.1`), Options{Timeout: 10 * time.Second})
	if err != nil || !strings.Contains(out.Text, "m\x82ximo") {
		t.Fatalf("%v %q", err, out.Text)
	}
	out, err = Run(context.Background(), helper(t, "exit", "BIG"), Options{Timeout: 20 * time.Second})
	if err != nil || len(out.Text) > maxText {
		t.Fatalf("%v %d", err, len(out.Text))
	}
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	spec := helper(t, "hang", "  1  192.168.0.1  1 ms  1 ms  1 ms")
	go func() { time.Sleep(700 * time.Millisecond); cancel() }()
	started := time.Now()
	_, err := Run(ctx, spec, Options{Timeout: time.Minute})
	if !errors.Is(err, context.Canceled) || time.Since(started) > 8*time.Second {
		t.Fatalf("%v after %v", err, time.Since(started))
	}
}

func TestRunReportsAToolThatFailsAndAToolThatIsMissing(t *testing.T) {
	if _, err := Run(context.Background(), helper(t, "exit3", ""), Options{Timeout: 10 * time.Second}); err != ErrToolFailed {
		t.Errorf("failure: %v", err)
	}
	if _, err := Run(context.Background(), Spec{Tool: "traceroute", Path: "/nonexistent/traceroute"}, Options{}); err == nil {
		t.Error("a missing program was started")
	}
	var _ = exec.ErrNotFound
}

func exists(paths ...string) func(string) bool {
	return func(p string) bool {
		for _, q := range paths {
			if p == q {
				return true
			}
		}
		return false
	}
}

func TestCommandForGivesAbsolutePathsAndOnlyTheAddress(t *testing.T) {
	v4 := netip.MustParseAddr("8.8.8.8")
	v6 := netip.MustParseAddr("2001:4860:4860::8888")
	cases := []struct {
		goos   string
		ip     netip.Addr
		exists func(string) bool
		root   string
		path   string
		args   string
		tool   string
	}{
		{"windows", v4, exists(`C:\Windows\System32\tracert.exe`), "", `C:\Windows\System32\tracert.exe`, "-d -h 30 -w 1000 -4 8.8.8.8", "tracert"},
		{"windows", v6, exists(`D:\Win\System32\tracert.exe`), `D:\Win\`, `D:\Win\System32\tracert.exe`, "-d -h 30 -w 1000 -6 2001:4860:4860::8888", "tracert"},
		{"darwin", v4, exists("/usr/sbin/traceroute"), "", "/usr/sbin/traceroute", "-n -m 30 -w 1 8.8.8.8", "traceroute"},
		{"darwin", v6, exists("/usr/sbin/traceroute6"), "", "/usr/sbin/traceroute6", "-n -m 30 -w 1 2001:4860:4860::8888", "traceroute"},
		{"linux", v4, exists("/usr/bin/traceroute", "/usr/bin/tracepath"), "", "/usr/bin/traceroute", "-n -m 30 -w 1 -4 8.8.8.8", "traceroute"},
		{"linux", v6, exists("/usr/sbin/traceroute"), "", "/usr/sbin/traceroute", "-n -m 30 -w 1 -6 2001:4860:4860::8888", "traceroute"},
		{"linux", v4, exists("/usr/bin/tracepath"), "", "/usr/bin/tracepath", "-n -m 30 -4 8.8.8.8", "tracepath"},
	}
	for _, c := range cases {
		spec, err := CommandFor(c.goos, c.ip, c.exists, c.root)
		if err != nil {
			t.Fatalf("%s %v: %v", c.goos, c.ip, err)
		}
		if spec.Path != c.path || strings.Join(spec.Args, " ") != c.args || spec.Tool != c.tool {
			t.Errorf("%s %v: %s %v (%s)", c.goos, c.ip, spec.Path, spec.Args, spec.Tool)
		}
		if spec.Args[len(spec.Args)-1] != c.ip.String() {
			t.Errorf("the last argument is not the address: %v", spec.Args)
		}
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		if _, err := CommandFor(goos, v4, exists(), ""); err != ErrNoTool {
			t.Errorf("%s with no tool: %v", goos, err)
		}
	}
}

type fakeResolver map[string][]netip.Addr

func (f fakeResolver) LookupNetIP(_ context.Context, network, host string) ([]netip.Addr, error) {
	var out []netip.Addr
	for _, a := range f[host] {
		if (network == "ip4" && !a.Is4()) || (network == "ip6" && !a.Is6()) {
			continue
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, errors.New("no such host")
	}
	return out, nil
}

func TestResolveGivesAPublicAddressAndNeverAName(t *testing.T) {
	r := fakeResolver{
		"server.example":   {netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("2001:4860:4860::8888"), netip.MustParseAddr("8.8.8.8")},
		"internal.example": {netip.MustParseAddr("10.0.0.5")},
		"mapped.example":   {netip.MustParseAddr("::ffff:8.8.8.8")},
	}
	ctx := context.Background()
	ok := map[string]struct {
		family int
		want   string
	}{
		"8.8.8.8":                {0, "8.8.8.8"},
		"[2001:4860:4860::8888]": {6, "2001:4860:4860::8888"},
		"server.example":         {0, "2001:4860:4860::8888"},
	}
	for host, c := range ok {
		a, err := Resolve(ctx, r, host, c.family)
		if err != nil || a.String() != c.want {
			t.Errorf("%s: %v %v", host, a, err)
		}
	}
	if a, err := Resolve(ctx, r, "server.example", 4); err != nil || a.String() != "8.8.8.8" {
		t.Errorf("v4: %v %v", a, err)
	}
	refused := map[string]error{
		"192.168.0.1": ErrNotGlobal, "100.64.0.1": ErrNotGlobal, "127.0.0.1": ErrNotGlobal, "::1": ErrNotGlobal, "2001:db8::1": ErrNotGlobal,
		"internal.example": ErrNotGlobal, "mapped.example": ErrNotGlobal,
		"-oProxyCommand=x": ErrBadHost, "a b": ErrBadHost, "a;b": ErrBadHost, "a$(id)": ErrBadHost, "": ErrBadHost, "x/y": ErrBadHost,
		strings.Repeat("a", 254): ErrBadHost, "missing.example": ErrNoAddress,
	}
	for host, want := range refused {
		if _, err := Resolve(ctx, r, host, 0); err != want {
			t.Errorf("%q: %v, want %v", host, err, want)
		}
	}
	if _, err := Resolve(ctx, r, "8.8.8.8", 6); err != ErrNoAddress {
		t.Errorf("v4 address with -6: %v", err)
	}
	// A name that starts with "-" would be an option for the tool if it were ever given as a name
	for _, host := range []string{"-v", "-4", "-x.example", "--", "-"} {
		if _, err := Resolve(ctx, r, host, 0); err != ErrBadHost {
			t.Errorf("%q: %v", host, err)
		}
	}
	if _, err := Resolve(ctx, r, "8.8.8.8", 5); err != ErrBadHost {
		t.Errorf("family 5: %v", err)
	}
}
