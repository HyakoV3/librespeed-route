package route

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustParse(t *testing.T, name string) *Result {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(vectors, name))
	if err != nil {
		t.Fatal(err)
	}
	res, perr := Parse(string(data))
	if perr != nil {
		t.Fatal(perr)
	}
	return res
}

const nonce = "0123456789abcdef"

func TestEncodeLSR1Form(t *testing.T) {
	res := mustParse(t, "linux-traceroute-v4-cgnat.txt")
	text, err := EncodeLSR1(res, "8.8.8.8", nonce, "0.1.0+traceroute", 1790000000)
	if err != nil {
		t.Fatal(err)
	}
	want := "LSR1!n=0123456789abcdef!f=4!t=8.8.8.8!s=1790000000!a=0.1.0+traceroute!h=" +
		"1,-p,1.2,3,3;2,-p,5.23,3,3;3,-p,6.27,3,3;4,-p,6.61,3,3;5,-p,5.24,3,3;6,-c,4.77,3,3;7,-c,12.13,3,3;" +
		"8,-c,17.86,3,3;9,-c,18.43,3,3;10,72.14.208.80,18.6,3,3;11,*,0,3,0;12,8.8.8.8,16.8,3,3"
	if text != want {
		t.Errorf("got:  %s\nwant: %s", text, want)
	}
	// IPv6, with the other routers of a hop
	v6 := mustParse(t, "linux-traceroute-v6-real.txt")
	text, err = EncodeLSR1(v6, "2001:4860:4860::8888", nonce, "0.1.0+traceroute", 1790000000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "!f=6!t=2001:4860:4860::8888!") || !strings.Contains(text, ";2,2602:fdb8:132:101::1,0.2,3,3,2602:fdb8:1f3:200::1;") {
		t.Errorf("v6 text: %s", text)
	}
}

func TestEncodeLSR1Refuses(t *testing.T) {
	res := mustParse(t, "linux-traceroute-v4-cgnat.txt")
	for _, target := range []string{"192.168.0.1", "100.64.0.1", "127.0.0.1", "nope", "", "fe80::1", "2001:db8::1"} {
		if _, err := EncodeLSR1(res, target, nonce, "0.1.0", 1); err != ErrLSR1Target {
			t.Errorf("target %q: %v", target, err)
		}
	}
	for _, bad := range []struct{ nonce, version string }{{"xyz", "0.1.0"}, {"0123456789ABCDEF", "0.1.0"}, {nonce, ""}, {nonce, "0123456789.123456"}, {nonce, "a b"}} {
		if _, err := EncodeLSR1(res, "8.8.8.8", bad.nonce, bad.version, 1); err == nil {
			t.Errorf("%+v was taken", bad)
		}
	}
	if _, err := EncodeLSR1(nil, "8.8.8.8", nonce, "0.1.0", 1); err == nil {
		t.Error("no route was taken")
	}
	// The limit is the one of the page: with the other routers of the hops left out first, then it is refused
	var hops []Hop
	for i := 1; i <= 64; i++ {
		ip := "2a01:4f8:ffff:ffff:ffff:ffff:ffff:ffff"
		alt := "2a01:4f8:eeee:eeee:eeee:eeee:eeee:eeee"
		rtt := 12345.67
		hops = append(hops, Hop{TTL: i, IP: &ip, AltIPs: []string{alt}, RTTMin: &rtt, RTTAvg: &rtt, Sent: 3, Answered: 3, Scope: ScopePublic})
	}
	if _, err := EncodeLSR1(&Result{Hops: hops}, "2001:4860:4860::8888", nonce, "0.1.0", 1); err != ErrLSR1TooLong {
		t.Errorf("64 long hops: %v", err)
	}
	few := &Result{Hops: hops[:20]}
	text, err := EncodeLSR1(few, "2001:4860:4860::8888", nonce, "0.1.0", 1)
	if err != nil || len(text) > MaxLSR1 {
		t.Errorf("20 hops: %v (%d)", err, len(text))
	}
}

// No address of an internal network is ever in the text, whatever the route is.
func TestNoInternalAddressInTheText(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for i := 0; i < 500; i++ {
		res, err := Parse(randomOutput(r))
		if err != nil {
			continue
		}
		text, err := EncodeLSR1(res, "8.8.8.8", nonce, "0.1.0", 1790000000)
		if err != nil {
			continue
		}
		hopsPart := text[strings.Index(text, "!h=")+3:]
		seen := map[string]bool{}
		for _, item := range strings.Split(hopsPart, ";") {
			for _, cell := range strings.Split(item, ",")[1:] {
				seen[cell] = true
			}
		}
		for _, h := range res.Hops {
			all := append([]string{}, h.AltIPs...)
			if h.IP != nil {
				all = append(all, *h.IP)
			}
			for _, ip := range all {
				if ScopeOf(ip) != ScopePublic && seen[ip] {
					t.Fatalf("the internal address %s is in %s", ip, text)
				}
			}
		}
	}
}

// What the page reads from the text is the route with the internal addresses taken out (and every time as the best one).
func TestThePageReadsTheText(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	var texts []string
	var routes []*Result
	files, _ := filepath.Glob(filepath.Join(vectors, "*.txt"))
	r := rand.New(rand.NewSource(11))
	var inputs []string
	for _, f := range files {
		data, _ := os.ReadFile(f)
		inputs = append(inputs, string(data))
	}
	for i := 0; i < 300; i++ {
		inputs = append(inputs, randomOutput(r))
	}
	for _, in := range inputs {
		res, err := Parse(in)
		if err != nil {
			continue
		}
		target := "8.8.8.8"
		if res.Family == 6 {
			target = "2001:4860:4860::8888"
		}
		text, err := EncodeLSR1(res, target, nonce, "0.1.0+tracert", 1790000000)
		if err != nil {
			continue
		}
		texts = append(texts, text)
		routes = append(routes, res)
	}
	if len(texts) < 100 {
		t.Fatalf("only %d texts", len(texts))
	}
	input, _ := json.Marshal(texts)
	cmd := exec.Command(node, filepath.Join(web, "diff.js"))
	cmd.Stdin = bytes.NewReader(input)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	var page []Result
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	for i, res := range routes {
		got := page[i]
		if got.Tool != "lsr1" || (got.Nonce == nil || *got.Nonce != nonce) {
			t.Fatalf("text %d: the page did not take it: %s", i, texts[i])
		}
		// The page leaves out the hops with no answer that come after its last public hop (it cuts the end of the route before the
		// internal hops are put back: a quirk of route-parse.js that costs nothing, as such a hop has nothing to say)
		lastPublic := 0
		for _, h := range res.Hops {
			if h.Scope == ScopePublic {
				lastPublic = h.TTL
			}
		}
		var wantHops []Hop
		for _, h := range res.Hops {
			if h.Scope == ScopeNone && h.TTL > lastPublic {
				continue
			}
			wantHops = append(wantHops, h)
		}
		if len(got.Hops) != len(wantHops) {
			t.Fatalf("text %d: %d hops, want %d\n%s", i, len(got.Hops), len(wantHops), texts[i])
		}
		for k, want := range wantHops {
			h := got.Hops[k]
			if h.TTL != want.TTL || h.Scope != want.Scope || h.Sent != want.Sent || h.Answered != want.Answered {
				t.Fatalf("text %d hop %d: got %+v, want %+v\n%s", i, k, h, want, texts[i])
			}
			if want.Scope == ScopePublic {
				if h.IP == nil || *h.IP != *want.IP {
					t.Fatalf("text %d hop %d: address %v, want %s", i, k, h.IP, *want.IP)
				}
			} else if h.IP != nil {
				t.Fatalf("text %d hop %d: the page got an address of the internal network: %s", i, k, *h.IP)
			}
			if want.RTTMin != nil && (h.RTTMin == nil || *h.RTTMin != *want.RTTMin) {
				t.Fatalf("text %d hop %d: time %v, want %v", i, k, h.RTTMin, *want.RTTMin)
			}
		}
	}
	_ = reflect.DeepEqual
}

// The limit of the page is 1900 characters, counted in the text: 1900 is taken and 1901 is not
func TestTheLimitIsExact(t *testing.T) {
	build := func(extra int) *Result {
		var hops []Hop
		for i := 1; i <= 35; i++ {
			ip := "2a01:4f8:ffff:ffff:ffff:ffff:ffff:ffff"
			rtt := 0.5
			hops = append(hops, Hop{TTL: i, IP: &ip, AltIPs: []string{}, RTTMin: &rtt, RTTAvg: &rtt, Sent: 3, Answered: 3, Scope: ScopePublic})
		}
		// Each of the first hops can grow by up to 5 characters ("0.5" -> "12345.67")
		for i := 0; i < len(hops) && extra > 0; i++ {
			grow := min(extra, 5)
			values := map[int]float64{0: 0.5, 1: 12.5, 2: 123.5, 3: 1234.5, 4: 12345.5, 5: 12345.67}
			v := values[grow]
			hops[i].RTTMin, hops[i].RTTAvg = &v, &v
			extra -= grow
		}
		return &Result{Hops: hops}
	}
	var sizes []int
	for extra := 0; extra <= 175; extra++ {
		text, err := EncodeLSR1(build(extra), "2001:4860:4860::8888", nonce, "0.1.0", 1790000000)
		if err == nil {
			sizes = append(sizes, len(text))
		} else if err != ErrLSR1TooLong {
			t.Fatal(err)
		}
	}
	if len(sizes) == 0 {
		t.Fatal("nothing fits")
	}
	biggest := 0
	for _, n := range sizes {
		if n > MaxLSR1 {
			t.Fatalf("a text of %d characters was taken", n)
		}
		biggest = max(biggest, n)
	}
	if biggest != MaxLSR1 {
		t.Fatalf("the biggest text that was taken has %d characters, want exactly %d", biggest, MaxLSR1)
	}
}
