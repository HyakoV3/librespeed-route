// The popup without a browser: the codes that the background can answer have a text in every language, every message that the code asks for
// exists in the four languages and is not empty, every id that the script looks for is in the page, and the links go to the project.
const { test, expect } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const src = path.resolve(__dirname, "..", "src");
const read = (...p) => fs.readFileSync(path.join(src, ...p), "utf8");
const LANGS = ["en", "pt_BR", "es", "sv"];
const messages = (lang) => JSON.parse(read("_locales", lang, "messages.json"));

const popup = read("popup.js");
const html = read("popup.html");
const background = read("background.js");

// The map { code: "messageKey" } of the popup
function errorMap() {
  const body = /var errors = \{([\s\S]*?)\};/.exec(popup)[1];
  const map = {};
  for (const m of body.matchAll(/"?([\w-]+)"?:\s*"(\w+)"/g)) map[m[1]] = m[2];
  return map;
}

test("every code that the background can answer to the popup has a text, in every language", () => {
  const map = errorMap();
  const codes = /var CODES = \{([\s\S]*?)\};/.exec(background)[1];
  // Only for the page, never for the popup: it does not ask a page for a trace
  const pageOnly = new Set(["not-allowed", "no-gesture"]);
  const known = [...codes.matchAll(/"?([\w-]+)"?:\s*1/g)].map((m) => m[1]);
  expect(known.length).toBeGreaterThan(5);
  for (const code of [...known, "not-https", "bad-request"]) {
    if (pageOnly.has(code)) continue;
    expect(map[code], `the popup has no text for ${code}`).toBeTruthy();
    for (const lang of LANGS)
      expect(
        messages(lang)[map[code]]?.message,
        `${lang} ${code}`,
      ).toBeTruthy();
  }
});

test("the four languages have the same messages, none empty, with the same placeholders", () => {
  const english = messages("en");
  for (const lang of LANGS) {
    const all = messages(lang);
    expect(Object.keys(all).sort(), lang).toEqual(Object.keys(english).sort());
    for (const [key, entry] of Object.entries(all)) {
      expect(entry.message?.trim(), `${lang} ${key}`).toBeTruthy();
      const used = [...entry.message.matchAll(/\$(\w+)\$/g)].map((m) => m[1]);
      expect(used.sort(), `${lang} ${key}`).toEqual(
        Object.keys(english[key].placeholders || {}).sort(),
      );
      expect(
        Object.keys(entry.placeholders || {}).sort(),
        `${lang} ${key}`,
      ).toEqual(Object.keys(english[key].placeholders || {}).sort());
    }
  }
});

test("every message key written in the popup exists", () => {
  const english = messages("en");
  const keys = new Set(
    [...popup.matchAll(/"([a-z]+(?:[A-Z][a-z0-9]*)+)"/g)].map((m) => m[1]),
  );
  // Names of properties and of the browser that are not messages
  const notMessages = new Set(["currentWindow", "userAgent"]);
  expect(keys.size).toBeGreaterThan(20);
  for (const key of keys)
    if (!notMessages.has(key)) expect(english, key).toHaveProperty(key);
});

test("every id that the popup script looks for is in the page, and every control has a name", () => {
  const ids = new Set([...html.matchAll(/\sid="(\w+)"/g)].map((m) => m[1]));
  for (const m of popup.matchAll(/byId\("(\w+)"\)/g))
    expect(ids, m[1]).toContain(m[1]);
  // A button, summary and link get their text from a message; a textarea gets its name from aria-label set in the script
  expect(popup).toMatch(/setAttribute\("aria-label", t\("resultLabel"\)\)/);
  expect(html).not.toMatch(/innerHTML/);
  expect(popup).not.toMatch(/innerHTML|insertAdjacentHTML|document\.write/);
});

test("the links go to the project, open on a click, and do not hand over the page", () => {
  const links = [...html.matchAll(/<a\s[^>]*>/g)].map((m) => m[0]);
  expect(links.length).toBe(3);
  for (const tag of links) {
    expect(tag).toMatch(
      /href="https:\/\/github\.com\/HyakoV3\/librespeed-route(?:[/#][^"]*)?"/,
    );
    expect(tag).toMatch(/rel="noopener noreferrer"/);
  }
});

test("the popup asks for no permission that the manifests do not have, and the manifests did not grow", () => {
  for (const browser of ["chromium", "firefox"]) {
    const manifest = JSON.parse(
      fs.readFileSync(
        path.resolve(__dirname, "..", browser, "manifest.json"),
        "utf8",
      ),
    );
    expect(manifest.permissions.sort(), browser).toEqual([
      "activeTab",
      "nativeMessaging",
      "scripting",
      "storage",
    ]);
    expect(manifest.optional_host_permissions).toEqual(["https://*/*"]);
    expect(manifest.host_permissions).toBeUndefined();
  }
});
