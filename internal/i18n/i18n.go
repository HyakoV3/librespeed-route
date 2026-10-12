// Package i18n has the words of the program in English, Portuguese, Spanish and Swedish: what the window of a double click says, what the
// installer, the doctor and the command say, and the errors. The language is the one of the system (or of LIBRESPEED_ROUTE_LANG, or of
// --lang), and English when it is none of these four.
package i18n

import (
	"fmt"
	"strings"
)

// Lang is one of En, Pt, Es and Sv. The zero value is English.
type Lang string

const (
	En Lang = "en"
	Pt Lang = "pt"
	Es Lang = "es"
	Sv Lang = "sv"
)

// All the languages, in the order that the help names them.
var All = []Lang{En, Pt, Es, Sv}

var catalogs = map[Lang]map[string]string{En: en, Pt: pt, Es: es, Sv: sv}

// Parse reads the name of a language ("pt", "pt-BR", "pt_BR.UTF-8", "Spanish" is not one): the first two letters, in any case. ok is false
// for anything that is not one of the four.
func Parse(name string) (Lang, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if len(name) < 2 {
		return En, false
	}
	if len(name) > 2 {
		c := name[2]
		if c != '-' && c != '_' && c != '.' && c != '@' && c != ':' {
			return En, false
		}
	}
	for _, l := range All {
		if name[:2] == string(l) {
			return l, true
		}
	}
	return En, false
}

// Detect gives the language of the system: LIBRESPEED_ROUTE_LANG, then LC_ALL, LC_MESSAGES, LANGUAGE (its first entry) and LANG; on Windows
// the language of the user (systemLocale, "pt-BR") when none of those is set. English when it is none of the four.
func Detect(getenv func(string) string, systemLocale string) Lang {
	for _, key := range []string{"LIBRESPEED_ROUTE_LANG", "LC_ALL", "LC_MESSAGES", "LANGUAGE", "LANG"} {
		v := getenv(key)
		if key == "LANGUAGE" {
			v = strings.SplitN(v, ":", 2)[0]
		}
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		if l, ok := Parse(v); ok {
			return l
		}
		// A language that is set and is not ours is the answer: English, not the one of the next variable
		if key == "LIBRESPEED_ROUTE_LANG" {
			return En
		}
	}
	if l, ok := Parse(systemLocale); ok {
		return l
	}
	return En
}

// T is the text of a key in the language, with the arguments in it (verbs of the form %[1]s, so that a language can put them in another
// order); a text with no arguments is given as it is. A key that a language has not is read in English, and one that none has is the key.
func (l Lang) T(key string, args ...any) string {
	text, ok := catalogs[l][key]
	if !ok {
		text, ok = en[key]
	}
	if !ok {
		return key
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}
