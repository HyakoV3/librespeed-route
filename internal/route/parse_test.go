package route

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const vectors = "../../testdata/vectors"

func generic(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Every output of the vectors gives what the page gives for it (the .expected.json files are the answers of route-parse.js, checked by
// hand against the text in HyakoV3/speedtest).
func TestVectorsEqualThePage(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join(vectors, "*.txt"))
	if len(files) < 15 {
		t.Fatalf("only %d vectors", len(files))
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".txt")
		t.Run(name, func(t *testing.T) {
			text, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(f, ".txt") + ".expected.json")
			if err != nil {
				t.Fatal(err)
			}
			got, perr := Parse(string(text))
			if m, isMap := generic(t, want).(map[string]any); isMap && m["error"] != nil {
				// The page says the text is not a route (every hop is silent): this port says the same
				if perr == nil || perr.Error() != m["error"] {
					t.Fatalf("%s: got %v, the page says %v", name, perr, m["error"])
				}
				return
			}
			if perr != nil {
				t.Fatalf("%s: %v", name, perr)
			}
			encoded, _ := json.Marshal(got)
			if !reflect.DeepEqual(generic(t, encoded), generic(t, want)) {
				t.Errorf("%s: differs from the page\n got: %s\nwant: %s", name, encoded, want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]ParseError{
		"":                             ErrEmpty,
		"   \n  ":                      ErrEmpty,
		"hello, this is not a route":   ErrUnknown,
		strings.Repeat("a", maxText+1): ErrTooLong,
		" 1  ::  \n":                   ErrUnknown,
		"LSR1!n=0123456789abcdef!f=4":  ErrUnsupported,
		"  1.|-- 192.168.0.1   0.0%   3   1.1   3.0   1.0   6.8   3.3": ErrUnsupported,
	}
	for text, want := range cases {
		_, err := Parse(text)
		if err != want {
			t.Errorf("%q: got %v, want %v", text[:min(len(text), 30)], err, want)
		}
	}
}
