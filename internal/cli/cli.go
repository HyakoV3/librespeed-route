// Package cli is the command librespeed-route: it traces the route from this computer to a host with the tool that the system already
// has (tracert, traceroute, tracepath), reads the output, and writes it as a table (to stderr) and as the text that the page reads
// (to stdout). It never asks a question and never reads its input.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/HyakoV3/librespeed-route/internal/install"
	"github.com/HyakoV3/librespeed-route/internal/route"
)

// Exit codes: stable, for a script or a browser extension that starts the command.
const (
	ExitOK          = 0
	ExitUsage       = 1
	ExitRefused     = 2 // the host is not one that can be traced (bad name, no address, not a public address)
	ExitNoTool      = 3 // the system has no tracert, traceroute or tracepath
	ExitFailed      = 4 // the tool failed, the time ran out with no hop, the output was not understood, or LSR1 does not fit
	ExitInterrupted = 130
)

// Env is what the command takes from the system, so that the tests can give their own.
type Env struct {
	Version    string
	GOOS       string
	SystemRoot string
	Resolver   route.Resolver
	Exists     func(string) bool
	Run        func(ctx context.Context, spec route.Spec, opt route.Options) (route.Output, error)
	Now        func() time.Time
	Nonce      func() (string, error)
	// Install is where the commands that install the host for the browsers work (the home of the user, the registry of Windows...)
	Install install.Env
}

// SystemEnv is the real system.
func SystemEnv(version string) Env {
	exe, _ := os.Executable()
	home, _ := os.UserHomeDir()
	return Env{
		Install: install.Env{
			GOOS:         runtime.GOOS,
			Home:         home,
			LocalAppData: os.Getenv("LOCALAPPDATA"),
			XDGDataHome:  os.Getenv("XDG_DATA_HOME"),
			Exe:          exe,
			Registry:     install.SystemRegistry{},
		},
		Version:    version,
		GOOS:       runtime.GOOS,
		SystemRoot: os.Getenv("SystemRoot"),
		Resolver:   net.DefaultResolver,
		Exists:     route.FileExists,
		Run:        route.Run,
		Now:        time.Now,
		Nonce: func() (string, error) {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				return "", err
			}
			return hex.EncodeToString(b), nil
		},
	}
}

const usage = `librespeed-route traces the route from this computer to a host, with the tool that the system already has
(tracert on Windows, traceroute on macOS and Linux, tracepath when there is no traceroute).

Usage: librespeed-route [options] HOST

  -4, -6          trace over IPv4 or IPv6 (default: the first public address of the host)
  --json          write a JSON object to stdout and no table
  --quiet         no table
  --timeout T     the longest to wait, as a duration (default 2m)
  --version       print the version

The table goes to stderr and one line of text for the page ("LSR1!...") to stdout, so "librespeed-route HOST | clip" copies only the
line. Only public addresses are written in it: the ones of your internal network are left out. Only public addresses are traced.

For the browser extension:
  install-native-host [--all]   copy this program to a folder of your user and tell Chrome, Edge and Firefox (the ones that you have used;
                                --all: every one) that the extension may start it. No administrator rights.
  uninstall-native-host         take away what install-native-host wrote, and only that
  doctor                        say what is where, and what would not work

Exit codes: 0 ok, 1 usage, 2 the host cannot be traced, 3 no traceroute tool on this system, 4 failed, 130 interrupted.
Options go before the host.
`

// subcommand runs install-native-host, uninstall-native-host and doctor (the first word of the arguments): the words are commands, not
// host names.
func subcommand(args []string, stdout, stderr io.Writer, env Env) (int, bool) {
	switch args[0] {
	case "install-native-host":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		all := fs.Bool("all", false, "")
		if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
			fmt.Fprint(stderr, usage)
			return ExitUsage, true
		}
		e := env.Install
		e.Out = stdout
		if err := install.Install(e, *all); err != nil {
			fmt.Fprintf(stderr, "librespeed-route: %v\n", err)
			return ExitFailed, true
		}
		return ExitOK, true
	case "uninstall-native-host", "doctor":
		if len(args) != 1 {
			fmt.Fprint(stderr, usage)
			return ExitUsage, true
		}
		e := env.Install
		e.Out = stdout
		run := install.Uninstall
		if args[0] == "doctor" {
			run = install.Doctor
		}
		if err := run(e); err != nil {
			fmt.Fprintf(stderr, "librespeed-route: %v\n", err)
			return ExitFailed, true
		}
		return ExitOK, true
	}
	return 0, false
}

// Main runs the command and gives the exit code.
func Main(ctx context.Context, args []string, stdout, stderr io.Writer, env Env) int {
	if len(args) >= 1 {
		if code, done := subcommand(args, stdout, stderr, env); done {
			return code
		}
	}
	fs := flag.NewFlagSet("librespeed-route", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	four := fs.Bool("4", false, "")
	six := fs.Bool("6", false, "")
	asJSON := fs.Bool("json", false, "")
	quiet := fs.Bool("quiet", false, "")
	timeout := fs.Duration("timeout", 2*time.Minute, "")
	showVersion := fs.Bool("version", false, "")
	help := fs.Bool("help", false, "")
	helpShort := fs.Bool("h", false, "")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "librespeed-route: %v\n\n%s", err, usage)
		return ExitUsage
	}
	if *help || *helpShort {
		fmt.Fprint(stdout, usage)
		return ExitOK
	}
	if *showVersion {
		fmt.Fprintf(stdout, "librespeed-route %s\n", env.Version)
		return ExitOK
	}
	if fs.NArg() != 1 || (*four && *six) || *timeout < time.Second || *timeout > 10*time.Minute {
		fmt.Fprint(stderr, usage)
		return ExitUsage
	}
	family := 0
	if *four {
		family = 4
	}
	if *six {
		family = 6
	}
	host := fs.Arg(0)

	ip, err := route.Resolve(ctx, env.Resolver, host, family)
	if err != nil {
		fmt.Fprintf(stderr, "librespeed-route: %s: %v\n", host, err)
		return ExitRefused
	}
	spec, err := route.CommandFor(env.GOOS, ip, env.Exists, env.SystemRoot)
	if err != nil {
		fmt.Fprintf(stderr, "librespeed-route: %v\n", err)
		return ExitNoTool
	}
	out, err := env.Run(ctx, spec, route.Options{Timeout: *timeout})
	if ctx.Err() != nil {
		fmt.Fprintln(stderr, "librespeed-route: interrupted")
		return ExitInterrupted
	}
	if err != nil {
		if errors.Is(err, route.ErrToolFailed) {
			fmt.Fprintf(stderr, "librespeed-route: %s failed\n", spec.Tool)
		} else {
			fmt.Fprintf(stderr, "librespeed-route: could not run %s: %v\n", spec.Tool, err)
		}
		return ExitFailed
	}
	res, perr := route.Parse(out.Text)
	if perr == route.ErrUnknown && silentHops(out.Text) > 0 {
		// Every hop was silent: the network drops the probes of this tool (a firewall, a runner, a captive network)
		fmt.Fprintf(stderr, "librespeed-route: no router answered (%d silent hops): the network may be dropping the packets of %s\n", silentHops(out.Text), spec.Tool)
		return ExitFailed
	}
	if perr != nil {
		fmt.Fprintf(stderr, "librespeed-route: the output of %s was not understood (%v)", spec.Tool, perr)
		if out.Stopped != "" {
			fmt.Fprintf(stderr, ", it was stopped: %s", out.Stopped)
		}
		fmt.Fprintln(stderr)
		return ExitFailed
	}
	if out.Stopped != "" && !*quiet && !*asJSON {
		fmt.Fprintf(stderr, "librespeed-route: %s was stopped (%s); this is what it had until then\n", spec.Tool, out.Stopped)
	}
	nonce, err := env.Nonce()
	if err != nil {
		fmt.Fprintf(stderr, "librespeed-route: %v\n", err)
		return ExitFailed
	}
	text, lerr := route.EncodeLSR1(res, ip.String(), nonce, route.VersionTag(env.Version, spec.Tool), env.Now().Unix())

	if *asJSON {
		doc := map[string]any{
			"version": env.Version,
			"tool":    res.Tool,
			"target":  ip.String(),
			"family":  res.Family,
			"stopped": out.Stopped,
			"hops":    res.Hops,
			"lsr1":    nil,
		}
		if lerr == nil {
			doc["lsr1"] = text
		}
		encoded, _ := json.Marshal(doc)
		fmt.Fprintln(stdout, string(encoded))
	} else {
		if !*quiet {
			route.WriteTable(stderr, res, ip.String())
		}
		if lerr == nil {
			fmt.Fprintln(stdout, text)
		}
	}
	if lerr != nil {
		fmt.Fprintf(stderr, "librespeed-route: %v\n", lerr)
		return ExitFailed
	}
	return ExitOK
}

// silentHops counts the lines of the output that are a hop in which nothing answered
func silentHops(text string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if route.SilentLine(strings.TrimRight(line, "\r")) {
			n++
		}
	}
	return n
}
