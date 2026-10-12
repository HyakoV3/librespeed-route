package route

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LSR1 is the text that the page reads (route-parse.js, "the text of the app"):
//
//	LSR1!n=<16 hex>!f=<4|6>!t=<address of the target>!s=<unix seconds>!a=<version>!h=<hop>;<hop>;...
//	hop = <ttl>,<address | * | -p | -c>,<rtt_min>,<sent>,<answered>[,<other address>]
//
// "*" is a hop that did not answer, "-p" an internal one and "-c" one in the range of CGNAT: the page never gets the addresses of the
// internal network of the person who ran this program, only that the hop was there and how long it took. Only public addresses are
// written.

// MaxLSR1 is the longest text that the page takes.
const MaxLSR1 = 1900

var (
	reNonce   = regexp.MustCompile(`^[0-9a-f]{16}$`)
	reVersion = regexp.MustCompile(`^[0-9A-Za-z.+-]{1,16}$`)

	ErrLSR1TooLong = errors.New("the route does not fit in the text that the page takes")
	ErrLSR1Target  = errors.New("the target is not a public address")
)

func rttText(v float64) (string, error) {
	if v < 0 || v > 60000 {
		return "", fmt.Errorf("a time of %v ms does not fit", v)
	}
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" {
		s = "0"
	}
	return s, nil
}

// EncodeLSR1 writes the route as the text of the page. target is the address that was traced (it must be public and of the family of the
// route), nonce 16 lower case hexadecimal digits, version up to 16 characters of [0-9A-Za-z.+-] and now the unix time in seconds.
func EncodeLSR1(res *Result, target, nonce, version string, now int64) (string, error) {
	if res == nil || len(res.Hops) == 0 {
		return "", errors.New("no hops")
	}
	t := Normalize(target)
	if t == "" || ScopeOf(t) != ScopePublic {
		return "", ErrLSR1Target
	}
	family := 4
	if strings.Contains(t, ":") {
		family = 6
	}
	if !reNonce.MatchString(nonce) || !reVersion.MatchString(version) || now < 0 || now > 999999999999 {
		return "", errors.New("nonce, version or time out of the form")
	}
	for _, withAlt := range []bool{true, false} {
		items := make([]string, 0, len(res.Hops))
		for _, h := range res.Hops {
			if h.TTL < 1 || h.TTL > maxTTL || h.Sent < 1 || h.Sent > 9 || h.Answered > h.Sent {
				return "", fmt.Errorf("hop %d is out of the form", h.TTL)
			}
			kind := "*"
			rtt := "0"
			if h.Answered > 0 && h.RTTMin != nil {
				var err error
				if rtt, err = rttText(*h.RTTMin); err != nil {
					return "", err
				}
				switch h.Scope {
				case ScopePublic:
					if h.IP == nil {
						return "", fmt.Errorf("hop %d has no address", h.TTL)
					}
					kind = *h.IP
				case ScopeCGNAT:
					kind = "-c"
				default:
					kind = "-p"
				}
			}
			cells := []string{strconv.Itoa(h.TTL), kind, rtt, strconv.Itoa(h.Sent), strconv.Itoa(h.Answered)}
			if withAlt && h.Scope == ScopePublic && h.Answered > 0 {
				for _, alt := range h.AltIPs {
					if ScopeOf(alt) == ScopePublic {
						cells = append(cells, alt)
						break
					}
				}
			}
			items = append(items, strings.Join(cells, ","))
		}
		text := fmt.Sprintf("LSR1!n=%s!f=%d!t=%s!s=%d!a=%s!h=%s", nonce, family, t, now, version, strings.Join(items, ";"))
		if len(text) <= MaxLSR1 {
			return text, nil
		}
	}
	return "", ErrLSR1TooLong
}

// VersionTag is the "a" field: the version of the program and the tool that was run ("0.1.0+tracert"), up to 16 characters of
// [0-9A-Za-z.+-].
func VersionTag(version, tool string) string {
	tag := version + "+" + tool
	if len(tag) > 16 {
		tag = tag[:16]
	}
	return strings.TrimRight(tag, "+-.")
}
