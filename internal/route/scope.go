package route

import (
	"net/netip"
	"strings"
)

// Scope names the kind of an address, the same words and the same ranges as routeHopScope() of the PHP backend and as the page
// (frontend/better/javascript/route-parse.js): "public" (a global address), "private" (not global: internal networks, loopback,
// documentation, link-local, unique-local, ...), "cgnat" (100.64.0.0/10, apart) and "none" (no address: the hop did not answer).
const (
	ScopePublic  = "public"
	ScopePrivate = "private"
	ScopeCGNAT   = "cgnat"
	ScopeNone    = "none"
)

type cidr4 struct {
	base [4]byte
	bits int
}

// The same ranges as FILTER_FLAG_GLOBAL_RANGE (PHP 8.2); the CGNAT range is told apart before them
var v4NotGlobal = []cidr4{
	{[4]byte{0, 0, 0, 0}, 8},
	{[4]byte{10, 0, 0, 0}, 8},
	{[4]byte{127, 0, 0, 0}, 8},
	{[4]byte{169, 254, 0, 0}, 16},
	{[4]byte{172, 16, 0, 0}, 12},
	{[4]byte{192, 0, 0, 0}, 24},
	{[4]byte{192, 0, 2, 0}, 24},
	{[4]byte{192, 168, 0, 0}, 16},
	{[4]byte{198, 18, 0, 0}, 15},
	{[4]byte{198, 51, 100, 0}, 24},
	{[4]byte{203, 0, 113, 0}, 24},
	{[4]byte{240, 0, 0, 0}, 4},
}

func inside(b [4]byte, c cidr4) bool {
	bits := c.bits
	for i := 0; i < 4 && bits > 0; i, bits = i+1, bits-8 {
		mask := byte(255)
		if bits < 8 {
			mask = byte(255<<(8-bits)) & 255
		}
		if b[i]&mask != c.base[i]&mask {
			return false
		}
	}
	return true
}

// parseAddr reads an address the way the page does: IPv4 as four numbers with no leading zeros, IPv6 in any form (no zone).
// ok is false for anything else.
func parseAddr(text string) (addr netip.Addr, ok bool) {
	if text == "" || len(text) > 45 || strings.ContainsAny(text, "%/ ") {
		return netip.Addr{}, false
	}
	for _, r := range text {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F' || r == ':' || r == '.') {
			return netip.Addr{}, false
		}
	}
	a, err := netip.ParseAddr(text)
	if err != nil || a.Zone() != "" {
		return netip.Addr{}, false
	}
	// A dotted address is an IPv4 address only when it is four numbers; "::1.2.3.4" (IPv6 with a tail in dots) is IPv6
	if a.Is4() && strings.Contains(text, ":") {
		return netip.Addr{}, false
	}
	return a, true
}

// Normalize is the address as the page keeps it: IPv4 as it came, IPv6 in the short form of the URL standard (lower case, the first longest
// run of zero groups as "::", an IPv4-mapped address in hexadecimal); "" when it is not an address.
func Normalize(text string) string {
	a, ok := parseAddr(text)
	if !ok {
		return ""
	}
	if a.Is4() {
		return text
	}
	return serializeV6(a.As16())
}

func groups(raw [16]byte) [8]uint16 {
	var g [8]uint16
	for i := 0; i < 8; i++ {
		g[i] = uint16(raw[2*i])<<8 | uint16(raw[2*i+1])
	}
	return g
}

// serializeV6 follows the "IPv6 serializer" of the URL standard, which is what the page gets from new URL("http://[...]/").hostname
func serializeV6(raw [16]byte) string {
	g := groups(raw)
	// The first longest run of zero groups, when it has two or more
	bestStart, bestLen := -1, 0
	for i := 0; i < 8; {
		if g[i] != 0 {
			i++
			continue
		}
		j := i
		for j < 8 && g[j] == 0 {
			j++
		}
		if j-i > bestLen {
			bestStart, bestLen = i, j-i
		}
		i = j
	}
	if bestLen < 2 {
		bestStart = -1
	}
	const hex = "0123456789abcdef"
	var out []byte
	for i := 0; i < 8; i++ {
		if i == bestStart {
			if i == 0 {
				out = append(out, ':')
			}
			out = append(out, ':')
			i += bestLen - 1
			continue
		}
		v := g[i]
		var digits []byte
		for {
			digits = append([]byte{hex[v&15]}, digits...)
			v >>= 4
			if v == 0 {
				break
			}
		}
		out = append(out, digits...)
		if i != 7 {
			out = append(out, ':')
		}
	}
	return string(out)
}

// ScopeOf is the scope of an address; "" (no address) is ScopeNone. Something that is not an address is "private", as in the PHP.
func ScopeOf(text string) string {
	if text == "" {
		return ScopeNone
	}
	a, ok := parseAddr(text)
	if !ok {
		return ScopePrivate
	}
	if a.Is4() {
		b := a.As4()
		if inside(b, cidr4{[4]byte{100, 64, 0, 0}, 10}) {
			return ScopeCGNAT
		}
		for _, c := range v4NotGlobal {
			if inside(b, c) {
				return ScopePrivate
			}
		}
		return ScopePublic
	}
	g := groups(a.As16())
	zeros := func(from, to int) bool {
		for n := from; n < to; n++ {
			if g[n] != 0 {
				return false
			}
		}
		return true
	}
	switch {
	case zeros(0, 7) && (g[7] == 0 || g[7] == 1):
		return ScopePrivate
	case zeros(0, 5) && g[5] == 0xffff:
		return ScopePrivate
	case g[0] == 0x64 && g[1] == 0xff9b && zeros(2, 6):
		return ScopePrivate
	case g[0] == 0x100 && zeros(1, 4):
		return ScopePrivate
	case g[0] == 0x2001 && (g[1] < 0x200 || g[1] == 0xdb8):
		return ScopePrivate
	case g[0] == 0x2002:
		return ScopePrivate
	case g[0]&0xfe00 == 0xfc00 || g[0]&0xffc0 == 0xfe80:
		return ScopePrivate
	}
	return ScopePublic
}
