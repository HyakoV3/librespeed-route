package route

import (
	"fmt"
	"io"
	"strings"

	"github.com/HyakoV3/librespeed-route/internal/i18n"
)

// WriteTable writes the route for the person who ran the program, in their own terminal: the real addresses are shown here (only the
// text of LSR1 hides the internal ones).
func WriteTable(w io.Writer, lang i18n.Lang, res *Result, target string) {
	fmt.Fprint(w, lang.T("table.title", target, res.Tool, len(res.Hops)))
	for _, h := range res.Hops {
		address := "*"
		if h.IP != nil {
			address = *h.IP
		}
		tag := ""
		switch h.Scope {
		case ScopePrivate:
			tag = "  " + lang.T("table.internal")
		case ScopeCGNAT:
			tag = "  " + lang.T("table.cgnat")
		}
		times := lang.T("table.no-answer")
		if h.RTTMin != nil && h.RTTAvg != nil {
			times = lang.T("table.times", fmt.Sprintf("%.2f", *h.RTTMin), fmt.Sprintf("%.2f", *h.RTTAvg))
		}
		loss := ""
		if h.Answered < h.Sent && h.Answered > 0 {
			loss = "  " + lang.T("table.lost", h.Sent-h.Answered, h.Sent)
		}
		extra := ""
		if len(h.AltIPs) > 0 {
			extra = "  " + lang.T("table.also") + " " + strings.Join(h.AltIPs, ", ")
		}
		fmt.Fprintf(w, "%3d  %-39s %s%s%s%s\n", h.TTL, address, times, loss, tag, extra)
	}
}
