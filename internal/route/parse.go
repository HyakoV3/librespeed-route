package route

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The output of tracert, traceroute and tracepath, read as the page reads it (frontend/better/javascript/route-parse.js of
// HyakoV3/speedtest): the words of the tool are never read (they come in the language of the system), only the number of a hop, times
// with a unit, "*" and addresses. testdata/vectors holds the outputs and what the page gives for each of them, and a test checks that
// this port gives the same, and another one that it gives the same as the page for outputs that are made by the test.
//
// Left out on purpose: mtr and the text of the app (LSR1), because this program runs the tool itself and always asks it for numbers
// ("-d", "-n"), and a Go encoder of LSR1 lives in lsr1.go.

const (
	maxText  = 32768
	maxLines = 300
	maxTTL   = 64
)

// Hop is one hop, in the shape that backend/route.php and the page use.
type Hop struct {
	TTL      int      `json:"ttl"`
	IP       *string  `json:"ip"`
	AltIPs   []string `json:"alt_ips"`
	RTTMin   *float64 `json:"rtt_min"`
	RTTAvg   *float64 `json:"rtt_avg"`
	Sent     int      `json:"sent"`
	Answered int      `json:"answered"`
	Flags    []string `json:"flags"`
	Scope    string   `json:"scope"`
}

// Result is what Parse reads from a text.
type Result struct {
	Tool       string  `json:"tool"`
	Family     int     `json:"family"`
	TargetName *string `json:"target_name"`
	TargetIP   *string `json:"target_ip"`
	Hops       []Hop   `json:"hops"`
	Skipped    int     `json:"skipped"`
	Reached    *bool   `json:"reached"`
	Nonce      *string `json:"nonce"`
}

// Errors of Parse, with the words that the page uses.
type ParseError string

func (e ParseError) Error() string { return string(e) }

const (
	ErrEmpty       ParseError = "empty"
	ErrTooLong     ParseError = "too_long"
	ErrUnknown     ParseError = "unknown"
	ErrUnsupported ParseError = "unsupported"
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

type hopEntry struct {
	ttl   int
	ips   map[string][]float64
	order []string
	lost  int
	flags []string
}

type table struct{ byTTL map[int]*hopEntry }

func newTable() *table { return &table{byTTL: map[int]*hopEntry{}} }

func (t *table) hop(ttl int) *hopEntry {
	h, ok := t.byTTL[ttl]
	if !ok {
		h = &hopEntry{ttl: ttl, ips: map[string][]float64{}, flags: []string{}}
		t.byTTL[ttl] = h
	}
	return h
}

func (t *table) add(ttl int, ip string, times []float64) {
	h := t.hop(ttl)
	if _, ok := h.ips[ip]; !ok {
		h.ips[ip] = []float64{}
		h.order = append(h.order, ip)
	}
	h.ips[ip] = append(h.ips[ip], times...)
}

// summarize is routeSummarize(): the address with the most times is the hop (the first one on a tie), the others are alt_ips; the hops
// with no answer at the end are left out.
func (t *table) summarize() []Hop {
	ttls := make([]int, 0, len(t.byTTL))
	for k := range t.byTTL {
		ttls = append(ttls, k)
	}
	sort.Ints(ttls)
	hops := make([]Hop, 0, len(ttls))
	for _, k := range ttls {
		h := t.byTTL[k]
		ip := ""
		haveIP := false
		best := 0
		var times []float64
		for _, a := range h.order {
			list := h.ips[a]
			times = append(times, list...)
			if len(list) > best {
				best = len(list)
				ip, haveIP = a, true
			}
		}
		if !haveIP && len(h.order) > 0 {
			ip, haveIP = h.order[0], true
		}
		others := []string{}
		for _, a := range h.order {
			if !haveIP || a != ip {
				others = append(others, a)
			}
		}
		answered := len(times)
		sum, min := 0.0, math.Inf(1)
		for _, v := range times {
			sum += v
			if v < min {
				min = v
			}
		}
		hop := Hop{TTL: h.ttl, AltIPs: others, Sent: answered + h.lost, Answered: answered, Flags: h.flags}
		if haveIP {
			s := ip
			hop.IP = &s
		}
		if answered > 0 {
			lo, avg := round2(min), round2(sum/float64(answered))
			hop.RTTMin, hop.RTTAvg = &lo, &avg
			hop.Scope = ScopeOf(ip)
		} else {
			hop.Scope = ScopeNone
		}
		hops = append(hops, hop)
	}
	for len(hops) > 0 && hops[len(hops)-1].Scope == ScopeNone {
		hops = hops[:len(hops)-1]
	}
	return hops
}

var (
	reTime      = regexp.MustCompile(`^(<?)(\d+(?:[.,]\d+)?)(\S*)$`)
	reUnit      = regexp.MustCompile(`^[^\s\d.:*\[\]()<>,;!=/-]{1,3}$`)
	reMS        = regexp.MustCompile(`(?i)^ms$`)
	reFlag      = regexp.MustCompile(`^!(H|N|P|S|X|V|C|F-\d+|\d+)$`)
	reAddrChars = regexp.MustCompile(`^[0-9a-fA-F:.]+$`)
	reTracepath = regexp.MustCompile(`^\s*(\d{1,2})\??:\s+(\S.*)$`)
	reCommon    = regexp.MustCompile(`^\s*(\d{1,2})\s+(\S.*)$`)
	reFirstTok  = regexp.MustCompile(`^\s+(\S+)`)
	reEndPath   = regexp.MustCompile(`^\s*(Resume:|Too many hops)`)
	rePmtu      = regexp.MustCompile(`\bpmtu\b`)
	reLocalhost = regexp.MustCompile(`\[LOCALHOST\]`)
	reNoReply   = regexp.MustCompile(`^no reply\b`)
	reReached   = regexp.MustCompile(`\breached\b`)
	reBracket   = regexp.MustCompile(`^[(\[]`)
	reMtr       = regexp.MustCompile(`^\s*\d{1,2}\.\|--\s`)
	reToolPath  = regexp.MustCompile(`^\s*\d{1,2}\??:\s+\S`)
	reToolPath2 = regexp.MustCompile(`(\bpmtu\b|\bno reply\b|\d\s*ms\b|\[LOCALHOST\])`)
	reToolCert  = regexp.MustCompile(`^\s*\d{1,2}\s+((?:(?:<?\d+\s*[^\s\d.:*\[\]()<>,;!=/-]{1,3}|\*)\s+){3})\S`)
	reDigit     = regexp.MustCompile(`\d`)
	reLineEnd   = regexp.MustCompile(`\r?\n`)
)

// addressOf is a token as an address: without the brackets or the parentheses around it, without the zone of a link-local address, and
// without a ":" at the end that is not part of "::". "" when it is not one.
func addressOf(token string) string {
	start, end := 0, len(token)
	for start < end && (token[start] == '(' || token[start] == '[') {
		start++
	}
	for end > start && strings.IndexByte(")],;", token[end-1]) >= 0 {
		end--
	}
	bare := token[start:end]
	if zone := strings.IndexByte(bare, '%'); zone >= 0 {
		bare = bare[:zone]
	}
	if len(bare) > 1 && bare[len(bare)-1] == ':' && bare[len(bare)-2] != ':' {
		bare = bare[:len(bare)-1]
	}
	return Normalize(bare)
}

type pending struct {
	ip   string
	time float64
}

type hopRead struct {
	lost    int
	flags   []string
	pending []pending
	seen    []string
}

func number(text string) (float64, bool) {
	v, err := strconv.ParseFloat(strings.Replace(text, ",", ".", 1), 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return 0, false
	}
	return v, true
}

// readHop reads the rest of a line of a hop: times with a unit ("12 ms", "<1 ms", "0.959ms"), "*" and addresses. The times before the first
// address belong to it (tracert writes them first); the others belong to the address before them (traceroute writes it first).
func readHop(rest string, hop *hopRead) bool {
	tokens := strings.Fields(rest)
	current := ""
	haveCurrent := false
	var wait []float64
	any := false
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "*" {
			hop.lost++
			any = true
			continue
		}
		if m := reTime.FindStringSubmatch(token); m != nil {
			attached := m[3]
			unit := false
			if attached == "" && i+1 < len(tokens) {
				unit = reUnit.MatchString(tokens[i+1])
			}
			if (attached != "" && reMS.MatchString(attached)) || unit {
				if value, ok := number(m[2]); ok {
					if m[1] == "<" {
						value = value / 2
					}
					if !haveCurrent {
						wait = append(wait, value)
					} else {
						hop.pending = append(hop.pending, pending{current, value})
					}
				}
				if unit {
					i++
				}
				continue
			}
		}
		if reFlag.MatchString(token) {
			hop.flags = append(hop.flags, token)
			continue
		}
		address := addressOf(token)
		if address == "" && reAddrChars.MatchString(token) {
			// Something that looks like an address but is not one: the times after it are not for the address before it
			haveCurrent = false
		}
		if address != "" {
			current, haveCurrent = address, true
			any = true
			hop.seen = append(hop.seen, address)
			for _, w := range wait {
				hop.pending = append(hop.pending, pending{address, w})
			}
			wait = nil
		}
	}
	return any
}

type headTarget struct {
	ip        string
	name      string
	bracketed bool
}

func firstAddress(line string) *headTarget {
	tokens := strings.Fields(line)
	for i, tok := range tokens {
		address := addressOf(tok)
		if address != "" {
			bracketed := reBracket.MatchString(tok)
			name := address
			if bracketed && i > 0 {
				name = strings.TrimRight(tokens[i-1], ",:")
			}
			return &headTarget{ip: address, name: name, bracketed: bracketed}
		}
	}
	return nil
}

type read struct {
	table   *table
	skipped int
	target  *headTarget
	reached *bool
}

func parseLines(lines []string, tool string) read {
	t := newTable()
	skipped := 0
	var target *headTarget
	seenHop := false
	last := -1
	arrived, ended := false, false
	for _, line := range lines {
		if tool == "tracepath" && reEndPath.MatchString(line) {
			ended = true
		}
		re := reCommon
		if tool == "tracepath" {
			re = reTracepath
		}
		match := re.FindStringSubmatch(line)
		if match == nil {
			// The target is in the lines before the first hop: the one with the address in brackets or parentheses wins over a line
			// that only has an address in it (a warning that says which of the addresses of a name was used)
			if !seenHop && strings.TrimSpace(line) != "" {
				head := firstAddress(line)
				if head != nil && (target == nil || (head.bracketed && !target.bracketed)) {
					target = head
				}
			}
			// The other routers of a hop that answered from more than one are written in lines of their own, with no number
			if first := reFirstTok.FindStringSubmatch(line); seenHop && last >= 0 && first != nil && addressOf(first[1]) != "" {
				more := &hopRead{}
				if readHop(line, more) {
					for _, ip := range more.seen {
						t.add(last, ip, nil)
					}
					for _, p := range more.pending {
						t.add(last, p.ip, []float64{p.time})
					}
				}
			}
			continue
		}
		ttl, _ := strconv.Atoi(match[1])
		if ttl < 1 || ttl > maxTTL {
			skipped++
			continue
		}
		rest := match[2]
		if tool == "tracepath" {
			// The lines about the MTU of the path are not probes
			if rePmtu.MatchString(rest) || reLocalhost.MatchString(rest) {
				continue
			}
			if reNoReply.MatchString(rest) {
				t.hop(ttl).lost++
				seenHop = true
				continue
			}
		}
		hop := &hopRead{}
		if tool == "tracepath" && reReached.MatchString(rest) {
			arrived = true
		}
		if !readHop(rest, hop) {
			skipped++
			continue
		}
		seenHop = true
		entry := t.hop(ttl)
		entry.lost += hop.lost
		for _, f := range hop.flags {
			found := false
			for _, e := range entry.flags {
				if e == f {
					found = true
				}
			}
			if !found {
				entry.flags = append(entry.flags, f)
			}
		}
		// An address with no time (a router that "reports" something) still tells who it was
		for _, ip := range hop.seen {
			t.add(ttl, ip, nil)
		}
		for _, p := range hop.pending {
			t.add(ttl, p.ip, []float64{p.time})
		}
		last = ttl
		// The last line of tracepath says "reached" when the target answered: the address of that line is the target
		if tool == "tracepath" && arrived && target == nil && len(hop.seen) > 0 {
			target = &headTarget{ip: hop.seen[0], name: hop.seen[0]}
		}
	}
	// Whether the target answered, when the text says it without the address of the target: nil when it does not say
	var reached *bool
	if tool == "tracepath" {
		switch {
		case arrived:
			v := true
			reached = &v
		case ended:
			v := false
			reached = &v
		}
	}
	return read{table: t, skipped: skipped, target: target, reached: reached}
}

// ToolOf says which tool wrote the lines: "tracepath", "tracert" or "traceroute" ("mtr" is told so that it can be refused).
func ToolOf(lines []string) string {
	for _, l := range lines {
		if reMtr.MatchString(l) {
			return "mtr"
		}
	}
	for _, l := range lines {
		if reToolPath.MatchString(l) && reToolPath2.MatchString(l) {
			return "tracepath"
		}
	}
	for _, l := range lines {
		// tracert writes three groups, each a whole number of milliseconds ("<1 ms", "12 ms") or "*", and then the address or the words
		if g := reToolCert.FindStringSubmatch(l); g != nil && reDigit.MatchString(g[1]) {
			return "tracert"
		}
	}
	return "traceroute"
}

// Parse reads the output of tracert, traceroute or tracepath. The text is cut at 32768 characters (more is an error) and 300 lines.
func Parse(text string) (*Result, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmpty
	}
	// The page counts characters of UTF-16; bytes are the safe side for a limit
	if len(text) > maxText {
		return nil, ErrTooLong
	}
	if strings.HasPrefix(strings.TrimSpace(text), "LSR1!") {
		return nil, ErrUnsupported
	}
	lines := reLineEnd.Split(text, -1)
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	tool := ToolOf(lines)
	if tool == "mtr" {
		return nil, ErrUnsupported
	}
	r := parseLines(lines, tool)
	hops := r.table.summarize()
	if len(hops) == 0 {
		return nil, ErrUnknown
	}
	family := 4
	reference := ""
	if r.target != nil {
		reference = r.target.ip
	}
	if reference == "" {
		for _, h := range hops {
			if h.IP != nil && h.Scope == ScopePublic {
				reference = *h.IP
				break
			}
		}
	}
	if strings.Contains(reference, ":") {
		family = 6
	}
	res := &Result{Tool: tool, Family: family, Hops: hops, Skipped: r.skipped, Reached: r.reached}
	if r.target != nil {
		name, ip := r.target.name, r.target.ip
		res.TargetName, res.TargetIP = &name, &ip
	}
	return res, nil
}
