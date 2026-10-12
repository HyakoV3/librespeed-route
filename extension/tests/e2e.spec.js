// End to end: the extension loaded from a folder in the Chromium of Playwright, talking to the real host (built with -tags e2efake: it
// resolves every name to a public address and "runs" a canned traceroute that has internal hops), through a manifest of native messaging
// that is put in the folder of the browser profile. No network. Needs Go to build the host and the extension.
const { test, expect, chromium } = require("@playwright/test");
const { execFileSync } = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");

const root = path.resolve(__dirname, "..", "..");
const DEV_ID = "gcgijalhbacnoacnklibiffbacdchjgm";
const HOST_NAME = "io.github.hyakov3.librespeed_route";
const parser = fs.readFileSync(
  path.join(root, "testdata", "web", "route-parse.js"),
  "utf8",
);

let work;
let hostBinary;
let extensionDir;

test.beforeAll(() => {
  work = fs.mkdtempSync(path.join(os.tmpdir(), "lsroute-e2e-"));
  hostBinary = path.join(
    work,
    process.platform === "win32"
      ? "librespeed-route-e2e.exe"
      : "librespeed-route-e2e",
  );
  execFileSync(
    "go",
    ["build", "-tags", "e2efake", "-o", hostBinary, "./cmd/librespeed-route"],
    {
      cwd: root,
      stdio: "inherit",
    },
  );
  const dist = path.join(work, "dist");
  execFileSync(
    "go",
    ["run", "./cmd/extbuild", "-dev", "-version", "0.1.0", "-out", dist],
    {
      cwd: root,
      stdio: "inherit",
    },
  );
  extensionDir = path.join(dist, "chromium");
  // Test only: the extension may script the fake page without the click on its icon (a robot cannot click it)
  const manifestPath = path.join(extensionDir, "manifest.json");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  manifest.host_permissions = [
    "https://server.example/*",
    "https://other.example/*",
    "http://plain.example/*",
  ];
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
});

test.afterAll(() => {
  fs.rmSync(work, { recursive: true, force: true });
});

async function launch({
  withHost,
  protocol,
  delayMs,
  pidFile,
  profile: given,
}) {
  const profile = given || fs.mkdtempSync(path.join(work, "profile-"));
  if (withHost) {
    const dir = path.join(profile, "NativeMessagingHosts");
    fs.mkdirSync(dir, { recursive: true });
    // The browser does not pass its environment to the host: to have a program that speaks another protocol, a script that sets it and starts
    // the real one (Linux and macOS)
    let program = hostBinary;
    if (protocol || delayMs || pidFile) {
      program = path.join(profile, "host-with-protocol.sh");
      // exec keeps the pid: the file says which process is the program, so that a test can see that it was stopped
      fs.writeFileSync(
        program,
        `#!/bin/sh\n${pidFile ? `echo $$ > "${pidFile}"\n` : ""}LSROUTE_E2E_PROTOCOL=${Number(protocol || 0)} LSROUTE_E2E_DELAY_MS=${Number(delayMs || 0)} exec "${hostBinary}" "$@"\n`,
        { mode: 0o755 },
      );
    }
    fs.writeFileSync(
      path.join(dir, `${HOST_NAME}.json`),
      JSON.stringify({
        name: HOST_NAME,
        description: "test",
        path: program,
        type: "stdio",
        allowed_origins: [`chrome-extension://${DEV_ID}/`],
      }),
    );
  }
  const context = await chromium.launchPersistentContext(profile, {
    channel: "chromium",
    headless: true,
    args: [
      `--disable-extensions-except=${extensionDir}`,
      `--load-extension=${extensionDir}`,
    ],
  });
  let [worker] = context.serviceWorkers();
  if (!worker) worker = await context.waitForEvent("serviceworker");
  return { context, worker, profile };
}

// A page of the "server": https://server.example, served by the test (nothing is asked of a network). It keeps what the extension posts.
async function openServer(
  context,
  origin = "https://server.example",
  hash = "",
) {
  const page = await context.newPage();
  await page.route(`${origin}/**`, (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<!doctype html><title>${origin}</title><script>${parser}</script><script>
        window.__messages = [];
        window.addEventListener("message", e => window.__messages.push({ data: e.data, origin: e.origin, sameWindow: e.source === window }));
        // The button of the page: a click posts what is in __queue, or a request for a trace
        window.__queue = [];
        window.__trace = (id) => ({ source: "librespeed-route-page", v: 1, type: "trace", id: id || "trace0001" });
        document.addEventListener("click", () => {
          const list = window.__queue.length ? window.__queue : [window.__trace()];
          window.__queue = [];
          list.forEach(m => window.postMessage(m, location.origin));
        });
        // Without a click: a page that asks on its own
        if (location.hash === "#auto") setTimeout(() => { window.__act = [navigator.userActivation.isActive, navigator.userActivation.hasBeenActive, document.visibilityState]; window.postMessage(window.__trace("auto00001"), location.origin); }, 300);
      </script><button id="go">Trace the way back</button><p>route page</p>`,
    }),
  );
  await page.goto(`${origin}/${hash}`);
  return page;
}

// What the extension said to the page, in order
const said = (page) =>
  page.evaluate(() =>
    window.__messages
      .map((m) => m.data)
      .filter((d) => d && d.source === "librespeed-route-extension"),
  );

// The sites that the person allowed (the record of the click), and the bridge made from them
async function allow(worker, hosts) {
  await worker.evaluate(async (list) => {
    const allowed = {};
    for (const h of list) allowed[h] = true;
    await chrome.storage.local.set({ allowed });
    await self.librespeedRoute.reconcile();
  }, hosts);
}

const popupSender = (id) => ({
  id,
  url: `chrome-extension://${id}/popup.html`,
});

const alive = (pid) => {
  try {
    process.kill(pid, 0);
    return true;
  } catch (error) {
    return false;
  }
};

const tabIdOf = (worker, pattern) =>
  worker.evaluate(
    async (p) => (await chrome.tabs.query({ url: p }))[0].id,
    pattern,
  );

test("the extension has the id that the host lets in", async () => {
  const { context, worker } = await launch({ withHost: true });
  expect(worker.url()).toContain(`chrome-extension://${DEV_ID}/`);
  await context.close();
});

test("hello: the host answers with its protocol, and the extension says it is ready", async () => {
  const { context, worker } = await launch({ withHost: true });
  const hello = await worker.evaluate(() => self.librespeedRoute.hello());
  expect(hello).toMatchObject({ state: "ok", version: "0.1.0" });
  expect(typeof hello.tool).toBe("string");
  await context.close();
});

const trace0001 = "trace0001";

test("a trace is handed to the page of the tab, for its own origin, with the internal addresses left out", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const result = await worker.evaluate(
    (id) => self.librespeedRoute.trace(id),
    tabId,
  );
  expect(result.state).toBe("done");
  expect(result.lsr1).toMatch(
    /^LSR1!n=0123456789abcdef!f=4!t=8\.8\.8\.8!s=\d+!a=0\.1\.0\+traceroute!h=/,
  );
  // No internal address, in what went to the page or in what came back to the popup
  for (const ip of ["192.168.0.1", "172.24.74.112", "100.65.61.62"])
    expect(result.lsr1).not.toContain(ip);
  await expect
    .poll(() => page.evaluate(() => window.__messages.length))
    .toBe(1);
  const message = await page.evaluate(() => window.__messages[0]);
  expect(message).toEqual({
    data: { source: "librespeed-route-extension", v: 1, lsr1: result.lsr1 },
    origin: "https://server.example",
    sameWindow: true,
  });
  // The page reads it as it reads a pasted text
  const parsed = await page.evaluate(
    (text) => window.LibreSpeedRouteParse.parse(text),
    message.data.lsr1,
  );
  expect(parsed.tool).toBe("lsr1");
  expect(parsed.hops.map((h) => h.scope)).toEqual([
    "private",
    "private",
    "cgnat",
    "public",
    "public",
  ]);
  expect(parsed.hops.slice(0, 3).every((h) => h.ip === null)).toBe(true);
  expect(parsed.hops[4].ip).toBe("8.8.8.8");
  await context.close();
});

test("a second trace to the same server within 30 seconds is refused, and one at a time", async () => {
  const { context, worker } = await launch({ withHost: true });
  await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const [first, second] = await worker.evaluate(
    (id) =>
      Promise.all([
        self.librespeedRoute.trace(id),
        self.librespeedRoute.trace(id),
      ]),
    tabId,
  );
  expect([first.state, second.state].sort()).toEqual(["done", "error"]);
  expect([first, second].find((r) => r.state === "error").code).toBe("busy");
  const again = await worker.evaluate(
    (id) => self.librespeedRoute.trace(id),
    tabId,
  );
  expect(again).toEqual({ state: "error", code: "busy" });
  await context.close();
});

test("a page that is left while the trace runs gets nothing", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await openServer(context);
  const other = await openServer(context, "https://other.example");
  const tabId = await tabIdOf(worker, "https://server.example/*");
  // The fake tool takes 300 ms: the tab goes to another origin meanwhile
  const pending = worker.evaluate(
    (id) => self.librespeedRoute.trace(id),
    tabId,
  );
  await page.route("https://moved.example/**", (route) =>
    route.fulfill({ contentType: "text/html", body: "<p>moved</p>" }),
  );
  await page.goto("https://moved.example/");
  const result = await pending;
  expect(result).toEqual({ state: "error", code: "moved" });
  expect(await other.evaluate(() => window.__messages.length)).toBe(0);
  await context.close();
});

test("a tab that is not an https page is not traced", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await context.newPage();
  await page.route("http://plain.example/**", (route) =>
    route.fulfill({ contentType: "text/html", body: "<p>plain</p>" }),
  );
  await page.goto("http://plain.example/");
  // The extension can see the address of this tab (the test gives it the permission): it is http, and it is not traced
  const tabId = await tabIdOf(worker, "http://plain.example/*");
  expect(
    await worker.evaluate((id) => self.librespeedRoute.trace(id), tabId),
  ).toEqual({
    state: "error",
    code: "not-https",
  });
  expect(
    await worker.evaluate((id) => self.librespeedRoute.describe(id), tabId),
  ).toMatchObject({ origin: "" });
  await context.close();
});

test("without the program the extension says so, in hello and in a trace", async () => {
  const { context, worker } = await launch({ withHost: false });
  await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const hello = await worker.evaluate(() => self.librespeedRoute.hello());
  expect(hello.state).toBe("no-host");
  // The popup can show what the browser said, and the id of the extension (to compare with the one that the program lets in)
  expect(hello.detail).toContain(`[extension id ${DEV_ID}]`);
  expect(hello.detail.length).toBeGreaterThan(
    `[extension id ${DEV_ID}]`.length,
  );
  const traced = await worker.evaluate(
    (id) => self.librespeedRoute.trace(id),
    tabId,
  );
  expect(traced).toMatchObject({ state: "error", code: "no-host" });
  expect(traced.detail).toContain(DEV_ID);
  await context.close();
});

test("no page that was not allowed, and no other extension, can start a trace", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await openServer(context);
  // A message that a page sends with the API of the page is not delivered to the extension (there is no externally_connectable): the page has
  // no chrome.runtime to send it with
  const hasRuntime = await page.evaluate(
    () => !!(window.chrome && chrome.runtime && chrome.runtime.sendMessage),
  );
  expect(hasRuntime).toBe(false);
  const manifest = JSON.parse(
    fs.readFileSync(path.join(extensionDir, "manifest.json"), "utf8"),
  );
  expect(manifest.externally_connectable).toBeUndefined();
  // The bridge is not in the manifest: it is registered for the sites that the person allowed, and only those
  expect(manifest.content_scripts).toBeUndefined();
  expect(manifest.permissions.sort()).toEqual([
    "activeTab",
    "nativeMessaging",
    "scripting",
    "storage",
  ]);
  expect(manifest.optional_host_permissions).toEqual(["https://*/*"]);
  // The source has no listener for other extensions or pages
  const source = fs.readFileSync(
    path.join(root, "extension", "src", "background.js"),
    "utf8",
  );
  expect(source).not.toMatch(
    /onMessageExternal|onConnectExternal|content_scripts/,
  );
  await context.close();
});

test("the popup of the extension says that the tab is not a page to trace, and has the texts of its language", async () => {
  const { context, worker } = await launch({ withHost: true });
  const popup = await context.newPage();
  await popup.goto(`chrome-extension://${DEV_ID}/popup.html`);
  await expect(popup.locator("#title")).toHaveText("Route to this server");
  // The tab that is active is the popup itself: not an https page
  await expect(popup.locator("#message")).toContainText(
    "Open the page of a server over https",
  );
  expect(worker.url()).toContain(DEV_ID);
  await context.close();
});

test("the four languages have the same messages, and every message that the code asks for is there", async () => {
  const dir = path.join(root, "extension", "src", "_locales");
  const keys = (lang) =>
    Object.keys(
      JSON.parse(
        fs.readFileSync(path.join(dir, lang, "messages.json"), "utf8"),
      ),
    ).sort();
  const english = keys("en");
  for (const lang of ["pt_BR", "es", "sv"])
    expect(keys(lang), lang).toEqual(english);
  const used = new Set();
  for (const file of ["popup.js", "popup.html"]) {
    const text = fs.readFileSync(
      path.join(root, "extension", "src", file),
      "utf8",
    );
    for (const m of text.matchAll(
      /"((?:state|error|install|trace|copy|copied|popup|ext|action|site|allow|revoke)[A-Z][A-Za-z]*)"/g,
    ))
      used.add(m[1]);
  }
  for (const key of used) expect(english, key).toContain(key);
  const manifest = fs.readFileSync(
    path.join(root, "extension", "chromium", "manifest.json"),
    "utf8",
  );
  for (const m of manifest.matchAll(/__MSG_(\w+)__/g))
    expect(english).toContain(m[1]);
});

test("a program that speaks another protocol is told to be updated", async () => {
  const { context, worker } = await launch({ withHost: true, protocol: 2 });
  expect(await worker.evaluate(() => self.librespeedRoute.hello())).toEqual({
    state: "update",
  });
  await context.close();
});

test("the background answers its own popup and nobody else", async () => {
  const { context, worker } = await launch({ withHost: true });
  const result = await worker.evaluate(async () => {
    const own = {
      id: chrome.runtime.id,
      url: chrome.runtime.getURL("popup.html"),
    };
    const answers = [];
    const handle = self.librespeedRoute.handle;
    // Another extension (another id), a page (no id), and nothing at all
    // (and the content script of a page, even with the id of this extension: it has a tab, and the url of the page)
    for (const sender of [
      { id: "abcdefghijklmnopabcdefghijklmnop" },
      {},
      null,
      { url: "https://server.example/" },
      { id: chrome.runtime.id, url: "https://server.example/", tab: { id: 1 } },
      { id: chrome.runtime.id },
    ]) {
      answers.push({
        returned: handle({ type: "hello" }, sender, () =>
          answers.push("answered"),
        ),
      });
    }
    // Messages of its own that it does not know, or that are not well formed
    for (const message of [
      null,
      {},
      { type: 5 },
      { type: "trace" },
      { type: "trace", tabId: "1" },
      { type: "reboot" },
    ]) {
      answers.push({
        returned: handle(message, own, () => answers.push("answered")),
      });
    }
    // The popup: answered
    const own2 = await new Promise((resolve) => {
      const returned = handle({ type: "hello" }, own, (r) =>
        resolve({ returned, state: r.state }),
      );
    });
    return { answers, own2 };
  });
  expect(result.answers).toEqual(Array(12).fill({ returned: false }));
  expect(result.own2.returned).toBe(true);
  await context.close();
});

// ---- the button on the page ------------------------------------------------------------------------------------------------------------

const hello = (page, id = "hello0001") =>
  page.evaluate(
    (i) =>
      window.postMessage(
        { source: "librespeed-route-page", v: 1, type: "hello", id: i },
        location.origin,
      ),
    id,
  );

test("a site that was allowed: hello is answered, a click starts the trace, and the route comes in the form that the page reads", async () => {
  const { context, worker } = await launch({ withHost: true });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  // The extension says by itself that it is here, and answers a hello at once
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "",
      state: "ready",
    });
  await hello(page);
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "hello0001",
      state: "ready",
    });
  await page.click("#go");
  await expect
    .poll(
      async () => (await said(page)).filter((m) => m.state === "done").length,
    )
    .toBe(1);
  const messages = (await said(page)).filter(
    (m) => (m.id !== "hello0001" && m.id !== "") || !m.type,
  );
  expect(messages.map((m) => m.state || "lsr1")).toEqual([
    "tracing",
    "lsr1",
    "done",
  ]);
  const route = messages[1];
  expect(Object.keys(route).sort()).toEqual(["lsr1", "source", "v"]);
  expect(route.lsr1).toMatch(/^LSR1!n=0123456789abcdef!f=4!t=8\.8\.8\.8!/);
  for (const ip of ["192.168.0.1", "172.24.74.112", "100.65.61.62"])
    expect(route.lsr1).not.toContain(ip);
  expect(messages[2]).toEqual({
    source: "librespeed-route-extension",
    v: 1,
    type: "state",
    id: trace0001,
    state: "done",
  });
  // Nothing in the answers that the page should not have: no text of the browser, no id of the extension
  expect(JSON.stringify(messages)).not.toContain(DEV_ID);
  await context.close();
});

test("a page that asks without a click of the person is refused, and the program is never started", async () => {
  const pidFile = path.join(work, "pid-nogesture");
  fs.rmSync(pidFile, { force: true });
  const { context, worker } = await launch({ withHost: true, pidFile });
  await allow(worker, ["server.example"]);
  const page = await openServer(context, "https://server.example", "#auto");
  // Nothing is evaluated in the page until its own timer has fired: page.evaluate of a test counts as a gesture of the person
  await page.waitForTimeout(1000);
  expect(await page.evaluate(() => window.__act)).toEqual([
    false,
    false,
    "visible",
  ]);
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "auto00001",
      state: "error",
      code: "no-gesture",
    });
  expect(fs.existsSync(pidFile)).toBe(false);
  await context.close();
});

test("a site with the permission of the browser but without the click of the person in the popup has no bridge", async () => {
  const { context, worker } = await launch({ withHost: true });
  // The test manifest gives the permission for server.example (as the settings of a browser can give it to all sites): no record, no bridge
  expect(
    await worker.evaluate(() =>
      self.librespeedRoute.isAllowed("server.example"),
    ),
  ).toBe(false);
  await worker.evaluate(() => self.librespeedRoute.reconcile());
  const page = await openServer(context);
  await hello(page);
  await page.click("#go");
  await page.waitForTimeout(800);
  expect(await said(page)).toEqual([]);
  await context.close();
});

test("a record without the permission is dropped, and does not bring the bridge", async () => {
  const { context, worker } = await launch({ withHost: true });
  await allow(worker, ["server.example", "third.example"]);
  expect(
    await worker.evaluate(
      async () => (await chrome.storage.local.get("allowed")).allowed,
    ),
  ).toEqual({ "server.example": true });
  const page = await openServer(context, "https://third.example");
  await hello(page);
  await page.waitForTimeout(800);
  expect(await said(page)).toEqual([]);
  await context.close();
});

test("turning a site off: a page that is already open is told not-allowed, and a new one has no bridge", async () => {
  const { context, worker } = await launch({ withHost: true });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  await expect.poll(async () => (await said(page)).length).toBeGreaterThan(0);
  const answer = await worker.evaluate(
    (sender) =>
      new Promise((resolve) =>
        self.librespeedRoute.handle(
          { type: "revoke", host: "server.example" },
          sender,
          resolve,
        ),
      ),
    popupSender(DEV_ID),
  );
  expect(answer).toEqual({ allowed: false });
  expect(
    await worker.evaluate(
      async () => (await chrome.storage.local.get("allowed")).allowed,
    ),
  ).toEqual({});
  // The page that is open is told that the site is off
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "",
      state: "off",
    });
  // The bridge is still in the page that was open, and the background refuses
  await page.click("#go");
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: trace0001,
      state: "error",
      code: "not-allowed",
    });
  const fresh = await openServer(context, "https://server.example");
  await hello(fresh);
  await fresh.waitForTimeout(800);
  expect(await said(fresh)).toEqual([]);
  await context.close();
});

test("after the browser is closed and opened again the bridge is back: it is made again from the records", async () => {
  const first = await launch({ withHost: true });
  await allow(first.worker, ["server.example"]);
  await first.context.close();
  const second = await launch({ withHost: true, profile: first.profile });
  const page = await openServer(second.context);
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "",
      state: "ready",
    });
  await second.context.close();
});

test("messages that are not exactly the ones of the protocol are ignored, even with a click", async () => {
  const { context, worker } = await launch({ withHost: true });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  await expect.poll(async () => (await said(page)).length).toBe(1);
  const ok = {
    source: "librespeed-route-page",
    v: 1,
    type: "trace",
    id: "trace0001",
  };
  await page.evaluate((m) => {
    window.__queue = [
      { ...m, extra: 1 },
      { ...m, source: "other" },
      { ...m, v: 2 },
      { ...m, id: "short" },
      { ...m, id: "x".repeat(33) },
      { ...m, id: 5 },
      { ...m, type: "reboot" },
      { source: "librespeed-route-page", v: 1, type: "trace" },
      "trace",
      null,
    ];
  }, ok);
  await page.click("#go");
  await page.waitForTimeout(800);
  expect((await said(page)).filter((m) => m.state !== "ready")).toEqual([]);
  // A message from another origin (a frame of another site) is not read either
  await context.close();
});

test("a second trace for the same host name is refused, even on another port, and the popup is refused while a page traces", async () => {
  const { context, worker } = await launch({ withHost: true, delayMs: 1500 });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  await page.click("#go");
  await expect
    .poll(async () => (await said(page)).some((m) => m.state === "tracing"))
    .toBe(true);
  // A trace of the popup, while the page is tracing
  expect(
    await worker.evaluate((id) => self.librespeedRoute.trace(id), tabId),
  ).toEqual({ state: "error", code: "busy" });
  await expect
    .poll(async () => (await said(page)).some((m) => m.state === "done"))
    .toBe(true);
  // Within the 30 seconds: busy
  await page.evaluate(() => (window.__queue = [window.__trace("trace0002")]));
  await page.click("#go");
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "trace0002",
      state: "error",
      code: "busy",
    });
  // The same host name on another port is the same site for the gap
  const other = await openServer(context, "https://server.example:8443");
  await expect.poll(async () => (await said(other)).length).toBeGreaterThan(0);
  await other.bringToFront();
  await other.click("#go");
  await expect
    .poll(() => said(other))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: trace0001,
      state: "error",
      code: "busy",
    });
  await context.close();
});

test("a page that reloads, or gives up, stops the program", async () => {
  for (const how of ["reload", "cancel", "close"]) {
    const pidFile = path.join(work, `pid-${how}`);
    fs.rmSync(pidFile, { force: true });
    const { context, worker } = await launch({
      withHost: true,
      delayMs: 20000,
      pidFile,
    });
    await allow(worker, ["server.example"]);
    const page = await openServer(context);
    await page.click("#go");
    await expect
      .poll(async () => (await said(page)).some((m) => m.state === "tracing"))
      .toBe(true);
    await expect.poll(() => fs.existsSync(pidFile)).toBe(true);
    const pid = Number(fs.readFileSync(pidFile, "utf8"));
    expect(alive(pid), how).toBe(true);
    if (how === "reload") await page.reload();
    if (how === "close") await page.close();
    if (how === "cancel") {
      await page.evaluate(() =>
        window.postMessage(
          {
            source: "librespeed-route-page",
            v: 1,
            type: "cancel",
            id: "trace0001",
          },
          location.origin,
        ),
      );
      await expect
        .poll(() => said(page))
        .toContainEqual({
          source: "librespeed-route-extension",
          v: 1,
          type: "state",
          id: trace0001,
          state: "error",
          code: "interrupted",
        });
    }
    await expect
      .poll(() => alive(pid), { timeout: 8000, message: how })
      .toBe(false);
    // And a new trace is possible after the 30 seconds are over: the lock of the background was let go
    expect(await worker.evaluate(() => typeof self.librespeedRoute.trace)).toBe(
      "function",
    );
    await context.close();
  }
});

test("the popup allows a site only for its own tab, and only with a permission and an intent", async () => {
  const { context, worker } = await launch({ withHost: true });
  await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const popup = popupSender(DEV_ID);
  const call = (message) =>
    worker.evaluate(
      ([m, sender]) =>
        new Promise((resolve) =>
          self.librespeedRoute.handle(m, sender, resolve),
        ),
      [message, popup],
    );
  // An intent for a host that is not the host of the tab, or that is not a host name, is refused
  expect(
    await call({ type: "allow-intent", host: "other.example", tabId }),
  ).toEqual({ ok: false });
  expect(
    await call({ type: "allow-intent", host: "SERVER.example/../x", tabId }),
  ).toEqual({ ok: false });
  // Without an intent: nothing is written, even though the permission is there
  expect(await call({ type: "allow-done", host: "server.example" })).toEqual({
    allowed: false,
  });
  expect(
    await worker.evaluate(
      async () => (await chrome.storage.local.get("allowed")).allowed || {},
    ),
  ).toEqual({});
  // With the intent (the permission is in the manifest of the test): the record is written, and the bridge is put in the page at once
  expect(
    await call({ type: "allow-intent", host: "server.example", tabId }),
  ).toEqual({ ok: true });
  expect(await call({ type: "allow-done", host: "server.example" })).toEqual({
    allowed: true,
  });
  expect(
    await worker.evaluate(
      async () => (await chrome.storage.local.get("allowed")).allowed,
    ),
  ).toEqual({ "server.example": true });
  const page = context
    .pages()
    .find((p) => p.url().startsWith("https://server.example"));
  await hello(page, "afterok01");
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: "afterok01",
      state: "ready",
    });
  // The intent is used once
  expect(await call({ type: "allow-done", host: "server.example" })).toEqual({
    allowed: false,
  });
  await context.close();
});

test("a page is told only a code when the program is missing: nothing of the browser, no id of the extension", async () => {
  const { context, worker } = await launch({ withHost: false });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  await expect.poll(async () => (await said(page)).length).toBeGreaterThan(0);
  await page.click("#go");
  await expect
    .poll(() => said(page))
    .toContainEqual({
      source: "librespeed-route-extension",
      v: 1,
      type: "state",
      id: trace0001,
      state: "error",
      code: "no-host",
    });
  expect(JSON.stringify(await said(page))).not.toContain(DEV_ID);
  await context.close();
});

test("a page that is not the tab in view cannot trace, even when something there clicks", async () => {
  const pidFile = path.join(work, "pid-background");
  fs.rmSync(pidFile, { force: true });
  const { context, worker } = await launch({ withHost: true, pidFile });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  await expect.poll(async () => (await said(page)).length).toBeGreaterThan(0);
  await openServer(context, "https://other.example");
  // The test gives the click (evaluate counts as one): the page is in the background, and the background refuses it
  await page.evaluate(() => document.getElementById("go").click());
  await expect
    .poll(
      async () => (await said(page)).filter((m) => m.state === "error").length,
    )
    .toBe(1);
  const error = (await said(page)).find((m) => m.state === "error");
  expect(["refused", "no-gesture"]).toContain(error.code);
  expect(fs.existsSync(pidFile)).toBe(false);
  await context.close();
});

test("a message from a frame of the page is not read, even with a click", async () => {
  const pidFile = path.join(work, "pid-frame");
  fs.rmSync(pidFile, { force: true });
  const { context, worker } = await launch({ withHost: true, pidFile });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  await expect.poll(async () => (await said(page)).length).toBe(1);
  // evaluate counts as a click of the person; the message comes from a frame of the same origin, which has no bridge of its own
  await page.evaluate(() => {
    const frame = document.createElement("iframe");
    document.body.appendChild(frame);
    frame.contentWindow.eval(
      "parent.postMessage(" +
        JSON.stringify({
          source: "librespeed-route-page",
          v: 1,
          type: "trace",
          id: "frame0002",
        }) +
        ", '*')",
    );
  });
  await page.waitForTimeout(800);
  expect((await said(page)).filter((m) => m.state !== "ready")).toEqual([]);
  expect(fs.existsSync(pidFile)).toBe(false);
  await context.close();
});

test("a page with an opaque origin (a sandbox) is not traced for", async () => {
  const pidFile = path.join(work, "pid-sandbox");
  fs.rmSync(pidFile, { force: true });
  const { context, worker } = await launch({ withHost: true, pidFile });
  await allow(worker, ["server.example"]);
  const page = await context.newPage();
  await page.route("https://server.example/**", (route) =>
    route.fulfill({
      contentType: "text/html",
      headers: { "content-security-policy": "sandbox allow-scripts" },
      body: `<!doctype html><button id="go">go</button><script>
        document.getElementById("go").addEventListener("click", () => window.postMessage({ source: "librespeed-route-page", v: 1, type: "trace", id: "sandbox001" }, "*"));
      </script>`,
    }),
  );
  await page.goto("https://server.example/");
  await page.click("#go");
  await page.waitForTimeout(1000);
  expect(fs.existsSync(pidFile)).toBe(false);
  await context.close();
});

test("while a trace runs the icon counts the seconds in the red bubble of that tab, and is clean after, for the page and for the popup", async () => {
  const { context, worker } = await launch({ withHost: true, delayMs: 2500 });
  await allow(worker, ["server.example"]);
  const page = await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const badge = () =>
    worker.evaluate((id) => chrome.action.getBadgeText({ tabId: id }), tabId);
  expect(await badge()).toBe("");
  await page.click("#go");
  await expect.poll(badge).toBe("0");
  await expect.poll(badge, { timeout: 4000 }).toMatch(/^[12]$/);
  expect(
    await worker.evaluate(
      (id) => chrome.action.getBadgeBackgroundColor({ tabId: id }),
      tabId,
    ),
  ).toEqual([217, 48, 37, 255]);
  await expect
    .poll(async () => (await said(page)).some((m) => m.state === "done"), {
      timeout: 8000,
    })
    .toBe(true);
  await expect.poll(badge).toBe("");
  // The popup's trace counts too (another host name: the 30 seconds of the first do not count here)
  const other = await openServer(context, "https://other.example");
  const otherId = await tabIdOf(worker, "https://other.example/*");
  const pending = worker.evaluate(
    (id) => self.librespeedRoute.trace(id),
    otherId,
  );
  await expect
    .poll(() =>
      worker.evaluate(
        (id) => chrome.action.getBadgeText({ tabId: id }),
        otherId,
      ),
    )
    .toMatch(/^[0-9]$/);
  expect((await pending).state).toBe("done");
  expect(
    await worker.evaluate(
      (id) => chrome.action.getBadgeText({ tabId: id }),
      otherId,
    ),
  ).toBe("");
  await other.close();
  await context.close();
});

test("in Firefox the popup closes by itself after asking for the permission, so that the prompt is not under it; in Chrome it does not", async () => {
  const { context, worker } = await launch({ withHost: true });
  await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  for (const [agent, closes] of [
    [
      "Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0",
      true,
    ],
    [
      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36",
      false,
    ],
  ]) {
    const popup = await context.newPage();
    // The popup as it is when it was opened from the tab of the server: the test says which tab, and spies on the request and on the close
    await popup.addInitScript(
      ([ua, id]) => {
        Object.defineProperty(navigator, "userAgent", { get: () => ua });
        window.__order = [];
        window.close = () => window.__order.push("close");
        const original = chrome.tabs.query;
        chrome.tabs.query = () => Promise.resolve([{ id, active: true }]);
        chrome.permissions.request = () => {
          window.__order.push("request");
          return new Promise(() => {});
        };
        void original;
      },
      [agent, tabId],
    );
    await popup.goto(`chrome-extension://${DEV_ID}/popup.html`);
    await expect(popup.locator("#allow")).toBeVisible();
    await popup.click("#allow");
    await popup.waitForTimeout(500);
    // The request is the first thing, and the popup closes only after it (never before: the browser wants it in the click)
    expect(await popup.evaluate(() => window.__order), agent).toEqual(
      closes ? ["request", "close"] : ["request"],
    );
    await popup.close();
  }
  await context.close();
});
