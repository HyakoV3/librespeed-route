package route

import (
	"context"
	"errors"
	"net/netip"
	"regexp"
	"strings"
)

// The target is resolved once, here, and the tool is then given the address, never the name: the address that was checked is the one that
// is traced, and a name that starts with "-" is never given to a program as an option.

var (
	ErrBadHost   = errors.New("not a host name or an address that can be traced")
	ErrNoAddress = errors.New("the name has no address")
	ErrNotGlobal = errors.New("the address is not a global one (internal, documentation, loopback...): only public addresses are traced")

	reHost = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,253}$`)
)

// Resolver is the part of net.Resolver that is used.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Resolve gives the public address to trace for a host name or an address. family is 4, 6 or 0 (any: the first public one that the
// resolver gives, in its order).
func Resolve(ctx context.Context, r Resolver, host string, family int) (netip.Addr, error) {
	name := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if !reHost.MatchString(name) || strings.HasPrefix(name, "-") || family < 0 || family > 6 || (family != 0 && family != 4 && family != 6) {
		return netip.Addr{}, ErrBadHost
	}
	var list []netip.Addr
	if a, ok := parseAddr(name); ok {
		list = []netip.Addr{a}
	} else {
		network := map[int]string{0: "ip", 4: "ip4", 6: "ip6"}[family]
		var err error
		if list, err = r.LookupNetIP(ctx, network, name); err != nil || len(list) == 0 {
			return netip.Addr{}, ErrNoAddress
		}
	}
	found := false
	for _, a := range list {
		a = a.WithZone("")
		if (family == 4 && !a.Is4()) || (family == 6 && !a.Is6()) {
			continue
		}
		found = true
		if ScopeOf(a.String()) == ScopePublic || ScopeOf(Normalize(a.String())) == ScopePublic {
			return a, nil
		}
	}
	if !found {
		return netip.Addr{}, ErrNoAddress
	}
	return netip.Addr{}, ErrNotGlobal
}
