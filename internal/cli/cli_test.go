package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HyakoV3/librespeed-route/internal/route"
)

type resolver map[string][]netip.Addr

func (r resolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	if l, ok := r[host]; ok {
		return l, nil
	}
	return nil, errors.New("no such host")
}

func vector(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vectors", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type fake struct {
	output route.Output
	err    error
	spec   route.Spec
}

func (f *fake) env() Env {
	return Env{
		Version:  "0.1.0",
		GOOS:     "linux",
		Resolver: resolver{"dns.example": {netip.MustParseAddr("8.8.8.8")}, "inside.example": {netip.MustParseAddr("10.1.2.3")}},
		Exists:   func(p string) bool { return p == "/usr/bin/traceroute" },
		Run: func(_ context.Context, spec route.Spec, _ route.Options) (route.Output, error) {
			f.spec = spec
			return f.output, f.err
		},
		Now:   func() time.Time { return time.Unix(1790000000, 0) },
		Nonce: func() (string, error) { return "0123456789abcdef", nil },
	}
}

func run(t *testing.T, ctx context.Context, env Env, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Main(ctx, args, &out, &errOut, env)
	return code, out.String(), errOut.String()
}

func TestTheTableGoesToStderrAndOneLineToStdout(t *testing.T) {
	f := &fake{output: route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}}
	code, out, errOut := run(t, context.Background(), f.env(), "8.8.8.8")
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if strings.Count(out, "\n") != 1 || !strings.HasPrefix(out, "LSR1!n=0123456789abcdef!f=4!t=8.8.8.8!s=1790000000!a=0.1.0+traceroute!h=") {
		t.Errorf("stdout is not one line of LSR1: %q", out)
	}
	// The internal addresses are in the table of the person, and never in the line for the page
	for _, ip := range []string{"192.168.0.1", "172.24.74.112", "100.64.10.69", "100.65.61.62"} {
		if !strings.Contains(errOut, ip) {
			t.Errorf("%s is not in the table", ip)
		}
		if strings.Contains(out, ip) {
			t.Errorf("%s is in the line for the page", ip)
		}
	}
	// The tool was given the address and not the name, by its absolute path
	if f.spec.Path != "/usr/bin/traceroute" || f.spec.Args[len(f.spec.Args)-1] != "8.8.8.8" {
		t.Errorf("spec %+v", f.spec)
	}
	f = &fake{output: route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}}
	if code, out, errOut = run(t, context.Background(), f.env(), "dns.example"); code != ExitOK || f.spec.Args[len(f.spec.Args)-1] != "8.8.8.8" || strings.Contains(strings.Join(f.spec.Args, " "), "dns.example") {
		t.Errorf("a name: %d %+v", code, f.spec)
	}
	// --quiet: no table; --json: one JSON object and no table
	f = &fake{output: route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}}
	if code, out, errOut = run(t, context.Background(), f.env(), "--quiet", "8.8.8.8"); code != ExitOK || errOut != "" || !strings.HasPrefix(out, "LSR1!") {
		t.Errorf("quiet: %d %q %q", code, out, errOut)
	}
	code, out, errOut = run(t, context.Background(), f.env(), "--json", "8.8.8.8")
	var doc struct {
		Version, Tool, Target, Stopped string
		Family                         int
		Hops                           []route.Hop
		LSR1                           *string
	}
	if code != ExitOK || errOut != "" || json.Unmarshal([]byte(out), &doc) != nil || strings.Count(out, "\n") != 1 {
		t.Fatalf("json: %d %q %q", code, out, errOut)
	}
	if doc.Tool != "traceroute" || doc.Target != "8.8.8.8" || doc.Family != 4 || len(doc.Hops) != 12 || doc.LSR1 == nil || !strings.HasPrefix(*doc.LSR1, "LSR1!") {
		t.Errorf("json doc: %+v", doc)
	}
}

func TestExitCodes(t *testing.T) {
	good := route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}
	cases := []struct {
		name string
		f    *fake
		args []string
		want int
	}{
		{"no host", &fake{output: good}, nil, ExitUsage},
		{"two hosts", &fake{output: good}, []string{"8.8.8.8", "1.1.1.1"}, ExitUsage},
		{"-4 and -6", &fake{output: good}, []string{"-4", "-6", "8.8.8.8"}, ExitUsage},
		{"unknown option", &fake{output: good}, []string{"--open", "8.8.8.8"}, ExitUsage},
		{"timeout too small", &fake{output: good}, []string{"--timeout", "10ms", "8.8.8.8"}, ExitUsage},
		{"help", &fake{output: good}, []string{"--help"}, ExitOK},
		{"version", &fake{output: good}, []string{"--version"}, ExitOK},
		{"internal address", &fake{output: good}, []string{"192.168.0.1"}, ExitRefused},
		{"loopback", &fake{output: good}, []string{"127.0.0.1"}, ExitRefused},
		{"name of an internal address", &fake{output: good}, []string{"inside.example"}, ExitRefused},
		{"an option as host", &fake{output: good}, []string{"--", "-oProxyCommand=x"}, ExitRefused},
		{"unknown name", &fake{output: good}, []string{"nowhere.example"}, ExitRefused},
		{"the tool failed", &fake{err: route.ErrToolFailed}, []string{"8.8.8.8"}, ExitFailed},
		{"the tool could not start", &fake{err: errors.New("exec: permission denied")}, []string{"8.8.8.8"}, ExitFailed},
		{"output that is not a route", &fake{output: route.Output{Text: "hello"}}, []string{"8.8.8.8"}, ExitFailed},
		{"stopped by the silent hops", &fake{output: route.Output{Text: good.Text, Stopped: "silent"}}, []string{"8.8.8.8"}, ExitOK},
	}
	for _, c := range cases {
		if code, _, errOut := run(t, context.Background(), c.f.env(), c.args...); code != c.want {
			t.Errorf("%s: exit %d, want %d (%s)", c.name, code, c.want, errOut)
		}
	}
	// No tool on the system
	f := &fake{output: good}
	env := f.env()
	env.Exists = func(string) bool { return false }
	if code, _, errOut := run(t, context.Background(), env, "8.8.8.8"); code != ExitNoTool || !strings.Contains(errOut, "no tracert") {
		t.Errorf("no tool: %d %s", code, errOut)
	}
	// Ctrl-C
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code, out, _ := run(t, ctx, (&fake{err: context.Canceled}).env(), "8.8.8.8"); code != ExitInterrupted || out != "" {
		t.Errorf("interrupted: %d %q", code, out)
	}
}

// A route that does not fit in the text of the page: the table is there, the exit code says it failed, nothing is written to stdout
func TestARouteThatDoesNotFit(t *testing.T) {
	var b strings.Builder
	b.WriteString("traceroute to 2001:4860:4860::8888 (2001:4860:4860::8888), 30 hops max, 80 byte packets\n")
	for i := 1; i <= 40; i++ {
		b.WriteString(strings.Replace("%d  2a01:4f8:aaaa:bbbb:cccc:dddd:eeee:ffff  12345.67 ms  12345.67 ms  12345.67 ms\n", "%d", strings.Repeat(" ", 0)+itoa(i), 1))
	}
	f := &fake{output: route.Output{Text: b.String()}}
	env := f.env()
	env.Resolver = resolver{"v6.example": {netip.MustParseAddr("2001:4860:4860::8888")}}
	code, out, errOut := run(t, context.Background(), env, "v6.example")
	if code != ExitFailed || out != "" || !strings.Contains(errOut, "does not fit") || !strings.Contains(errOut, "Route to") {
		t.Errorf("%d %q %q", code, out, errOut)
	}
}

func itoa(i int) string {
	if i < 10 {
		return " " + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

func TestTheCommandNeverReadsItsInput(t *testing.T) {
	data, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "os.Stdin") || strings.Contains(string(data), "bufio.NewScanner(os.Stdin") {
		t.Error("the command reads stdin")
	}
}

// The codes are part of what a script or an extension relies on: they do not change by accident
func TestTheExitCodesAreTheDocumentedOnes(t *testing.T) {
	want := map[string][2]int{
		"ok": {ExitOK, 0}, "usage": {ExitUsage, 1}, "refused": {ExitRefused, 2}, "no tool": {ExitNoTool, 3},
		"failed": {ExitFailed, 4}, "interrupted": {ExitInterrupted, 130},
	}
	for name, c := range want {
		if c[0] != c[1] {
			t.Errorf("%s is %d, documented as %d", name, c[0], c[1])
		}
	}
	if !strings.Contains(usage, "Exit codes: 0 ok, 1 usage, 2 the host cannot be traced, 3 no traceroute tool on this system, 4 failed, 130 interrupted.") {
		t.Error("the help does not say the codes")
	}
}

// A network that drops every probe (the Windows runner of the CI did): the message says so, and the exit code is "failed"
func TestEverySilentHop(t *testing.T) {
	f := &fake{output: route.Output{Text: vector(t, "windows-en-real-allsilent-ci.txt"), Stopped: "silent"}}
	env := f.env()
	env.GOOS = "windows"
	env.Exists = func(string) bool { return true }
	code, out, errOut := run(t, context.Background(), env, "8.8.8.8")
	if code != ExitFailed || out != "" || !strings.Contains(errOut, "no router answered (5 silent hops)") {
		t.Errorf("%d %q %q", code, out, errOut)
	}
}
