package nativehost

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HyakoV3/librespeed-route/internal/route"
)

const chromeID = "gcgijalhbacnoacnklibiffbacdchjgm"

var chromeArgs = []string{"chrome-extension://" + chromeID + "/"}

func frame(body string) []byte {
	out := make([]byte, 4+len(body))
	binary.NativeEndian.PutUint32(out, uint32(len(body)))
	copy(out[4:], body)
	return out
}

func request(typ, origin string) []byte {
	if typ == "hello" {
		return frame(`{"v":1,"id":"a1","type":"hello"}`)
	}
	return frame(`{"v":1,"id":"a1","type":"` + typ + `","origin":"` + origin + `","family":0}`)
}

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

func deps(t *testing.T, out route.Output, runErr error) Deps {
	t.Helper()
	return Deps{
		Version:   "0.1.0",
		GOOS:      "linux",
		Resolver:  resolver{"server.example": {netip.MustParseAddr("8.8.8.8")}, "inside.example": {netip.MustParseAddr("10.1.2.3")}},
		Exists:    func(p string) bool { return p == "/usr/bin/traceroute" },
		Run:       func(context.Context, route.Spec, route.Options) (route.Output, error) { return out, runErr },
		Now:       time.Now,
		Nonce:     func() (string, error) { return "0123456789abcdef", nil },
		LockDir:   t.TempDir(),
		ChromeIDs: []string{chromeID},
		GeckoIDs:  []string{"librespeed-route@hyakov3.github.io"},
	}
}

// replies reads every frame that the host wrote
func replies(t *testing.T, out []byte) []map[string]any {
	t.Helper()
	var list []map[string]any
	r := bytes.NewReader(out)
	for r.Len() > 0 {
		body, err := ReadFrame(r, MaxReply)
		if err != nil {
			t.Fatalf("a frame that is not one: %v (%q)", err, out)
		}
		var m map[string]any
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatal(err)
		}
		list = append(list, m)
	}
	return list
}

// blocked is the input of a browser that keeps the connection open: after what it sent there is nothing, and no end, until the test is over
type blocked struct{ done chan struct{} }

func (b blocked) Read([]byte) (int, error) {
	<-b.done
	return 0, io.EOF
}

func run(t *testing.T, d Deps, args []string, input []byte) (int, []byte) {
	t.Helper()
	var out bytes.Buffer
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	code := Main(context.Background(), args, io.MultiReader(bytes.NewReader(input), blocked{done}), &out, d)
	return code, out.Bytes()
}

func TestFrames(t *testing.T) {
	body, err := ReadFrame(bytes.NewReader(frame(`{"a":1}`)), 100)
	if err != nil || string(body) != `{"a":1}` {
		t.Fatalf("%v %q", err, body)
	}
	for name, in := range map[string][]byte{
		"zero":           frame(""),
		"too big":        frame(strings.Repeat("a", MaxRequest+1)),
		"short header":   {1, 0},
		"short body":     append(frame("0123456789")[:8], 'x'),
		"empty":          nil,
		"huge length":    {0xff, 0xff, 0xff, 0xff},
		"limit plus one": frame(strings.Repeat("a", 101)),
	} {
		if _, err := ReadFrame(bytes.NewReader(in), map[bool]int{true: 100, false: MaxRequest}[name == "limit plus one"]); err == nil {
			t.Errorf("%s: taken", name)
		}
	}
	var out bytes.Buffer
	if err := WriteFrame(&out, []byte(`{"ok":true}`)); err != nil || !bytes.Equal(out.Bytes(), frame(`{"ok":true}`)) {
		t.Errorf("write: %v %q", err, out.Bytes())
	}
	if WriteFrame(&out, nil) == nil || WriteFrame(&out, bytes.Repeat([]byte("a"), MaxReply+1)) == nil {
		t.Error("an empty or a big frame was written")
	}
}

func TestParseRequest(t *testing.T) {
	good := map[string]struct {
		host   string
		family int
	}{
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example"}`:                       {"server.example", 0},
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example:8443","family":4}`:       {"server.example", 4},
		`{"v":1,"id":"a-b_C9","type":"trace","origin":"https://xn--e1afmkfd.xn--p1ai","family":6}`: {"xn--e1afmkfd.xn--p1ai", 6},
		`{"v":1,"id":"a1","type":"trace","origin":"https://8.8.8.8","family":0}`:                   {"8.8.8.8", 0},
		` {"v":1,"id":"a1","type":"trace","origin":"https://a.b"} `:                                {"a.b", 0},
	}
	for in, want := range good {
		_, host, family, err := ParseRequest([]byte(in))
		if err != nil || host != want.host || family != want.family {
			t.Errorf("%s: %v %q %d", in, err, host, family)
		}
	}
	for _, in := range []string{
		`{"v":1,"id":"a1","type":"hello"}`,
		`{"v":1,"id":"a1","type":"cancel"}`,
	} {
		if _, _, _, err := ParseRequest([]byte(in)); err != nil {
			t.Errorf("%s: %v", in, err)
		}
	}
	bad := []string{
		``, `{}`, `[]`, `null`, `"x"`, `{"v":1`,
		`{"v":2,"id":"a1","type":"hello"}`, `{"v":"1","id":"a1","type":"hello"}`, `{"v":1.0,"id":"a1","type":"hello"}`, `{"id":"a1","type":"hello"}`,
		`{"v":1,"id":"","type":"hello"}`, `{"v":1,"id":"a b","type":"hello"}`, `{"v":1,"id":"` + strings.Repeat("a", 33) + `","type":"hello"}`,
		`{"v":1,"id":"a1","type":"bye"}`, `{"v":1,"id":"a1","type":"Hello"}`, `{"v":1,"id":"a1"}`,
		`{"v":1,"id":"a1","type":"hello","extra":1}`, `{"v":1,"id":"a1","type":"hello","origin":"https://a.b"}`, `{"v":1,"id":"a1","type":"cancel","family":4}`,
		`{"v":1,"id":"a1","type":"hello"}{"v":1}`, `{"v":1,"id":"a1","type":"hello"} x`, `{"v":1,"id":"a1","type":"hello"},`,
		`{"v":1,"id":"a1","type":"trace"}`, `{"v":1,"id":"a1","type":"trace","origin":""}`,
		`{"v":1,"id":"a1","type":"trace","origin":"http://server.example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example/"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example/path"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example?x=1"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example#x"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://user@server.example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://user:pw@server.example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://Server.Example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://sérver.example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://[2001:db8::1]"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example:"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example:99999999"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://-server.example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server..example"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example\n"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","family":"4"}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","family":5}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","family":4.0}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","family":null}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","family":-4}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","timeout":5}`,
		`{"v":1,"id":"a1","type":"trace","origin":"https://` + strings.Repeat("a", 64) + `.example"}`,
	}
	for _, in := range bad {
		if _, _, _, err := ParseRequest([]byte(in)); err == nil {
			t.Errorf("taken: %s", in)
		}
	}
}

func TestCheckCaller(t *testing.T) {
	chrome, gecko := []string{chromeID}, []string{"librespeed-route@hyakov3.github.io"}
	manifest := "/home/u/.mozilla/native-messaging-hosts/" + HostName + ".json"
	ok := []struct {
		goos string
		args []string
	}{
		{"linux", chromeArgs},
		{"windows", chromeArgs},
		{"windows", []string{chromeArgs[0], "--parent-window=1234"}},
		{"linux", []string{manifest, gecko[0]}},
		{"windows", []string{`C:\Users\u\AppData\Local\librespeed-route\` + HostName + ".json", gecko[0]}},
	}
	for _, c := range ok {
		if !CheckCaller(c.args, c.goos, chrome, gecko) {
			t.Errorf("refused: %s %v", c.goos, c.args)
		}
	}
	other := "chrome-extension://" + strings.Repeat("a", 32) + "/"
	for _, c := range []struct {
		goos string
		args []string
	}{
		{"linux", nil},
		{"linux", []string{other}},
		{"linux", []string{"chrome-extension://" + chromeID}},
		{"linux", []string{"chrome-extension://" + chromeID + "/x"}},
		{"linux", []string{"CHROME-EXTENSION://" + chromeID + "/"}},
		{"linux", []string{chromeArgs[0], "extra"}},
		{"linux", []string{chromeArgs[0], "--parent-window=1234"}},
		{"windows", []string{chromeArgs[0], "--parent-window=x"}},
		{"windows", []string{chromeArgs[0], "--parent-window=1", "x"}},
		{"windows", []string{chromeArgs[0], "--other"}},
		{"linux", []string{gecko[0], manifest}},
		{"linux", []string{manifest, "other@example.org"}},
		{"linux", []string{"relative/" + HostName + ".json", gecko[0]}},
		{"linux", []string{"/x/other.json", gecko[0]}},
		{"linux", []string{manifest}},
		{"linux", []string{manifest, gecko[0], "x"}},
		{"linux", []string{"8.8.8.8"}},
		{"windows", []string{"/x/" + HostName + ".json", gecko[0]}},
	} {
		if CheckCaller(c.args, c.goos, chrome, gecko) {
			t.Errorf("taken: %s %v", c.goos, c.args)
		}
	}
	// Which arguments make the program the host: a browser's, and nothing else
	if !LooksLikeABrowserCall(chromeArgs, gecko) || !LooksLikeABrowserCall([]string{other}, gecko) || !LooksLikeABrowserCall([]string{manifest, gecko[0]}, gecko) {
		t.Error("a browser call was not seen")
	}
	for _, args := range [][]string{nil, {"8.8.8.8"}, {"--json", "example.org"}, {manifest}, {"x.json", "y"}} {
		if LooksLikeABrowserCall(args, gecko) {
			t.Errorf("%v is not a browser call", args)
		}
	}
}

type mustNotRead struct{ t *testing.T }

func (m mustNotRead) Read([]byte) (int, error) {
	m.t.Error("the host read its input")
	return 0, io.EOF
}

func TestACallerThatIsNotOursGetsNothing(t *testing.T) {
	d := deps(t, route.Output{}, nil)
	var out bytes.Buffer
	code := Main(context.Background(), []string{"chrome-extension://" + strings.Repeat("b", 32) + "/"}, mustNotRead{t}, &out, d)
	if code != 1 || out.Len() != 0 {
		t.Errorf("%d %q", code, out.Bytes())
	}
}

func TestHelloAndTrace(t *testing.T) {
	d := deps(t, route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}, nil)
	code, out := run(t, d, chromeArgs, request("hello", ""))
	got := replies(t, out)
	if code != 0 || len(got) != 1 || got[0]["type"] != "hello" || got[0]["protocol"] != float64(1) || got[0]["version"] != "0.1.0" || got[0]["tool"] != "traceroute" || got[0]["id"] != "a1" {
		t.Fatalf("hello: %d %v", code, got)
	}
	code, out = run(t, d, chromeArgs, request("trace", "https://server.example"))
	got = replies(t, out)
	if code != 0 || len(got) != 1 || got[0]["type"] != "result" || got[0]["id"] != "a1" {
		t.Fatalf("trace: %d %v", code, got)
	}
	lsr1, _ := got[0]["lsr1"].(string)
	if !strings.HasPrefix(lsr1, "LSR1!n=0123456789abcdef!f=4!t=8.8.8.8!") || !strings.Contains(lsr1, "!a=0.1.0+traceroute!") {
		t.Errorf("lsr1: %s", lsr1)
	}
	// The keys of the reply are the fixed ones
	if len(got[0]) != 4 {
		t.Errorf("keys: %v", got[0])
	}
	// No tool: hello says so
	d.Exists = func(string) bool { return false }
	_, out = run(t, d, chromeArgs, request("hello", ""))
	if replies(t, out)[0]["tool"] != "none" {
		t.Errorf("no tool: %s", out)
	}
}

func TestErrorsAreOfAClosedSet(t *testing.T) {
	good := route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}
	allSilent := route.Output{Text: vector(t, "windows-en-real-allsilent-ci.txt")}
	long := func() route.Output {
		var b strings.Builder
		for i := 1; i <= 40; i++ {
			b.WriteString(strings.Repeat(" ", 0) + itoa(i) + "  2a01:4f8:aaaa:bbbb:cccc:dddd:eeee:ffff  12345.67 ms  12345.67 ms  12345.67 ms\n")
		}
		return route.Output{Text: b.String()}
	}()
	cases := []struct {
		name string
		d    func() Deps
		req  []byte
		code string
	}{
		{"an internal address", func() Deps { return deps(t, good, nil) }, request("trace", "https://inside.example"), CodeRefused},
		{"an unknown name", func() Deps { return deps(t, good, nil) }, request("trace", "https://nowhere.example"), CodeRefused},
		{"a loopback", func() Deps { return deps(t, good, nil) }, request("trace", "https://127.0.0.1"), CodeRefused},
		{"no tool", func() Deps { d := deps(t, good, nil); d.Exists = func(string) bool { return false }; return d }, request("trace", "https://server.example"), CodeNoTool},
		{"the tool failed", func() Deps { return deps(t, route.Output{}, route.ErrToolFailed) }, request("trace", "https://server.example"), CodeFailed},
		{"output that is not a route", func() Deps { return deps(t, route.Output{Text: "hello"}, nil) }, request("trace", "https://server.example"), CodeFailed},
		{"every hop silent", func() Deps { return deps(t, allSilent, nil) }, request("trace", "https://server.example"), CodeSilent},
		{"too long", func() Deps {
			d := deps(t, long, nil)
			d.Resolver = resolver{"server.example": {netip.MustParseAddr("2001:4860:4860::8888")}}
			return d
		}, request("trace", "https://server.example"), CodeTooLong},
		{"a cancel before any trace", func() Deps { return deps(t, good, nil) }, frame(`{"v":1,"id":"a1","type":"cancel"}`), CodeBadRequest},
	}
	for _, c := range cases {
		code, out := run(t, c.d(), chromeArgs, c.req)
		got := replies(t, out)
		if code != 0 || len(got) != 1 || got[0]["type"] != "error" || got[0]["code"] != c.code || len(got[0]) != 4 {
			t.Errorf("%s: %d %v", c.name, code, got)
		}
	}
	// A request that is not a request: an answer of "bad-request" and the exit code 1
	for name, in := range map[string][]byte{
		"not json": frame("hello"), "empty frame": frame(""), "big": frame(strings.Repeat("a", MaxRequest+1)), "nothing": nil,
		"family as a string": frame(`{"v":1,"id":"a1","type":"trace","origin":"https://server.example","family":"4"}`),
	} {
		d := deps(t, good, nil)
		d.RequestTimeout = 300 * time.Millisecond
		code, out := run(t, d, chromeArgs, in)
		got := replies(t, out)
		if code != 1 || len(got) != 1 || got[0]["code"] != CodeBadRequest {
			t.Errorf("%s: %d %v", name, code, got)
		}
	}
}

func itoa(i int) string {
	if i < 10 {
		return " " + string(rune('0'+i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// What goes out is only frames of the closed set, whatever the output of the tool is: no address of an internal network, no hop, nothing on
// the error output (the host has none to write to).
func TestNoInternalAddressEverLeavesTheHost(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "testdata", "vectors", "*.txt"))
	if len(files) < 15 {
		t.Fatalf("%d vectors", len(files))
	}
	for _, f := range files {
		data, _ := os.ReadFile(f)
		res, perr := route.Parse(string(data))
		d := deps(t, route.Output{Text: string(data)}, nil)
		d.Resolver = resolver{"server.example": {netip.MustParseAddr("8.8.8.8")}}
		_, out := run(t, d, chromeArgs, request("trace", "https://server.example"))
		got := replies(t, out)
		if len(got) != 1 || len(out) > MaxReply+4 {
			t.Fatalf("%s: %d frames, %d bytes", f, len(got), len(out))
		}
		for k := range got[0] {
			if k != "v" && k != "id" && k != "type" && k != "lsr1" && k != "code" {
				t.Errorf("%s: a key that is not of the set: %s", f, k)
			}
		}
		if perr != nil {
			continue
		}
		for _, h := range res.Hops {
			ips := append([]string{}, h.AltIPs...)
			if h.IP != nil {
				ips = append(ips, *h.IP)
			}
			for _, ip := range ips {
				if route.ScopeOf(ip) != route.ScopePublic && bytes.Contains(out, []byte(ip)) {
					t.Errorf("%s: the internal address %s is in what the host wrote", f, ip)
				}
			}
		}
	}
}

// The browser closes the connection (or sends anything) while the tool runs: the tool is stopped and nothing is left behind.
func TestTheTraceIsStoppedWhenTheBrowserLetsGo(t *testing.T) {
	d := deps(t, route.Output{}, nil)
	stopped := make(chan struct{})
	d.Run = func(ctx context.Context, _ route.Spec, _ route.Options) (route.Output, error) {
		<-ctx.Done()
		close(stopped)
		return route.Output{}, ctx.Err()
	}
	pr, pw := io.Pipe()
	var out bytes.Buffer
	done := make(chan int)
	go func() { done <- Main(context.Background(), chromeArgs, pr, &out, d) }()
	if _, err := pw.Write(request("trace", "https://server.example")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	_ = pw.Close()
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("the tool was not stopped")
	}
	select {
	case code := <-done:
		got := replies(t, out.Bytes())
		if code != 0 || len(got) != 1 || got[0]["code"] != CodeInterrupted {
			t.Errorf("%d %v", code, got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the host did not end")
	}
	// A cancel frame does the same
	pr, pw = io.Pipe()
	out.Reset()
	d.LockDir = t.TempDir()
	stopped = make(chan struct{})
	go func() { done <- Main(context.Background(), chromeArgs, pr, &out, d) }()
	_, _ = pw.Write(request("trace", "https://server.example"))
	time.Sleep(200 * time.Millisecond)
	_, _ = pw.Write(frame(`{"v":1,"id":"a1","type":"cancel"}`))
	select {
	case <-stopped:
	case <-time.After(3 * time.Second):
		t.Fatal("the cancel did not stop the tool")
	}
	<-done
	_ = pw.Close()
}

func TestOneTraceAtATime(t *testing.T) {
	dir := t.TempDir()
	release, ok := acquire(dir, time.Now())
	if !ok {
		t.Fatal("the first lock was refused")
	}
	if _, again := acquire(dir, time.Now()); again {
		t.Error("a second lock was given while the first one is held")
	}
	// Through the host: busy
	d := deps(t, route.Output{Text: vector(t, "linux-traceroute-v4-cgnat.txt")}, nil)
	d.LockDir = dir
	_, out := run(t, d, chromeArgs, request("trace", "https://server.example"))
	if got := replies(t, out); got[0]["code"] != CodeBusy {
		t.Errorf("%v", got)
	}
	release()
	// Just after one: the gap
	if _, again := acquire(dir, time.Now()); again {
		t.Error("a trace was allowed right after another one")
	}
	old := time.Now().Add(-MinGap - time.Second)
	if err := os.Chtimes(filepath.Join(dir, "trace.last"), old, old); err != nil {
		t.Fatal(err)
	}
	release2, ok := acquire(dir, time.Now())
	if !ok {
		t.Fatal("a trace was refused after the gap")
	}
	release2()
	// A lock that was left over is taken away after its time
	if err := os.Chtimes(filepath.Join(dir, "trace.last"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "trace.lock"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, again := acquire(dir, time.Now()); again {
		t.Error("a fresh lock was taken away")
	}
	stale := time.Now().Add(-StaleLock - time.Second)
	if err := os.Chtimes(filepath.Join(dir, "trace.lock"), stale, stale); err != nil {
		t.Fatal(err)
	}
	if release3, ok := acquire(dir, time.Now()); !ok {
		t.Error("a left over lock was not taken away")
	} else {
		release3()
	}
	// No name or address is kept in the folder
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		if len(data) != 0 {
			t.Errorf("%s has %d bytes", e.Name(), len(data))
		}
	}
}
