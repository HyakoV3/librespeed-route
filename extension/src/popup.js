/*
 * The popup of the extension: what it says about the tab that is open (the program is there or not, the page can be traced or not) and one
 * button. The work is in background.js; this only asks it, and writes what it answers with textContent.
 */
(function () {
  "use strict";

  var api = typeof browser !== "undefined" ? browser : chrome;
  var t = function (key, substitution) {
    return (
      api.i18n.getMessage(
        key,
        substitution === undefined ? undefined : [substitution],
      ) || key
    );
  };
  var byId = function (id) {
    return document.getElementById(id);
  };
  var message = byId("message");
  var hint = byId("hint");
  var trace = byId("trace");
  var result = byId("result");
  var tabId = null;

  // How long the background may take to answer before the popup stops waiting: a trace takes up to two minutes (and the program kills the tool
  // by itself after that), everything else is an answer at once
  var ANSWER_MS = 10000;
  var TRACE_MS = 150000;

  document.documentElement.lang = api.i18n.getUILanguage
    ? api.i18n.getUILanguage()
    : "en";
  document.title = t("popupTitle");
  byId("title").textContent = t("brandName");
  byId("tracelabel").textContent = t("traceButton");
  byId("copy").textContent = t("copyButton");
  byId("line").setAttribute("aria-label", t("resultLabel"));

  // The help and the about: text of the extension only, set as text. The version is the one of the manifest, not a copy of it.
  [
    ["detailsummary", "detailsSummary"],
    ["sitetitle", "siteTitle"],
    ["sitelabel", "siteSwitch"],
    ["helplabel", "helpSummary"],
    ["aboutlabel", "aboutSummary"],
    ["helpsummary", "helpSummary"],
    ["helpnohosttitle", "helpNoHostTitle"],
    ["helpnohost", "helpNoHost"],
    ["helpoldtitle", "helpOldTitle"],
    ["helpold", "helpOld"],
    ["helppagetitle", "helpPageTitle"],
    ["helppage", "helpPage"],
    ["aboutsummary", "aboutSummary"],
    ["aboutdescription", "aboutDescription"],
    ["abouthowtitle", "aboutHowTitle"],
    ["abouthow", "aboutHow"],
    ["aboutprivacytitle", "aboutPrivacyTitle"],
    ["aboutprivacy", "aboutPrivacy"],
    ["linkrepo", "aboutLinkRepo"],
    ["linkreleases", "aboutLinkReleases"],
    ["linklicense", "aboutLinkLicense"],
  ].forEach(function (pair) {
    byId(pair[0]).textContent = t(pair[1]);
  });
  var version = "";
  try {
    version = api.runtime.getManifest().version || "";
  } catch (error) {
    // The version is only for the person who asks for help
  }
  byId("aboutversion").textContent = t("aboutVersion", version);
  byId("version").textContent = version ? "v" + version : "";
  byId("version").hidden = !version;

  // Help and About: two panels under the main part, one open at a time, opened and closed by the buttons of the footer
  [
    ["helpbutton", "helppanel", "aboutbutton", "aboutpanel"],
    ["aboutbutton", "aboutpanel", "helpbutton", "helppanel"],
  ].forEach(function (ids) {
    byId(ids[0]).addEventListener("click", function () {
      var open = byId(ids[1]).hidden;
      byId(ids[1]).hidden = !open;
      byId(ids[0]).setAttribute("aria-expanded", String(open));
      byId(ids[3]).hidden = true;
      byId(ids[2]).setAttribute("aria-expanded", "false");
    });
  });

  var detail = byId("detail");
  var site = { host: "" };

  // What the browser said when it could not start the program (shown as text, small, in a closed box under the message)
  function showDetail(text) {
    detail.textContent = text || "";
    byId("detailbox").hidden = !text;
  }

  // The answer of the background, or an error if it does not come: a popup that waits for ever is worse than one that says it failed
  function within(promise, ms) {
    return new Promise(function (resolve, reject) {
      var timer = window.setTimeout(function () {
        reject(new Error("timeout"));
      }, ms);
      promise.then(
        function (value) {
          window.clearTimeout(timer);
          resolve(value);
        },
        function (error) {
          window.clearTimeout(timer);
          reject(error);
        },
      );
    });
  }

  function show(key, substitution, install) {
    showDetail("");
    message.textContent = t(key, substitution);
    hint.hidden = !install;
    byId("command").textContent = install
      ? "librespeed-route install-native-host"
      : "";
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

  function ask(request, ms) {
    return within(
      Promise.resolve(api.runtime.sendMessage(request)),
      ms || ANSWER_MS,
    );
  }

  // The button that a page of the site may have on its own: allowed here, one site at a time, by this click and the prompt of the browser
  function drawSite(allowed) {
    byId("site").hidden = false;
    byId("sitetext").textContent = t(
      allowed ? "siteOnText" : "siteOffText",
      site.host,
    );
    byId("sitetoggle").setAttribute("aria-checked", String(allowed));
    byId("sitetoggle").disabled = false;
  }

  function siteNoteSet(key, substitution) {
    byId("sitenote").textContent = key ? t(key, substitution) : "";
    byId("sitenote").hidden = !key;
  }

  function start() {
    return api.tabs
      .query({ active: true, currentWindow: true })
      .then(function (tabs) {
        tabId = tabs && tabs[0] ? tabs[0].id : null;
        if (typeof tabId !== "number") return show("stateNotHttps");
        return Promise.all([
          ask({ type: "describe", tabId: tabId }),
          ask({ type: "hello" }),
        ]).then(function (both) {
          var tab = both[0];
          var host = both[1];
          // Something that is not an answer of the background (it should not happen): said as a failure, not left blank
          if (!host || typeof host.state !== "string")
            return show("errorFailed");
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
    ask({ type: "trace", tabId: tabId }, TRACE_MS).then(
      function (answer) {
        if (answer && answer.state === "done") {
          show(answer.delivered === false ? "stateNotDelivered" : "stateDone");
          byId("line").value = answer.lsr1;
          result.hidden = false;
          trace.hidden = true;
        } else {
          show(
            errors[answer && answer.code] || "errorFailed",
            undefined,
            answer && answer.code === "no-host",
          );
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

  // The request to the browser is the first thing done in the click, with nothing awaited before it (the browser wants it to come from the
  // click). The background is told first, without waiting, so that it knows what the permission is for even if the prompt closes this popup.
  function allowSite() {
    if (!site.host) return;
    var host = site.host;
    byId("sitetoggle").disabled = true;
    siteNoteSet("sitePending");
    ask({ type: "allow-intent", host: host, tabId: tabId });
    var asked;
    try {
      asked = api.permissions.request({ origins: ["https://" + host + "/*"] });
    } catch (error) {
      asked = Promise.resolve(false);
    }
    // Firefox leaves this popup open over the prompt that it just asked for, and the prompt is under it: the popup goes away, as the one of Chrome
    // does by itself. The background already has what it needs (the intent, and the permission when it is given) to finish the job.
    if (/Firefox\//.test(navigator.userAgent))
      window.setTimeout(window.close.bind(window), 150);
    asked
      .then(
        function (granted) {
          if (!granted) {
            byId("sitetoggle").disabled = false;
            return siteNoteSet("siteDenied");
          }
          return ask({ type: "allow-done", host: host }).then(
            function (answer) {
              drawSite(!!(answer && answer.allowed));
              if (answer && answer.allowed) siteNoteSet("siteAllowed", host);
              else siteNoteSet("errorFailed");
            },
          );
        },
        function () {
          byId("sitetoggle").disabled = false;
          siteNoteSet("siteDenied");
        },
      )
      .catch(function () {
        byId("sitetoggle").disabled = false;
        siteNoteSet("errorFailed");
      });
  }

  function revokeSite() {
    if (!site.host) return;
    var host = site.host;
    byId("sitetoggle").disabled = true;
    siteNoteSet("sitePending");
    ask({ type: "revoke", host: host }).then(
      function (answer) {
        byId("sitetoggle").disabled = false;
        if (answer && answer.allowed === false) {
          drawSite(false);
          siteNoteSet("siteRevoked", host);
        } else siteNoteSet("errorFailed");
      },
      function () {
        byId("sitetoggle").disabled = false;
        siteNoteSet("errorFailed");
      },
    );
  }

  byId("sitetoggle").addEventListener("click", function () {
    if (byId("sitetoggle").getAttribute("aria-checked") === "true")
      revokeSite();
    else allowSite();
  });

  // "Copied" is said only when the browser wrote to the clipboard. When it cannot, the line is selected and the person is told to copy it.
  function copyNote(key) {
    byId("copynote").textContent = key ? t(key) : "";
    byId("copynote").hidden = !key;
  }

  byId("copy").addEventListener("click", function () {
    var line = byId("line");
    var manual = function () {
      line.focus();
      line.select();
      byId("copy").textContent = t("copyButton");
      copyNote("copyManual");
    };
    var done = function () {
      byId("copy").textContent = t("copiedButton");
      copyNote("copyDone");
    };
    try {
      if (navigator.clipboard && navigator.clipboard.writeText)
        navigator.clipboard.writeText(line.value).then(done, manual);
      else manual();
    } catch (error) {
      manual();
    }
  });

  start().catch(function () {
    show("errorFailed");
  });
})();
