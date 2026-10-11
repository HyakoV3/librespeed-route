# librespeed-route

Traces the route from **your computer** to a server, with the tool your system already has, and writes the result in the text that the
route page of [better pages of LibreSpeed](https://github.com/HyakoV3/speedtest) (`route-better.html`, "You → server") reads.

It does not send probes of its own. It runs `tracert` (Windows), `traceroute` (macOS, Linux) or `tracepath` (Linux, when there is no
`traceroute`), reads what they print and does three things with it: a table for you, a line of text for the page, and (with `--json`) a
JSON object for a script.

**Status: 0.1, not released.** What is checked and what is not is at the end.

## Use

```
librespeed-route [options] HOST

  -4, -6          trace over IPv4 or IPv6 (default: the first public address of the host)
  --json          write a JSON object to stdout and no table
  --quiet         no table
  --timeout T     the longest to wait, as a duration (default 2m)
  --version
```

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
input.

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
LSROUTE_LIVE=1 go test -run Loopback -v ./internal/route/    # the real tool of the system, on 127.0.0.1
```

`go test` needs Node only for the comparison with the page (it is skipped without it). CI runs everything on Linux, Windows and macOS.

## What is checked, and what is not

Checked by tests: the parser against the real outputs of Linux (IPv4 and IPv6, `traceroute` and `tracepath`) and Windows in Portuguese, and
against the page's parser on random outputs; the scopes of addresses against the PHP backend of the page's repository; the text for the
page (the page reads back what was written, no internal address is ever in it, the limit of 1900 characters is exact); the command line that is
built for each system; that a tool that never ends is killed; the exit codes.

**Not verified**, and to be said before anyone relies on it:

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
