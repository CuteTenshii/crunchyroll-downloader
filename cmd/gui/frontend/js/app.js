/* Normal + Advanced mode UI, Inspect, Download, and index tools. */
(function () {
  "use strict";

  var DEFAULT_4294_RETRIES = 2;
  var DEFAULT_4294_BACKOFF = 8;
  var DEFAULT_CIRCUIT = 3;
  var DEFAULT_INDEX_WINDOW = 25;

  var LANGUAGE_NAMES = {
    "ja-JP": "日本語",
    "en-US": "English",
    "en-IN": "English (India)",
    "id-ID": "Bahasa Indonesia",
    "ms-MY": "Bahasa Melayu",
    "ca-ES": "Català",
    "de-DE": "Deutsch",
    "es-419": "Español (América Latina)",
    "es-ES": "Español (España)",
    "fr-FR": "Français",
    "it-IT": "Italiano",
    "pl-PL": "Polski",
    "pt-BR": "Português (Brasil)",
    "pt-PT": "Português (Portugal)",
    "vi-VN": "Tiếng Việt",
    "tr-TR": "Türkçe",
    "ru-RU": "Русский",
    "ar-SA": "العربية",
    "hi-IN": "हिंदी",
    "ta-IN": "தமிழ்",
    "te-IN": "తెలుగు",
    "zh-CN": "中文 (普通话)",
    "zh-HK": "中文 (粵語)",
    "zh-TW": "中文 (國語)",
    "ko-KR": "한국어",
    "th-TH": "ไทย",
  };

  var TICK_SVG =
    '<svg class="cr-tick" viewBox="0 0 12 12" fill="none" aria-hidden="true">' +
    '<path d="M2.2 6.2L4.8 8.8L9.8 3.2" stroke="#0c0a09" stroke-width="2" ' +
    'stroke-linecap="round" stroke-linejoin="round"/>' +
    "</svg>";

  var state = {
    mode: "normal",
    view: "home", // home | download
    cookieFile: "",
    outputDir: "./Downloads",
    url: "",
    locale: "pt-BR",
    catalog: null,
    selectedSeason: null,
    selectedEpisodeIds: {},
    selectedAudio: {},
    noSubtitles: true,
    selectedSubs: {},
    noCaptions: true,
    selectedCaptions: {},
    videoQuality: "max",
    audioQuality: "max",
    inspecting: false,
    downloading: false,
    indexing: false,
    lastSeason: 0,
    strictLanguages: false,
    captionLangs: [],
    wvdPath: "",
    clientIdPath: "",
    privateKeyPath: "",
    batchUrls: "",
    probeEveryEpisode: false,
    debugManifest: false,
    playback4294Retries: DEFAULT_4294_RETRIES,
    playback4294BackoffSec: DEFAULT_4294_BACKOFF,
    indexCircuitLimit: DEFAULT_CIRCUIT,
    indexWindow: DEFAULT_INDEX_WINDOW,
    queueLabels: {}, // episodeId -> label for queue display
    homeNextStart: 0,
    homeLoaded: false,
    homeLoading: false,
    activeProfileID: "",
    activeCRProfileID: "",
    activeProfileName: "",
    activeCRProfileName: "",
    // After Home card open + Inspect: select this episode id and scroll to it.
    pendingSelectEpisodeId: "",
  };

  var prefsTimer = null;
  var progressUnsub = null;
  var playIdleTimer = null;
  var playOverlayPlaying = false;
  var els = {};

  function $(id) {
    return document.getElementById(id);
  }

  function cacheEls() {
    els = {
      url: $("url"),
      cookie: $("btn-cookie"),
      cookieHome: $("btn-cookie-home"),
      inspect: $("btn-inspect"),
      download: $("btn-download"),
      downloadAdv: $("btn-download-adv"),
      btnPlaySelected: $("btn-play-selected"),
      playPage: $("page-play"),
      btnPlayBack: $("btn-play-back"),
      btnPlayToggle: $("btn-play-toggle"),
      playTitle: $("play-title"),
      playShow: $("play-show"),
      playLock: $("play-lock"),
      playTime: $("play-time"),
      playStage: $("play-stage"),
      playError: $("play-error"),
      output: $("output-dir"),
      outputBtn: $("btn-output"),
      mediaHero: $("media-hero"),
      mediaPoster: $("media-poster"),
      mediaKicker: $("media-kicker"),
      mediaTitle: $("media-title"),
      seasons: $("season-chips"),
      seasonHeading: $("season-heading"),
      seasonHeadingText: $("season-heading-text"),
      episodes: $("episode-list"),
      audio: $("audio-list"),
      subs: $("sub-list"),
      ccList: $("cc-list"),
      selectAll: $("select-all-eps"),
      ddVideo: $("dd-video"),
      ddAudio: $("dd-audio"),
      activity: $("activity"),
      queue: $("queue"),
      progressBar: $("progress-bar"),
      progressFill: $("progress-fill"),
      progressLabel: $("progress-label"),
      progressValue: $("progress-value"),
      activityDock: $("activity-dock"),
      activityDockBody: $("activity-dock-body"),
      btnToggleActivity: $("btn-toggle-activity"),
      activityHint: $("activity-hint"),
      queueSummary: $("queue-summary"),
      progressSummary: $("progress-summary"),
      banner: $("banner"),
      bannerText: $("banner-text"),
      bannerClose: $("banner-close"),
      mainPane: $("main-pane"),
      viewHome: $("view-home"),
      viewNormal: $("view-normal"),
      viewAdvanced: $("view-advanced"),
      modeNormal: $("mode-normal"),
      modeAdvanced: $("mode-advanced"),
      modeToggle: $("mode-toggle"),
      navHome: $("nav-home"),
      navDownload: $("nav-download"),
      navQueue: $("nav-queue"),
      navHistory: $("nav-history"),
      navSettings: $("nav-settings"),
      brandHome: $("brand-home"),
      topbarTitle: $("topbar-title"),
      pageDownload: $("page-download"),
      pageQueue: $("page-queue"),
      pageHistory: $("page-history"),
      pageSettings: $("page-settings"),
      downloadChrome: $("download-chrome"),
      homeChrome: $("home-chrome"),
      homeSearch: $("home-search"),
      btnHomeSearch: $("btn-home-search"),
      btnHomeRefresh: $("btn-home-refresh"),
      btnHomeMore: $("btn-home-more"),
      homeMoreWrap: $("home-more-wrap"),
      homeStatus: $("home-status"),
      homeFeed: $("home-feed"),
      homeSearchResults: $("home-search-results"),
      homeSearchGrid: $("home-search-grid"),
      btnAccount: $("btn-account"),
      accountWrap: $("account-wrap"),
      accountMenu: $("account-menu"),
      accountName: $("account-name"),
      accountSub: $("account-sub"),
      accountAvatar: $("account-avatar"),
      tabActivity: $("tab-activity"),
      tabQueue: $("tab-queue"),
      tabProgress: $("tab-progress"),
      panelActivity: $("panel-activity"),
      panelQueue: $("panel-queue"),
      panelProgress: $("panel-progress"),
      btnClearActivity: $("btn-clear-activity"),
      btnGotoDownloadFromQueue: $("btn-goto-download-from-queue"),
      btnGotoHomeFromHistory: $("btn-goto-home-from-history"),
      btnSettingsCookie: $("btn-settings-cookie"),
      btnGotoDownloadSettings: $("btn-goto-download-settings"),
      sidebarStatusTitle: $("sidebar-status-title"),
      sidebarStatusSub: $("sidebar-status-sub"),
      batchUrls: $("batch-urls"),
      batchFile: $("btn-batch-file"),
      wvdPath: $("wvd-path"),
      clientIdPath: $("client-id-path"),
      privateKeyPath: $("private-key-path"),
      btnWvd: $("btn-wvd"),
      btnClientId: $("btn-client-id"),
      btnPrivateKey: $("btn-private-key"),
      swProbeEvery: $("sw-probe-every"),
      swDebugManifest: $("sw-debug-manifest"),
      swStrictLangs: $("sw-strict-langs"),
      num4294Retries: $("num-4294-retries"),
      num4294Backoff: $("num-4294-backoff"),
      numCircuitLimit: $("num-circuit-limit"),
      numIndexWindow: $("num-index-window"),
      btnBuildCatalog: $("btn-build-catalog"),
      btnIndexSubs: $("btn-index-subs"),
    };
  }

  function goApp() {
    try {
      if (window.go && window.go.main && window.go.main.App) {
        return window.go.main.App;
      }
    } catch (e) {
      /* ignore */
    }
    return null;
  }

  function localeLabel(code) {
    if (!code) return "";
    var name = LANGUAGE_NAMES[code];
    if (name) return name + " (" + code + ")";
    return code;
  }

  function pad2(n) {
    var v = Number(n) || 0;
    return v < 10 ? "0" + v : String(v);
  }

  function epCode(ep) {
    return "S" + pad2(ep.SeasonNumber) + "E" + pad2(ep.EpisodeNumber);
  }

  function isActivityCollapsed() {
    return !els.activityDock || els.activityDock.classList.contains("is-collapsed");
  }

  function setActivityCollapsed(collapsed) {
    if (!els.activityDock) return;
    if (collapsed) {
      els.activityDock.classList.add("is-collapsed");
      if (els.activityDockBody) els.activityDockBody.hidden = true;
      if (els.btnToggleActivity) {
        els.btnToggleActivity.setAttribute("aria-expanded", "false");
        els.btnToggleActivity.textContent = "Show";
      }
      if (els.activityHint) els.activityHint.textContent = "Show";
    } else {
      els.activityDock.classList.remove("is-collapsed");
      if (els.activityDockBody) els.activityDockBody.hidden = false;
      if (els.btnToggleActivity) {
        els.btnToggleActivity.setAttribute("aria-expanded", "true");
        els.btnToggleActivity.textContent = "Hide";
      }
      if (els.activityHint) els.activityHint.textContent = "Hide";
    }
    try {
      localStorage.setItem("crdl.activityCollapsed", collapsed ? "1" : "0");
    } catch (e) {
      /* ignore */
    }
  }

  function setActivityTab(tab) {
    tab = tab || "activity";
    var tabs = [
      { btn: els.tabActivity, panel: els.panelActivity, id: "activity" },
      { btn: els.tabQueue, panel: els.panelQueue, id: "queue" },
      { btn: els.tabProgress, panel: els.panelProgress, id: "progress" },
    ];
    tabs.forEach(function (t) {
      var on = t.id === tab;
      if (t.btn) {
        t.btn.classList.toggle("is-on", on);
        t.btn.setAttribute("aria-selected", on ? "true" : "false");
      }
      if (t.panel) {
        t.panel.classList.toggle("is-on", on);
        t.panel.hidden = !on;
      }
    });
  }

  function expandActivityForWork() {
    // Auto-open when something useful is happening so the user sees logs.
    if (isActivityCollapsed()) setActivityCollapsed(false);
  }

  function updateDockSummary() {
    if (els.queueSummary && els.queue) {
      var q = (els.queue.textContent || "").trim() || "No jobs";
      if (q.length > 48) q = q.slice(0, 45) + "…";
      els.queueSummary.textContent = q;
    }
    if (els.progressSummary) {
      var label = (els.progressLabel && els.progressLabel.textContent) || "Progress";
      var value = (els.progressValue && els.progressValue.textContent) || "—";
      els.progressSummary.textContent = label + " " + value;
    }
  }

  function logLine(text, cls, opts) {
    if (!els.activity) return;
    opts = opts || {};
    var div = document.createElement("div");
    if (cls) div.className = cls;
    div.textContent = text;
    els.activity.appendChild(div);
    els.activity.scrollTop = els.activity.scrollHeight;
    // Do not auto-expand on Home↔Download navigation chatter; only real work/errors.
    if (!opts.quiet && (cls === "err" || cls === "warn")) {
      expandActivityForWork();
    }
    updateDockSummary();
  }

  function clearBanner() {
    if (!els.banner) return;
    els.banner.hidden = true;
    els.banner.classList.remove("err", "warn", "ok");
    els.bannerText.textContent = "";
  }

  function showBanner(message, kind) {
    if (!els.banner) return;
    kind = kind || "err";
    els.banner.hidden = false;
    els.banner.classList.remove("err", "warn", "ok");
    els.banner.classList.add(kind);
    els.bannerText.textContent = message;
  }

  function formIsBusy() {
    return !!(state.inspecting || state.downloading || state.indexing);
  }

  function refreshBusyChrome() {
    var busy = formIsBusy();
    if (els.mainPane) {
      els.mainPane.classList.toggle("is-busy", busy);
    }
    if (els.inspect) els.inspect.disabled = busy;
    if (els.cookie) els.cookie.disabled = busy;
    if (els.outputBtn) els.outputBtn.disabled = busy;
    if (els.btnBuildCatalog) els.btnBuildCatalog.disabled = busy;
    if (els.btnIndexSubs) els.btnIndexSubs.disabled = busy;
  }

  function forEachDownloadBtn(fn) {
    if (els.download) fn(els.download);
    if (els.downloadAdv) fn(els.downloadAdv);
  }

  function setBusy(busy) {
    state.inspecting = !!busy;
    refreshBusyChrome();
    if (busy) {
      expandActivityForWork();
      if (els.progressBar) els.progressBar.classList.add("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Inspect";
      if (els.progressValue) els.progressValue.textContent = "working…";
    } else if (!state.downloading && !state.indexing) {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Progress";
      if (els.progressValue) els.progressValue.textContent = "—";
      if (els.progressFill) els.progressFill.style.width = "0%";
    }
    updateDockSummary();
  }

  function setDownloading(on) {
    state.downloading = !!on;
    refreshBusyChrome();
    forEachDownloadBtn(function (btn) {
      if (on) {
        btn.textContent = "CANCEL";
        btn.classList.add("is-cancel");
        btn.disabled = false;
      } else {
        btn.textContent = "DOWNLOAD SELECTED";
        btn.classList.remove("is-cancel");
        btn.disabled = false;
      }
    });
    if (on) {
      expandActivityForWork();
      if (els.progressBar) els.progressBar.classList.add("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Download";
      if (els.progressValue) els.progressValue.textContent = "starting…";
    } else {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Progress";
    }
    updateDockSummary();
  }

  function setIndexing(on, label) {
    state.indexing = !!on;
    refreshBusyChrome();
    if (on) {
      expandActivityForWork();
      if (els.progressBar) els.progressBar.classList.add("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = label || "Index";
      if (els.progressValue) els.progressValue.textContent = "working…";
    } else if (!state.downloading && !state.inspecting) {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Progress";
      if (els.progressValue) els.progressValue.textContent = "—";
      if (els.progressFill) els.progressFill.style.width = "0%";
    }
    updateDockSummary();
  }

  function setSwitch(el, on) {
    if (!el) return;
    el.classList.toggle("is-on", !!on);
    el.setAttribute("aria-pressed", on ? "true" : "false");
  }

  function readIntInput(el, fallback, min, max) {
    if (!el) return fallback;
    var n = parseInt(el.value, 10);
    if (isNaN(n)) n = fallback;
    if (typeof min === "number" && n < min) n = min;
    if (typeof max === "number" && n > max) n = max;
    return n;
  }

  function syncAdvancedInputsFromState() {
    if (els.wvdPath) els.wvdPath.value = state.wvdPath || "";
    if (els.clientIdPath) els.clientIdPath.value = state.clientIdPath || "";
    if (els.privateKeyPath) els.privateKeyPath.value = state.privateKeyPath || "";
    if (els.batchUrls) els.batchUrls.value = state.batchUrls || "";
    if (els.num4294Retries) els.num4294Retries.value = String(state.playback4294Retries);
    if (els.num4294Backoff) els.num4294Backoff.value = String(state.playback4294BackoffSec);
    if (els.numCircuitLimit) els.numCircuitLimit.value = String(state.indexCircuitLimit);
    if (els.numIndexWindow) els.numIndexWindow.value = String(state.indexWindow);
    setSwitch(els.swProbeEvery, state.probeEveryEpisode);
    setSwitch(els.swDebugManifest, state.debugManifest);
    setSwitch(els.swStrictLangs, state.strictLanguages);
  }

  function pullAdvancedInputsToState() {
    if (els.wvdPath) state.wvdPath = els.wvdPath.value.trim();
    if (els.clientIdPath) state.clientIdPath = els.clientIdPath.value.trim();
    if (els.privateKeyPath) state.privateKeyPath = els.privateKeyPath.value.trim();
    if (els.batchUrls) state.batchUrls = els.batchUrls.value;
    state.playback4294Retries = readIntInput(
      els.num4294Retries,
      DEFAULT_4294_RETRIES,
      0,
      5
    );
    state.playback4294BackoffSec = readIntInput(
      els.num4294Backoff,
      DEFAULT_4294_BACKOFF,
      1,
      60
    );
    state.indexCircuitLimit = readIntInput(
      els.numCircuitLimit,
      DEFAULT_CIRCUIT,
      1,
      10
    );
    state.indexWindow = readIntInput(
      els.numIndexWindow,
      DEFAULT_INDEX_WINDOW,
      1,
      100
    );
    state.probeEveryEpisode = !!(
      els.swProbeEvery && els.swProbeEvery.classList.contains("is-on")
    );
    state.debugManifest = !!(
      els.swDebugManifest && els.swDebugManifest.classList.contains("is-on")
    );
    state.strictLanguages = !!(
      els.swStrictLangs && els.swStrictLangs.classList.contains("is-on")
    );
  }

  function selectedCaptionLangs() {
    if (state.noCaptions) return [];
    return Object.keys(state.selectedCaptions).filter(function (k) {
      return state.selectedCaptions[k];
    });
  }

  /** Normalize ProgressEvent fields (json tags vs Go names). */
  function pe(ev, camel, pascal) {
    if (!ev || typeof ev !== "object") return undefined;
    if (ev[camel] !== undefined && ev[camel] !== null && ev[camel] !== "") {
      return ev[camel];
    }
    if (ev[pascal] !== undefined) return ev[pascal];
    return ev[camel];
  }

  function appendLog(ev) {
    var msg = pe(ev, "message", "Message") || "";
    var level = pe(ev, "level", "Level") || "info";
    var phase = pe(ev, "phase", "Phase") || "";
    var label = pe(ev, "episodeLabel", "EpisodeLabel") || "";
    var cls = "info";
    if (level === "ok") cls = "ok";
    else if (level === "warn") cls = "warn";
    else if (level === "error") cls = "err";
    var line = msg;
    if (phase && phase !== "download") {
      line = "[" + phase + "] " + line;
    }
    if (label && msg.indexOf(label) < 0) {
      line = label + " · " + line;
    }
    logLine(line, cls);
  }

  function updateQueue(ev) {
    if (!els.queue) return;
    var qi = pe(ev, "queueIndex", "QueueIndex");
    var qt = pe(ev, "queueTotal", "QueueTotal");
    var phase = pe(ev, "phase", "Phase") || "";
    var level = pe(ev, "level", "Level") || "";
    var label =
      pe(ev, "episodeLabel", "EpisodeLabel") ||
      pe(ev, "episodeId", "EpisodeID") ||
      "";
    var epId = pe(ev, "episodeId", "EpisodeID") || "";
    if (epId && label) state.queueLabels[epId] = label;

    if (phase === "done") {
      if (level === "ok") {
        els.queue.textContent =
          qt != null && qt > 0
            ? "Done · " + qt + " episode(s)"
            : "Done";
      } else if (level === "warn") {
        els.queue.textContent = "Cancelled";
      } else if (level === "error") {
        els.queue.textContent =
          qt != null && qt > 0
            ? "Failed · " +
              (typeof qi === "number" ? qi + 1 : "?") +
              "/" +
              qt
            : "Failed";
      }
      return;
    }

    if (qt != null && qt > 0 && typeof qi === "number") {
      var n = qi + 1;
      if (n > qt) n = qt;
      var head = n + " / " + qt;
      if (label) {
        els.queue.textContent = head + " · " + label;
      } else {
        els.queue.textContent = head + " active";
      }
    }
    updateDockSummary();
  }

  function updateProgressBar(ev) {
    if (!els.progressFill) return;
    var fraction = pe(ev, "fraction", "Fraction");
    var phase = pe(ev, "phase", "Phase") || "";
    var segDone = pe(ev, "segmentDone", "SegmentDone");
    var segTotal = pe(ev, "segmentTotal", "SegmentTotal");
    var msg = pe(ev, "message", "Message") || "";
    var level = pe(ev, "level", "Level") || "";

    if (els.progressLabel) {
      els.progressLabel.textContent = phase || "Progress";
    }

    var known =
      typeof fraction === "number" && fraction >= 0 && !isNaN(fraction);
    if (known) {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      var pct = Math.max(0, Math.min(100, Math.round(fraction * 100)));
      els.progressFill.style.width = pct + "%";
      if (els.progressValue) {
        if (segTotal > 0) {
          els.progressValue.textContent =
            segDone + "/" + segTotal + " · " + pct + "%";
        } else {
          els.progressValue.textContent = pct + "%";
        }
      }
    } else {
      if (els.progressBar && phase !== "done") {
        els.progressBar.classList.add("is-indet");
      }
      if (els.progressValue) {
        if (segTotal > 0) {
          els.progressValue.textContent = segDone + "/" + segTotal;
        } else if (msg) {
          els.progressValue.textContent =
            msg.length > 40 ? msg.slice(0, 37) + "…" : msg;
        } else {
          els.progressValue.textContent = "working…";
        }
      }
    }

    if (phase === "done") {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (level === "ok") {
        els.progressFill.style.width = "100%";
        if (els.progressValue) els.progressValue.textContent = "100%";
      } else if (level === "warn") {
        if (els.progressValue) els.progressValue.textContent = "cancelled";
      } else if (level === "error") {
        if (els.progressValue) els.progressValue.textContent = "error";
      }
    }
    updateDockSummary();
  }

  function onProgressEvent(ev) {
    if (!ev || typeof ev !== "object") return;
    appendLog(ev);
    updateQueue(ev);
    updateProgressBar(ev);

    var phase = pe(ev, "phase", "Phase") || "";
    var level = pe(ev, "level", "Level") || "";
    var msg = pe(ev, "message", "Message") || "";

    if (level === "error") {
      showBanner(msg || "Download error", "err");
    } else if (level === "warn" && phase === "done") {
      showBanner(msg || "Cancelled", "warn");
    } else if (level === "ok" && phase === "done") {
      showBanner(msg || "All done", "ok");
      offerOpenOutputFolder();
    } else if (level === "warn" && /4294|backoff|rate/i.test(msg)) {
      showBanner(msg, "warn");
    }

    if (phase === "done" && state.downloading) {
      setDownloading(false);
    }
  }

  function currentOutputDir() {
    return (
      (els.output && els.output.value.trim()) ||
      state.outputDir ||
      "./Downloads"
    );
  }

  /** Append a one-shot "Open output folder" control after a successful job. */
  function offerOpenOutputFolder() {
    if (!els.activity) return;
    var app = goApp();
    if (!app || typeof app.OpenOutputFolder !== "function") return;

    // Avoid stacking duplicate offers for repeated terminal events.
    var existing = els.activity.querySelector("[data-open-output]");
    if (existing) existing.remove();

    var dir = currentOutputDir();
    var row = document.createElement("div");
    row.className = "ok";
    row.setAttribute("data-open-output", "1");
    row.appendChild(document.createTextNode("Finished · "));
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "cr-btn cr-btn-ghost";
    btn.style.cssText =
      "display:inline-block;padding:2px 10px;font-size:12px;vertical-align:baseline;";
    btn.textContent = "Open output folder";
    btn.addEventListener("click", async function () {
      try {
        await app.OpenOutputFolder(dir);
        logLine("Opened folder: " + dir, "ok");
      } catch (err) {
        logLine("Could not open folder: " + errMessage(err), "err");
        showBanner("Could not open folder: " + errMessage(err), "err");
      }
    });
    row.appendChild(btn);
    els.activity.appendChild(row);
    els.activity.scrollTop = els.activity.scrollHeight;
  }

  function subscribeProgress() {
    if (progressUnsub) return;
    try {
      if (window.runtime && typeof window.runtime.EventsOn === "function") {
        progressUnsub = window.runtime.EventsOn("progress", onProgressEvent);
        return;
      }
    } catch (e) {
      /* ignore */
    }
    // Retry shortly — runtime may not be injected yet at first paint.
    setTimeout(function () {
      if (progressUnsub) return;
      try {
        if (window.runtime && typeof window.runtime.EventsOn === "function") {
          progressUnsub = window.runtime.EventsOn("progress", onProgressEvent);
        }
      } catch (e2) {
        /* browser preview: no events */
      }
    }, 200);
  }

  function setMode(mode) {
    state.mode = mode === "advanced" ? "advanced" : "normal";
    var isAdv = state.mode === "advanced";
    if (els.modeNormal) {
      els.modeNormal.classList.toggle("is-on", !isAdv);
      els.modeNormal.setAttribute("aria-selected", !isAdv ? "true" : "false");
    }
    if (els.modeAdvanced) {
      els.modeAdvanced.classList.toggle("is-on", isAdv);
      els.modeAdvanced.setAttribute("aria-selected", isAdv ? "true" : "false");
    }
    if (els.viewNormal) {
      els.viewNormal.classList.toggle("is-on", !isAdv);
      els.viewNormal.hidden = isAdv;
    }
    if (els.viewAdvanced) {
      els.viewAdvanced.classList.toggle("is-on", isAdv);
      els.viewAdvanced.hidden = !isAdv;
    }
    schedulePersist();
  }

  function setAppView(view) {
    // Back-compat: home | download (and shell pages).
    if (view === "download") setShellPage("download");
    else setShellPage("home");
  }

  function setShellPage(page) {
    closePlayOverlay();
    var map = {
      home: { title: "Home", el: els.viewHome, nav: els.navHome },
      download: { title: "Downloads", el: els.pageDownload, nav: els.navDownload },
      queue: { title: "Queue", el: els.pageQueue, nav: els.navQueue },
      history: { title: "History", el: els.pageHistory, nav: els.navHistory },
      settings: { title: "Settings", el: els.pageSettings, nav: els.navSettings },
    };
    if (!map[page]) page = "home";
    state.view = page === "download" ? "download" : page === "home" ? "home" : page;

    Object.keys(map).forEach(function (key) {
      var m = map[key];
      var on = key === page;
      if (m.el) {
        m.el.classList.toggle("is-on", on);
        m.el.hidden = !on;
      }
      if (m.nav) {
        m.nav.classList.toggle("is-active", on);
      }
    });
    if (els.topbarTitle) els.topbarTitle.textContent = map[page].title;

    // Mode toggle is most useful on Downloads.
    if (els.modeToggle) {
      els.modeToggle.style.visibility =
        page === "download" || page === "settings" ? "visible" : "hidden";
    }

    if (page === "home" && !state.homeLoaded && !state.homeLoading && state.cookieFile) {
      loadHomeFeed(false);
    }
    if (page === "queue") {
      setActivityTab("queue");
      setActivityCollapsed(false);
    }
    if (page === "download") {
      // Keep activity visible by default in new shell (user can Hide).
      if (isActivityCollapsed()) {
        /* leave user preference */
      }
    }
  }

  function isPlayOverlayOpen() {
    return !!(els.playPage && !els.playPage.hidden);
  }

  function isTypingTarget(el) {
    if (!el) return false;
    var tag = el.tagName;
    if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
    if (el.isContentEditable) return true;
    return false;
  }

  function setPlayToggleIcon(playing) {
    var icon = $("play-icon");
    if (icon) {
      icon.innerHTML = playing
        ? '<path d="M6 5h4v14H6zm8 0h4v14h-4z"/>'
        : '<path d="M8 5v14l11-7z"/>';
    }
    if (els.btnPlayToggle) {
      var label = playing ? "Pause" : "Play";
      els.btnPlayToggle.setAttribute("aria-label", label);
      els.btnPlayToggle.title = label;
    }
  }

  function wakePlayChrome() {
    if (!els.playPage || els.playPage.hidden) return;
    els.playPage.classList.remove("is-idle");
    if (playIdleTimer) clearTimeout(playIdleTimer);
    playIdleTimer = setTimeout(function () {
      if (els.playPage && !els.playPage.hidden) {
        els.playPage.classList.add("is-idle");
      }
    }, 2800);
  }

  function togglePlayOverlayIcon() {
    if (!isPlayOverlayOpen()) return;
    playOverlayPlaying = !playOverlayPlaying;
    setPlayToggleIcon(playOverlayPlaying);
    wakePlayChrome();
  }

  function playErrText(err) {
    if (!err) return "";
    if (typeof err === "string") return err;
    if (err.message) return String(err.message);
    return String(err);
  }

  function setPlayStageError(msg) {
    var text = msg || "libmpv surface";
    if (els.playError) els.playError.textContent = text;
    else if (els.playStage) {
      var label = els.playStage.querySelector(".play-stage-label");
      if (label) label.textContent = text;
    }
  }

  function layoutPlayStage() {
    if (!els.playStage) return;
    var r = els.playStage.getBoundingClientRect();
    var app = goApp();
    if (!app || typeof app.PlayLayout !== "function") return;
    try {
      var p = app.PlayLayout(r.left, r.top, r.width, r.height);
      if (p && typeof p.then === "function") {
        p.catch(function () {
          /* ignore layout errors */
        });
      }
    } catch (e) {
      /* ignore */
    }
  }

  function startPlaySurface() {
    layoutPlayStage();
    var app = goApp();
    if (!app || typeof app.StartPlay !== "function") {
      setPlayStageError("player library missing");
      return;
    }
    Promise.resolve(app.StartPlay(""))
      .then(function () {
        layoutPlayStage();
      })
      .catch(function (err) {
        var msg = playErrText(err);
        if (msg.indexOf("player library missing") !== -1) {
          setPlayStageError(msg);
        } else if (msg) {
          setPlayStageError(msg);
        } else {
          setPlayStageError("player library missing");
        }
      });
  }

  function closePlayOverlay() {
    if (playIdleTimer) {
      clearTimeout(playIdleTimer);
      playIdleTimer = null;
    }
    playOverlayPlaying = false;
    setPlayToggleIcon(false);
    var app = goApp();
    if (app && typeof app.StopPlay === "function") {
      Promise.resolve(app.StopPlay()).catch(function () {
        /* ignore */
      });
    }
    setPlayStageError("");
    if (!els.playPage) return;
    els.playPage.hidden = true;
    els.playPage.classList.remove("is-idle");
    els.playPage.setAttribute("aria-hidden", "true");
  }

  function playShowLabel(meta) {
    var code =
      "S" + pad2(meta && meta.seasonNumber) + "E" + pad2(meta && meta.episodeNumber);
    var series = (meta && meta.seriesTitle) || "";
    return series ? series + " · " + code : code;
  }

  function openPlayOverlay(meta) {
    meta = meta || {};
    if (!els.playPage) return;
    if (els.playTitle) els.playTitle.textContent = meta.episodeTitle || "Untitled";
    if (els.playShow) els.playShow.textContent = playShowLabel(meta);
    if (els.playLock) els.playLock.textContent = "1080p locked";
    if (els.playTime) els.playTime.textContent = "0:00 / 0:00";
    playOverlayPlaying = false;
    setPlayToggleIcon(false);
    els.playPage.hidden = false;
    els.playPage.setAttribute("aria-hidden", "false");
    els.playPage.classList.remove("is-idle");
    try {
      els.playPage.focus();
    } catch (e) {
      /* ignore */
    }
    setPlayStageError("");
    wakePlayChrome();
    requestAnimationFrame(function () {
      startPlaySurface();
    });
  }

  /** Extract episode content id from a /watch/{id}/… URL. */
  function episodeIdFromOpenUrl(url) {
    url = String(url || "");
    var m = url.match(/\/watch\/([^/?#]+)/i);
    return m && m[1] ? m[1] : "";
  }

  function scrollEpisodeIntoView(episodeId) {
    if (!episodeId || !els.episodes) return;
    var row = els.episodes.querySelector(
      '.cr-check[data-ep="' + CSS.escape(episodeId) + '"]'
    );
    if (!row) {
      // Fallback without CSS.escape for older WebViews
      var rows = els.episodes.querySelectorAll(".cr-check[data-ep]");
      for (var i = 0; i < rows.length; i++) {
        if (rows[i].getAttribute("data-ep") === episodeId) {
          row = rows[i];
          break;
        }
      }
    }
    if (!row) return;
    try {
      row.scrollIntoView({ block: "center", behavior: "smooth" });
    } catch (e) {
      row.scrollIntoView(true);
    }
    row.classList.add("is-flash");
    setTimeout(function () {
      row.classList.remove("is-flash");
    }, 1200);
  }

  function setHomeStatus(text, kind) {
    if (!els.homeStatus) return;
    els.homeStatus.textContent = text || "";
    els.homeStatus.classList.remove("is-err", "is-ok");
    if (kind === "err") els.homeStatus.classList.add("is-err");
    if (kind === "ok") els.homeStatus.classList.add("is-ok");
  }

  function fHome(obj, camel, pascal) {
    return field(obj, pascal, camel);
  }

  function normalizeHomePage(raw) {
    if (!raw || typeof raw !== "object") {
      return { blocks: [], heroes: [], rails: [], nextStart: 0, pageSize: 0 };
    }
    var blocks = fHome(raw, "blocks", "Blocks") || [];
    var heroes = fHome(raw, "heroes", "Heroes") || [];
    var rails = fHome(raw, "rails", "Rails") || [];
    return {
      blocks: Array.isArray(blocks) ? blocks : [],
      heroes: Array.isArray(heroes) ? heroes : [],
      rails: Array.isArray(rails) ? rails : [],
      nextStart: Number(fHome(raw, "nextStart", "NextStart") || 0),
      pageSize: Number(fHome(raw, "pageSize", "PageSize") || 0),
      totalApprox: Number(fHome(raw, "totalApprox", "TotalApprox") || 0),
    };
  }

  function normalizeCard(raw) {
    if (!raw || typeof raw !== "object") return null;
    var id = fHome(raw, "id", "ID") || "";
    var openUrl = fHome(raw, "openUrl", "OpenURL") || "";
    var title = fHome(raw, "title", "Title") || "";
    if (!id && !openUrl) return null;
    var progressRaw = fHome(raw, "progress", "Progress");
    var progress = null;
    if (progressRaw != null && progressRaw !== "" && !isNaN(Number(progressRaw))) {
      progress = Math.max(0, Math.min(1, Number(progressRaw)));
    }
    var rank = Number(fHome(raw, "rank", "Rank") || 0) || 0;
    return {
      id: id,
      type: fHome(raw, "type", "Type") || "",
      title: title,
      description: fHome(raw, "description", "Description") || "",
      posterUrl: fHome(raw, "posterUrl", "PosterURL") || "",
      wideUrl: fHome(raw, "wideUrl", "WideURL") || "",
      openUrl: openUrl,
      progress: progress,
      rank: rank,
      subtitle: fHome(raw, "subtitle", "Subtitle") || "",
      seriesId: fHome(raw, "seriesId", "SeriesID") || "",
      episodeTitle: fHome(raw, "episodeTitle", "EpisodeTitle") || "",
      remainingLabel: fHome(raw, "remainingLabel", "RemainingLabel") || "",
      durationMs: Number(fHome(raw, "durationMs", "DurationMS") || 0) || 0,
    };
  }

  function normalizeBlock(raw) {
    if (!raw || typeof raw !== "object") return null;
    var kind = String(fHome(raw, "kind", "Kind") || "").toLowerCase();
    if (!kind) return null;
    var cardsRaw = fHome(raw, "cards", "Cards") || [];
    var cards = [];
    (cardsRaw || []).forEach(function (c) {
      var card = normalizeCard(c);
      if (card) cards.push(card);
    });
    var heroesRaw = fHome(raw, "heroes", "Heroes") || [];
    var heroes = Array.isArray(heroesRaw) ? heroesRaw : [];
    var bannerRaw = fHome(raw, "banner", "Banner");
    var banner = null;
    if (bannerRaw && typeof bannerRaw === "object") {
      banner = {
        id: fHome(bannerRaw, "id", "ID") || "",
        title: fHome(bannerRaw, "title", "Title") || "",
        wideUrl: fHome(bannerRaw, "wideUrl", "WideURL") || "",
        openUrl: fHome(bannerRaw, "openUrl", "OpenURL") || "",
      };
    }
    return {
      id: fHome(raw, "id", "ID") || "",
      kind: kind,
      title: fHome(raw, "title", "Title") || "",
      rankStyle: String(fHome(raw, "rankStyle", "RankStyle") || "none").toLowerCase(),
      cards: cards,
      heroes: heroes,
      banner: banner,
    };
  }

  function makeArtEl(className, url, emptyLabel) {
    var art = document.createElement("div");
    art.className = className + (url ? "" : " is-empty");
    if (url) {
      var img = document.createElement("img");
      img.src = url;
      img.alt = "";
      img.loading = "lazy";
      img.decoding = "async";
      art.appendChild(img);
    } else {
      art.textContent = emptyLabel || "No art";
    }
    return art;
  }

  function posterCardButton(card, rankStyle) {
    var ranked = rankStyle === "top10" && card.rank > 0;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "home-card" + (ranked ? " is-ranked" : "");
    btn.title = card.title || card.id || "Open";
    btn.dataset.openUrl = card.openUrl || "";
    btn.dataset.id = card.id || "";

    if (ranked) {
      var rankEl = document.createElement("span");
      rankEl.className = "home-card-rank" + (card.rank <= 3 ? " is-top" : "");
      rankEl.textContent = String(card.rank);
      rankEl.setAttribute("aria-hidden", "true");
      btn.appendChild(rankEl);
    }

    var host = btn;
    if (ranked) {
      host = document.createElement("div");
      host.className = "home-card-body";
      btn.appendChild(host);
    }

    host.appendChild(
      makeArtEl("home-card-art", card.posterUrl || card.wideUrl, "No art")
    );

    var title = document.createElement("div");
    title.className = "home-card-title";
    title.textContent = card.title || card.id || "Untitled";
    host.appendChild(title);

    btn.addEventListener("click", function () {
      openDiscoverCard(card);
    });
    return btn;
  }

  function landscapeCardButton(card) {
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "home-cw-card";
    var seriesName = card.title || card.id || "Untitled";
    var epLabel = "";
    if (card.episodeTitle) {
      epLabel = card.subtitle
        ? card.subtitle + " - " + card.episodeTitle
        : card.episodeTitle;
    } else if (card.subtitle) {
      epLabel = card.subtitle;
    }
    btn.title = seriesName + (epLabel ? " · " + epLabel : "");
    btn.dataset.openUrl = card.openUrl || "";
    btn.dataset.id = card.id || "";
    btn.dataset.seriesId = card.seriesId || "";

    var artUrl = card.wideUrl || card.posterUrl || "";
    var art = makeArtEl("home-cw-art", artUrl, "No art");
    if (card.progress != null && card.progress >= 0) {
      var bar = document.createElement("div");
      bar.className = "home-cw-progress";
      bar.setAttribute("aria-hidden", "true");
      var fill = document.createElement("i");
      fill.style.width = Math.round(Math.min(1, Math.max(0, card.progress)) * 100) + "%";
      bar.appendChild(fill);
      art.appendChild(bar);
    }
    if (card.remainingLabel) {
      var rem = document.createElement("span");
      rem.className = "home-cw-remaining";
      rem.textContent = card.remainingLabel;
      art.appendChild(rem);
    }
    btn.appendChild(art);

    // CR layout: series name UPPERCASE (small) + episode title (larger).
    var showTitle = document.createElement("div");
    showTitle.className = "home-cw-show";
    showTitle.textContent = seriesName;
    btn.appendChild(showTitle);

    var epTitle = document.createElement("div");
    epTitle.className = "home-cw-ep";
    epTitle.textContent = epLabel || seriesName;
    btn.appendChild(epTitle);

    btn.addEventListener("click", function () {
      openDiscoverCard(card);
    });
    return btn;
  }

  var heroCarouselTimers = [];

  function clearHeroCarousels() {
    heroCarouselTimers.forEach(function (id) {
      clearInterval(id);
      clearTimeout(id);
    });
    heroCarouselTimers = [];
  }

  function renderHeroBlock(block) {
    var section = document.createElement("section");
    section.className = "home-block home-block-hero";
    section.setAttribute("aria-label", block.title || "Featured");

    var heroes = block.heroes || [];
    if (!heroes.length && block.cards && block.cards.length) {
      // Tolerate cards-shaped hero payloads.
      heroes = block.cards.map(function (c) {
        return {
          title: c.title,
          description: c.description,
          wideUrl: c.wideUrl,
          posterUrl: c.posterUrl,
          openUrl: c.openUrl,
          buttonText: "Open title",
        };
      });
    }
    if (!heroes.length) return null;

    var slides = [];
    var slideMeta = [];
    heroes.forEach(function (h, idx) {
      var title = fHome(h, "title", "Title") || h.title || "";
      var openUrl = fHome(h, "openUrl", "OpenURL") || h.openUrl || "";
      var wide = fHome(h, "wideUrl", "WideURL") || h.wideUrl || "";
      var poster = fHome(h, "posterUrl", "PosterURL") || h.posterUrl || "";
      var desc = fHome(h, "description", "Description") || h.description || "";
      var btnText =
        fHome(h, "buttonText", "ButtonText") || h.buttonText || "Open title";
      if (!title && !openUrl) return;

      var slide = document.createElement("button");
      slide.type = "button";
      slide.className = "home-hero-slide" + (idx === 0 ? " is-on" : "");
      slide.title = title;
      slide.setAttribute("aria-hidden", idx === 0 ? "false" : "true");

      if (wide || poster) {
        var img = document.createElement("img");
        img.src = wide || poster;
        img.alt = title || "";
        img.loading = idx === 0 ? "eager" : "lazy";
        // Omit no-referrer — CR CDN art often requires a normal referrer.
        img.decoding = "async";
        img.addEventListener("error", function () {
          // Fallback: try the other art field once.
          if (wide && poster && img.src.indexOf(poster) < 0) {
            img.src = poster;
          }
        });
        slide.appendChild(img);
      }
      var fade = document.createElement("div");
      fade.className = "home-hero-fade";
      slide.appendChild(fade);

      var meta = document.createElement("div");
      meta.className = "home-hero-meta";
      var kicker = document.createElement("div");
      kicker.className = "kicker";
      kicker.textContent = "Featured";
      meta.appendChild(kicker);
      var t = document.createElement("div");
      t.className = "title";
      t.textContent = title || "Featured";
      meta.appendChild(t);
      if (desc) {
        var d = document.createElement("div");
        d.className = "desc";
        d.textContent = desc;
        meta.appendChild(d);
      }
      var cta = document.createElement("span");
      cta.className = "home-hero-cta";
      cta.textContent = btnText || "Open title";
      meta.appendChild(cta);
      slide.appendChild(meta);

      slide.addEventListener("click", function () {
        openDiscoverCard({
          title: title,
          openUrl: openUrl,
          posterUrl: poster,
          wideUrl: wide,
        });
      });
      section.appendChild(slide);
      slides.push(slide);
      slideMeta.push({ title: title, openUrl: openUrl });
    });

    if (!slides.length) return null;

    var current = 0;
    var dots = null;
    var paused = false;

    function goTo(index, userAction) {
      if (!slides.length) return;
      current = ((index % slides.length) + slides.length) % slides.length;
      slides.forEach(function (s, j) {
        var on = j === current;
        s.classList.toggle("is-on", on);
        s.setAttribute("aria-hidden", on ? "false" : "true");
      });
      if (dots) {
        Array.prototype.forEach.call(dots.children, function (d, j) {
          d.classList.toggle("is-on", j === current);
        });
      }
      if (userAction) {
        // Restart autoplay after manual interaction.
        restartAutoplay();
      }
    }

    function next(userAction) {
      goTo(current + 1, userAction);
    }
    function prev(userAction) {
      goTo(current - 1, userAction);
    }

    var autoplayId = null;
    function stopAutoplay() {
      if (autoplayId != null) {
        clearTimeout(autoplayId);
        clearInterval(autoplayId);
        var ix = heroCarouselTimers.indexOf(autoplayId);
        if (ix >= 0) heroCarouselTimers.splice(ix, 1);
        autoplayId = null;
      }
    }
    function restartAutoplay() {
      stopAutoplay();
      if (slides.length < 2) return;
      // setTimeout chain is more reliable than setInterval in some WebViews.
      function tick() {
        autoplayId = setTimeout(function () {
          // Keep the global list in sync for clearHeroCarousels on re-render.
          next(false);
          tick();
        }, 5500);
        heroCarouselTimers = heroCarouselTimers.filter(function (id) {
          return id !== autoplayId;
        });
        heroCarouselTimers.push(autoplayId);
      }
      tick();
    }

    if (slides.length > 1) {
      // Edge fade + chevron arrows (CR-style)
      var arrowPrev = document.createElement("button");
      arrowPrev.type = "button";
      arrowPrev.className = "home-hero-arrow home-hero-arrow-prev";
      arrowPrev.setAttribute("aria-label", "Previous featured title");
      arrowPrev.innerHTML =
        '<svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path d="M15 5l-7 7 7 7" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
      arrowPrev.addEventListener("click", function (e) {
        e.stopPropagation();
        prev(true);
      });

      var arrowNext = document.createElement("button");
      arrowNext.type = "button";
      arrowNext.className = "home-hero-arrow home-hero-arrow-next";
      arrowNext.setAttribute("aria-label", "Next featured title");
      arrowNext.innerHTML =
        '<svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path d="M9 5l7 7-7 7" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"/></svg>';
      arrowNext.addEventListener("click", function (e) {
        e.stopPropagation();
        next(true);
      });

      section.appendChild(arrowPrev);
      section.appendChild(arrowNext);

      dots = document.createElement("div");
      dots.className = "home-hero-dots";
      slides.forEach(function (_slide, i) {
        var dot = document.createElement("button");
        dot.type = "button";
        dot.className = i === 0 ? "is-on" : "";
        dot.setAttribute("aria-label", "Featured slide " + (i + 1));
        dot.addEventListener("click", function (e) {
          e.stopPropagation();
          goTo(i, true);
        });
        dots.appendChild(dot);
      });
      section.appendChild(dots);

      // Do NOT pause autoplay on hover — looking at the hero should still advance.
      restartAutoplay();
    }
    return section;
  }

  function railShell(title) {
    var section = document.createElement("section");
    section.className = "home-block home-rail";
    var h = document.createElement("div");
    h.className = "home-rail-title";
    h.textContent = title || "Collection";
    section.appendChild(h);
    var scroller = document.createElement("div");
    scroller.className = "home-rail-scroller";
    section.appendChild(scroller);
    return { section: section, scroller: scroller };
  }

  function renderPosterRail(block) {
    if (!block.cards || !block.cards.length) return null;
    var shell = railShell(block.title || "Collection");
    var rankStyle = block.rankStyle || "none";
    block.cards.forEach(function (card) {
      shell.scroller.appendChild(posterCardButton(card, rankStyle));
    });
    return shell.section;
  }

  function renderLandscapeRail(block) {
    if (!block.cards || !block.cards.length) return null;
    var shell = railShell(block.title || "Continue Watching");
    block.cards.forEach(function (card) {
      shell.scroller.appendChild(landscapeCardButton(card));
    });
    return shell.section;
  }

  function renderBannerBlock(block) {
    var banner = block.banner;
    if (!banner && block.title) {
      banner = {
        title: block.title,
        wideUrl: "",
        openUrl: (block.cards && block.cards[0] && block.cards[0].openUrl) || "",
      };
    }
    if (!banner) return null;
    var openUrl = banner.openUrl || "";
    var el = document.createElement(openUrl ? "button" : "div");
    if (openUrl) el.type = "button";
    el.className = "home-block home-banner" + (openUrl ? " has-link" : "");
    if (banner.wideUrl) {
      var img = document.createElement("img");
      img.src = banner.wideUrl;
      img.alt = "";
      img.loading = "lazy";
      img.decoding = "async";
      el.appendChild(img);
    }
    var fade = document.createElement("div");
    fade.className = "home-banner-fade";
    el.appendChild(fade);
    var title = document.createElement("span");
    title.className = "home-banner-title";
    title.textContent = banner.title || block.title || "Featured";
    el.appendChild(title);
    var tag = document.createElement("em");
    tag.className = "home-banner-tag";
    tag.textContent = openUrl ? "Open" : "banner";
    el.appendChild(tag);
    if (openUrl) {
      el.addEventListener("click", function () {
        openDiscoverCard({
          title: banner.title || block.title || "",
          openUrl: openUrl,
          wideUrl: banner.wideUrl || "",
        });
      });
    }
    return el;
  }

  function renderHomeBlocks(blocks, append) {
    if (!els.homeFeed) return;
    if (!append) {
      clearHeroCarousels();
      els.homeFeed.innerHTML = "";
    }
    var list = Array.isArray(blocks) ? blocks : [];
    var rendered = 0;
    list.forEach(function (raw) {
      var block = normalizeBlock(raw);
      if (!block) return;
      var node = null;
      switch (block.kind) {
        case "hero":
          node = renderHeroBlock(block);
          break;
        case "landscape_rail":
          node = renderLandscapeRail(block);
          break;
        case "poster_rail":
          node = renderPosterRail(block);
          break;
        case "banner":
          node = renderBannerBlock(block);
          break;
        default:
          // Unknown kinds: try poster rail if cards present.
          if (block.cards && block.cards.length) {
            node = renderPosterRail(block);
          }
          break;
      }
      if (node) {
        els.homeFeed.appendChild(node);
        rendered++;
      }
    });
    return rendered;
  }

  function renderSearchResults(cards) {
    if (!els.homeSearchResults || !els.homeSearchGrid) return;
    els.homeSearchGrid.innerHTML = "";
    var list = Array.isArray(cards) ? cards : [];
    list.forEach(function (raw) {
      var card = normalizeCard(raw);
      if (card) els.homeSearchGrid.appendChild(posterCardButton(card, "none"));
    });
    els.homeSearchResults.hidden = !list.length;
  }

  function clearSearchResults() {
    if (els.homeSearchResults) els.homeSearchResults.hidden = true;
    if (els.homeSearchGrid) els.homeSearchGrid.innerHTML = "";
  }

  function escapeHtml(s) {
    return String(s || "")
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function profileInitial(name) {
    var s = String(name || "").trim();
    if (!s) return "?";
    return s.charAt(0).toUpperCase();
  }

  function updateAccountChrome() {
    var name = state.activeProfileName || "Account";
    var sub = state.activeCRProfileName
      ? "Profile: " + state.activeCRProfileName
      : state.cookieFile
        ? "Cookie set"
        : "No profile";
    if (els.accountName) els.accountName.textContent = name;
    if (els.accountSub) els.accountSub.textContent = sub;
    if (els.accountAvatar) {
      els.accountAvatar.textContent = profileInitial(
        state.activeCRProfileName || state.activeProfileName || name
      );
    }
    if (els.btnAccount) {
      els.btnAccount.title =
        name + (state.activeCRProfileName ? " · " + state.activeCRProfileName : "");
    }
  }

  function closeAccountMenu() {
    if (!els.accountMenu) return;
    els.accountMenu.hidden = true;
    if (els.btnAccount) els.btnAccount.setAttribute("aria-expanded", "false");
  }

  function menuItem(label, opts) {
    opts = opts || {};
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className =
      "account-menu-item" +
      (opts.active ? " is-active" : "") +
      (opts.muted ? " is-muted" : "") +
      (opts.danger ? " is-danger" : "");
    btn.setAttribute("role", "menuitem");
    btn.textContent = label;
    if (opts.onClick) {
      btn.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        opts.onClick();
      });
    }
    return btn;
  }

  function menuSection(label) {
    var div = document.createElement("div");
    div.className = "account-menu-section";
    div.textContent = label;
    return div;
  }

  function menuSep() {
    var div = document.createElement("div");
    div.className = "account-menu-sep";
    div.setAttribute("role", "separator");
    return div;
  }

  function normalizeCookieProfile(raw) {
    if (!raw || typeof raw !== "object") return null;
    var id = fHome(raw, "id", "ID") || "";
    var cookieFile = fHome(raw, "cookieFile", "CookieFile") || "";
    if (!id && !cookieFile) return null;
    return {
      id: id,
      name: fHome(raw, "name", "Name") || cookieFile || "Profile",
      cookieFile: cookieFile,
    };
  }

  function normalizeCRProfile(raw) {
    if (!raw || typeof raw !== "object") return null;
    var id = fHome(raw, "id", "ID") || "";
    if (!id) return null;
    return {
      id: id,
      name: fHome(raw, "name", "Name") || id,
      isSelected: !!(fHome(raw, "isSelected", "IsSelected")),
    };
  }

  async function refreshAccountFromPrefs() {
    var app = goApp();
    if (!app || typeof app.GetPreferences !== "function") {
      updateAccountChrome();
      return;
    }
    try {
      var prefs = await app.GetPreferences();
      var cookie = fHome(prefs, "cookieFile", "CookieFile") || "";
      if (cookie) state.cookieFile = cookie;
      state.activeProfileID =
        fHome(prefs, "activeProfileId", "ActiveProfileID") || "";
      state.activeCRProfileID =
        fHome(prefs, "activeCrProfileId", "ActiveCRProfileID") || "";
      var profiles = fHome(prefs, "cookieProfiles", "CookieProfiles") || [];
      state.activeProfileName = "";
      if (Array.isArray(profiles)) {
        profiles.forEach(function (p) {
          var prof = normalizeCookieProfile(p);
          if (prof && prof.id === state.activeProfileID) {
            state.activeProfileName = prof.name;
            if (prof.cookieFile) state.cookieFile = prof.cookieFile;
          }
        });
        if (!state.activeProfileName && profiles.length === 1) {
          var only = normalizeCookieProfile(profiles[0]);
          if (only) {
            state.activeProfileName = only.name;
            state.activeProfileID = only.id;
            if (only.cookieFile) state.cookieFile = only.cookieFile;
          }
        }
      }
      if (!state.activeProfileName && state.cookieFile) {
        state.activeProfileName = "Cookie account";
      }
    } catch (e) {
      /* ignore */
    }
    updateAccountChrome();
  }

  async function switchCookieProfile(id) {
    var app = goApp();
    if (!app || typeof app.SwitchCookieProfile !== "function") {
      showBanner("Profile switch unavailable.", "err");
      return;
    }
    closeAccountMenu();
    try {
      await app.SwitchCookieProfile(id);
      await refreshAccountFromPrefs();
      state.homeLoaded = false;
      logLine("Switched cookie profile", "ok");
      if (state.view === "home") {
        loadHomeFeed(false);
      }
    } catch (err) {
      var msg = errMessage(err);
      showBanner("Switch profile failed: " + msg, "err");
      logLine("Switch cookie profile: " + msg, "err");
    }
  }

  async function addCookieProfile() {
    var app = goApp();
    closeAccountMenu();
    var name = window.prompt("Profile name (e.g. Premium-BR):", "");
    if (name == null) return;
    name = String(name).trim();
    if (!name) {
      showBanner("Profile name is required.", "warn");
      return;
    }
    var path = "";
    if (app && typeof app.PickCookieFile === "function") {
      try {
        var picked = await app.PickCookieFile();
        if (picked == null || picked === "") return;
        path = String(picked).trim();
      } catch (err) {
        showBanner("Cookie file dialog failed: " + errMessage(err), "err");
        return;
      }
    } else {
      path = window.prompt("Path to etp_rt cookie file:", state.cookieFile || "") || "";
      path = String(path).trim();
    }
    if (!path) {
      showBanner("Cookie file path is required.", "warn");
      return;
    }
    if (!app || typeof app.UpsertCookieProfile !== "function") {
      state.cookieFile = path;
      state.activeProfileName = name;
      updateAccountChrome();
      schedulePersist();
      return;
    }
    try {
      await app.UpsertCookieProfile({
        id: "",
        name: name,
        cookieFile: path,
      });
      // Upsert may activate when none active; list + switch to newest by name/path.
      var list =
        typeof app.ListCookieProfiles === "function"
          ? await app.ListCookieProfiles()
          : [];
      var match = null;
      (list || []).forEach(function (raw) {
        var p = normalizeCookieProfile(raw);
        if (!p) return;
        if (p.cookieFile === path || p.name === name) match = p;
      });
      if (match && typeof app.SwitchCookieProfile === "function") {
        await app.SwitchCookieProfile(match.id);
      }
      await refreshAccountFromPrefs();
      state.cookieFile = path;
      state.activeProfileName = name;
      updateAccountChrome();
      state.homeLoaded = false;
      logLine("Added cookie profile · " + name, "ok");
      if (state.view === "home") loadHomeFeed(false);
    } catch (err) {
      showBanner("Add profile failed: " + errMessage(err), "err");
      logLine("Upsert cookie profile: " + errMessage(err), "err");
    }
  }

  async function switchCRProfile(id) {
    var app = goApp();
    if (!app || typeof app.SwitchCRProfile !== "function") return;
    closeAccountMenu();
    try {
      await app.SwitchCRProfile(id);
      state.activeCRProfileID = id;
      state.homeLoaded = false;
      // Refresh name from list.
      try {
        if (typeof app.ListCRProfiles === "function") {
          var list = await app.ListCRProfiles();
          (list || []).forEach(function (raw) {
            var p = normalizeCRProfile(raw);
            if (p && p.id === id) state.activeCRProfileName = p.name;
          });
        }
      } catch (e) {
        /* ignore */
      }
      updateAccountChrome();
      logLine("Switched CR multiprofile", "ok");
      if (state.view === "home") loadHomeFeed(false);
    } catch (err) {
      showBanner("CR profile switch failed: " + errMessage(err), "err");
      logLine("Switch CR profile: " + errMessage(err), "err");
    }
  }

  async function openAccountMenu() {
    if (!els.accountMenu || !els.btnAccount) return;
    if (!els.accountMenu.hidden) {
      closeAccountMenu();
      return;
    }
    var app = goApp();
    var menu = els.accountMenu;
    menu.innerHTML = "";
    menu.appendChild(menuSection("Cookie profiles"));

    var profiles = [];
    if (app && typeof app.ListCookieProfiles === "function") {
      try {
        var rawList = await app.ListCookieProfiles();
        (rawList || []).forEach(function (raw) {
          var p = normalizeCookieProfile(raw);
          if (p) profiles.push(p);
        });
      } catch (e) {
        /* ignore */
      }
    }
    if (!profiles.length) {
      var hint = document.createElement("div");
      hint.className = "account-menu-hint";
      hint.textContent = state.cookieFile
        ? "One cookie path in use. Add a named profile to switch accounts."
        : "No profiles yet. Add one or use Cookie.";
      menu.appendChild(hint);
    } else {
      profiles.forEach(function (p) {
        var active =
          p.id === state.activeProfileID ||
          (!state.activeProfileID && p.cookieFile === state.cookieFile);
        menu.appendChild(
          menuItem(p.name || p.cookieFile || p.id, {
            active: active,
            onClick: function () {
              switchCookieProfile(p.id);
            },
          })
        );
      });
    }

    menu.appendChild(menuSep());
    menu.appendChild(
      menuItem("Add cookie profile…", {
        muted: true,
        onClick: function () {
          addCookieProfile();
        },
      })
    );
    menu.appendChild(
      menuItem("Set cookie file…", {
        muted: true,
        onClick: function () {
          closeAccountMenu();
          onCookie();
        },
      })
    );

    // CR multiprofile submenu when available.
    if (app && typeof app.ListCRProfiles === "function" && state.cookieFile) {
      try {
        var crRaw = await app.ListCRProfiles();
        var crList = [];
        (crRaw || []).forEach(function (raw) {
          var p = normalizeCRProfile(raw);
          if (p) crList.push(p);
        });
        if (crList.length > 0) {
          menu.appendChild(menuSep());
          menu.appendChild(menuSection("Crunchyroll profiles"));
          crList.forEach(function (p) {
            var selected =
              p.isSelected || p.id === state.activeCRProfileID;
            if (selected) state.activeCRProfileName = p.name;
            menu.appendChild(
              menuItem(p.name || p.id, {
                active: selected,
                onClick: function () {
                  switchCRProfile(p.id);
                },
              })
            );
          });
          updateAccountChrome();
        }
      } catch (e) {
        /* multiprofile optional */
      }
    }

    menu.hidden = false;
    els.btnAccount.setAttribute("aria-expanded", "true");
  }

  async function loadHomeFeed(append) {
    var app = goApp();
    if (!app || typeof app.GetHomeFeed !== "function") {
      setHomeStatus("Go bindings unavailable. Run inside the Wails app.", "err");
      return;
    }
    if (!state.cookieFile) {
      setHomeStatus("Set a cookie file (Cookie) to load your Discover feed.", "err");
      return;
    }
    if (state.homeLoading) return;

    state.homeLoading = true;
    var start = append ? state.homeNextStart || 0 : 0;
    if (!append) {
      setHomeStatus("Loading Discover feed…");
      clearSearchResults();
    } else {
      setHomeStatus("Loading more…");
    }
    if (els.btnHomeMore) els.btnHomeMore.disabled = true;
    if (els.btnHomeRefresh) els.btnHomeRefresh.disabled = true;

    try {
      // Persist cookie path first so GetHomeFeed reads current prefs.
      await persistPrefs();
      var raw = await app.GetHomeFeed(start, 20);
      var page = normalizeHomePage(raw);
      var blocks = page.blocks;
      // Fallback: older payloads with only heroes/rails.
      if ((!blocks || !blocks.length) && (page.heroes.length || page.rails.length)) {
        blocks = [];
        if (page.heroes.length) {
          blocks.push({ kind: "hero", heroes: page.heroes, rankStyle: "none" });
        }
        page.rails.forEach(function (rail) {
          blocks.push({
            kind: "poster_rail",
            title: fHome(rail, "title", "Title") || "Collection",
            cards: fHome(rail, "cards", "Cards") || [],
            rankStyle: "none",
          });
        });
      }
      var n = renderHomeBlocks(blocks, append) || 0;
      state.homeNextStart = page.nextStart || start + 20;
      state.homeLoaded = true;
      if (!append) {
        setHomeStatus(
          "Discover · " +
            n +
            " block(s) · locale " +
            (state.locale || "pt-BR"),
          "ok"
        );
      } else {
        setHomeStatus("Loaded more · +" + n + " block(s)", "ok");
      }
      if (els.homeMoreWrap) {
        els.homeMoreWrap.hidden = !(page.nextStart > start);
      }
      logLine(
        "Home feed · start=" + start + " blocks=" + n,
        "ok"
      );
    } catch (err) {
      var msg = errMessage(err);
      setHomeStatus("Home failed: " + msg, "err");
      logLine("Home feed error: " + msg, "err");
      showBanner("Home failed: " + msg, "err");
    } finally {
      state.homeLoading = false;
      if (els.btnHomeMore) els.btnHomeMore.disabled = false;
      if (els.btnHomeRefresh) els.btnHomeRefresh.disabled = false;
    }
  }

  async function onHomeSearch() {
    var q = (els.homeSearch && els.homeSearch.value.trim()) || "";
    if (!q) {
      clearSearchResults();
      setHomeStatus("Enter a search query.", "err");
      return;
    }
    var app = goApp();
    if (!app || typeof app.SearchTitles !== "function") {
      setHomeStatus("Go bindings unavailable.", "err");
      return;
    }
    if (!state.cookieFile) {
      setHomeStatus("Cookie file path required for search.", "err");
      return;
    }
    setHomeStatus("Searching “" + q + "”…");
    try {
      await persistPrefs();
      var raw = await app.SearchTitles(q);
      var cards = Array.isArray(raw) ? raw : [];
      renderSearchResults(cards);
      setHomeStatus(
        cards.length
          ? "Search · " + cards.length + " result(s) for “" + q + "”"
          : "No results for “" + q + "”",
        cards.length ? "ok" : "err"
      );
      logLine("Search “" + q + "” · " + cards.length + " hit(s)", "ok");
    } catch (err) {
      var msg = errMessage(err);
      setHomeStatus("Search failed: " + msg, "err");
      logLine("Search error: " + msg, "err");
      showBanner("Search failed: " + msg, "err");
    }
  }

  function openDiscoverCard(card) {
    var rawUrl = (card && card.openUrl) || "";
    var seriesId =
      (card && (card.seriesId || card.SeriesID || "")) || "";
    seriesId = String(seriesId).trim();
    // Prefer explicit episode id on card, else parse /watch/{id}/.
    var epId = "";
    if (card && card.id && /episode/i.test(String(card.type || ""))) {
      epId = String(card.id);
    }
    if (!epId) epId = episodeIdFromOpenUrl(rawUrl);
    // Full series catalog + select clicked episode (do NOT inspect bare /watch/ only).
    var url = rawUrl;
    if (seriesId) {
      url = "https://www.crunchyroll.com/series/" + encodeURIComponent(seriesId);
    } else if (epId) {
      // Fallback: still try series-style if openUrl was watch-only without seriesId.
      // Inspect of /watch/ returns a single-episode catalog — avoid that when possible.
      url = rawUrl;
    }
    if (!url) {
      showBanner("This title has no openable URL.", "warn");
      logLine("Open card blocked: missing openUrl", "warn");
      return;
    }
    if (els.url) els.url.value = url;
    state.url = url;
    state.pendingSelectEpisodeId = epId || "";
    logLine(
      "Open · " +
        ((card && card.title) || url) +
        (epId ? " (select " + epId + ")" : ""),
      "info",
      { quiet: true }
    );
    setAppView("download");
    schedulePersist();
    // Auto-Inspect when cookie is ready.
    if (state.cookieFile) {
      onInspect();
    } else {
      showBanner("Title opened. Set Cookie, then Inspect.", "warn");
    }
  }

  /* ── Preferences ── */

  function collectPreferences() {
    pullAdvancedInputsToState();
    var audioLangs = Object.keys(state.selectedAudio).filter(function (k) {
      return state.selectedAudio[k];
    });
    var subtitleLangs = [];
    if (!state.noSubtitles) {
      subtitleLangs = Object.keys(state.selectedSubs).filter(function (k) {
        return state.selectedSubs[k];
      });
    }
    var captionLangs = selectedCaptionLangs();
    state.captionLangs = captionLangs;
    return {
      URL: (els.url && els.url.value.trim()) || state.url || "",
      CookieFile: state.cookieFile || "",
      OutputDir: (els.output && els.output.value.trim()) || state.outputDir || "./Downloads",
      Mode: state.mode || "normal",
      Locale: state.locale || "pt-BR",
      AudioLangs: audioLangs,
      SubtitleLangs: subtitleLangs,
      CaptionLangs: captionLangs,
      VideoQuality: state.videoQuality || "max",
      AudioQuality: state.audioQuality || "max",
      LastSeason: state.selectedSeason || state.lastSeason || 0,
      WVDPath: state.wvdPath || "",
      ClientIDPath: state.clientIdPath || "",
      PrivateKeyPath: state.privateKeyPath || "",
      StrictLanguages: !!state.strictLanguages,
      Playback4294Retries: state.playback4294Retries,
      Playback4294BackoffSec: state.playback4294BackoffSec,
      IndexWindow: state.indexWindow,
      IndexCircuitLimit: state.indexCircuitLimit,
      DebugManifest: !!state.debugManifest,
      ProbeEveryEpisode: !!state.probeEveryEpisode,
    };
  }

  function applyPreferences(p) {
    if (!p || typeof p !== "object") return;
    var url = p.URL != null ? p.URL : p.url;
    var cookie = p.CookieFile != null ? p.CookieFile : p.cookieFile;
    var out = p.OutputDir != null ? p.OutputDir : p.outputDir;
    var mode = p.Mode != null ? p.Mode : p.mode;
    var vq = p.VideoQuality != null ? p.VideoQuality : p.videoQuality;
    var aq = p.AudioQuality != null ? p.AudioQuality : p.audioQuality;
    var last = p.LastSeason != null ? p.LastSeason : p.lastSeason;

    if (url && els.url) {
      els.url.value = url;
      state.url = url;
    }
    var locale = p.Locale != null ? p.Locale : p.locale;
    if (locale && String(locale).trim()) {
      state.locale = String(locale).trim();
    } else if (!state.locale) {
      state.locale = "pt-BR";
    }
    if (cookie) state.cookieFile = cookie;
    var activePid =
      p.ActiveProfileID != null ? p.ActiveProfileID : p.activeProfileId;
    if (activePid != null) state.activeProfileID = String(activePid || "");
    var activeCr =
      p.ActiveCRProfileID != null ? p.ActiveCRProfileID : p.activeCrProfileId;
    if (activeCr != null) state.activeCRProfileID = String(activeCr || "");
    var cookieProfiles =
      p.CookieProfiles != null ? p.CookieProfiles : p.cookieProfiles;
    if (Array.isArray(cookieProfiles)) {
      state.activeProfileName = "";
      cookieProfiles.forEach(function (raw) {
        var prof = normalizeCookieProfile(raw);
        if (!prof) return;
        if (prof.id === state.activeProfileID) {
          state.activeProfileName = prof.name;
          if (prof.cookieFile) state.cookieFile = prof.cookieFile;
        }
      });
      if (!state.activeProfileName && cookieProfiles.length === 1) {
        var only = normalizeCookieProfile(cookieProfiles[0]);
        if (only) {
          state.activeProfileName = only.name;
          if (!state.activeProfileID) state.activeProfileID = only.id;
          if (only.cookieFile) state.cookieFile = only.cookieFile;
        }
      }
    }
    if (!state.activeProfileName && state.cookieFile) {
      state.activeProfileName = "Cookie account";
    }
    updateAccountChrome();
    if (out) {
      state.outputDir = out;
      if (els.output) els.output.value = out;
    } else if (els.output && !els.output.value) {
      els.output.value = "./Downloads";
    }
    if (vq) {
      state.videoQuality = vq;
      setDropdownValue(els.ddVideo, vq, displayQuality(vq, true));
    }
    if (aq) {
      state.audioQuality = aq;
      setDropdownValue(els.ddAudio, aq, displayQuality(aq, false));
    }
    if (last) state.lastSeason = Number(last) || 0;

    var strict =
      p.StrictLanguages != null ? p.StrictLanguages : p.strictLanguages;
    if (strict != null) state.strictLanguages = !!strict;

    var caps = p.CaptionLangs != null ? p.CaptionLangs : p.captionLangs;
    if (Array.isArray(caps)) {
      state.captionLangs = caps.slice();
      state.selectedCaptions = {};
      state.noCaptions = true;
      caps.forEach(function (code) {
        if (!code) return;
        state.selectedCaptions[code] = true;
        state.noCaptions = false;
      });
    }

    var wvd = p.WVDPath != null ? p.WVDPath : p.wvdPath;
    if (wvd != null) state.wvdPath = wvd || "";
    var cid = p.ClientIDPath != null ? p.ClientIDPath : p.clientIdPath;
    if (cid != null) state.clientIdPath = cid || "";
    var pk = p.PrivateKeyPath != null ? p.PrivateKeyPath : p.privateKeyPath;
    if (pk != null) state.privateKeyPath = pk || "";

    var probe =
      p.ProbeEveryEpisode != null ? p.ProbeEveryEpisode : p.probeEveryEpisode;
    if (probe != null) state.probeEveryEpisode = !!probe;
    var dbg = p.DebugManifest != null ? p.DebugManifest : p.debugManifest;
    if (dbg != null) state.debugManifest = !!dbg;

    // Engine treats 0 / omitempty as "use default" for these numerics.
    var r =
      p.Playback4294Retries != null
        ? p.Playback4294Retries
        : p.playback4294Retries;
    if (r != null && Number(r) > 0) {
      state.playback4294Retries = Number(r);
    }

    var b =
      p.Playback4294BackoffSec != null
        ? p.Playback4294BackoffSec
        : p.playback4294BackoffSec;
    if (b != null && Number(b) > 0) {
      state.playback4294BackoffSec = Number(b);
    }

    var iw = p.IndexWindow != null ? p.IndexWindow : p.indexWindow;
    if (iw != null && Number(iw) > 0) state.indexWindow = Number(iw);

    var ic =
      p.IndexCircuitLimit != null ? p.IndexCircuitLimit : p.indexCircuitLimit;
    if (ic != null && Number(ic) > 0) state.indexCircuitLimit = Number(ic);

    syncAdvancedInputsFromState();
    renderCaptions();

    if (mode === "advanced" || mode === "normal") {
      setMode(mode);
    }
  }

  function ensureLocalWidevinePaths() {
    // Prefer explicit prefs; otherwise point Advanced fields at CDMs next to the app repo.
    // Engine also auto-discovers these; this keeps the UI honest about what will be used.
    var base =
      "C:\\Users\\Admin\\Documents\\Github\\crunchyroll-downloader";
    if (!state.clientIdPath) {
      state.clientIdPath = base + "\\client_id.bin";
    }
    if (!state.privateKeyPath) {
      state.privateKeyPath = base + "\\private_key.pem";
    }
  }

  async function loadPreferences() {
    var app = goApp();
    if (!app || typeof app.GetPreferences !== "function") {
      logLine("Preferences: runtime not ready (browser preview)", "warn");
      ensureLocalWidevinePaths();
      syncAdvancedInputsFromState();
      return;
    }
    try {
      var prefs = await app.GetPreferences();
      applyPreferences(prefs);
      ensureLocalWidevinePaths();
      syncAdvancedInputsFromState();
      schedulePersist();
      if (state.cookieFile) {
        logLine("Loaded preferences · cookie path set", "ok");
      } else {
        logLine("Loaded preferences", "ok");
      }
      if (state.clientIdPath && state.privateKeyPath) {
        logLine("Widevine CDM paths ready", "ok");
      }
    } catch (err) {
      logLine("Failed to load preferences: " + errMessage(err), "err");
      ensureLocalWidevinePaths();
      syncAdvancedInputsFromState();
    }
  }

  async function persistPrefs() {
    var app = goApp();
    if (!app || typeof app.SavePreferences !== "function") return;
    var p = collectPreferences();
    try {
      await app.SavePreferences(p);
    } catch (err) {
      logLine("Save preferences failed: " + errMessage(err), "err");
    }
  }

  function schedulePersist() {
    if (prefsTimer) clearTimeout(prefsTimer);
    prefsTimer = setTimeout(function () {
      prefsTimer = null;
      persistPrefs();
    }, 350);
  }

  function errMessage(err) {
    if (!err) return "unknown error";
    if (typeof err === "string") return err;
    // Wails often returns { message: "..." } or plain Error-like objects.
    if (err.message) return err.message;
    if (err.Message) return err.Message;
    if (err.error) return String(err.error);
    try {
      var s = JSON.stringify(err);
      if (s && s !== "{}" && s !== "null") return s;
      return String(err);
    } catch (e) {
      return "unknown error";
    }
  }

  /** Read a field that may be PascalCase (Go) or camelCase (json tags). */
  function field(obj, pascal, camel) {
    if (!obj) return undefined;
    if (obj[pascal] !== undefined && obj[pascal] !== null) return obj[pascal];
    if (camel && obj[camel] !== undefined && obj[camel] !== null) return obj[camel];
    return undefined;
  }

  /** Normalize InspectResult so the rest of the UI always sees PascalCase. */
  function normalizeInspectResult(raw) {
    if (!raw) return null;
    var episodes = field(raw, "Episodes", "episodes") || [];
    var seasons = field(raw, "Seasons", "seasons") || [];
    return {
      ContentType: field(raw, "ContentType", "contentType") || "",
      ContentID: field(raw, "ContentID", "contentID") || field(raw, "ContentID", "contentId") || "",
      Seasons: seasons.map(function (s) {
        return {
          ID: field(s, "ID", "id") || "",
          SeasonNumber: field(s, "SeasonNumber", "seasonNumber") || 0,
          Title: field(s, "Title", "title") || "",
          DisplayNumber:
            field(s, "DisplayNumber", "displayNumber") ||
            field(s, "SeasonDisplayNumber", "seasonDisplayNumber") ||
            "",
        };
      }),
      Episodes: episodes.map(function (ep) {
        return {
          ID: field(ep, "ID", "id") || "",
          SeasonNumber: field(ep, "SeasonNumber", "seasonNumber") || 0,
          EpisodeNumber: field(ep, "EpisodeNumber", "episodeNumber") || 0,
          Title: field(ep, "Title", "title") || "",
          SeriesTitle: field(ep, "SeriesTitle", "seriesTitle") || "",
          AudioLocales: field(ep, "AudioLocales", "audioLocales") || [],
          ThumbnailURL:
            field(ep, "ThumbnailURL", "thumbnailUrl") ||
            field(ep, "ThumbnailURL", "thumbnailURL") ||
            "",
        };
      }),
      AudioLocales: field(raw, "AudioLocales", "audioLocales") || [],
      SubtitleLocales: field(raw, "SubtitleLocales", "subtitleLocales") || [],
      CaptionLocales: field(raw, "CaptionLocales", "captionLocales") || [],
      VideoQualities: field(raw, "VideoQualities", "videoQualities") || [],
      AudioQualities: field(raw, "AudioQualities", "audioQualities") || [],
      DefaultEpisodeID:
        field(raw, "DefaultEpisodeID", "defaultEpisodeID") ||
        field(raw, "DefaultEpisodeID", "defaultEpisodeId") ||
        "",
      OriginalAudio: field(raw, "OriginalAudio", "originalAudio") || "",
      PosterURL:
        field(raw, "PosterURL", "posterUrl") ||
        field(raw, "PosterURL", "posterURL") ||
        "",
      DisplayTitle:
        field(raw, "DisplayTitle", "displayTitle") ||
        "",
    };
  }

  /* ── Catalog rendering ── */

  function seasonsFromCatalog(result) {
    var seasons = (result && result.Seasons) || [];
    if (seasons.length) return seasons.slice();
    // Watch-only / no season list: synthesize from episodes.
    var seen = {};
    var out = [];
    var eps = (result && result.Episodes) || [];
    eps.forEach(function (ep) {
      var n = ep.SeasonNumber || 0;
      if (seen[n]) return;
      seen[n] = true;
      out.push({ ID: "s" + n, SeasonNumber: n, Title: "", DisplayNumber: "" });
    });
    if (!out.length && result && result.ContentType === "watch") {
      out.push({ ID: "watch", SeasonNumber: 0, Title: "", DisplayNumber: "" });
    }
    return out;
  }

  function episodesForSeason(result, seasonNumber) {
    var eps = (result && result.Episodes) || [];
    if (seasonNumber == null) return eps.slice();
    // Season 0 / watch-only: show all.
    if (seasonNumber === 0 && !((result.Seasons || []).length)) {
      return eps.slice();
    }
    return eps.filter(function (ep) {
      return (ep.SeasonNumber || 0) === seasonNumber;
    });
  }

  function renderCheckbox(opts) {
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className =
      "cr-check" +
      (opts.on ? " is-on" : "") +
      (opts.thumbHtml ? " has-thumb" : "");
    if (opts.dataset) {
      Object.keys(opts.dataset).forEach(function (k) {
        btn.dataset[k] = opts.dataset[k];
      });
    }
    btn.innerHTML =
      '<span class="cr-box" aria-hidden="true">' +
      TICK_SVG +
      "</span>" +
      (opts.thumbHtml || "") +
      '<span class="cr-check-label">' +
      opts.labelHtml +
      "</span>";
    if (opts.onClick) btn.addEventListener("click", opts.onClick);
    return btn;
  }

  function renderMediaHero(result) {
    if (!els.mediaHero) return;
    if (!result) {
      els.mediaHero.hidden = true;
      return;
    }
    var title =
      result.DisplayTitle ||
      (result.Episodes && result.Episodes[0] && result.Episodes[0].SeriesTitle) ||
      (result.Episodes && result.Episodes[0] && result.Episodes[0].Title) ||
      "";
    var poster = result.PosterURL || "";
    // Fall back to first episode thumb if no series poster.
    if (!poster && result.Episodes && result.Episodes.length) {
      for (var i = 0; i < result.Episodes.length; i++) {
        if (result.Episodes[i].ThumbnailURL) {
          poster = result.Episodes[i].ThumbnailURL;
          break;
        }
      }
    }
    if (!title && !poster) {
      els.mediaHero.hidden = true;
      return;
    }
    els.mediaHero.hidden = false;
    if (els.mediaTitle) els.mediaTitle.textContent = title || "Untitled";
    if (els.mediaKicker) {
      els.mediaKicker.textContent =
        result.ContentType === "watch" ? "Episode / Movie" : "Series";
    }
    if (els.mediaPoster) {
      if (poster) {
        els.mediaPoster.hidden = false;
        els.mediaPoster.alt = title || "";
        els.mediaPoster.loading = "lazy";
        els.mediaPoster.referrerPolicy = "no-referrer";
        els.mediaPoster.src = poster;
        els.mediaPoster.onerror = function () {
          els.mediaPoster.hidden = true;
        };
      } else {
        els.mediaPoster.removeAttribute("src");
        els.mediaPoster.hidden = true;
      }
    }
  }

  function renderCatalog(result) {
    state.catalog = result || null;
    renderMediaHero(result);
    renderSeasons();
    renderSeasonHeading();
    renderEpisodes();
    renderAudio();
    renderSubs();
    renderCaptions();
    renderQualities();
  }

  /** Crunchyroll custom season label for the selected season. */
  function seasonDisplayTitle(season) {
    if (!season) return "";
    var custom = (season.Title || "").trim();
    if (custom) return custom;
    var disp = (season.DisplayNumber || "").trim();
    if (disp) return "Season " + disp;
    if (season.SeasonNumber === 0) return "Episodes";
    return "Season " + season.SeasonNumber;
  }

  function findSeasonByNumber(seasonNumber) {
    var seasons = seasonsFromCatalog(state.catalog);
    for (var i = 0; i < seasons.length; i++) {
      if (seasons[i].SeasonNumber === seasonNumber) return seasons[i];
    }
    return null;
  }

  function renderSeasonHeading() {
    if (!els.seasonHeading || !els.seasonHeadingText) return;
    if (!state.catalog || state.selectedSeason == null) {
      els.seasonHeading.hidden = true;
      els.seasonHeadingText.textContent = "";
      return;
    }
    var season = findSeasonByNumber(state.selectedSeason);
    // Synthesized watch-only season may only have a number.
    if (!season) {
      season = { SeasonNumber: state.selectedSeason, Title: "", DisplayNumber: "" };
    }
    var title = seasonDisplayTitle(season);
    if (!title) {
      els.seasonHeading.hidden = true;
      els.seasonHeadingText.textContent = "";
      return;
    }
    els.seasonHeading.hidden = false;
    els.seasonHeadingText.textContent = title;
  }

  function renderSeasons() {
    var root = els.seasons;
    if (!root) return;
    root.innerHTML = "";
    var seasons = seasonsFromCatalog(state.catalog);
    if (!seasons.length) {
      root.innerHTML = '<span class="muted-hint">Inspect a series to load seasons</span>';
      if (els.seasonHeading) els.seasonHeading.hidden = true;
      return;
    }
    seasons.forEach(function (s) {
      var chip = document.createElement("button");
      chip.type = "button";
      chip.className = "cr-chip";
      chip.dataset.season = String(s.SeasonNumber);
      if (s.SeasonNumber === 0 && seasons.length === 1) {
        chip.textContent = "Episode";
      } else {
        // Keep tabs compact — full CR season name is shown below the chips.
        chip.textContent = "Season " + s.SeasonNumber;
      }
      if (state.selectedSeason === s.SeasonNumber) {
        chip.classList.add("is-on");
      }
      chip.addEventListener("click", function () {
        chip.classList.add("is-press");
        setTimeout(function () {
          chip.classList.remove("is-press");
        }, 120);
        // Exclusive season selection only — never touch Select all or episodes here beyond filter.
        root.querySelectorAll(".cr-chip").forEach(function (c) {
          c.classList.remove("is-on");
        });
        void chip.offsetWidth;
        chip.classList.add("is-on");
        state.selectedSeason = s.SeasonNumber;
        state.lastSeason = s.SeasonNumber;
        renderSeasonHeading();
        schedulePersist();
        // Inspect only preloads one season (usually S1). Lazily fetch others.
        ensureSeasonEpisodesLoaded(s).then(function () {
          renderEpisodes();
        });
      });
      root.appendChild(chip);
    });
  }

  function seasonHasEpisodes(seasonNumber) {
    if (!state.catalog || !state.catalog.Episodes) return false;
    return state.catalog.Episodes.some(function (ep) {
      return (ep.SeasonNumber || 0) === seasonNumber;
    });
  }

  function mergeSeasonEpisodes(episodes) {
    if (!state.catalog) state.catalog = { Episodes: [], AudioLocales: [] };
    if (!state.catalog.Episodes) state.catalog.Episodes = [];
    var byId = {};
    state.catalog.Episodes.forEach(function (ep) {
      byId[ep.ID] = true;
    });
    var audioSeen = {};
    (state.catalog.AudioLocales || []).forEach(function (a) {
      audioSeen[a] = true;
    });
    (episodes || []).forEach(function (raw) {
      var ep = {
        ID: field(raw, "ID", "id") || "",
        SeasonNumber: field(raw, "SeasonNumber", "seasonNumber") || 0,
        EpisodeNumber: field(raw, "EpisodeNumber", "episodeNumber") || 0,
        Title: field(raw, "Title", "title") || "",
        SeriesTitle: field(raw, "SeriesTitle", "seriesTitle") || "",
        AudioLocales: field(raw, "AudioLocales", "audioLocales") || [],
        ThumbnailURL:
          field(raw, "ThumbnailURL", "thumbnailUrl") ||
          field(raw, "ThumbnailURL", "thumbnailURL") ||
          "",
      };
      if (!ep.ID || byId[ep.ID]) return;
      byId[ep.ID] = true;
      state.catalog.Episodes.push(ep);
      (ep.AudioLocales || []).forEach(function (a) {
        if (a && !audioSeen[a]) {
          audioSeen[a] = true;
          if (!state.catalog.AudioLocales) state.catalog.AudioLocales = [];
          state.catalog.AudioLocales.push(a);
        }
      });
    });
  }

  async function ensureSeasonEpisodesLoaded(season) {
    if (!season) return;
    var seasonNumber = season.SeasonNumber;
    if (seasonHasEpisodes(seasonNumber)) return;

    var seasonId = season.ID || "";
    if (!seasonId) {
      logLine("Season " + seasonNumber + " has no CMS id — cannot load episodes", "err");
      return;
    }

    var app = goApp();
    if (!app || typeof app.LoadSeasonEpisodes !== "function") {
      logLine("LoadSeasonEpisodes binding missing", "err");
      return;
    }

    if (els.episodes) {
      els.episodes.innerHTML =
        '<span class="muted-hint">Loading season ' + seasonNumber + "…</span>";
    }
    logLine("Loading episodes for season " + seasonNumber + "…", "info");
    try {
      var rows = await app.LoadSeasonEpisodes(seasonId);
      mergeSeasonEpisodes(rows || []);
      // Refresh audio list if new locales appeared
      renderAudio();
      logLine(
        "Season " +
          seasonNumber +
          " · " +
          (rows ? rows.length : 0) +
          " episode(s)",
        "ok"
      );
    } catch (err) {
      var msg = errMessage(err);
      showBanner("Could not load season " + seasonNumber + ": " + msg, "err");
      logLine("Season " + seasonNumber + " load failed: " + msg, "err");
    }
  }

  function renderEpisodes() {
    var root = els.episodes;
    if (!root) return;
    root.innerHTML = "";
    if (!state.catalog) {
      root.innerHTML = '<span class="muted-hint">No catalog yet</span>';
      return;
    }
    var list = episodesForSeason(state.catalog, state.selectedSeason);
    if (!list.length) {
      root.innerHTML =
        '<span class="muted-hint">No episodes loaded for this season</span>';
      return;
    }
    list.forEach(function (ep) {
      var on = !!state.selectedEpisodeIds[ep.ID];
      var label =
        '<strong class="ep-code">' +
        escapeHtml(epCode(ep)) +
        "</strong> · " +
        escapeHtml(ep.Title || "Untitled");
      var thumbHtml = "";
      if (ep.ThumbnailURL) {
        thumbHtml =
          '<img class="ep-thumb" alt="" loading="lazy" referrerpolicy="no-referrer" src="' +
          escapeHtml(ep.ThumbnailURL) +
          '" onerror="this.replaceWith(Object.assign(document.createElement(\'span\'),{className:\'ep-thumb-fallback\'}))" />';
      } else {
        thumbHtml = '<span class="ep-thumb-fallback" aria-hidden="true"></span>';
      }
      var row = renderCheckbox({
        on: on,
        dataset: { ep: ep.ID },
        labelHtml: label,
        thumbHtml: thumbHtml,
        onClick: function () {
          var next = !row.classList.contains("is-on");
          row.classList.toggle("is-on", next);
          if (next) state.selectedEpisodeIds[ep.ID] = true;
          else delete state.selectedEpisodeIds[ep.ID];
        },
      });
      root.appendChild(row);
    });
    refreshScrollHints(root);
  }

  /** Selected locales first, then optional preferred (e.g. original), then A–Z. */
  function sortLocalesSelectedFirst(locales, selectedMap, preferredCode) {
    return (locales || []).slice().sort(function (a, b) {
      var aOn = !!(selectedMap && selectedMap[a]);
      var bOn = !!(selectedMap && selectedMap[b]);
      if (aOn !== bOn) return aOn ? -1 : 1;
      if (preferredCode) {
        if (a === preferredCode && b !== preferredCode) return -1;
        if (b === preferredCode && a !== preferredCode) return 1;
      }
      return localeLabel(a).localeCompare(localeLabel(b), undefined, {
        sensitivity: "base",
      });
    });
  }

  /** Edge fade when a check-list can scroll (shading only, no “more” label). */
  function refreshScrollHints(el) {
    if (!el || !el.classList || !el.classList.contains("check-list")) return;
    function update() {
      var can = el.scrollHeight > el.clientHeight + 2;
      var top = el.scrollTop > 2;
      var bottom = el.scrollTop + el.clientHeight < el.scrollHeight - 2;
      el.classList.toggle("can-scroll", can);
      el.classList.toggle("scroll-more-top", can && top);
      el.classList.toggle("scroll-more-bottom", can && bottom);
    }
    if (!el._scrollHintsWired) {
      el._scrollHintsWired = true;
      el.addEventListener("scroll", update, { passive: true });
    }
    requestAnimationFrame(function () {
      requestAnimationFrame(update);
    });
  }

  function renderAudio() {
    var root = els.audio;
    if (!root) return;
    root.innerHTML = "";
    var locales =
      (state.catalog && state.catalog.AudioLocales) || [];
    if (!locales.length) {
      root.innerHTML = '<span class="muted-hint">Available after Inspect</span>';
      root.classList.remove("can-scroll", "scroll-more-top", "scroll-more-bottom");
      return;
    }
    var original = (state.catalog && state.catalog.OriginalAudio) || "";
    // Selected (e.g. Japanese) first so you don't scroll past the whole list.
    locales = sortLocalesSelectedFirst(locales, state.selectedAudio, original);
    locales.forEach(function (code) {
      var on = !!state.selectedAudio[code];
      var extra =
        code === original
          ? ' <span class="lang-code">(original)</span>'
          : "";
      var label = escapeHtml(localeLabel(code)) + extra;
      var row = renderCheckbox({
        on: on,
        dataset: { audio: code },
        labelHtml: label,
        onClick: function () {
          var next = !row.classList.contains("is-on");
          if (next) state.selectedAudio[code] = true;
          else delete state.selectedAudio[code];
          schedulePersist();
          renderAudio(); // re-sort selected to top
        },
      });
      root.appendChild(row);
    });
    refreshScrollHints(root);
  }

  function renderSubs() {
    var root = els.subs;
    if (!root) return;
    root.innerHTML = "";

    var noneRow = renderCheckbox({
      on: state.noSubtitles,
      dataset: { sub: "none" },
      labelHtml: "No subtitles",
      onClick: function () {
        state.noSubtitles = true;
        state.selectedSubs = {};
        schedulePersist();
        renderSubs();
      },
    });
    root.appendChild(noneRow);

    var locales = (state.catalog && state.catalog.SubtitleLocales) || [];
    locales = sortLocalesSelectedFirst(
      locales,
      state.noSubtitles ? {} : state.selectedSubs,
      null
    );
    locales.forEach(function (code) {
      var on = !state.noSubtitles && !!state.selectedSubs[code];
      var row = renderCheckbox({
        on: on,
        dataset: { sub: code },
        labelHtml: escapeHtml(localeLabel(code)),
        onClick: function () {
          var next = !row.classList.contains("is-on");
          if (next) {
            state.noSubtitles = false;
            state.selectedSubs[code] = true;
          } else {
            delete state.selectedSubs[code];
            var any = Object.keys(state.selectedSubs).some(function (k) {
              return state.selectedSubs[k];
            });
            if (!any) state.noSubtitles = true;
          }
          schedulePersist();
          renderSubs();
        },
      });
      root.appendChild(row);
    });
    refreshScrollHints(root);
  }

  function renderCaptions() {
    var root = els.ccList;
    if (!root) return;
    root.innerHTML = "";

    var noneRow = renderCheckbox({
      on: state.noCaptions,
      dataset: { cc: "none" },
      labelHtml: "No CC",
      onClick: function () {
        state.noCaptions = true;
        state.selectedCaptions = {};
        state.captionLangs = [];
        schedulePersist();
        renderCaptions();
      },
    });
    root.appendChild(noneRow);

    var locales = (state.catalog && state.catalog.CaptionLocales) || [];
    // Also surface previously selected caption codes even if inspect had none.
    var extra = Object.keys(state.selectedCaptions || {});
    extra.forEach(function (code) {
      if (locales.indexOf(code) < 0) locales = locales.concat([code]);
    });
    locales = sortLocalesSelectedFirst(
      locales,
      state.noCaptions ? {} : state.selectedCaptions,
      null
    );

    if (!locales.length && state.noCaptions) {
      // Keep No CC only until Inspect discovers caption tracks.
    }

    locales.forEach(function (code) {
      var on = !state.noCaptions && !!state.selectedCaptions[code];
      var row = renderCheckbox({
        on: on,
        dataset: { cc: code },
        labelHtml: escapeHtml(localeLabel(code)) + " [CC]",
        onClick: function () {
          var next = !row.classList.contains("is-on");
          if (next) {
            state.noCaptions = false;
            state.selectedCaptions[code] = true;
          } else {
            delete state.selectedCaptions[code];
            var any = Object.keys(state.selectedCaptions).some(function (k) {
              return state.selectedCaptions[k];
            });
            if (!any) state.noCaptions = true;
          }
          state.captionLangs = selectedCaptionLangs();
          schedulePersist();
          renderCaptions();
        },
      });
      root.appendChild(row);
    });
    refreshScrollHints(root);
  }

  function displayQuality(val, isVideo) {
    if (!val || val === "max") return "max";
    return val;
  }

  function qualityMenuLabel(val, index) {
    if (index === 0) return val + " (max)";
    return val;
  }

  function fillDropdown(root, values, selected, isVideo) {
    if (!root) return;
    var menu = root.querySelector(".cr-dd-menu");
    var valueEl = root.querySelector(".cr-dd-value");
    if (!menu || !valueEl) return;
    menu.innerHTML = "";

    var items = values && values.length ? values.slice() : ["max"];
    var sel = selected || "max";
    // "max" policy highlights the first (highest) entry.
    if (sel !== "max" && items.indexOf(sel) < 0) {
      sel = "max";
    }

    items.forEach(function (val, i) {
      var item = document.createElement("button");
      item.type = "button";
      item.className = "cr-dd-item";
      item.dataset.val = val;
      item.textContent = val === "max" ? "max" : qualityMenuLabel(val, i);
      var active = sel === "max" ? i === 0 : val === sel;
      if (active) item.classList.add("is-active");
      item.addEventListener("click", function (e) {
        e.stopPropagation();
        menu.querySelectorAll(".cr-dd-item").forEach(function (el) {
          el.classList.remove("is-active");
        });
        item.classList.add("is-active");
        // First entry = max policy; otherwise store fixed label.
        var store = i === 0 ? "max" : val;
        valueEl.textContent =
          i === 0
            ? val === "max"
              ? "max"
              : qualityMenuLabel(val, 0)
            : val;
        root.dataset.value = store;
        if (isVideo) state.videoQuality = store;
        else state.audioQuality = store;
        closeDropdown(root);
        schedulePersist();
      });
      menu.appendChild(item);
    });

    if (sel === "max") {
      valueEl.textContent =
        items[0] === "max" ? "max" : qualityMenuLabel(items[0], 0);
      root.dataset.value = "max";
    } else {
      valueEl.textContent = displayQuality(sel, isVideo);
      root.dataset.value = sel;
    }
  }

  function setDropdownValue(root, value, label) {
    if (!root) return;
    root.dataset.value = value || "max";
    var valueEl = root.querySelector(".cr-dd-value");
    if (valueEl) valueEl.textContent = label || value || "max";
    var menu = root.querySelector(".cr-dd-menu");
    if (!menu) return;
    menu.querySelectorAll(".cr-dd-item").forEach(function (item) {
      var active =
        item.dataset.val === value ||
        (value === "max" && item === menu.querySelector(".cr-dd-item"));
      item.classList.toggle("is-active", !!active);
    });
  }

  function renderQualities() {
    var vq = (state.catalog && state.catalog.VideoQualities) || [];
    var aq = (state.catalog && state.catalog.AudioQualities) || [];
    fillDropdown(els.ddVideo, vq.length ? vq : ["max"], state.videoQuality, true);
    fillDropdown(els.ddAudio, aq.length ? aq : ["max"], state.audioQuality, false);
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  /* ── Defaults after Inspect ── */

  function applyDefaults(result) {
    if (!result) return;

    // Season: prefer lastSeason if present, else season of default ep, else first.
    var seasons = seasonsFromCatalog(result);
    var seasonPick = null;
    if (state.lastSeason && seasons.some(function (s) {
      return s.SeasonNumber === state.lastSeason;
    })) {
      seasonPick = state.lastSeason;
    } else if (result.DefaultEpisodeID) {
      var defEp = (result.Episodes || []).find(function (ep) {
        return ep.ID === result.DefaultEpisodeID;
      });
      if (defEp) seasonPick = defEp.SeasonNumber;
    }
    if (seasonPick == null && seasons.length) {
      seasonPick = seasons[0].SeasonNumber;
    }
    state.selectedSeason = seasonPick;

    // Episodes: prefer pending Home click (Continue Watching /watch id), else defaults.
    state.selectedEpisodeIds = {};
    var eps = result.Episodes || [];
    var pendingEp = String(state.pendingSelectEpisodeId || "").trim();
    state.pendingSelectEpisodeId = "";
    var pendingMatch = null;
    if (pendingEp) {
      pendingMatch = eps.find(function (ep) {
        return ep.ID === pendingEp;
      });
      // Some catalogs use slightly different casing or nested ids.
      if (!pendingMatch) {
        pendingMatch = eps.find(function (ep) {
          return String(ep.ID || "").toUpperCase() === pendingEp.toUpperCase();
        });
      }
    }
    if (pendingMatch) {
      state.selectedEpisodeIds[pendingMatch.ID] = true;
      state.selectedSeason = pendingMatch.SeasonNumber;
      seasonPick = pendingMatch.SeasonNumber;
    } else if (result.ContentType === "watch" && eps.length === 1) {
      state.selectedEpisodeIds[eps[0].ID] = true;
    } else if (result.DefaultEpisodeID) {
      state.selectedEpisodeIds[result.DefaultEpisodeID] = true;
    } else {
      var s1e1 = eps.find(function (ep) {
        return ep.SeasonNumber === 1 && ep.EpisodeNumber === 1;
      });
      if (s1e1) {
        state.selectedEpisodeIds[s1e1.ID] = true;
      } else if (eps.length === 1) {
        state.selectedEpisodeIds[eps[0].ID] = true;
      }
    }

    // Audio: originalAudio only
    state.selectedAudio = {};
    var original = result.OriginalAudio || "";
    if (original) {
      state.selectedAudio[original] = true;
    } else if ((result.AudioLocales || []).length) {
      state.selectedAudio[result.AudioLocales[0]] = true;
    }

    // Subs: none
    state.noSubtitles = true;
    state.selectedSubs = {};

    // Captions: keep prior selection if still available; else default none
    var availableCC = result.CaptionLocales || [];
    if (availableCC.length && !state.noCaptions) {
      var nextCaps = {};
      Object.keys(state.selectedCaptions).forEach(function (code) {
        if (availableCC.indexOf(code) >= 0) nextCaps[code] = true;
      });
      state.selectedCaptions = nextCaps;
      state.noCaptions = !Object.keys(nextCaps).length;
      state.captionLangs = selectedCaptionLangs();
    } else if (!availableCC.length) {
      // Inspect found no CC — keep No CC exclusive default for UI
      state.noCaptions = true;
      state.selectedCaptions = {};
      state.captionLangs = [];
    }

    // Video/audio quality: first entry if non-empty (max first), else "max"
    if ((result.VideoQualities || []).length) {
      state.videoQuality = "max";
    } else {
      state.videoQuality = "max";
    }
    if ((result.AudioQualities || []).length) {
      state.audioQuality = "max";
    } else {
      state.audioQuality = "max";
    }

    renderCatalog(result);
    // After catalog paints, focus the Continue Watching (or other) episode we opened.
    if (pendingMatch) {
      var focusId = pendingMatch.ID;
      setTimeout(function () {
        scrollEpisodeIntoView(focusId);
      }, 80);
    }
  }

  /* ── Dropdown open/close ── */

  function closeDropdown(root) {
    if (!root) return;
    if (!root.classList.contains("is-open") && !root.classList.contains("is-closing")) {
      return;
    }
    root.classList.remove("is-open");
    root.classList.add("is-closing");
    var btn = root.querySelector(".cr-dd-btn");
    if (btn) btn.setAttribute("aria-expanded", "false");
    setTimeout(function () {
      root.classList.remove("is-closing");
    }, 200);
  }

  function openDropdown(root) {
    document.querySelectorAll(".cr-dd.is-open, .cr-dd.is-closing").forEach(function (other) {
      if (other !== root) closeDropdown(other);
    });
    root.classList.remove("is-closing");
    root.classList.add("is-open");
    var btn = root.querySelector(".cr-dd-btn");
    if (btn) btn.setAttribute("aria-expanded", "true");
  }

  function wireDropdown(root) {
    var btn = root.querySelector(".cr-dd-btn");
    if (!btn) return;
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      if (root.classList.contains("is-open")) closeDropdown(root);
      else openDropdown(root);
    });
  }

  /* ── Actions ── */

  async function onInspect() {
    clearBanner();
    var url = (els.url && els.url.value.trim()) || "";
    if (!url) {
      showBanner("Paste a Crunchyroll series or episode URL first.", "warn");
      logLine("Inspect blocked: missing URL", "warn");
      return;
    }
    if (!state.cookieFile) {
      showBanner(
        "Cookie file path is missing. Click Cookie and enter the path to your etp_rt file.",
        "err"
      );
      logLine("Inspect blocked: no cookie file path", "err");
      return;
    }

    var app = goApp();
    if (!app || typeof app.Inspect !== "function") {
      showBanner("Go bindings unavailable. Run inside the Wails app.", "err");
      logLine("Inspect failed: window.go.main.App.Inspect missing", "err");
      return;
    }

    setBusy(true);
    logLine("Inspecting " + url + "…", "info");
    state.url = url;
    pullAdvancedInputsToState();

    // Always probe playback once for subs/CC/qualities. Multi-episode probing
    // (ProbeEveryEpisode) is stored in prefs for future multi-probe work;
    // Task 8 still uses a single probe pass.
    if (state.probeEveryEpisode) {
      logLine(
        "Probe every episode is enabled — Inspect still probes once for catalog qualities (per-ep probe deferred)",
        "warn"
      );
    }

    var audioHint = Object.keys(state.selectedAudio).filter(function (k) {
      return state.selectedAudio[k];
    })[0] || "";
    var subsHint = "";
    if (!state.noSubtitles) {
      subsHint =
        Object.keys(state.selectedSubs).filter(function (k) {
          return state.selectedSubs[k];
        })[0] || "";
    }

    // camelCase keys match Go json tags (required by Wails bindings).
    var req = {
      url: url,
      etpRtFile: state.cookieFile,
      primaryAudioHint: audioHint,
      primarySubsHint: subsHint,
      probePlayback: true,
      probeContentId: "",
    };

    try {
      await persistPrefs();
      var raw = await app.Inspect(req);
      var result = normalizeInspectResult(raw);
      applyDefaults(result);
      await persistPrefs();
      var epCount = (result.Episodes || []).length;
      var seasonCount = (result.Seasons || []).length;
      var ccCount = (result.CaptionLocales || []).length;
      logLine(
        "Inspect complete · " +
          epCount +
          " episode(s)" +
          (seasonCount ? ", " + seasonCount + " season(s)" : "") +
          (result.OriginalAudio ? " · original " + result.OriginalAudio : "") +
          (ccCount ? " · " + ccCount + " CC locale(s)" : ""),
        "ok"
      );
      if ((result.VideoQualities || []).length) {
        logLine(
          "Qualities · video " +
            result.VideoQualities.join(", ") +
            " · audio " +
            ((result.AudioQualities || []).join(", ") || "—"),
          "info"
        );
      }
    } catch (err) {
      var msg = errMessage(err);
      showBanner("Inspect failed: " + msg, "err");
      logLine("Inspect error: " + msg, "err");
    } finally {
      setBusy(false);
    }
  }

  function onSelectAllEpisodes(e) {
    if (e) {
      e.preventDefault();
      e.stopPropagation();
    }
    // MUST NOT mark select-all as selected or clear seasons.
    if (els.selectAll) {
      els.selectAll.classList.remove("is-on");
    }
    var rows = els.episodes
      ? els.episodes.querySelectorAll(".cr-check")
      : [];
    if (!rows.length) return;
    var allOn = Array.prototype.every.call(rows, function (r) {
      return r.classList.contains("is-on");
    });
    var next = !allOn;
    Array.prototype.forEach.call(rows, function (row) {
      row.classList.toggle("is-on", next);
      var id = row.dataset.ep;
      if (!id) return;
      if (next) state.selectedEpisodeIds[id] = true;
      else delete state.selectedEpisodeIds[id];
    });
    // Seasons intentionally untouched.
  }

  async function onCookie() {
    var app = goApp();
    if (app && typeof app.PickCookieFile === "function") {
      try {
        var picked = await app.PickCookieFile();
        if (picked == null || picked === "") return; // cancelled
        state.cookieFile = String(picked).trim();
        // Keep active profile's cookie path in sync when we have a named profile.
        if (
          app &&
          state.activeProfileID &&
          typeof app.UpsertCookieProfile === "function"
        ) {
          try {
            await app.UpsertCookieProfile({
              id: state.activeProfileID,
              name: state.activeProfileName || "Account",
              cookieFile: state.cookieFile,
            });
          } catch (e) {
            /* path still set on CookieFile via prefs */
          }
        }
        if (!state.activeProfileName) state.activeProfileName = "Cookie account";
        updateAccountChrome();
        logLine("Cookie path set", "ok");
        schedulePersist();
        if (state.view === "home" && state.cookieFile) {
          state.homeLoaded = false;
          loadHomeFeed(false);
        }
        return;
      } catch (err) {
        logLine("Cookie file dialog failed: " + errMessage(err), "err");
        showBanner("Cookie file dialog failed: " + errMessage(err), "err");
        return;
      }
    }
    // Browser preview fallback when Wails bindings are unavailable.
    var current = state.cookieFile || "";
    var path = window.prompt(
      "Path to etp_rt cookie file (regular file, private):",
      current
    );
    if (path == null) return;
    path = path.trim();
    state.cookieFile = path;
    if (path) {
      if (!state.activeProfileName) state.activeProfileName = "Cookie account";
      updateAccountChrome();
      logLine("Cookie path set", "ok");
      if (state.view === "home") {
        state.homeLoaded = false;
        loadHomeFeed(false);
      }
    } else {
      logLine("Cookie path cleared", "warn");
      updateAccountChrome();
    }
    schedulePersist();
  }

  async function onOutputBrowse() {
    var app = goApp();
    if (app && typeof app.PickOutputDir === "function") {
      try {
        var picked = await app.PickOutputDir();
        if (picked == null || picked === "") return; // cancelled
        var dir = String(picked).trim() || "./Downloads";
        state.outputDir = dir;
        if (els.output) els.output.value = dir;
        schedulePersist();
        return;
      } catch (err) {
        logLine("Output folder dialog failed: " + errMessage(err), "err");
        showBanner("Output folder dialog failed: " + errMessage(err), "err");
        return;
      }
    }
    var current =
      (els.output && els.output.value) || state.outputDir || "./Downloads";
    var path = window.prompt("Output folder path:", current);
    if (path == null) return;
    path = path.trim();
    if (!path) path = "./Downloads";
    state.outputDir = path;
    if (els.output) els.output.value = path;
    schedulePersist();
  }

  function selectedEpisodeIdsInCatalogOrder() {
    var ids = [];
    var seen = {};
    var eps = (state.catalog && state.catalog.Episodes) || [];
    eps.forEach(function (ep) {
      var id = ep.ID || ep.id;
      if (!id || !state.selectedEpisodeIds[id] || seen[id]) return;
      seen[id] = true;
      ids.push(id);
    });
    Object.keys(state.selectedEpisodeIds).forEach(function (id) {
      if (state.selectedEpisodeIds[id] && !seen[id]) {
        seen[id] = true;
        ids.push(id);
      }
    });
    return ids;
  }

  function buildDownloadJob() {
    pullAdvancedInputsToState();
    var epIds = selectedEpisodeIdsInCatalogOrder();
    var audio = Object.keys(state.selectedAudio).filter(function (k) {
      return state.selectedAudio[k];
    });
    var subs = [];
    if (!state.noSubtitles) {
      subs = Object.keys(state.selectedSubs).filter(function (k) {
        return state.selectedSubs[k];
      });
    }
    var captions = selectedCaptionLangs();
    state.captionLangs = captions;
    var output =
      (els.output && els.output.value.trim()) || state.outputDir || "./Downloads";
    // camelCase keys match Go json tags (required by Wails bindings).
    return {
      episodeIds: epIds,
      audioLangs: audio,
      subtitleLangs: subs,
      captionLangs: captions,
      videoQuality: state.videoQuality || "max",
      audioQuality: state.audioQuality || "max",
      outputDir: output,
      strictLangs: !!state.strictLanguages,
    };
  }

  function catalogEpisodeById(id) {
    var eps = (state.catalog && state.catalog.Episodes) || [];
    for (var i = 0; i < eps.length; i++) {
      if ((eps[i].ID || eps[i].id) === id) return eps[i];
    }
    return null;
  }

  function onPlaySelected() {
    if (!state.catalog || !state.catalog.Episodes || !state.catalog.Episodes.length) {
      showBanner("Inspect a series or episode before playing.", "warn");
      return;
    }
    var ids = selectedEpisodeIdsInCatalogOrder();
    if (!ids.length) {
      showBanner("Select at least one episode before playing.", "warn");
      return;
    }
    var ep = catalogEpisodeById(ids[0]);
    if (!ep) {
      showBanner("Selected episode is not in the catalog.", "warn");
      return;
    }
    openPlayOverlay({
      seriesTitle:
        ep.SeriesTitle || (state.catalog && state.catalog.DisplayTitle) || "",
      episodeTitle: ep.Title || "Untitled",
      seasonNumber: ep.SeasonNumber,
      episodeNumber: ep.EpisodeNumber,
      episodeId: ep.ID,
    });
  }

  async function onDownload() {
    clearBanner();

    // While running, the download button becomes Cancel.
    if (state.downloading) {
      var appCancel = goApp();
      if (appCancel && typeof appCancel.CancelDownload === "function") {
        try {
          await appCancel.CancelDownload();
          logLine("Cancel requested…", "warn");
          showBanner("Cancelling download…", "warn");
        } catch (err) {
          logLine("Cancel failed: " + errMessage(err), "err");
        }
      }
      return;
    }

    var job = buildDownloadJob();
    if (!job.episodeIds.length) {
      showBanner("Select at least one episode before downloading.", "warn");
      logLine("Download blocked: no episodes selected", "warn");
      return;
    }
    if (!job.audioLangs.length) {
      showBanner("Select at least one audio language.", "warn");
      logLine("Download blocked: no audio selected", "warn");
      return;
    }
    if (!state.cookieFile) {
      showBanner("Cookie file path required.", "err");
      logLine("Download blocked: no cookie path", "err");
      return;
    }

    var app = goApp();
    if (!app || typeof app.StartDownload !== "function") {
      showBanner("Go bindings unavailable. Run inside the Wails app.", "err");
      logLine("Download failed: window.go.main.App.StartDownload missing", "err");
      return;
    }

    await persistPrefs();

    try {
      await app.StartDownload(job);
    } catch (err) {
      var msg = errMessage(err);
      showBanner("Could not start download: " + msg, "err");
      logLine("StartDownload error: " + msg, "err");
      return;
    }

    setDownloading(true);
    subscribeProgress();
    logLine(
      "Download started · " +
        job.episodeIds.length +
        " episode(s) · audio " +
        job.audioLangs.join(",") +
        (job.subtitleLangs.length
          ? " · subs " + job.subtitleLangs.join(",")
          : " · no subs") +
        (job.captionLangs.length
          ? " · CC " + job.captionLangs.join(",")
          : "") +
        (job.strictLangs ? " · strict langs" : ""),
      "ok"
    );
    if (els.queue) {
      els.queue.textContent =
        "Queued · " + job.episodeIds.length + " episode(s)";
    }
  }

  function promptPath(title, current) {
    var path = window.prompt(title, current || "");
    if (path == null) return null;
    return path.trim();
  }

  async function pickDevicePath(title) {
    var app = goApp();
    if (app && typeof app.PickDeviceFile === "function") {
      var picked = await app.PickDeviceFile(title || "Select file");
      if (picked == null || picked === "") return null; // cancelled
      return String(picked).trim();
    }
    return promptPath(title, "");
  }

  function wirePathField(inputEl, btnEl, title, apply) {
    function commit(path) {
      if (path == null) return;
      apply(path);
      if (inputEl) inputEl.value = path;
      schedulePersist();
    }
    if (btnEl) {
      btnEl.addEventListener("click", async function () {
        try {
          var path = await pickDevicePath(title);
          commit(path);
        } catch (err) {
          logLine("File dialog failed: " + errMessage(err), "err");
          showBanner("File dialog failed: " + errMessage(err), "err");
        }
      });
    }
    if (inputEl) {
      inputEl.addEventListener("change", function () {
        commit(inputEl.value.trim());
      });
    }
  }

  function wireSwitch(el, apply) {
    if (!el) return;
    el.addEventListener("click", function () {
      var next = !el.classList.contains("is-on");
      setSwitch(el, next);
      apply(next);
      schedulePersist();
    });
  }

  function wireNumber(el, apply) {
    if (!el) return;
    var handler = function () {
      pullAdvancedInputsToState();
      apply();
      schedulePersist();
    };
    el.addEventListener("change", handler);
    el.addEventListener("blur", handler);
  }

  async function onBuildIndex(fetchSubs) {
    clearBanner();
    if (formIsBusy()) return;

    var url = (els.url && els.url.value.trim()) || state.url || "";
    if (!url) {
      showBanner("Paste a series URL in the toolbar first.", "warn");
      logLine("Index blocked: missing URL", "warn");
      return;
    }
    if (!state.cookieFile) {
      showBanner("Cookie file path required for index tools.", "err");
      logLine("Index blocked: no cookie path", "err");
      return;
    }
    if (url.indexOf("/series/") < 0) {
      showBanner("Index tools require a /series/ URL.", "warn");
      logLine("Index blocked: not a series URL", "warn");
      return;
    }

    var app = goApp();
    if (!app || typeof app.BuildIndex !== "function") {
      showBanner("Go bindings unavailable. Run inside the Wails app.", "err");
      logLine("Index failed: window.go.main.App.BuildIndex missing", "err");
      return;
    }

    pullAdvancedInputsToState();
    await persistPrefs();

    var label = fetchSubs ? "Index subtitles" : "Build catalog";
    setIndexing(true, label);
    logLine(label + " · " + url + "…", "info");

    try {
      await app.BuildIndex(url, !!fetchSubs);
      showBanner(label + " finished", "ok");
      logLine(label + " complete", "ok");
    } catch (err) {
      var msg = errMessage(err);
      showBanner(label + " failed: " + msg, "err");
      logLine(label + " error: " + msg, "err");
    } finally {
      setIndexing(false);
    }
  }

  function onLoadBatchFile() {
    var path = promptPath(
      "Path to a .txt file with one Crunchyroll URL per line:",
      ""
    );
    if (path == null || !path) return;
    // Browser sandbox cannot read arbitrary files; ask the user to paste if needed.
    // When a native dialog lands (Task 9), this can load contents automatically.
    logLine(
      "Batch list path noted: " +
        path +
        " — paste file contents into Batch URLs (native open deferred)",
      "warn"
    );
    if (els.batchUrls && !els.batchUrls.value.trim()) {
      els.batchUrls.placeholder =
        "Paste URLs from " + path + " (one per line)…";
    }
    schedulePersist();
  }

  function wireUI() {
    if (els.modeNormal) {
      els.modeNormal.addEventListener("click", function () {
        setMode("normal");
      });
    }
    if (els.modeAdvanced) {
      els.modeAdvanced.addEventListener("click", function () {
        setMode("advanced");
      });
    }
    function wireNav(el, page) {
      if (!el) return;
      el.addEventListener("click", function () {
        setShellPage(page);
      });
    }
    wireNav(els.navHome, "home");
    wireNav(els.navDownload, "download");
    wireNav(els.navQueue, "queue");
    wireNav(els.navHistory, "history");
    wireNav(els.navSettings, "settings");
    if (els.brandHome) {
      els.brandHome.addEventListener("click", function () {
        setShellPage("home");
      });
      els.brandHome.addEventListener("keydown", function (e) {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          setShellPage("home");
        }
      });
    }
    if (els.tabActivity) {
      els.tabActivity.addEventListener("click", function () {
        setActivityTab("activity");
      });
    }
    if (els.tabQueue) {
      els.tabQueue.addEventListener("click", function () {
        setActivityTab("queue");
      });
    }
    if (els.tabProgress) {
      els.tabProgress.addEventListener("click", function () {
        setActivityTab("progress");
      });
    }
    if (els.btnClearActivity) {
      els.btnClearActivity.addEventListener("click", function () {
        if (!els.activity) return;
        els.activity.innerHTML = "";
        logLine("● Ready", "ok", { quiet: true });
      });
    }
    if (els.btnGotoDownloadFromQueue) {
      els.btnGotoDownloadFromQueue.addEventListener("click", function () {
        setShellPage("download");
        setActivityTab("queue");
        setActivityCollapsed(false);
      });
    }
    if (els.btnGotoHomeFromHistory) {
      els.btnGotoHomeFromHistory.addEventListener("click", function () {
        setShellPage("home");
      });
    }
    if (els.btnSettingsCookie) {
      els.btnSettingsCookie.addEventListener("click", onCookie);
    }
    if (els.btnGotoDownloadSettings) {
      els.btnGotoDownloadSettings.addEventListener("click", function () {
        setShellPage("download");
        setMode("advanced");
      });
    }
    if (els.btnHomeSearch) {
      els.btnHomeSearch.addEventListener("click", onHomeSearch);
    }
    if (els.btnHomeRefresh) {
      els.btnHomeRefresh.addEventListener("click", function () {
        state.homeLoaded = false;
        loadHomeFeed(false);
      });
    }
    if (els.btnHomeMore) {
      els.btnHomeMore.addEventListener("click", function () {
        loadHomeFeed(true);
      });
    }
    if (els.homeSearch) {
      els.homeSearch.addEventListener("keydown", function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          onHomeSearch();
        }
      });
    }
    if (els.btnToggleActivity) {
      els.btnToggleActivity.addEventListener("click", function () {
        setActivityCollapsed(!isActivityCollapsed());
      });
    }
    if (els.inspect) els.inspect.addEventListener("click", onInspect);
    if (els.cookie) els.cookie.addEventListener("click", onCookie);
    if (els.cookieHome) els.cookieHome.addEventListener("click", onCookie);
    if (els.btnAccount) {
      els.btnAccount.addEventListener("click", function (e) {
        e.preventDefault();
        e.stopPropagation();
        openAccountMenu();
      });
    }
    document.addEventListener("click", function (e) {
      if (!els.accountWrap || !els.accountMenu || els.accountMenu.hidden) return;
      if (els.accountWrap.contains(e.target)) return;
      closeAccountMenu();
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") closeAccountMenu();
      if (e.key !== " " && e.code !== "Space") return;
      if (!isPlayOverlayOpen()) return;
      if (e.repeat) return;
      if (isTypingTarget(document.activeElement)) return;
      e.preventDefault();
      togglePlayOverlayIcon();
    });
    if (els.outputBtn) els.outputBtn.addEventListener("click", onOutputBrowse);
    if (els.download) els.download.addEventListener("click", onDownload);
    if (els.downloadAdv) els.downloadAdv.addEventListener("click", onDownload);
    if (els.btnPlaySelected) {
      els.btnPlaySelected.addEventListener("click", onPlaySelected);
    }
    if (els.btnPlayBack) {
      els.btnPlayBack.addEventListener("click", closePlayOverlay);
    }
    if (els.btnPlayToggle) {
      els.btnPlayToggle.addEventListener("click", function () {
        togglePlayOverlayIcon();
      });
    }
    if (els.playPage) {
      els.playPage.addEventListener("mousemove", wakePlayChrome);
    }
    window.addEventListener("resize", function () {
      if (!isPlayOverlayOpen()) return;
      layoutPlayStage();
    });
    if (els.selectAll) {
      els.selectAll.addEventListener("click", onSelectAllEpisodes);
    }
    if (els.bannerClose) {
      els.bannerClose.addEventListener("click", clearBanner);
    }
    if (els.url) {
      els.url.addEventListener("change", schedulePersist);
      els.url.addEventListener("keydown", function (e) {
        if (e.key === "Enter") {
          e.preventDefault();
          onInspect();
        }
      });
    }
    if (els.output) {
      els.output.addEventListener("change", function () {
        state.outputDir = els.output.value.trim() || "./Downloads";
        schedulePersist();
      });
    }

    wirePathField(els.wvdPath, els.btnWvd, "Path to Widevine .wvd device file:", function (p) {
      state.wvdPath = p;
    });
    wirePathField(
      els.clientIdPath,
      els.btnClientId,
      "Path to client_id.bin:",
      function (p) {
        state.clientIdPath = p;
      }
    );
    wirePathField(
      els.privateKeyPath,
      els.btnPrivateKey,
      "Path to private_key.pem:",
      function (p) {
        state.privateKeyPath = p;
      }
    );

    if (els.batchUrls) {
      els.batchUrls.addEventListener("change", function () {
        state.batchUrls = els.batchUrls.value;
        schedulePersist();
      });
      els.batchUrls.addEventListener("input", function () {
        state.batchUrls = els.batchUrls.value;
      });
    }
    if (els.batchFile) {
      els.batchFile.addEventListener("click", onLoadBatchFile);
    }

    wireSwitch(els.swProbeEvery, function (on) {
      state.probeEveryEpisode = on;
    });
    wireSwitch(els.swDebugManifest, function (on) {
      state.debugManifest = on;
    });
    wireSwitch(els.swStrictLangs, function (on) {
      state.strictLanguages = on;
    });

    wireNumber(els.num4294Retries, function () {});
    wireNumber(els.num4294Backoff, function () {});
    wireNumber(els.numCircuitLimit, function () {});
    wireNumber(els.numIndexWindow, function () {});

    if (els.btnBuildCatalog) {
      els.btnBuildCatalog.addEventListener("click", function () {
        onBuildIndex(false);
      });
    }
    if (els.btnIndexSubs) {
      els.btnIndexSubs.addEventListener("click", function () {
        onBuildIndex(true);
      });
    }

    document.querySelectorAll(".cr-dd").forEach(wireDropdown);
    document.addEventListener("click", function () {
      document.querySelectorAll(".cr-dd.is-open").forEach(closeDropdown);
    });
  }

  async function init() {
    cacheEls();
    wireUI();
    // Activity card: default expanded in shell; remember preference.
    var collapsed = false;
    try {
      var saved = localStorage.getItem("crdl.activityCollapsed");
      if (saved === "0") collapsed = false;
      if (saved === "1") collapsed = true;
    } catch (e) {
      /* ignore */
    }
    setActivityCollapsed(collapsed);
    setActivityTab("activity");
    updateDockSummary();
    syncAdvancedInputsFromState();
    setMode("normal");
    setShellPage("home");
    subscribeProgress();
    await loadPreferences();
    updateAccountChrome();
    // After prefs (cookie path), load Discover if still on Home.
    if (state.view === "home" && state.cookieFile && !state.homeLoaded) {
      loadHomeFeed(false);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
