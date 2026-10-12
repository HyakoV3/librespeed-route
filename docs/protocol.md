# Protocol: browser extension ↔ program

The extension (`extension/`) asks the program (`librespeed-route`, started by the browser as a *native messaging host*) to trace the route from
this computer to the server of the page that the person is on, and hands the answer to that page.

## Who talks to whom

```
person clicks the icon on the tab of https://server.example  →  popup  →  background  →  connectNative  →  program (host)
or, on a site that the person allowed: clicks the button of the page  →  bridge  →  background  →  connectNative  →  program (host)
program traces with the tracert/traceroute/tracepath of the system  →  one reply frame  →  background  →  postMessage into the tab
```

- **A page can ask for a trace only if the person allowed that site** (see "The button on the page" below). Without that, the extension has
  no content script in the page, no `externally_connectable`, and no listener for messages of pages or of other extensions. The target is
  the origin of the tab, read by the browser (`tabs.get`), never something that a page says.
- The answer is handed to the **top frame** of the same tab only if the tab is still on the same origin; the page gets
  `window.postMessage({ source: "librespeed-route-extension", v: 1, lsr1 }, <its own origin>)`.

## The button on the page

A page of a server may have its own button "Trace the way back". For that the person allows the site once, in the popup of the extension
("Allow this site"): the popup asks the browser for the optional permission `https://<host name>/*` (a prompt of the browser) and the
background writes the record of that click. A site is allowed only when **both** exist, the permission of the browser and the record; a
permission given to all sites in the settings of the browser turns nothing on. The background registers the bridge (`bridge.js`) for the allowed
host names only, in their top frame only, and registers it again from the records every time it starts (the browser forgets registered scripts
when the extension is updated). "Turn it off" in the popup takes both away.

Messages, all by `window.postMessage(message, location.origin)`; the bridge reads only `event.source === window`, `event.origin ===
location.origin`, exactly these keys, and `id` matching `/^[A-Za-z0-9_-]{8,32}$/`:

| From | Message | Meaning |
|---|---|---|
| page | `{ source: "librespeed-route-page", v: 1, type: "hello", id }` | is the extension here? Answered by the bridge, without the program |
| page | `{ …, type: "trace", id }` | trace the route to my server |
| page | `{ …, type: "cancel", id }` | give up; the program stops the tool |
| extension | `{ source: "librespeed-route-extension", v: 1, type: "state", id, state, code? }` | `state` is `ready` (`id` is `""` for the one that the bridge posts when it loads), `tracing`, `done` or `error`; `code` is one of `not-allowed no-gesture busy no-host update refused no-tool failed silent interrupted too-long moved` |
| extension | `{ source: "librespeed-route-extension", v: 1, lsr1 }` | the route, as above; comes before `state: "done"` |

What the extension requires before a trace that a page asked for (nothing of it is read from the message):

1. the bridge has a port of its own (`lsr-page`); the sender is this extension, has a tab, and is the **top frame** (`frameId` 0);
2. the target is the origin of that tab (https, no user name) and is the origin that the browser says the bridge is in;
3. the host name is allowed (permission **and** record), and the tab is the one in view (`tab.active`);
4. the bridge says that the person clicked on the page a moment ago (`navigator.userActivation.isActive`) and that the page is visible;
5. nothing else is being traced, and the host name was not traced in the last 30 seconds (the host name, not the origin: another port of the
   same site is the same site);
6. while it traces, the tab shows a mark on the icon of the extension; if the page reloads, navigates, closes or sends `cancel`, the port closes and
   the program stops the tool.

A page is told only a code, never the text of the browser or the id of the extension. The program is not told who asked (the same two
messages, `hello` and `trace`, come only from the extension).

## The host

- Name: `io.github.hyakov3.librespeed_route` (`internal/nativehost.HostName`).
- The browser starts the program with its own arguments, and the program is the host only for these (anything else that starts like a
  browser call exits at once, reading and writing nothing):
  - Chrome, Chromium, Edge: `chrome-extension://<id>/` (and `--parent-window=<n>` after it on Windows); `<id>` must be one of `ChromeIDs`.
  - Firefox: `<absolute path of the manifest> <gecko id>`, the path ending in `io.github.hyakov3.librespeed_route.json`; the id must be one of
    `GeckoIDs`.
- **Ids.** Chrome: the extension that is loaded from the folder (`extbuild -dev`) has the id `gcgijalhbacnoacnklibiffbacdchjgm`, from the key in
  `extension/dev/public-key.txt`. The ids of the stores are added to `ChromeIDs` (and the manifests) when the items exist. Firefox:
  `librespeed-route@hyakov3.github.io` (`browser_specific_settings.gecko.id`).
- **Frames:** 4 bytes of length in the byte order of the machine, then JSON. A request is 1 to 4096 bytes; a reply is 1 to 4096 bytes (the browser
  takes 1 MiB). One request per process; the program answers once and ends.

### Requests

One JSON object, nothing after it, no field that is not listed:

| type | fields |
|---|---|
| `hello` | `v` (the number 1), `id` (`[A-Za-z0-9_-]{1,32}`) |
| `trace` | the same + `origin` (`https://host` or `https://host:port`: lower case letters, digits, `-` and `.`; no user name, path, query or fragment) + `family` (the number 0, 4 or 6; may be left out) |
| `cancel` | the same as `hello`; sent while a trace runs (a frame of any kind, or the end of the input, cancels it too) |

The host name of `origin` is resolved once, must be a public address (the program traces the address, never the name) and the port is not used.

### Replies (a closed set; no free text, no address, nothing on the error output)

```
{ "v":1, "id":..., "type":"hello",  "protocol":1, "version":"0.1.0", "tool":"tracert|traceroute|tracepath|none" }
{ "v":1, "id":..., "type":"result", "lsr1":"LSR1!n=...!h=1,-p,1.85,3,3;..." }
{ "v":1, "id":..., "type":"error",  "code":"refused|no-tool|failed|silent|busy|bad-request|interrupted|too-long" }
```

The `lsr1` is the text that the route page reads: only public addresses are in it (`-p` for an internal hop, `-c` for CGNAT, `*` for a silent
one). The `--json` of the command has the real addresses of the internal network and is **never** reachable from here.

### Limits

- One trace at a time and not two within 10 seconds, across processes (files `trace.lock` and `trace.last` in the cache folder of the user,
  with no name or address in them); a lock that is left over is taken away after 150 seconds.
- The extension: one trace at a time and one per origin in 30 seconds (kept in `storage.session`).
- The tool is stopped after 5 silent hops in a row and after 120 seconds.

## Installing the host

`librespeed-route install-native-host [--all]` copies the program to a folder of the user and writes one manifest per browser family:
Linux `~/.config/{google-chrome,chromium,microsoft-edge}/NativeMessagingHosts/` and `~/.mozilla/native-messaging-hosts/`; macOS
`~/Library/Application Support/{Google/Chrome,Chromium,Microsoft Edge,Mozilla}/NativeMessagingHosts/`; Windows the files in
`%LOCALAPPDATA%\librespeed-route\chromium\` and `...\firefox\` (always named like the host, because Firefox gives the path of the manifest to the program and the program takes only that name) and the keys `HKCU\Software\{Google\Chrome,Chromium,Microsoft\Edge,Mozilla}\NativeMessagingHosts\<name>`.
Only the browsers that have been used get a manifest (`--all`: every one). `uninstall-native-host` removes what was written and only that;
`doctor` says what is where. On Windows a folder with `% " ^ & !` is refused (cmd.exe starts the host of a Chromium browser). Browsers in a
snap or a flatpak are not supported.

## Tests

`go test ./...` covers the frames, the requests (strictly), the callers, the closed replies, the purity of what is written (no internal address,
only the keys of the set), the cancel, the lock, and the installer for each system. `extension/tests/e2e.spec.js` loads the extension in the
Chromium of Playwright with the real program built with `-tags e2efake` (a canned traceroute, no network) and a manifest in the profile.
