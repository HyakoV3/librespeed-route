/*
 * Route back for LibreSpeed pages: the background of the extension.
 *
 * The person clicks the icon of the extension on the page of a server and presses "Trace" in the popup. The extension asks the program
 * librespeed-route (a native messaging host, see docs/protocol.md) to trace the route from this computer to THE SERVER OF THE PAGE: the
 * target is the origin of the tab, read here from the browser, never something that the page says. The program answers with one line of
 * text (LSR1) in which the internal addresses of the network are left out, and it is handed to the page that is on the tab, which reads it
 * as it reads a text that was pasted. Nothing is kept, nothing is logged, nothing is sent anywhere else.
 *
 * A page can ask for a trace only if the person allowed that site (a click in the popup, and the permission of the browser for that one host
 * name), only in the top frame, only for its own server, only within a moment of a click on the page, one at a time and one in 30 seconds for
 * each host name. The bridge (bridge.js) is registered for the allowed sites only, and talks to this file through a port of its own; other
 * extensions and other pages have no way in.
 */
(function () {
  "use strict";

  var api = typeof browser !== "undefined" ? browser : chrome;
  var HOST = "io.github.hyakov3.librespeed_route";
  var PROTOCOL = 1;
  // One trace at a time, and one for the same host name in 30 seconds (the host name, not the origin: a site cannot go round the gap with
  // another port)
  var GAP_MS = 30000;
  var running = false;
  var BRIDGE = "bridge";
  // A host name that can be asked of the browser as a permission
  var HOSTNAME = /^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$/;
  var CODES = {
    "not-allowed": 1,
    "no-gesture": 1,
    busy: 1,
    "no-host": 1,
    update: 1,
    refused: 1,
    "no-tool": 1,
    failed: 1,
    silent: 1,
    interrupted: 1,
    "too-long": 1,
    moved: 1,
  };

  // When each host name was traced last, kept in the session storage: the worker of an extension is stopped and started again by the browser
  function lastTrace(host) {
    if (!api.storage || !api.storage.session) return Promise.resolve(0);
    return api.storage.session.get("recent").then(function (got) {
      var recent = (got && got.recent) || {};
      return typeof recent[host] === "number" ? recent[host] : 0;
    });
  }

  function markTrace(host, now) {
    if (!api.storage || !api.storage.session) return Promise.resolve();
    return api.storage.session.get("recent").then(function (got) {
      var recent = (got && got.recent) || {};
      recent[host] = now;
      return api.storage.session.set({ recent: recent });
    });
  }

  // What the page may be told: a code of the list, never the text of the browser or the id of the extension
  function pageCode(code) {
    if (code === "bad-request" || code === "not-https") return "failed";
    return CODES[code] === 1 ? code : "failed";
  }
  function ident() {
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

  // Asks the host for one thing and waits for its one answer: { reply } or { error: "no-host" | "update" | the code of the host }.
  // onPort(port, cancel) is called when the port is there: cancel() closes it (the program stops the tool) and the answer is "interrupted"
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
      if (onPort)
        onPort(port, function () {
          finish({ error: "interrupted" });
        });
      port.postMessage(message);
    });
  }

  // The seconds that a trace has been running, counted on the icon of the extension for that tab (the red bubble), until it ends. Returns the
  // function that stops it and clears the icon.
  function startBadge(tabId) {
    var started = Date.now();
    var timer = null;
    function draw() {
      try {
        api.action.setBadgeText({ tabId: tabId, text: String(Math.floor((Date.now() - started) / 1000)) });
      } catch (error) {
        // The tab is gone
      }
    }
    try {
      api.action.setBadgeBackgroundColor({ tabId: tabId, color: "#d93025" });
    } catch (error) {
      // The color is only a convenience
    }
    draw();
    timer = setInterval(draw, 1000);
    return function stop() {
      if (timer === null) return;
      clearInterval(timer);
      timer = null;
      try {
        api.action.setBadgeText({ tabId: tabId, text: "" });
      } catch (error) {
        // The tab is gone
      }
    };
  }

  // Is the program there, and which version of the protocol does it speak: "ok" with its version and tool, "no-host" or "update"
  function hello() {
    return ask({ v: 1, id: ident(), type: "hello" }).then(function (r) {
      if (r.error) return { state: r.error, detail: r.detail };
      var m = r.reply;
      if (!m || m.type !== "hello" || m.protocol !== PROTOCOL) return { state: "update" };
      return { state: "ok", version: String(m.version || ""), tool: String(m.tool || "") };
    });
  }

  // The state of a tab for the popup: the origin that would be traced (or "" when the tab is not one of an https page), and whether the page
  // of that host name may ask for a trace by itself
  function describe(tabId) {
    return api.tabs.get(tabId).then(function (tab) {
      var origin = originOf(tab && tab.url);
      var host = tab && tab.url ? safeHost(tab.url) : "";
      if (!origin || !HOSTNAME.test(host)) return { origin: origin, host: host, canAllow: false, allowed: false };
      return isAllowed(host).then(function (allowed) {
        return { origin: origin, host: host, canAllow: true, allowed: allowed };
      });
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
      var host = safeHost(tab.url);
      return lastTrace(host).then(function (before) {
        if (running || (before && now - before < GAP_MS)) return { state: "error", code: "busy" };
        running = true;
        return markTrace(host, now).then(function () {
          return run(tabId, origin);
        });
      });
    });
  }

  // The trace itself: ask the host, and hand the line to the tab if it is still on the same origin
  function run(tabId, origin) {
    var stopBadge = startBadge(tabId);
    return ask({ v: 1, id: ident(), type: "trace", origin: origin, family: 0 })
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
          stopBadge();
          return result;
        },
        function () {
          running = false;
          stopBadge();
          return { state: "error", code: "failed" };
        },
      );
  }

  // ---- the sites that the person allowed -------------------------------------------------------------------------------------------------

  // A site is allowed when BOTH are true: the browser gave the permission for its host name, and this extension has the record of the click of
  // the person in its popup. A permission that the person gave to all sites in the settings of the browser turns nothing on by itself.
  function isAllowed(host) {
    return api.storage.local.get("allowed").then(function (got) {
      var allowed = (got && got.allowed) || {};
      if (allowed[host] !== true || !HOSTNAME.test(host)) return false;
      return api.permissions.contains({ origins: ["https://" + host + "/*"] });
    });
  }

  // The bridge is registered for the allowed hosts and nothing else. The browser forgets registered scripts when the extension is updated (and
  // may on a restart), so this is made again from the records every time the worker starts; one at a time, in the order they were asked.
  var chain = Promise.resolve();
  function reconcile() {
    chain = chain.then(doReconcile, doReconcile);
    return chain;
  }

  function doReconcile() {
    return Promise.all([api.storage.local.get("allowed"), api.permissions.getAll()]).then(function (both) {
      var allowed = (both[0] && both[0].allowed) || {};
      var origins = (both[1] && both[1].origins) || [];
      var keep = {};
      var matches = [];
      Object.keys(allowed).forEach(function (host) {
        var pattern = "https://" + host + "/*";
        // A record whose permission is gone is dropped: if the browser gives the permission again some day, that is not a click of the person
        if (allowed[host] === true && HOSTNAME.test(host) && origins.indexOf(pattern) >= 0) {
          keep[host] = true;
          matches.push(pattern);
        }
      });
      var changed = Object.keys(allowed).length !== matches.length;
      return (changed ? api.storage.local.set({ allowed: keep }) : Promise.resolve())
        .then(function () {
          return api.scripting.unregisterContentScripts({ ids: [BRIDGE] }).catch(function () {
            // It was not registered
          });
        })
        .then(function () {
          if (!matches.length) return undefined;
          return api.scripting.registerContentScripts([
            {
              id: BRIDGE,
              matches: matches,
              js: ["bridge.js"],
              allFrames: false,
              runAt: "document_start",
              persistAcrossSessions: true,
            },
          ]);
        });
    });
  }

  // The click on "Allow" in the popup. The popup tells here first, then asks the browser for the permission (a prompt of the browser that may
  // close the popup): the record is written here, when the permission is there and the intent is, whoever of the two learns it first.
  var INTENT_MS = 120000;

  function intent(host, tabId) {
    if (typeof host !== "string" || !HOSTNAME.test(host) || typeof tabId !== "number") return Promise.resolve({ ok: false });
    return api.tabs.get(tabId).then(function (tab) {
      if (!originOf(tab && tab.url) || safeHost(tab.url) !== host) return { ok: false };
      return api.storage.session.set({ intent: { host: host, tabId: tabId, t: Date.now() } }).then(function () {
        return { ok: true };
      });
    });
  }

  function finalize(host) {
    return api.storage.session.get("intent").then(function (got) {
      var wanted = got && got.intent;
      if (!wanted || wanted.host !== host || Date.now() - wanted.t > INTENT_MS) return { allowed: false };
      return api.permissions.contains({ origins: ["https://" + host + "/*"] }).then(function (has) {
        if (!has) return { allowed: false };
        return api.storage.session
          .remove("intent")
          .then(function () {
            return api.storage.local.get("allowed");
          })
          .then(function (stored) {
            var allowed = (stored && stored.allowed) || {};
            allowed[host] = true;
            return api.storage.local.set({ allowed: allowed });
          })
          .then(reconcile)
          .then(function () {
            // The page that is open now is not loaded again by the browser: the bridge goes into it once
            return api.scripting
              .executeScript({ target: { tabId: wanted.tabId, frameIds: [0] }, files: ["bridge.js"] })
              .catch(function () {
                // The tab went away or moved
              });
          })
          .then(function () {
            return { allowed: true };
          });
      });
    });
  }

  // The pages of the host that are open are told that the site is off (the button on the page goes off), while the extension can still reach them:
  // once the permission is taken away it cannot
  function tellOff(host) {
    return api.tabs.query({ url: ["https://" + host + "/*"] }).then(function (tabs) {
      return Promise.all(
        (tabs || []).map(function (tab) {
          var origin = originOf(tab.url);
          if (!origin || safeHost(tab.url) !== host) return undefined;
          return api.scripting
            .executeScript({
              target: { tabId: tab.id, frameIds: [0] },
              func: function (to) {
                window.postMessage({ source: "librespeed-route-extension", v: 1, type: "state", id: "", state: "off" }, to);
              },
              args: [origin],
            })
            .catch(function () {
              // The tab went away
            });
        }),
      );
    });
  }

  function revoke(host) {
    if (typeof host !== "string" || !HOSTNAME.test(host)) return Promise.resolve({ allowed: false });
    return tellOff(host)
      .catch(function () {})
      .then(function () {
        return api.storage.local.get("allowed");
      })
      .then(function (stored) {
        var allowed = (stored && stored.allowed) || {};
        delete allowed[host];
        return api.storage.local.set({ allowed: allowed });
      })
      .then(function () {
        // Taking the permission away is for the optional ones; one that the manifest asks for stays (the record is gone, and that is what counts)
        return api.permissions.remove({ origins: ["https://" + host + "/*"] }).catch(function () {});
      })
      .then(reconcile)
      .then(function () {
        return { allowed: false };
      });
  }

  // ---- a trace that a page asked for ------------------------------------------------------------------------------------------------------

  // The bridge of a page connects with the port "lsr-page", and the first and only thing it says is { type: "trace", gesture: true }.
  // Everything that matters is taken from what the browser says about the port, never from the message.
  function onPagePort(port) {
    var sender = port.sender || {};
    if (port.name !== "lsr-page" || sender.id !== api.runtime.id || !sender.tab || sender.frameId !== 0) {
      try {
        port.disconnect();
      } catch (error) {
        // Already closed
      }
      return;
    }
    var gone = false;
    var cancel = null;
    var tabId = sender.tab.id;
    var host = "";
    var stopBadge = null;

    function endBadge() {
      if (stopBadge) stopBadge();
      stopBadge = null;
    }

    function answer(message) {
      if (gone) return;
      try {
        port.postMessage(message);
      } catch (error) {
        // The page is gone
      }
    }

    function fail(code) {
      answer({ type: "error", code: code });
      gone = true;
      try {
        port.disconnect();
      } catch (error) {
        // Already closed
      }
    }

    // The page left (it was reloaded, it navigated, the tab closed, or it gave up): the program is told to stop the tool
    port.onDisconnect.addListener(function () {
      gone = true;
      if (cancel) cancel();
    });

    port.onMessage.addListener(function onFirst(m) {
      port.onMessage.removeListener(onFirst);
      var keys = m && typeof m === "object" ? Object.keys(m) : [];
      if (keys.length !== 2 || m.type !== "trace" || m.gesture !== true) return fail("failed");
      api.tabs
        .get(tabId)
        .then(function (tab) {
          var origin = originOf(tab && tab.url);
          host = origin ? safeHost(tab.url) : "";
          // The target is the tab's own origin, and it must be the origin that the browser says the bridge is in
          if (!origin || sender.origin !== origin || tab.active !== true) return fail("refused");
          return isAllowed(host).then(function (allowed) {
            if (!allowed) return fail("not-allowed");
            var now = Date.now();
            return lastTrace(host).then(function (before) {
              if (gone) return undefined;
              if (running || (before && now - before < GAP_MS)) return fail("busy");
              running = true;
              return markTrace(host, now).then(function () {
                return pageTrace(tabId, origin);
              });
            });
          });
        })
        .catch(function () {
          fail("failed");
        });
    });

    function pageTrace(id, origin) {
      answer({ type: "state", state: "tracing" });
      stopBadge = startBadge(id);
      return ask({ v: 1, id: ident(), type: "trace", origin: origin, family: 0 }, function (_port, stop) {
        cancel = stop;
        if (gone) stop();
      })
        .then(function (r) {
          if (gone) return undefined;
          if (r.error) return fail(pageCode(r.error));
          var reply = r.reply;
          if (!reply || reply.type === "error") return fail(pageCode(reply && reply.code));
          if (reply.type !== "result" || typeof reply.lsr1 !== "string" || reply.lsr1.length > 1900) return fail("failed");
          // The tab must still be on the same origin; the line goes over the same port, so it reaches the same document or nothing
          return api.tabs.get(id).then(function (again) {
            if (originOf(again && again.url) !== origin) return fail("moved");
            answer({ type: "result", lsr1: reply.lsr1 });
            gone = true;
            try {
              port.disconnect();
            } catch (error) {
              // Already closed
            }
            return undefined;
          });
        })
        .then(
          function () {
            running = false;
            endBadge();
          },
          function () {
            running = false;
            endBadge();
            fail("failed");
          },
        );
    }
  }

  // Only the popup of this extension talks to the background by message: not another extension, not a page, and not the bridge (which has a
  // port of its own and a content script that runs in a page)
  function fromPopup(sender) {
    // The url is the one of the page of the extension; the content script of a page has the url of that page
    return !!sender && sender.id === api.runtime.id && sender.url === api.runtime.getURL("popup.html");
  }

  function handle(message, sender, sendResponse) {
    if (!fromPopup(sender) || !message || typeof message.type !== "string") return false;
    var work;
    if (message.type === "hello") work = hello();
    else if (message.type === "describe" && typeof message.tabId === "number") work = describe(message.tabId);
    else if (message.type === "trace" && typeof message.tabId === "number") work = trace(message.tabId);
    else if (message.type === "allow-intent") work = intent(message.host, message.tabId);
    else if (message.type === "allow-done" && typeof message.host === "string") work = finalize(message.host);
    else if (message.type === "revoke" && typeof message.host === "string") work = revoke(message.host);
    else return false;
    work.then(sendResponse, function () {
      sendResponse({ state: "error", code: "failed" });
    });
    return true;
  }

  api.runtime.onMessage.addListener(handle);
  api.runtime.onConnect.addListener(onPagePort);

  // The prompt of the browser may have closed the popup: the permission is noticed here too
  if (api.permissions.onAdded)
    api.permissions.onAdded.addListener(function (added) {
      (added.origins || []).forEach(function (pattern) {
        var m = /^https:\/\/([^/*]+)\/\*$/.exec(pattern);
        if (m) finalize(m[1]);
      });
    });
  if (api.permissions.onRemoved) api.permissions.onRemoved.addListener(reconcile);
  api.runtime.onInstalled.addListener(reconcile);
  if (api.runtime.onStartup) api.runtime.onStartup.addListener(reconcile);
  // Every time the worker starts: the registrations are made again from the records
  reconcile();

  // For the end to end test (the worker of an extension cannot be reached by a page)
  self.librespeedRoute = {
    hello: hello,
    describe: describe,
    trace: trace,
    handle: handle,
    reconcile: reconcile,
    isAllowed: isAllowed,
  };
})();
