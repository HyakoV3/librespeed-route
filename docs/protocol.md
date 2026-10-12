# Protocol: browser extension ↔ program

The extension (`extension/`) asks the program (`librespeed-route`, started by the browser as a *native messaging host*) to trace the route from
this computer to the server of the page that the person is on, and hands the answer to that page.

## Who talks to whom

```
person clicks the icon on the tab of https://server.example  →  popup  →  background  →  connectNative  →  program (host)
program traces with the tracert/traceroute/tracepath of the system  →  one reply frame  →  background  →  postMessage into the tab
```

- **No page can start a trace.** The extension has no content script, no `externally_connectable`, and no listener for messages of pages or of
  other extensions. The target is the origin of the tab, read by the browser (`tabs.get`) when the person presses "Trace".
- The answer is handed to the **top frame** of the same tab only if the tab is still on the same origin; the page gets
  `window.postMessage({ source: "librespeed-route-extension", v: 1, lsr1 }, <its own origin>)`.

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
`%LOCALAPPDATA%\librespeed-route\` and the keys `HKCU\Software\{Google\Chrome,Chromium,Microsoft\Edge,Mozilla}\NativeMessagingHosts\<name>`.
Only the browsers that have been used get a manifest (`--all`: every one). `uninstall-native-host` removes what was written and only that;
`doctor` says what is where. On Windows a folder with `% " ^ & !` is refused (cmd.exe starts the host of a Chromium browser). Browsers in a
snap or a flatpak are not supported.

## Tests

`go test ./...` covers the frames, the requests (strictly), the callers, the closed replies, the purity of what is written (no internal address,
only the keys of the set), the cancel, the lock, and the installer for each system. `extension/tests/e2e.spec.js` loads the extension in the
Chromium of Playwright with the real program built with `-tags e2efake` (a canned traceroute, no network) and a manifest in the profile.
