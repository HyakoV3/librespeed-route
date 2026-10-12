package i18n

var en = map[string]string{
	"usage": `librespeed-route traces the route from this computer to a host, with the tool that the system already has
(tracert on Windows, traceroute on macOS and Linux, tracepath when there is no traceroute).

Usage: librespeed-route [options] HOST

  -4, -6          trace over IPv4 or IPv6 (default: the first public address of the host)
  --json          write a JSON object to stdout and no table
  --quiet         no table
  --timeout T     the longest to wait, as a duration (default 2m)
  --lang L        the language of the messages: en, pt, es or sv (default: the one of the system)
  --version

The table goes to stderr and one line of text for the page ("LSR1!...") to stdout, so "librespeed-route HOST | clip" copies only the
line. Only public addresses are written in it: the ones of your internal network are left out. Only public addresses are traced.

For the browser extension:
  install-native-host [--all]   copy this program to a folder of your user and tell Chrome, Edge and Firefox (the ones that you have used;
                                --all: every one) that the extension may start it. No administrator rights.
  uninstall-native-host         take away what install-native-host wrote, and only that
  doctor                        say what is where, and what would not work

Exit codes: 0 ok, 1 usage, 2 the host cannot be traced, 3 no traceroute tool on this system, 4 failed, 130 interrupted.
Options go before the host.
`,
	"err.prefix":           "librespeed-route: %[1]s\n",
	"err.host":             "librespeed-route: %[1]s: %[2]s\n",
	"err.interrupted":      "librespeed-route: interrupted\n",
	"err.tool-failed":      "librespeed-route: %[1]s failed\n",
	"err.could-not-run":    "librespeed-route: could not run %[1]s: %[2]v\n",
	"err.silent":           "librespeed-route: no router answered (%[1]d silent hops): the network may be dropping the packets of %[2]s\n",
	"err.not-understood":   "librespeed-route: the output of %[1]s was not understood (%[2]s)",
	"err.stopped-suffix":   ", it was stopped: %[1]s",
	"msg.stopped":          "librespeed-route: %[1]s was stopped (%[2]s); this is what it had until then\n",
	"reason.bad-host":      "not a host name or an address that can be traced",
	"reason.no-address":    "the name has no address",
	"reason.not-global":    "the address is not a global one (internal, documentation, loopback...): only public addresses are traced",
	"reason.no-tool":       "no tracert, traceroute or tracepath was found in the places where the system keeps them",
	"reason.lsr1-too-long": "the route does not fit in the text that the page takes",
	"reason.path":          "the folder of the program has a character (% \" ^ & !) that Windows would read as part of a command: use another user, or a folder without them",
	"table.title":          "Route to %[1]s (%[2]s, %[3]d hops)\n",
	"table.times":          "%[1]s ms (best), %[2]s ms (mean)",
	"table.no-answer":      "no answer",
	"table.lost":           "%[1]d of %[2]d lost",
	"table.internal":       "internal",
	"table.cgnat":          "CGNAT",
	"table.also":           "also",
	"inst.not-used":        "%[1]s: not used on this computer (nothing written; --all writes it anyway)\n",
	"inst.program-in":      "The program is in %[1]s\n",
	"inst.no-browser":      "No browser was found: open Chrome, Edge or Firefox once and run this again, or use --all.\n",
	"inst.quarantine":      "If the browser cannot start it (a file that was downloaded is quarantined): xattr -d com.apple.quarantine %[1]q\n",
	"inst.no-exe":          "the path of the running program is not known",
	"inst.copying":         "copying the program: %[1]v",
	"inst.registry":        "registry %[1]s: %[2]v",
	"uninst.removed":       "removed %[1]s\n",
	"doctor.program":       "program: %[1]s (%[2]s)\n",
	"doctor.there":         "there",
	"doctor.not-there":     "not there: run install-native-host",
	"doctor.no-manifest":   "no manifest",
	"doctor.points-at":     "points at %[1]s, which is not there",
	"doctor.ok":            "ok -> %[1]s",
	"doctor.container":     "warning: %[1]s: a browser in a snap or flatpak cannot start a program of the user; it is not supported\n",
	"doctor.quarantine":    "macOS: if the browser cannot start the program, clear the quarantine: xattr -d com.apple.quarantine %[1]q\n",
	"doctor.problems":      "%[1]d problem(s)",
	"win.menu": `librespeed-route

This program connects the browser extension "Route back for LibreSpeed pages" (Chrome, Edge, Firefox) to this computer: when you click the
extension on the page of a server, it runs the tracert of Windows and hands the route to the page, without the addresses of your own network.

  [I] Install it for this user (it is copied to %LOCALAPPDATA%\librespeed-route and registered for the browsers; no administrator rights)
  [U] Uninstall it (takes away only what the install wrote)
  [Q] Quit

To trace a route from a terminal instead:  librespeed-route.exe HOST   (or --help)

Choose [I/U/Q]: `,
	"win.failed":    "\nIt did not work: %[1]v\n",
	"win.installed": "\nDone. Now add the extension to the browser, and click its icon on the page of a server.\n",
	"win.done":      "\nDone.\n",
	"win.nothing":   "\nNothing was changed.\n",
	"win.press":     "\nPress Enter to close. ",
}
