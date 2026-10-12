package nativehost

import (
	"context"
	"encoding/json"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/HyakoV3/librespeed-route/internal/route"
)

// Deps is what the host takes from the system, so that the tests (and the build of the end to end test) give their own. The release build
// takes it from SystemDeps (deps_real.go) and nothing in the environment changes how it behaves.
type Deps struct {
	Version    string
	GOOS       string
	SystemRoot string
	Resolver   route.Resolver
	Exists     func(string) bool
	Run        func(ctx context.Context, spec route.Spec, opt route.Options) (route.Output, error)
	Now        func() time.Time
	Nonce      func() (string, error)
	LockDir    string
	ChromeIDs  []string
	GeckoIDs   []string
	// RequestTimeout is how long the browser has to send the request (10 s when it is zero): a connection that sends nothing is let go
	RequestTimeout time.Duration
	// HelloProtocol is the protocol that "hello" says (0: Protocol). Only the end to end build sets it, to see what the extension does with a
	// program that speaks another one.
	HelloProtocol int
}

// Replies: a closed set of three shapes. No free text, no address, no stderr.
type helloReply struct {
	V        int    `json:"v"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Protocol int    `json:"protocol"`
	Version  string `json:"version"`
	Tool     string `json:"tool"`
}

type resultReply struct {
	V    int    `json:"v"`
	ID   string `json:"id"`
	Type string `json:"type"`
	LSR1 string `json:"lsr1"`
}

type errorReply struct {
	V    int    `json:"v"`
	ID   string `json:"id"`
	Type string `json:"type"`
	Code string `json:"code"`
}

func send(w io.Writer, v any) bool {
	body, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return WriteFrame(w, body) == nil
}

func fail(w io.Writer, id, code string) {
	send(w, errorReply{V: Protocol, ID: id, Type: "error", Code: code})
}

// Main is the host: args are the arguments of the program (without its name). It returns the exit code: 1 when the caller is not one of ours
// (nothing is read or written) and 0 when it answered, whatever the answer was.
func Main(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, d Deps) int {
	if !CheckCaller(args, d.GOOS, d.ChromeIDs, d.GeckoIDs) {
		return 1
	}
	body, err := readRequest(stdin, d.RequestTimeout)
	if err != nil {
		fail(stdout, "", CodeBadRequest)
		return 1
	}
	req, host, family, perr := ParseRequest(body)
	if perr != nil {
		fail(stdout, "", CodeBadRequest)
		return 1
	}
	switch req.Type {
	case "hello":
		tool := "none"
		if spec, err := route.CommandFor(d.GOOS, netip.MustParseAddr("8.8.8.8"), d.Exists, d.SystemRoot); err == nil {
			tool = spec.Tool
		}
		protocol := Protocol
		if d.HelloProtocol != 0 {
			protocol = d.HelloProtocol
		}
		send(stdout, helloReply{V: Protocol, ID: req.ID, Type: "hello", Protocol: protocol, Version: d.Version, Tool: tool})
		return 0
	case "trace":
		return trace(ctx, stdin, stdout, d, req.ID, host, family)
	}
	// "cancel" first, before any trace, is nothing to cancel
	fail(stdout, req.ID, CodeBadRequest)
	return 0
}

// readRequest reads the one request, and gives up when it does not come in time
func readRequest(stdin io.Reader, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	type result struct {
		body []byte
		err  error
	}
	got := make(chan result, 1)
	go func() {
		body, err := ReadFrame(stdin, MaxRequest)
		got <- result{body, err}
	}()
	select {
	case r := <-got:
		return r.body, r.err
	case <-time.After(timeout):
		return nil, ErrFrameSize
	}
}

// watch cancels the trace when the browser closes the connection, or sends a cancel, or anything else: one request per process.
func watch(stdin io.Reader, cancel context.CancelFunc) {
	for {
		body, err := ReadFrame(stdin, MaxRequest)
		if err != nil {
			cancel()
			return
		}
		cancel()
		_ = body
		return
	}
}

func trace(ctx context.Context, stdin io.Reader, stdout io.Writer, d Deps, id, host string, family int) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go watch(stdin, cancel)

	release, ok := acquire(d.LockDir, d.Now())
	if !ok {
		fail(stdout, id, CodeBusy)
		return 0
	}
	defer release()

	ip, err := route.Resolve(ctx, d.Resolver, host, family)
	if err != nil {
		fail(stdout, id, CodeRefused)
		return 0
	}
	spec, err := route.CommandFor(d.GOOS, ip, d.Exists, d.SystemRoot)
	if err != nil {
		fail(stdout, id, CodeNoTool)
		return 0
	}
	out, err := d.Run(ctx, spec, route.Options{Timeout: 120 * time.Second})
	if ctx.Err() != nil {
		fail(stdout, id, CodeInterrupted)
		return 0
	}
	if err != nil {
		fail(stdout, id, CodeFailed)
		return 0
	}
	res, perr := route.Parse(out.Text)
	if perr != nil {
		if perr == route.ErrUnknown && silentHops(out.Text) > 0 {
			fail(stdout, id, CodeSilent)
		} else {
			fail(stdout, id, CodeFailed)
		}
		return 0
	}
	nonce, err := d.Nonce()
	if err != nil {
		fail(stdout, id, CodeFailed)
		return 0
	}
	text, lerr := route.EncodeLSR1(res, ip.String(), nonce, route.VersionTag(d.Version, spec.Tool), d.Now().Unix())
	if lerr == route.ErrLSR1TooLong {
		fail(stdout, id, CodeTooLong)
		return 0
	}
	if lerr != nil {
		fail(stdout, id, CodeFailed)
		return 0
	}
	send(stdout, resultReply{V: Protocol, ID: id, Type: "result", LSR1: text})
	return 0
}

func silentHops(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if route.SilentLine(strings.TrimRight(line, "\r")) {
			n++
		}
	}
	return n
}
