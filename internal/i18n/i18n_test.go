package i18n

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every key is in the four languages, and nowhere is there a key that the others have not
func TestTheFourLanguagesHaveTheSameKeys(t *testing.T) {
	keys := func(m map[string]string) []string {
		var k []string
		for key := range m {
			k = append(k, key)
		}
		sort.Strings(k)
		return k
	}
	want := keys(en)
	if len(want) < 40 {
		t.Fatalf("only %d keys", len(want))
	}
	for _, l := range All {
		got := keys(catalogs[l])
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s: keys differ from English: %v", l, diff(want, got))
		}
		for key, text := range catalogs[l] {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s: %s is empty", l, key)
			}
		}
	}
}

func diff(a, b []string) []string {
	in := map[string]bool{}
	for _, x := range a {
		in[x] = true
	}
	var out []string
	for _, x := range b {
		if !in[x] {
			out = append(out, "+"+x)
		}
		delete(in, x)
	}
	for x := range in {
		out = append(out, "-"+x)
	}
	return out
}

var verb = regexp.MustCompile(`%(\[\d+\])?[-#+ 0-9.]*[a-zA-Z%]`)

// The arguments of a text are the same in every language (the verbs name them by number: a language may put them in another order), and a
// text that has none has no verb at all (it is given as it is, so a % in it is a % and not the start of one)
func TestTheArgumentsAreTheSameInEveryLanguage(t *testing.T) {
	args := func(text string) string {
		var found []string
		for _, m := range verb.FindAllString(text, -1) {
			if m == "%%" {
				continue
			}
			found = append(found, regexp.MustCompile(`\[\d+\]`).FindString(m))
		}
		sort.Strings(found)
		return strings.Join(found, "")
	}
	for key, text := range en {
		want := args(text)
		hasArgs := want != ""
		for _, l := range All {
			got := args(catalogs[l][key])
			if got != want {
				t.Errorf("%s %s: arguments %q, English %q", l, key, got, want)
			}
			if strings.Contains(want, "[]") || (hasArgs && strings.Contains(catalogs[l][key], "%s") && !strings.Contains(catalogs[l][key], "%[")) {
				t.Errorf("%s %s: a verb with no number", l, key)
			}
		}
		// A text with arguments never has a bare "%" that is not a verb (it would be read as the start of one)
		if hasArgs {
			for _, l := range All {
				if strings.Count(catalogs[l][key], "%")-strings.Count(catalogs[l][key], "%%")*2 != strings.Count(catalogs[l][key], "%[") {
					t.Errorf("%s %s: a lone %%", l, key)
				}
			}
		}
	}
}

// The things that a person must be able to find in every language: the exit codes and the options
func TestTheHelpSaysTheSameInEveryLanguage(t *testing.T) {
	for _, l := range All {
		usage := l.T("usage")
		for _, must := range []string{"-4, -6", "--json", "--quiet", "--timeout", "--lang", "--version", "install-native-host", "uninstall-native-host", "doctor", "LSR1!", "| clip", "130", "tracepath"} {
			if !strings.Contains(usage, must) {
				t.Errorf("%s: the help does not have %q", l, must)
			}
		}
		for _, code := range []string{"0 ", "1 ", "2 ", "3 ", "4 ", "130 "} {
			if !strings.Contains(usage, code) {
				t.Errorf("%s: no exit code %q", l, code)
			}
		}
		menu := l.T("win.menu")
		for _, must := range []string{"[I]", "[U]", "[Q]", "[I/U/Q]", "%LOCALAPPDATA%", "librespeed-route.exe"} {
			if !strings.Contains(menu, must) {
				t.Errorf("%s: the menu does not have %q", l, must)
			}
		}
	}
}

func TestParseAndDetect(t *testing.T) {
	for in, want := range map[string]Lang{"pt": Pt, "pt-BR": Pt, "pt_BR.UTF-8": Pt, "PT_br": Pt, "es": Es, "es_ES@euro": Es, "sv": Sv, "sv_SE.UTF-8": Sv, "en": En, "en_US.UTF-8": En} {
		if got, ok := Parse(in); !ok || got != want {
			t.Errorf("Parse(%q) = %v %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "p", "fr", "fr_FR.UTF-8", "pol", "portuguese", "spanish", "sve", "C", "POSIX"} {
		if _, ok := Parse(in); ok && in != "C" && in != "POSIX" {
			t.Errorf("Parse(%q) was taken", in)
		}
	}
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		name   string
		env    map[string]string
		locale string
		want   Lang
	}{
		{"nothing", nil, "", En},
		{"LANG", map[string]string{"LANG": "pt_BR.UTF-8"}, "", Pt},
		{"LC_ALL wins over LANG", map[string]string{"LC_ALL": "sv_SE.UTF-8", "LANG": "pt_BR.UTF-8"}, "", Sv},
		{"LC_MESSAGES wins over LANG", map[string]string{"LC_MESSAGES": "es_ES.UTF-8", "LANG": "pt_BR.UTF-8"}, "", Es},
		{"LANGUAGE first entry", map[string]string{"LANGUAGE": "es:pt", "LANG": "sv_SE"}, "", Es},
		{"our variable wins over all", map[string]string{"LIBRESPEED_ROUTE_LANG": "pt", "LC_ALL": "sv_SE"}, "", Pt},
		{"ours that is not a language is English", map[string]string{"LIBRESPEED_ROUTE_LANG": "fr", "LC_ALL": "sv_SE"}, "", En},
		{"C and POSIX are skipped", map[string]string{"LC_ALL": "C", "LANG": "pt_BR"}, "", Pt},
		{"a language that is not ours is skipped", map[string]string{"LC_ALL": "fr_FR", "LANG": "sv_SE"}, "", Sv},
		{"Windows", nil, "pt-BR", Pt},
		{"Windows Swedish", nil, "sv-SE", Sv},
		{"Windows English", nil, "en-US", En},
		{"environment before the Windows locale", map[string]string{"LANG": "es_ES"}, "pt-BR", Es},
		{"other Windows language", nil, "de-DE", En},
	}
	for _, c := range cases {
		if got := Detect(env(c.env), c.locale); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestT(t *testing.T) {
	if got := Pt.T("uninst.removed", "x"); got != "removido x\n" {
		t.Errorf("%q", got)
	}
	// A language that is not ours, and the zero value, are English
	if got := Lang("fr").T("uninst.removed", "x"); got != "removed x\n" {
		t.Errorf("%q", got)
	}
	if got := Lang("").T("uninst.removed", "x"); got != "removed x\n" {
		t.Errorf("%q", got)
	}
	// A key that nobody has is the key; a text with no argument is given as it is (a % in it stays)
	if got := Sv.T("nobody.has.this"); got != "nobody.has.this" {
		t.Errorf("%q", got)
	}
	if got := En.T("reason.path"); !strings.Contains(got, `(% " ^ & !)`) {
		t.Errorf("%q", got)
	}
	// The order of the arguments is the one of the language
	if got := Sv.T("table.title", "8.8.8.8", "tracert", 5); !strings.HasPrefix(got, "Väg till 8.8.8.8 (tracert, 5 hopp)") {
		t.Errorf("%q", got)
	}
}
