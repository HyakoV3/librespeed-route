package route

import (
	"fmt"
	"io"
	"strings"
)

// WriteTable writes the route for the person who ran the program, in their own terminal: the real addresses are shown here (only the
// text of LSR1 hides the internal ones).
func WriteTable(w io.Writer, res *Result, target string) {
	fmt.Fprintf(w, "Route to %s (%s, %d hops)\n", target, res.Tool, len(res.Hops))
	for _, h := range res.Hops {
		address := "*"
		if h.IP != nil {
			address = *h.IP
		}
		tag := ""
		switch h.Scope {
		case ScopePrivate:
			tag = "  internal"
		case ScopeCGNAT:
			tag = "  CGNAT"
		}
		times := "no answer"
		if h.RTTMin != nil && h.RTTAvg != nil {
			times = fmt.Sprintf("%.2f ms (best), %.2f ms (mean)", *h.RTTMin, *h.RTTAvg)
		}
		loss := ""
		if h.Answered < h.Sent && h.Answered > 0 {
			loss = fmt.Sprintf("  %d of %d lost", h.Sent-h.Answered, h.Sent)
		}
		extra := ""
		if len(h.AltIPs) > 0 {
			extra = "  also " + strings.Join(h.AltIPs, ", ")
		}
		fmt.Fprintf(w, "%3d  %-39s %s%s%s%s\n", h.TTL, address, times, loss, tag, extra)
	}
}
