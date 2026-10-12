# librespeed-route

Traces the route from **your computer** to a server, with the tool your system already has (`tracert` on Windows, `traceroute` on macOS and
Linux, `tracepath` when there is no `traceroute`), and hands it to the route page of the
[better pages of LibreSpeed](https://github.com/HyakoV3/speedtest) (`route-better.html`, "You → server"): the path **your** traffic takes to
that server, which a web page cannot measure by itself.

It does not send probes of its own. There are two ways to use it:

- **With the browser extension** (Chrome, Edge, Firefox): on the page of a server, click the icon of the extension and press **Trace**; the
  route appears on the page by itself. [Quick start](#quick-start-the-browser-extension).
- **From a terminal**: `librespeed-route HOST` prints a table for you and one line of text for the page (and `--json` for a script).
  [The command line](#the-command-line).

Only **public** addresses ever leave the program: the addresses of your own network (a hop of your router, of your provider's CGNAT) are
replaced by a mark, and the page only learns how long each took.

## Quick start: the browser extension

Tried by hand on Windows 11, with Firefox and with Chrome. (macOS, Linux and Edge: see [what is checked](#what-is-checked-and-what-is-not).)

**1. The program.** Download the archive for your system from the [releases](https://github.com/HyakoV3/librespeed-route/releases)
(`librespeed-route_<version>_windows_amd64.zip`, `..._darwin_arm64.tar.gz`, `..._linux_amd64.tar.gz`, and so on), unpack it and let it install
itself for your browsers (your user only, no administrator rights):

- **Windows:** double-click `librespeed-route.exe` and press `I` (or, in a terminal, `.\librespeed-route.exe install-native-host`).
  SmartScreen may warn the first time ("More info" → "Run anyway"): the binaries are not signed.
- **macOS and Linux:** `./librespeed-route install-native-host`. On macOS, a file that was downloaded is quarantined; if the browser cannot
  start it, `xattr -d com.apple.quarantine librespeed-route` (the one in the folder that the command says) lets it run. On Linux,
  `chmod +x librespeed-route` first if the archive did not keep the mode.

The command copies the program to a folder of your user and tells Chrome, Edge and Firefox (the ones that you have used) that the extension
may start it. `./librespeed-route doctor` says what is where, and what would not work.

**2. The extension.** There is no store item yet, so it is loaded from a folder:

- **Firefox:** unpack `librespeed-route-extension-firefox-<version>.zip`, open `about:debugging#/runtime/this-firefox`, "Load Temporary
  Add-on", and choose the `manifest.json` of the folder. A temporary add-on is gone when Firefox closes: load it again next time (a signed one
  comes with the add-ons site).
- **Chrome and Edge:** unpack `librespeed-route-extension-chromium-sideload-<version>.zip` **into a folder that you keep** (Chrome reads it
  from there every time), open `chrome://extensions` (Edge: `edge://extensions`), turn on "Developer mode", "Load unpacked", and choose the
  folder. Its id must be `gcgijalhbacnoacnklibiffbacdchjgm`. Use the `sideload` zip and not the plain `...chromium-<version>.zip`: that one is
  for a store, has no key, and Chrome gives it another id, which the program refuses ("Access to the specified native messaging host is
  forbidden").

**3. Use it.** Open the route page of a server (over https), click the icon of the extension (pin it to the toolbar if you do not see it)
and press **Trace**. It takes a few seconds, up to two minutes when routers do not answer. The route comes to the page by itself
("Route received from the browser extension"), and the popup also shows the line, with a Copy button, in case you want to paste it.

If the popup says that the program is not installed or cannot be started, it shows, in small letters, what the browser said and the id of
the extension:

| What the browser says | What it means |
|---|---|
| `Access to the specified native messaging host is forbidden` | the id of the extension is not the one that the program lets in (see step 2) |
| `Specified native messaging host not found` | the program was not installed for this browser: run the install again, and `doctor` |
| `Native host has exited` | the program closed at once: `doctor` and, on Windows, a security program may have blocked the file |

**Update:** download the new release, run the install again (it replaces the copy), and load the new extension (Firefox: remove the old
temporary add-on first). **Uninstall:** on Windows, double-click the program and press `U`; elsewhere `./librespeed-route uninstall-native-host`;
it removes only what the install wrote. Then remove the extension from the browser.

What goes where is in [docs/protocol.md](docs/protocol.md). The extension has no content script and no permission for any site: only your
click starts a trace, and the target is always the address of the tab you are on. No page can start a trace.

## The command line

Unpacked from a release (or built: `go install github.com/HyakoV3/librespeed-route/cmd/librespeed-route@latest`, Go 1.26):

```
librespeed-route [options] HOST

  -4, -6          trace over IPv4 or IPv6 (default: the first public address of the host)
  --json          write a JSON object to stdout and no table
  --quiet         no table
  --timeout T     the longest to wait, as a duration (default 2m)
  --lang L        the language of the messages: en, pt, es or sv
  --version
```

**Languages.** The program speaks English, Portuguese, Spanish and Swedish: the window of a double click, the installer, `doctor`, the table, the
help and the errors (the extension has the same four). It uses the language of the system (on Windows the one of your user; elsewhere `LC_ALL`,
`LC_MESSAGES`, `LANGUAGE` or `LANG`), English when it is none of them. To choose: `LIBRESPEED_ROUTE_LANG=pt` (for everything) or `--lang pt` (for a
trace). The line for the page (`LSR1`), the JSON and the exit codes do not depend on the language.

Options go before the host. Only a **public** address is traced (an internal one, loopback, documentation addresses and the like are
refused with exit code 2).

The table goes to **stderr** and one line to **stdout**, so you can copy only the line and still see the table:

```
librespeed-route better.example.org | clip        # Windows
librespeed-route better.example.org | pbcopy      # macOS
librespeed-route better.example.org | xclip -selection clipboard   # Linux
```

then press "Paste and show" on the page. The line looks like this:

```
LSR1!n=a026c48e3a0e0fb8!f=4!t=8.8.8.8!s=1791758173!a=0.1.0+traceroute!h=1,-p,1.85,3,3;...;6,-c,2.8,3,3;10,72.14.208.80,18.19,3,3;11,*,0,3,0;12,8.8.8.8,16.36,3,3
```

Only **public addresses** are written in it: a hop of your internal network is `-p`, one in the range of CGNAT (`100.64.0.0/10`) is `-c`,
one that did not answer is `*`; the page gets how long each took, never the addresses of your network. The table in your terminal does
show them.

Exit codes (stable): `0` ok, `1` usage, `2` the host cannot be traced, `3` no tracert/traceroute/tracepath on this system, `4` failed (the
tool, the output, or a route that does not fit in the text of the page), `130` interrupted. It never asks a question and never reads its
input, except in the window that a double click opens on Windows (install, uninstall or quit).

## How it is built

- **One address, one absolute path, no shell.** The name is resolved once, the address must be public, and the tool is started by its
  absolute path (`%SystemRoot%\System32\tracert.exe`, `/usr/sbin/traceroute`, ...) with the **address** as the last argument, never the name.
  A host that starts with `-` is refused.
- **It stops the tool** after five hops in a row with no answer and after two minutes (or `--timeout`), and keeps what it had.
- **The output is read as bytes.** The words of the tool come in the language and code page of your system (a Portuguese `tracert` writes
  "Esgotado o tempo limite do pedido." in CP850); they are never read, only numbers, times with a unit, `*` and addresses.
- **The parser is a port of the one of the page** (`route-parse.js`), held to it by tests: the real outputs of the page repository are the
  vectors (`testdata/vectors`), and a test runs the page's own parser with Node over 2000 outputs made at random and compares the two.
  `testdata/PIN` says which version of the page file is the one.

## Development

```
go vet ./... && go test ./...
(cd extension && npm ci && npx playwright install chromium && npx playwright test)   # the extension in a browser, with the real program (-tags e2efake)
LSROUTE_LIVE=1 go test -run Loopback -v ./internal/route/    # the real tool of the system, on 127.0.0.1
```

`go test` needs Node only for the comparison with the page (it is skipped without it). CI runs everything on Linux, Windows and macOS.

## What is checked, and what is not

Checked by tests: the parser against the real outputs of Linux (IPv4 and IPv6, `traceroute` and `tracepath`) and Windows in Portuguese, and
against the page's parser on random outputs; the scopes of addresses against the PHP backend of the page's repository; the text for the
page (the page reads back what was written, no internal address is ever in it, the limit of 1900 characters is exact); the command line that is
built for each system; that a tool that never ends is killed; the exit codes.

By hand: the whole chain (the program started by the browser, the `tracert` of Windows, the line handed to the page) on Windows 11 with
Firefox and with Chrome, with the release 0.2.1. By the CI, on Linux, Windows and macOS: the tests above, the installer (the manifests, and the
registry of the user on Windows), the host (frames, requests, replies, cancel, lock), and the extension in the Chromium of Playwright with the real
program and a canned traceroute.

**Not verified**, and to be said before anyone relies on it:

- macOS and Linux with a real browser starting the program, and Edge (it reads the same key as Chromium, so it should work).
- The language of Windows (`GetUserDefaultLocaleName`) was only compiled for Windows, not run there; the translations were written by one person and
  should be read by someone who speaks each language.
- Chrome and Edge need the id of the folder-loaded extension to be the one in the program: there is no store item yet.

- macOS: the output of `traceroute` and `traceroute6` of a real Mac (the vectors for macOS were written by hand from the documented format).
- Windows: the exact bytes of `tracert` when its output goes to a pipe (the code page), `tracert` in languages other than Portuguese,
  and a multi-hop route on the CI runners (only the loopback is traced there).
- `traceroute` of a Linux without root on a network that filters ICMP, and a snap or container with another `PATH`.
- A binary that is not signed: SmartScreen (Windows) and Gatekeeper (macOS) may warn.
- The text is cut as the page does at 1900 characters (the other routers of a hop are dropped first); a very long IPv6 route can be refused.
- The page quirk: a hop with no answer that comes after the last public hop is not shown by the page (it cuts the end of the route before it
  puts the internal hops back).

## Licence

MIT OR Apache-2.0, at your choice (`LICENSE-MIT`, `LICENSE-APACHE`).
