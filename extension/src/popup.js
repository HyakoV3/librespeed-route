/*
 * The popup of the extension: what it says about the tab that is open (the program is there or not, the page can be traced or not) and one
 * button. The work is in background.js; this only asks it, and writes what it answers with textContent.
 */
(function () {
  "use strict";

  var api = typeof browser !== "undefined" ? browser : chrome;
  var t = function (key, substitution) {
    return api.i18n.getMessage(key, substitution === undefined ? undefined : [substitution]) || key;
  };
  var byId = function (id) {
    return document.getElementById(id);
  };
  var message = byId("message");
  var hint = byId("hint");
  var trace = byId("trace");
  var result = byId("result");
  var tabId = null;

  document.title = t("popupTitle");
  byId("title").textContent = t("popupTitle");
  trace.textContent = t("traceButton");
  byId("copy").textContent = t("copyButton");

  var detail = byId("detail");
  var site = { host: "" };

  // What the browser said when it could not start the program (shown as text, small, under the message)
  function showDetail(text) {
    detail.textContent = text || "";
    detail.hidden = !text;
  }

  function show(key, substitution, install) {
    showDetail("");
    message.textContent = t(key, substitution);
    hint.hidden = !install;
    byId("command").textContent = install ? "librespeed-route install-native-host" : "";
    if (install) message.textContent = t(key) + " " + t("installHint");
  }

  var errors = {
    refused: "errorRefused",
    "no-tool": "errorNoTool",
    failed: "errorFailed",
    silent: "errorSilent",
    busy: "errorBusy",
    "too-long": "errorTooLong",
    interrupted: "errorInterrupted",
    moved: "errorMoved",
    "bad-request": "errorBadRequest",
    "not-https": "stateNotHttps",
    "no-host": "stateNoHost",
    update: "stateOldHost",
  };

  function ask(request) {
    return api.runtime.sendMessage(request);
  }

  // The button that a page of the site may have on its own: allowed here, one site at a time, by this click and the prompt of the browser
  function drawSite(allowed) {
    byId("site").hidden = false;
    byId("sitetext").textContent = t(allowed ? "siteOnText" : "siteOffText", site.host);
    byId("allow").hidden = allowed;
    byId("revoke").hidden = !allowed;
    byId("allow").disabled = false;
  }

  function siteNoteSet(key) {
    byId("sitenote").textContent = key ? t(key) : "";
    byId("sitenote").hidden = !key;
  }

  function start() {
    return api.tabs.query({ active: true, currentWindow: true }).then(function (tabs) {
      tabId = tabs && tabs[0] ? tabs[0].id : null;
      if (typeof tabId !== "number") return show("stateNotHttps");
      return Promise.all([ask({ type: "describe", tabId: tabId }), ask({ type: "hello" })]).then(function (both) {
        var tab = both[0];
        var host = both[1];
        if (host.state === "no-host") {
          show("stateNoHost", undefined, true);
          return showDetail(host.detail);
        }
        if (host.state === "update") return show("stateOldHost");
        if (!tab || !tab.origin) return show("stateNotHttps");
        show("stateReady", tab.host);
        trace.hidden = false;
        if (tab.canAllow) {
          site.host = tab.host;
          drawSite(tab.allowed === true);
        }
      });
    });
  }

  trace.addEventListener("click", function () {
    trace.disabled = true;
    show("stateTracing");
    ask({ type: "trace", tabId: tabId }).then(
      function (answer) {
        if (answer && answer.state === "done") {
          show(answer.delivered === false ? "stateNotDelivered" : "stateDone");
          byId("line").value = answer.lsr1;
          result.hidden = false;
          trace.hidden = true;
        } else {
          show(errors[answer && answer.code] || "errorFailed", undefined, answer && answer.code === "no-host");
          if (answer && answer.code === "no-host") showDetail(answer.detail);
          trace.disabled = false;
        }
      },
      function () {
        show("errorFailed");
        trace.disabled = false;
      },
    );
  });

  byId("allow").textContent = t("allowButton");
  byId("revoke").textContent = t("revokeButton");

  // The request to the browser is the first thing done in the click, with nothing awaited before it (the browser wants it to come from the
  // click). The background is told first, without waiting, so that it knows what the permission is for even if the prompt closes this popup.
  byId("allow").addEventListener("click", function () {
    if (!site.host) return;
    var host = site.host;
    byId("allow").disabled = true;
    siteNoteSet("");
    ask({ type: "allow-intent", host: host, tabId: tabId });
    var asked;
    try {
      asked = api.permissions.request({ origins: ["https://" + host + "/*"] });
    } catch (error) {
      asked = Promise.resolve(false);
    }
    asked.then(
      function (granted) {
        if (!granted) {
          byId("allow").disabled = false;
          return siteNoteSet("siteDenied");
        }
        return ask({ type: "allow-done", host: host }).then(function (answer) {
          drawSite(!!(answer && answer.allowed));
          if (!(answer && answer.allowed)) siteNoteSet("errorFailed");
        });
      },
      function () {
        byId("allow").disabled = false;
        siteNoteSet("siteDenied");
      },
    );
  });

  byId("revoke").addEventListener("click", function () {
    if (!site.host) return;
    byId("revoke").disabled = true;
    ask({ type: "revoke", host: site.host }).then(function () {
      byId("revoke").disabled = false;
      siteNoteSet("");
      drawSite(false);
    });
  });

  byId("copy").addEventListener("click", function () {
    var line = byId("line");
    var done = function () {
      byId("copy").textContent = t("copiedButton");
    };
    if (navigator.clipboard && navigator.clipboard.writeText)
      navigator.clipboard.writeText(line.value).then(done, function () {});
    else {
      line.select();
      done();
    }
  });

  start();
})();
