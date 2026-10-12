/*
 * Route back for LibreSpeed pages: the background of the extension.
 *
 * The person clicks the icon of the extension on the page of a server and presses "Trace" in the popup. The extension asks the program
 * librespeed-route (a native messaging host, see docs/protocol.md) to trace the route from this computer to THE SERVER OF THE PAGE: the
 * target is the origin of the tab, read here from the browser, never something that the page says. The program answers with one line of
 * text (LSR1) in which the internal addresses of the network are left out, and it is handed to the page that is on the tab, which reads it
 * as it reads a text that was pasted. Nothing is kept, nothing is logged, nothing is sent anywhere else.
 *
 * No page can start a trace: there is no content script and no listener for messages of pages or of other extensions.
 */
(function () {
  "use strict";

  var api = typeof browser !== "undefined" ? browser : chrome;
  var HOST = "io.github.hyakov3.librespeed_route";
  var PROTOCOL = 1;
  // One trace at a time, and one for the same origin in 30 seconds
  var GAP_MS = 30000;
  var running = false;

  // When each origin was traced last, kept in the session storage: the worker of an extension is stopped and started again by the browser
  function lastTrace(origin) {
    if (!api.storage || !api.storage.session) return Promise.resolve(0);
    return api.storage.session.get("recent").then(function (got) {
      var recent = (got && got.recent) || {};
      return typeof recent[origin] === "number" ? recent[origin] : 0;
    });
  }

  function markTrace(origin, now) {
    if (!api.storage || !api.storage.session) return Promise.resolve();
    return api.storage.session.get("recent").then(function (got) {
      var recent = (got && got.recent) || {};
      recent[origin] = now;
      return api.storage.session.set({ recent: recent });
    });
  }

  function id() {
    var bytes = new Uint8Array(8);
    crypto.getRandomValues(bytes);
    return Array.prototype.map
      .call(bytes, function (b) {
        return ("0" + b.toString(16)).slice(-2);
      })
      .join("");
  }

  // The origin of an address that can be traced: https, a host name, no user name; "" for anything else (the host checks it again)
  function originOf(url) {
    try {
      var u = new URL(url);
      if (u.protocol !== "https:" || u.username || u.password || !u.hostname) return "";
      return u.origin;
    } catch (error) {
      return "";
    }
  }

  // Asks the host for one thing and waits for its one answer: { reply } or { error: "no-host" | "update" | the code of the host }
  function ask(message, onPort) {
    return new Promise(function (resolve) {
      var port;
      var done = false;
      function finish(result) {
        if (done) return;
        done = true;
        try {
          port.disconnect();
        } catch (error) {
          // The port is already closed
        }
        resolve(result);
      }
      try {
        port = api.runtime.connectNative(HOST);
      } catch (error) {
        resolve({ error: "no-host" });
        return;
      }
      port.onMessage.addListener(function (reply) {
        finish({ reply: reply });
      });
      port.onDisconnect.addListener(function () {
        // The host is not there (not installed, the browser may not start it, or this extension is not one that the program lets in), or
        // it ended without answering. What the browser says about it is kept, to be shown (it is the text of the browser, not of a page)
        var problem = api.runtime.lastError && api.runtime.lastError.message;
        if (!done) {
          done = true;
          resolve({ error: "no-host", detail: String(problem || "") + " [extension id " + api.runtime.id + "]" });
        }
      });
      if (onPort) onPort(port);
      port.postMessage(message);
    });
  }

  // Is the program there, and which version of the protocol does it speak: "ok" with its version and tool, "no-host" or "update"
  function hello() {
    return ask({ v: 1, id: id(), type: "hello" }).then(function (r) {
      if (r.error) return { state: r.error, detail: r.detail };
      var m = r.reply;
      if (!m || m.type !== "hello" || m.protocol !== PROTOCOL) return { state: "update" };
      return { state: "ok", version: String(m.version || ""), tool: String(m.tool || "") };
    });
  }

  // The state of a tab for the popup: the origin that would be traced (or "" when the tab is not one of an https page)
  function describe(tabId) {
    return api.tabs.get(tabId).then(function (tab) {
      return { origin: originOf(tab && tab.url), host: tab && tab.url ? safeHost(tab.url) : "" };
    });
  }

  function safeHost(url) {
    try {
      return new URL(url).hostname;
    } catch (error) {
      return "";
    }
  }

  // Hands the line to the page of the tab, in its top frame only, for its own origin only
  function deliver(tabId, origin, lsr1) {
    return api.scripting.executeScript({
      target: { tabId: tabId, frameIds: [0] },
      func: function (text, to) {
        window.postMessage({ source: "librespeed-route-extension", v: 1, lsr1: text }, to);
      },
      args: [lsr1, origin],
    });
  }

  // Traces to the origin of the tab. Resolves { state: "done", lsr1 } or { state: "error", code }.
  function trace(tabId) {
    if (running) return Promise.resolve({ state: "error", code: "busy" });
    return api.tabs.get(tabId).then(function (tab) {
      var origin = originOf(tab && tab.url);
      if (!origin) return { state: "error", code: "not-https" };
      var now = Date.now();
      return lastTrace(origin).then(function (before) {
        if (running || (before && now - before < GAP_MS)) return { state: "error", code: "busy" };
        running = true;
        return markTrace(origin, now).then(function () {
          return run(tabId, origin);
        });
      });
    });
  }

  // The trace itself: ask the host, and hand the line to the tab if it is still on the same origin
  function run(tabId, origin) {
    return ask({ v: 1, id: id(), type: "trace", origin: origin, family: 0 })
      .then(function (r) {
        if (r.error) return { state: "error", code: r.error, detail: r.detail };
        var m = r.reply;
        if (!m || m.type === "error")
          return { state: "error", code: m && typeof m.code === "string" ? m.code : "failed" };
        if (m.type !== "result" || typeof m.lsr1 !== "string" || m.lsr1.length > 1900)
          return { state: "error", code: "failed" };
        // The tab must still be on the same origin: a page that was left in the meantime does not get the route of the other one
        return api.tabs.get(tabId).then(function (again) {
          if (originOf(again && again.url) !== origin) return { state: "error", code: "moved" };
          return deliver(tabId, origin, m.lsr1).then(
            function () {
              return { state: "done", lsr1: m.lsr1 };
            },
            function () {
              return { state: "done", lsr1: m.lsr1, delivered: false };
            },
          );
        });
      })
      .then(
        function (result) {
          running = false;
          return result;
        },
        function () {
          running = false;
          return { state: "error", code: "failed" };
        },
      );
  }

  // Only the popup of this extension talks to the background: a message from anywhere else (another extension, a page) is not answered
  function handle(message, sender, sendResponse) {
    if (!sender || sender.id !== api.runtime.id || !message || typeof message.type !== "string") return false;
    var work;
    if (message.type === "hello") work = hello();
    else if (message.type === "describe" && typeof message.tabId === "number") work = describe(message.tabId);
    else if (message.type === "trace" && typeof message.tabId === "number") work = trace(message.tabId);
    else return false;
    work.then(sendResponse, function () {
      sendResponse({ state: "error", code: "failed" });
    });
    return true;
  }

  api.runtime.onMessage.addListener(handle);

  // For the end to end test (the worker of an extension cannot be reached by a page)
  self.librespeedRoute = { hello: hello, describe: describe, trace: trace, handle: handle };
})();
