/*
 * The bridge between a page and the extension: it is in the pages of the sites that the person allowed, in their top frame only (the
 * background registers it for those sites, see background.js), and it does three things:
 *
 *   hello  the page asks whether the extension is here: answered here, at once, without asking the program anything
 *   trace  the page asks for a trace of the route to its own server: sent to the background only if the person clicked on the page a moment
 *          ago and the page is the one in view; the target is never in the message, the background takes it from the tab
 *   cancel the page gives up: the port to the background is closed, and that stops the program
 *
 * It reads only messages of this same window and origin, with exactly the keys of docs/protocol.md; everything else is ignored. It answers
 * with the state, and the route in the form that the page already reads. Nothing here is kept.
 */
(function () {
  "use strict";

  // Once for each page: the background may inject it again into a page that was open when the site was allowed
  if (window.__librespeedRouteBridge) return;
  window.__librespeedRouteBridge = true;

  var api = typeof browser !== "undefined" ? browser : chrome;
  var ID = /^[A-Za-z0-9_-]{8,32}$/;
  var CODES = [
    "not-allowed",
    "no-gesture",
    "busy",
    "no-host",
    "update",
    "refused",
    "no-tool",
    "failed",
    "silent",
    "interrupted",
    "too-long",
    "moved",
  ];
  var current = null;

  function post(message) {
    try {
      window.postMessage(message, location.origin);
    } catch (error) {
      // A page with an opaque origin cannot be answered
    }
  }

  function state(id, name, code) {
    var message = { source: "librespeed-route-extension", v: 1, type: "state", id: id, state: name };
    if (code) message.code = code;
    post(message);
  }

  function finish(id, name, code) {
    current = null;
    state(id, name, code);
  }

  function start(id) {
    if (current) return state(id, "error", "busy");
    // A click that the person made a moment ago, on the page that is in view: a page that is not looked at cannot trace
    var activation = navigator.userActivation;
    if (!activation || activation.isActive !== true || document.visibilityState !== "visible")
      return state(id, "error", "no-gesture");
    var port;
    try {
      port = api.runtime.connect({ name: "lsr-page" });
    } catch (error) {
      return state(id, "error", "failed");
    }
    current = { id: id, port: port };
    port.onMessage.addListener(function (m) {
      if (!current || current.port !== port || !m || typeof m !== "object") return;
      if (m.type === "state" && m.state === "tracing") return state(id, "tracing");
      if (m.type === "result" && typeof m.lsr1 === "string" && m.lsr1.length <= 1900) {
        post({ source: "librespeed-route-extension", v: 1, lsr1: m.lsr1 });
        return finish(id, "done");
      }
      if (m.type === "error") return finish(id, "error", CODES.indexOf(m.code) >= 0 ? m.code : "failed");
    });
    port.onDisconnect.addListener(function () {
      // The background went away (the extension was stopped or updated) without answering
      if (current && current.port === port) finish(id, "error", "interrupted");
    });
    port.postMessage({ type: "trace", gesture: true });
  }

  window.addEventListener("message", function (event) {
    if (event.source !== window || event.origin !== location.origin) return;
    var d = event.data;
    if (!d || typeof d !== "object" || d.source !== "librespeed-route-page" || d.v !== 1) return;
    var keys = Object.keys(d);
    if (keys.length !== 4 || typeof d.type !== "string" || typeof d.id !== "string" || !ID.test(d.id)) return;
    if (d.type === "hello") return state(d.id, "ready");
    if (d.type === "trace") return start(d.id);
    if (d.type === "cancel" && current && current.id === d.id) {
      var port = current.port;
      finish(d.id, "error", "interrupted");
      port.disconnect();
    }
  });

  // For a page that was open when the extension was allowed there, or that was loaded before it: it hears that the extension is here
  state("", "ready");
})();
