package route

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const web = "../../testdata/web"

// The scopes that the PHP backend gives (tests/route_parity.php of HyakoV3/speedtest) are the ones of this port.
func TestScopesEqualThePHP(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(web, "route-parity.json"))
	if err != nil {
		t.Fatal(err)
	}
	var parity struct {
		Scopes map[string]string `json:"scopes"`
	}
	if err := json.Unmarshal(data, &parity); err != nil {
		t.Fatal(err)
	}
	if len(parity.Scopes) < 50 {
		t.Fatalf("only %d addresses", len(parity.Scopes))
	}
	for ip, want := range parity.Scopes {
		if got := ScopeOf(ip); got != want {
			t.Errorf("%s: got %s, want %s", ip, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4":                     "1.2.3.4",
		"010.0.0.1":                   "",
		"256.1.1.1":                   "",
		"2001:4860:4860:0:0:0:0:8888": "2001:4860:4860::8888",
		"2001:DB8::1":                 "2001:db8::1",
		"::ffff:1.2.3.4":              "::ffff:102:304",
		"::":                          "::",
		"::1":                         "::1",
		"2400:cb00:1292:3::":          "2400:cb00:1292:3::",
		"1:0:0:2:0:0:3:4":             "1::2:0:0:3:4",
		"1:2:3:4:5:6:7:0":             "1:2:3:4:5:6:7:0",
		"fe80::1%en0":                 "",
		"not-an-address":              "",
		"1.2.3":                       "",
		"2001:db8:::1":                "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func randomAddr(r *rand.Rand) string {
	switch r.Intn(8) {
	case 0:
		return fmt.Sprintf("192.168.%d.%d", r.Intn(256), 1+r.Intn(254))
	case 1:
		return fmt.Sprintf("100.%d.%d.%d", 64+r.Intn(64), r.Intn(256), 1+r.Intn(254))
	case 2:
		return fmt.Sprintf("10.%d.%d.%d", r.Intn(256), r.Intn(256), 1+r.Intn(254))
	case 3, 4:
		return fmt.Sprintf("%d.%d.%d.%d", 1+r.Intn(223), r.Intn(256), r.Intn(256), 1+r.Intn(254))
	case 5:
		return fmt.Sprintf("2%03x:%x:%x::%x", r.Intn(0x400), r.Intn(0x10000), r.Intn(0x10000), r.Intn(0x10000))
	case 6:
		return fmt.Sprintf("2a01:4f8:%X:%x:0:0:0:%x", r.Intn(0x10000), r.Intn(0x10000), r.Intn(0x10000))
	default:
		return []string{"fe80::1%eth0", "fd00::5", "2001:db8::7", "::ffff:8.8.8.8", "2400:cb00:1292:3::"}[r.Intn(5)]
	}
}

func randomTime(r *rand.Rand) float64 {
	return float64(r.Intn(30000)) / 100
}

// Outputs of the three tools made at random (the same shapes as the real ones), to compare this port with the page
func randomOutput(r *rand.Rand) string {
	var b strings.Builder
	hops := 1 + r.Intn(20)
	tool := r.Intn(3)
	target := randomAddr(r)
	switch tool {
	case 0:
		fmt.Fprintf(&b, "traceroute to %s (%s), 30 hops max, 60 byte packets\n", target, target)
	case 1:
		fmt.Fprintf(&b, "\r\nTracing route to %s over a maximum of 30 hops\r\n\r\n", target)
	default:
		fmt.Fprintf(&b, " 1?: [LOCALHOST]                      pmtu 1500\n")
	}
	for i := 1; i <= hops; i++ {
		switch tool {
		case 0:
			fmt.Fprintf(&b, "%2d ", i)
			n := 1 + r.Intn(3)
			silent := r.Intn(6) == 0
			for p := 0; p < 3; p++ {
				if silent || r.Intn(8) == 0 {
					b.WriteString(" *")
					continue
				}
				if p == 0 || (n > 1 && r.Intn(3) == 0) {
					fmt.Fprintf(&b, "  %s", randomAddr(r))
				}
				fmt.Fprintf(&b, "  %.3f ms", randomTime(r))
			}
			if r.Intn(10) == 0 {
				b.WriteString("  !H")
			}
			b.WriteString("\n")
		case 1:
			fmt.Fprintf(&b, "%3d ", i)
			silent := r.Intn(6) == 0
			for p := 0; p < 3; p++ {
				if silent || r.Intn(8) == 0 {
					b.WriteString("    *    ")
				} else if r.Intn(3) == 0 {
					b.WriteString("   <1 ms ")
				} else {
					fmt.Fprintf(&b, "  %3d ms ", r.Intn(400))
				}
			}
			if silent {
				b.WriteString("  Request timed out.\r\n")
			} else {
				fmt.Fprintf(&b, "  %s \r\n", randomAddr(r))
			}
		default:
			if r.Intn(6) == 0 {
				fmt.Fprintf(&b, "%2d:  no reply\n", i)
			} else {
				fmt.Fprintf(&b, "%2d:  %s   %.3fms asymm %d\n", i, randomAddr(r), randomTime(r), r.Intn(9))
			}
		}
	}
	if tool == 2 && r.Intn(2) == 0 {
		fmt.Fprintf(&b, "     Resume: pmtu 1500 hops %d back %d\n", hops, hops)
	}
	return b.String()
}

// This port and the page (route-parse.js, run by Node) give the same for every vector and for 2000 outputs made at random.
func TestPortEqualsThePageOnRandomOutputs(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed: the comparison with the page is not run")
	}
	var texts []string
	files, _ := filepath.Glob(filepath.Join(vectors, "*.txt"))
	for _, f := range files {
		data, _ := os.ReadFile(f)
		texts = append(texts, string(data))
	}
	r := rand.New(rand.NewSource(20261011))
	for i := 0; i < 2000; i++ {
		texts = append(texts, randomOutput(r))
	}
	input, _ := json.Marshal(texts)
	cmd := exec.Command(node, filepath.Join(web, "diff.js"))
	cmd.Stdin = bytes.NewReader(input)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("node: %v: %s", err, stderr.String())
	}
	var page []any
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page) != len(texts) {
		t.Fatalf("%d answers for %d texts", len(page), len(texts))
	}
	different := 0
	for i, text := range texts {
		var mine any
		res, perr := Parse(text)
		if perr != nil {
			mine = map[string]any{"error": perr.Error()}
		} else {
			encoded, _ := json.Marshal(res)
			_ = json.Unmarshal(encoded, &mine)
		}
		if !reflect.DeepEqual(mine, page[i]) {
			different++
			if different <= 3 {
				a, _ := json.Marshal(mine)
				b, _ := json.Marshal(page[i])
				t.Errorf("text %d differs from the page:\n%s\n got: %s\nwant: %s", i, text, a, b)
			}
		}
	}
	if different > 0 {
		t.Errorf("%d of %d texts differ from the page", different, len(texts))
	}
}
