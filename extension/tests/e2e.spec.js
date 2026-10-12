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
const parser = fs.readFileSync(path.join(root, "testdata", "web", "route-parse.js"), "utf8");

let work;
let hostBinary;
let extensionDir;

test.beforeAll(() => {
  work = fs.mkdtempSync(path.join(os.tmpdir(), "lsroute-e2e-"));
  hostBinary = path.join(work, process.platform === "win32" ? "librespeed-route-e2e.exe" : "librespeed-route-e2e");
  execFileSync("go", ["build", "-tags", "e2efake", "-o", hostBinary, "./cmd/librespeed-route"], {
    cwd: root,
    stdio: "inherit",
  });
  const dist = path.join(work, "dist");
  execFileSync("go", ["run", "./cmd/extbuild", "-dev", "-version", "0.1.0", "-out", dist], {
    cwd: root,
    stdio: "inherit",
  });
  extensionDir = path.join(dist, "chromium");
  // Test only: the extension may script the fake page without the click on its icon (a robot cannot click it)
  const manifestPath = path.join(extensionDir, "manifest.json");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  manifest.host_permissions = ["https://server.example/*", "https://other.example/*", "http://plain.example/*"];
  fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2));
});

test.afterAll(() => {
  fs.rmSync(work, { recursive: true, force: true });
});

async function launch({ withHost, protocol }) {
  const profile = fs.mkdtempSync(path.join(work, "profile-"));
  if (withHost) {
    const dir = path.join(profile, "NativeMessagingHosts");
    fs.mkdirSync(dir, { recursive: true });
    // The browser does not pass its environment to the host: to have a program that speaks another protocol, a script that sets it and starts
    // the real one (Linux and macOS)
    let program = hostBinary;
    if (protocol) {
      program = path.join(profile, "host-with-protocol.sh");
      fs.writeFileSync(program, `#!/bin/sh\nLSROUTE_E2E_PROTOCOL=${Number(protocol)} exec "${hostBinary}" "$@"\n`, {
        mode: 0o755,
      });
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
    args: [`--disable-extensions-except=${extensionDir}`, `--load-extension=${extensionDir}`],
  });
  let [worker] = context.serviceWorkers();
  if (!worker) worker = await context.waitForEvent("serviceworker");
  return { context, worker };
}

// A page of the "server": https://server.example, served by the test (nothing is asked of a network). It keeps what the extension posts.
async function openServer(context, origin = "https://server.example") {
  const page = await context.newPage();
  await page.route(`${origin}/**`, (route) =>
    route.fulfill({
      contentType: "text/html",
      body: `<!doctype html><title>${origin}</title><script>${parser}</script><script>
        window.__messages = [];
        window.addEventListener("message", e => window.__messages.push({ data: e.data, origin: e.origin, sameWindow: e.source === window }));
      </script><p>route page</p>`,
    }),
  );
  await page.goto(`${origin}/`);
  return page;
}

const tabIdOf = (worker, pattern) => worker.evaluate(async (p) => (await chrome.tabs.query({ url: p }))[0].id, pattern);

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

test("a trace is handed to the page of the tab, for its own origin, with the internal addresses left out", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const result = await worker.evaluate((id) => self.librespeedRoute.trace(id), tabId);
  expect(result.state).toBe("done");
  expect(result.lsr1).toMatch(/^LSR1!n=0123456789abcdef!f=4!t=8\.8\.8\.8!s=\d+!a=0\.1\.0\+traceroute!h=/);
  // No internal address, in what went to the page or in what came back to the popup
  for (const ip of ["192.168.0.1", "172.24.74.112", "100.65.61.62"]) expect(result.lsr1).not.toContain(ip);
  await expect.poll(() => page.evaluate(() => window.__messages.length)).toBe(1);
  const message = await page.evaluate(() => window.__messages[0]);
  expect(message).toEqual({
    data: { source: "librespeed-route-extension", v: 1, lsr1: result.lsr1 },
    origin: "https://server.example",
    sameWindow: true,
  });
  // The page reads it as it reads a pasted text
  const parsed = await page.evaluate((text) => window.LibreSpeedRouteParse.parse(text), message.data.lsr1);
  expect(parsed.tool).toBe("lsr1");
  expect(parsed.hops.map((h) => h.scope)).toEqual(["private", "private", "cgnat", "public", "public"]);
  expect(parsed.hops.slice(0, 3).every((h) => h.ip === null)).toBe(true);
  expect(parsed.hops[4].ip).toBe("8.8.8.8");
  await context.close();
});

test("a second trace to the same server within 30 seconds is refused, and one at a time", async () => {
  const { context, worker } = await launch({ withHost: true });
  await openServer(context);
  const tabId = await tabIdOf(worker, "https://server.example/*");
  const [first, second] = await worker.evaluate(
    (id) => Promise.all([self.librespeedRoute.trace(id), self.librespeedRoute.trace(id)]),
    tabId,
  );
  expect([first.state, second.state].sort()).toEqual(["done", "error"]);
  expect([first, second].find((r) => r.state === "error").code).toBe("busy");
  const again = await worker.evaluate((id) => self.librespeedRoute.trace(id), tabId);
  expect(again).toEqual({ state: "error", code: "busy" });
  await context.close();
});

test("a page that is left while the trace runs gets nothing", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await openServer(context);
  const other = await openServer(context, "https://other.example");
  const tabId = await tabIdOf(worker, "https://server.example/*");
  // The fake tool takes 300 ms: the tab goes to another origin meanwhile
  const pending = worker.evaluate((id) => self.librespeedRoute.trace(id), tabId);
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
  expect(await worker.evaluate((id) => self.librespeedRoute.trace(id), tabId)).toEqual({
    state: "error",
    code: "not-https",
  });
  expect(await worker.evaluate((id) => self.librespeedRoute.describe(id), tabId)).toMatchObject({ origin: "" });
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
  expect(hello.detail.length).toBeGreaterThan(`[extension id ${DEV_ID}]`.length);
  const traced = await worker.evaluate((id) => self.librespeedRoute.trace(id), tabId);
  expect(traced).toMatchObject({ state: "error", code: "no-host" });
  expect(traced.detail).toContain(DEV_ID);
  await context.close();
});

test("no page and no other extension can start a trace: the background answers only its own popup", async () => {
  const { context, worker } = await launch({ withHost: true });
  const page = await openServer(context);
  // A message that a page sends with the API of the page is not delivered to the extension (there is no externally_connectable): the page has
  // no chrome.runtime to send it with
  const hasRuntime = await page.evaluate(() => !!(window.chrome && chrome.runtime && chrome.runtime.sendMessage));
  expect(hasRuntime).toBe(false);
  const manifest = JSON.parse(fs.readFileSync(path.join(extensionDir, "manifest.json"), "utf8"));
  expect(manifest.externally_connectable).toBeUndefined();
  expect(manifest.content_scripts).toBeUndefined();
  expect(manifest.permissions.sort()).toEqual(["activeTab", "nativeMessaging", "scripting", "storage"]);
  // The source has no listener for other extensions or pages
  const source = fs.readFileSync(path.join(root, "extension", "src", "background.js"), "utf8");
  expect(source).not.toMatch(/onMessageExternal|onConnectExternal|content_scripts/);
  await context.close();
});

test("the popup of the extension says that the tab is not a page to trace, and has the texts of its language", async () => {
  const { context, worker } = await launch({ withHost: true });
  const popup = await context.newPage();
  await popup.goto(`chrome-extension://${DEV_ID}/popup.html`);
  await expect(popup.locator("#title")).toHaveText("Route to this server");
  // The tab that is active is the popup itself: not an https page
  await expect(popup.locator("#message")).toContainText("Open the page of a server over https");
  expect(worker.url()).toContain(DEV_ID);
  await context.close();
});

test("the four languages have the same messages, and every message that the code asks for is there", async () => {
  const dir = path.join(root, "extension", "src", "_locales");
  const keys = (lang) => Object.keys(JSON.parse(fs.readFileSync(path.join(dir, lang, "messages.json"), "utf8"))).sort();
  const english = keys("en");
  for (const lang of ["pt_BR", "es", "sv"]) expect(keys(lang), lang).toEqual(english);
  const used = new Set();
  for (const file of ["popup.js", "popup.html"]) {
    const text = fs.readFileSync(path.join(root, "extension", "src", file), "utf8");
    for (const m of text.matchAll(/"((?:state|error|install|trace|copy|copied|popup|ext|action)[A-Z][A-Za-z]*)"/g))
      used.add(m[1]);
  }
  for (const key of used) expect(english, key).toContain(key);
  const manifest = fs.readFileSync(path.join(root, "extension", "chromium", "manifest.json"), "utf8");
  for (const m of manifest.matchAll(/__MSG_(\w+)__/g)) expect(english).toContain(m[1]);
});

test("a program that speaks another protocol is told to be updated", async () => {
  const { context, worker } = await launch({ withHost: true, protocol: 2 });
  expect(await worker.evaluate(() => self.librespeedRoute.hello())).toEqual({ state: "update" });
  await context.close();
});

test("the background answers its own popup and nobody else", async () => {
  const { context, worker } = await launch({ withHost: true });
  const result = await worker.evaluate(async () => {
    const own = { id: chrome.runtime.id };
    const answers = [];
    const handle = self.librespeedRoute.handle;
    // Another extension (another id), a page (no id), and nothing at all
    for (const sender of [{ id: "abcdefghijklmnopabcdefghijklmnop" }, {}, null, { url: "https://server.example/" }]) {
      answers.push({ returned: handle({ type: "hello" }, sender, () => answers.push("answered")) });
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
      answers.push({ returned: handle(message, own, () => answers.push("answered")) });
    }
    // The popup: answered
    const own2 = await new Promise((resolve) => {
      const returned = handle({ type: "hello" }, own, (r) => resolve({ returned, state: r.state }));
    });
    return { answers, own2 };
  });
  expect(result.answers).toEqual(Array(10).fill({ returned: false }));
  expect(result.own2.returned).toBe(true);
  await context.close();
});
