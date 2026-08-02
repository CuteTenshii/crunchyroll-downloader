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
    cookieFile: "",
    outputDir: "./Downloads",
    url: "",
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
  };

  var prefsTimer = null;
  var progressUnsub = null;
  var els = {};

  function $(id) {
    return document.getElementById(id);
  }

  function cacheEls() {
    els = {
      url: $("url"),
      cookie: $("btn-cookie"),
      inspect: $("btn-inspect"),
      download: $("btn-download"),
      downloadAdv: $("btn-download-adv"),
      output: $("output-dir"),
      outputBtn: $("btn-output"),
      seasons: $("season-chips"),
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
      banner: $("banner"),
      bannerText: $("banner-text"),
      bannerClose: $("banner-close"),
      mainPane: $("main-pane"),
      viewNormal: $("view-normal"),
      viewAdvanced: $("view-advanced"),
      modeNormal: $("mode-normal"),
      modeAdvanced: $("mode-advanced"),
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

  function logLine(text, cls) {
    if (!els.activity) return;
    var div = document.createElement("div");
    if (cls) div.className = cls;
    div.textContent = text;
    els.activity.appendChild(div);
    els.activity.scrollTop = els.activity.scrollHeight;
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
      if (els.progressBar) els.progressBar.classList.add("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Inspect";
      if (els.progressValue) els.progressValue.textContent = "working…";
    } else if (!state.downloading && !state.indexing) {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Progress";
      if (els.progressValue) els.progressValue.textContent = "—";
      if (els.progressFill) els.progressFill.style.width = "0%";
    }
  }

  function setDownloading(on) {
    state.downloading = !!on;
    refreshBusyChrome();
    forEachDownloadBtn(function (btn) {
      if (on) {
        btn.textContent = "Cancel";
        btn.classList.add("is-cancel");
        btn.disabled = false;
      } else {
        btn.textContent = "Download selected";
        btn.classList.remove("is-cancel");
        btn.disabled = false;
      }
    });
    if (on) {
      if (els.progressBar) els.progressBar.classList.add("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Download";
      if (els.progressValue) els.progressValue.textContent = "starting…";
    } else {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Progress";
    }
  }

  function setIndexing(on, label) {
    state.indexing = !!on;
    refreshBusyChrome();
    if (on) {
      if (els.progressBar) els.progressBar.classList.add("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = label || "Index";
      if (els.progressValue) els.progressValue.textContent = "working…";
    } else if (!state.downloading && !state.inspecting) {
      if (els.progressBar) els.progressBar.classList.remove("is-indet");
      if (els.progressLabel) els.progressLabel.textContent = "Progress";
      if (els.progressValue) els.progressValue.textContent = "—";
      if (els.progressFill) els.progressFill.style.width = "0%";
    }
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
    if (cookie) state.cookieFile = cookie;
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

  async function loadPreferences() {
    var app = goApp();
    if (!app || typeof app.GetPreferences !== "function") {
      logLine("Preferences: runtime not ready (browser preview)", "warn");
      syncAdvancedInputsFromState();
      return;
    }
    try {
      var prefs = await app.GetPreferences();
      applyPreferences(prefs);
      if (state.cookieFile) {
        logLine("Loaded preferences · cookie path set", "ok");
      } else {
        logLine("Loaded preferences", "ok");
      }
    } catch (err) {
      logLine("Failed to load preferences: " + errMessage(err), "err");
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
      out.push({ ID: "s" + n, SeasonNumber: n });
    });
    if (!out.length && result && result.ContentType === "watch") {
      out.push({ ID: "watch", SeasonNumber: 0 });
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
    btn.className = "cr-check" + (opts.on ? " is-on" : "");
    if (opts.dataset) {
      Object.keys(opts.dataset).forEach(function (k) {
        btn.dataset[k] = opts.dataset[k];
      });
    }
    btn.innerHTML =
      '<span class="cr-box" aria-hidden="true">' +
      TICK_SVG +
      "</span>" +
      '<span class="cr-check-label">' +
      opts.labelHtml +
      "</span>";
    if (opts.onClick) btn.addEventListener("click", opts.onClick);
    return btn;
  }

  function renderCatalog(result) {
    state.catalog = result || null;
    renderSeasons();
    renderEpisodes();
    renderAudio();
    renderSubs();
    renderCaptions();
    renderQualities();
  }

  function renderSeasons() {
    var root = els.seasons;
    if (!root) return;
    root.innerHTML = "";
    var seasons = seasonsFromCatalog(state.catalog);
    if (!seasons.length) {
      root.innerHTML = '<span class="muted-hint">Inspect a series to load seasons</span>';
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
        renderEpisodes();
        schedulePersist();
      });
      root.appendChild(chip);
    });
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
      var row = renderCheckbox({
        on: on,
        dataset: { ep: ep.ID },
        labelHtml: label,
        onClick: function () {
          var next = !row.classList.contains("is-on");
          row.classList.toggle("is-on", next);
          if (next) state.selectedEpisodeIds[ep.ID] = true;
          else delete state.selectedEpisodeIds[ep.ID];
        },
      });
      root.appendChild(row);
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
      return;
    }
    var original = (state.catalog && state.catalog.OriginalAudio) || "";
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
          row.classList.toggle("is-on", next);
          if (next) state.selectedAudio[code] = true;
          else delete state.selectedAudio[code];
          schedulePersist();
        },
      });
      root.appendChild(row);
    });
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
        root.querySelectorAll(".cr-check").forEach(function (r) {
          r.classList.remove("is-on");
        });
        noneRow.classList.add("is-on");
        schedulePersist();
      },
    });
    root.appendChild(noneRow);

    var locales = (state.catalog && state.catalog.SubtitleLocales) || [];
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
            noneRow.classList.remove("is-on");
            row.classList.add("is-on");
          } else {
            delete state.selectedSubs[code];
            row.classList.remove("is-on");
            var any = Object.keys(state.selectedSubs).some(function (k) {
              return state.selectedSubs[k];
            });
            if (!any) {
              state.noSubtitles = true;
              noneRow.classList.add("is-on");
            }
          }
          schedulePersist();
        },
      });
      root.appendChild(row);
    });
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
        root.querySelectorAll(".cr-check").forEach(function (r) {
          r.classList.remove("is-on");
        });
        noneRow.classList.add("is-on");
        schedulePersist();
      },
    });
    root.appendChild(noneRow);

    var locales = (state.catalog && state.catalog.CaptionLocales) || [];
    // Also surface previously selected caption codes even if inspect had none.
    var extra = Object.keys(state.selectedCaptions || {});
    extra.forEach(function (code) {
      if (locales.indexOf(code) < 0) locales = locales.concat([code]);
    });

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
            noneRow.classList.remove("is-on");
            row.classList.add("is-on");
          } else {
            delete state.selectedCaptions[code];
            row.classList.remove("is-on");
            var any = Object.keys(state.selectedCaptions).some(function (k) {
              return state.selectedCaptions[k];
            });
            if (!any) {
              state.noCaptions = true;
              noneRow.classList.add("is-on");
            }
          }
          state.captionLangs = selectedCaptionLangs();
          schedulePersist();
        },
      });
      root.appendChild(row);
    });
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

    // Episodes: only S1E1 if present; watch-only → that single episode; else DefaultEpisodeID.
    state.selectedEpisodeIds = {};
    var eps = result.Episodes || [];
    if (result.ContentType === "watch" && eps.length === 1) {
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
        logLine("Cookie path set", "ok");
        schedulePersist();
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
      logLine("Cookie path set", "ok");
    } else {
      logLine("Cookie path cleared", "warn");
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
    if (els.inspect) els.inspect.addEventListener("click", onInspect);
    if (els.cookie) els.cookie.addEventListener("click", onCookie);
    if (els.outputBtn) els.outputBtn.addEventListener("click", onOutputBrowse);
    if (els.download) els.download.addEventListener("click", onDownload);
    if (els.downloadAdv) els.downloadAdv.addEventListener("click", onDownload);
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
    syncAdvancedInputsFromState();
    setMode("normal");
    subscribeProgress();
    await loadPreferences();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
