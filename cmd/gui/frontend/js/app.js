/* Normal mode UI + Inspect wiring (Task 6). Download job lands in Task 7. */
(function () {
  "use strict";

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
    videoQuality: "max",
    audioQuality: "max",
    inspecting: false,
    lastSeason: 0,
  };

  var prefsTimer = null;
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
      output: $("output-dir"),
      outputBtn: $("btn-output"),
      seasons: $("season-chips"),
      episodes: $("episode-list"),
      audio: $("audio-list"),
      subs: $("sub-list"),
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

  function setBusy(busy) {
    state.inspecting = !!busy;
    if (els.mainPane) {
      els.mainPane.classList.toggle("is-busy", !!busy);
    }
    if (els.inspect) els.inspect.disabled = !!busy;
    if (els.progressBar) {
      els.progressBar.classList.toggle("is-indet", !!busy);
    }
    if (els.progressLabel) {
      els.progressLabel.textContent = busy ? "Inspect" : "Progress";
    }
    if (els.progressValue) {
      els.progressValue.textContent = busy ? "working…" : "—";
    }
    if (!busy && els.progressFill) {
      els.progressFill.style.width = "0%";
    }
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
    var audioLangs = Object.keys(state.selectedAudio).filter(function (k) {
      return state.selectedAudio[k];
    });
    var subtitleLangs = [];
    if (!state.noSubtitles) {
      subtitleLangs = Object.keys(state.selectedSubs).filter(function (k) {
        return state.selectedSubs[k];
      });
    }
    return {
      URL: (els.url && els.url.value.trim()) || state.url || "",
      CookieFile: state.cookieFile || "",
      OutputDir: (els.output && els.output.value.trim()) || state.outputDir || "./Downloads",
      Mode: state.mode || "normal",
      AudioLangs: audioLangs,
      SubtitleLangs: subtitleLangs,
      CaptionLangs: [],
      VideoQuality: state.videoQuality || "max",
      AudioQuality: state.audioQuality || "max",
      LastSeason: state.selectedSeason || state.lastSeason || 0,
      WVDPath: "",
      ClientIDPath: "",
      PrivateKeyPath: "",
      StrictLanguages: false,
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
    if (mode === "advanced" || mode === "normal") {
      setMode(mode);
    }
  }

  async function loadPreferences() {
    var app = goApp();
    if (!app || typeof app.GetPreferences !== "function") {
      logLine("Preferences: runtime not ready (browser preview)", "warn");
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
    }
  }

  async function persistPrefs() {
    var app = goApp();
    if (!app || typeof app.SavePreferences !== "function") return;
    var p = collectPreferences();
    // Preserve advanced fields already in memory via a merge-friendly full write.
    try {
      var current = await app.GetPreferences();
      if (current && typeof current === "object") {
        p.WVDPath = current.WVDPath || current.wvdPath || "";
        p.ClientIDPath = current.ClientIDPath || current.clientIdPath || "";
        p.PrivateKeyPath = current.PrivateKeyPath || current.privateKeyPath || "";
        p.StrictLanguages = !!(current.StrictLanguages != null
          ? current.StrictLanguages
          : current.strictLanguages);
        p.CaptionLangs =
          current.CaptionLangs || current.captionLangs || [];
        if (current.Playback4294Retries != null) {
          p.Playback4294Retries = current.Playback4294Retries;
        }
        if (current.Playback4294BackoffSec != null) {
          p.Playback4294BackoffSec = current.Playback4294BackoffSec;
        }
        if (current.IndexWindow != null) p.IndexWindow = current.IndexWindow;
        if (current.IndexCircuitLimit != null) {
          p.IndexCircuitLimit = current.IndexCircuitLimit;
        }
        if (current.DebugManifest != null) p.DebugManifest = current.DebugManifest;
        if (current.ProbeEveryEpisode != null) {
          p.ProbeEveryEpisode = current.ProbeEveryEpisode;
        }
      }
    } catch (e) {
      /* first-run ok */
    }
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
    if (err.message) return err.message;
    try {
      return String(err);
    } catch (e) {
      return "unknown error";
    }
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

    var req = {
      URL: url,
      ETPRTFile: state.cookieFile,
      PrimaryAudioHint: "",
      PrimarySubsHint: "",
      ProbePlayback: true,
      ProbeContentID: "",
    };

    try {
      var result = await app.Inspect(req);
      applyDefaults(result);
      await persistPrefs();
      var epCount = (result.Episodes || []).length;
      var seasonCount = (result.Seasons || []).length;
      logLine(
        "Inspect complete · " +
          epCount +
          " episode(s)" +
          (seasonCount ? ", " + seasonCount + " season(s)" : "") +
          (result.OriginalAudio ? " · original " + result.OriginalAudio : ""),
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

  function onCookie() {
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

  function onOutputBrowse() {
    var current = (els.output && els.output.value) || state.outputDir || "./Downloads";
    var path = window.prompt("Output folder path:", current);
    if (path == null) return;
    path = path.trim();
    if (!path) path = "./Downloads";
    state.outputDir = path;
    if (els.output) els.output.value = path;
    schedulePersist();
  }

  function onDownload() {
    clearBanner();
    var epIds = Object.keys(state.selectedEpisodeIds).filter(function (id) {
      return state.selectedEpisodeIds[id];
    });
    if (!epIds.length) {
      showBanner("Select at least one episode before downloading.", "warn");
      logLine("Download blocked: no episodes selected", "warn");
      return;
    }
    var audio = Object.keys(state.selectedAudio).filter(function (k) {
      return state.selectedAudio[k];
    });
    if (!audio.length) {
      showBanner("Select at least one audio language.", "warn");
      logLine("Download blocked: no audio selected", "warn");
      return;
    }
    if (!state.cookieFile) {
      showBanner("Cookie file path required.", "err");
      logLine("Download blocked: no cookie path", "err");
      return;
    }
    // Task 7 will call StartDownload / job runner.
    logLine(
      "Download selected (" +
        epIds.length +
        " ep, audio " +
        audio.join(",") +
        ") — job runner lands in Task 7",
      "info"
    );
    if (els.queue) {
      els.queue.textContent =
        epIds.length + " episode(s) ready · start in Task 7";
    }
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

    document.querySelectorAll(".cr-dd").forEach(wireDropdown);
    document.addEventListener("click", function () {
      document.querySelectorAll(".cr-dd.is-open").forEach(closeDropdown);
    });
  }

  async function init() {
    cacheEls();
    wireUI();
    setMode("normal");
    await loadPreferences();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
