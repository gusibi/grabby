(function () {
  "use strict";

  var params = new URLSearchParams(location.search);
  var platform = params.get("platform") === "windows" ? "win" : "mac";
  var desktopMode = params.get("desktop") === "1";
  document.documentElement.dataset.platform = platform;

  var TITLES = {
    today: "今天", library: "全部内容", sources: "RSS 源", news: "新闻源与采集",
    ai: "AI 分析", daily: "AI 日报", devices: "浏览器设备", apihub: "API 接入",
    settings: "设置"
  };

  function pad(n) { return n < 10 ? "0" + n : String(n); }
  function dateStr(d) { return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()); }

  var initialView = params.get("view");
  function storedLayout() {
    try {
      var stored = localStorage.getItem("grabby_layout");
      if (stored === "list" || stored === "grid") return stored;
    } catch (e) { /* storage may be unavailable */ }
    return params.get("layout") === "list" ? "list" : "grid";
  }
  var state = {
    view: initialView && TITLES[initialView] ? initialView : "today",
    layout: storedLayout(),
    query: "",
    filter: "all",
    category: "",
    sourceCategory: "",
    aiCategory: "",
    health: null, stats: null, items: [], browsers: [], itemsCursor: "",
    sources: [], sourceCategories: [], aiCategories: [], logs: [],
    aiSettings: null, aiStats: null, aiItems: [], aiCatalog: null, aiUsage: null,
    aiTab: "items",
    discoveredModels: [],
    pendingImport: null,
    daily: null, dailyList: [], dailyDate: dateStr(new Date()), dailyType: "daily",
    adapter: "twitter-search", adapterResult: null, adapterBusy: false,
    captures: [],
    desktopSettings: null,
    tokenEnabled: params.get("auth") === "1",
    external: params.get("external") === "1",
    error: "",
    busy: false,
    reader: { open: false, id: null, item: null, analysis: null, loading: false, error: "" },
    readerLastFocus: null,
    readerSeq: 0
  };

  var $ = function (id) { return document.getElementById(id); };
  var esc = function (value) {
    return String(value == null ? "" : value)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  };
  var attr = esc;

  async function api(path, options) {
    var res = await fetch(path, Object.assign({ credentials: "same-origin" }, options || {}));
    var text = await res.text();
    var data;
    try { data = JSON.parse(text); } catch (e) { data = { success: false, error: text }; }
    if (res.status === 401 || (data && (data.error === "admin login required" || data.error === "missing or invalid API token"))) {
      // The desktop app's own UI is always trusted on loopback, so it never
      // shows the admin-key dialog. A token is only configured by the user in
      // the settings page, and never typed into a login prompt.
      if (!desktopMode) showLogin();
      throw new Error("unauthorized");
    }
    if (!res.ok && data && !data.error) data.error = data.detail || ("HTTP " + res.status);
    return data;
  }
  function jsonReq(method, path, body) {
    return api(path, {
      method: method,
      headers: { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body)
    });
  }
  var post = function (p, b) { return jsonReq("POST", p, b || {}); };
  var put = function (p, b) { return jsonReq("PUT", p, b || {}); };
  var del = function (p) { return jsonReq("DELETE", p); };

  function showLogin() { $("login-modal").classList.remove("hidden"); }
  function hideLogin() { $("login-modal").classList.add("hidden"); }

  $("login-submit").addEventListener("click", async function () {
    var key = $("login-key").value.trim();
    if (!key) return;
    try {
      await post("/api/auth/login", { key: key });
      $("login-error").textContent = "";
      hideLogin();
      await loadView(state.view);
    } catch (e) {
      $("login-error").textContent = "密钥不正确或服务未就绪";
    }
  });
  $("login-key").addEventListener("keydown", function (e) {
    if (e.key === "Enter") $("login-submit").click();
  });

  var toastTimer = null;
  function toast(message, isError) {
    var el = $("toast");
    el.textContent = message;
    el.className = "toast" + (isError ? " error" : "");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { el.className = "toast hidden"; }, 2800);
  }

  function openDialog(title, body, footer, wide) {
    $("dialog-card").className = "modal-card dialog-card" + (wide ? " wide" : "");
    $("dialog-card").innerHTML =
      '<div class="dialog-head"><h2>' + esc(title) + '</h2>' +
      '<button class="dialog-x" data-action="dialog-close" type="button" aria-label="关闭">×</button></div>' +
      '<div class="dialog-body">' + body + "</div>" +
      (footer ? '<div class="dialog-foot">' + footer + "</div>" : "");
    $("dialog-modal").classList.remove("hidden");
  }
  function closeDialog() { $("dialog-modal").classList.add("hidden"); }

  function hue(text) {
    var h = 0, s = String(text || "");
    for (var i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 360;
    return h;
  }
  function fmtDate(value) {
    if (!value) return "";
    var d = new Date(value);
    if (isNaN(d.getTime())) return String(value).slice(0, 16);
    return d.toLocaleDateString("zh-CN", { month: "short", day: "numeric" });
  }
  function fmtTime(value) {
    if (!value) return "从未";
    var d = new Date(value);
    if (isNaN(d.getTime())) return String(value);
    return d.toLocaleString("zh-CN", { month: "numeric", day: "numeric", hour: "2-digit", minute: "2-digit" });
  }
  function head(desc) {
    return desc ? '<div class="page-desc">' + esc(desc) + "</div>" : "";
  }
  function todayLabel() {
    return new Intl.DateTimeFormat("zh-CN", { month: "long", day: "numeric", weekday: "long" }).format(new Date());
  }
  function syncTopbarHead() {
    var el = $("topbar-head");
    if (!el) return;
    var meta = state.view === "today"
      ? '<span class="today-desc">' + esc(todayLabel()) + " · 你的浏览器采集概览</span>"
      : "";
    el.innerHTML = '<h1 class="page-title">' + esc(TITLES[state.view] || "Grabby") + "</h1>" + meta;
  }
  function stat(value, label) {
    return '<div class="stat"><div class="stat-value">' + esc(value) + '</div>' +
      '<div class="stat-label">' + esc(label) + "</div></div>";
  }
  function statGrid(items) { return '<div class="stat-grid">' + items.map(function (x) { return stat(x[0], x[1]); }).join("") + "</div>"; }
  function empty(title, desc) {
    return '<div class="empty"><div class="empty-title">' + esc(title) + "</div>" +
      (desc ? "<div>" + esc(desc) + "</div>" : "") + "</div>";
  }
  function badge(text, ok) { return '<span class="badge' + (ok ? " ok" : "") + '">' + esc(text) + "</span>"; }
  function asNames(value) {
    if (!value) return [];
    if (Array.isArray(value)) {
      return value.map(function (x) { return typeof x === "string" ? x : (x && x.name); })
        .filter(function (x) { return !!x; });
    }
    if (typeof value === "object") return Object.keys(value);
    return [];
  }

  // itemVisual derives the gradient monogram used by both layouts. The hue is
  // stable per source so the same publication keeps the same colour.
  function itemVisual(item) {
    var label = item.origin_source || item.source_category || item.category || "GRABBY";
    var h = hue(label);
    return {
      label: label,
      letter: (label || "G").trim().charAt(0).toUpperCase(),
      cover: "linear-gradient(135deg, hsl(" + h + ",58%,52%), hsl(" + ((h + 48) % 360) + ",62%,38%))",
      score: item.quality_score != null && item.quality_score !== "" ? Number(item.quality_score) : null
    };
  }

  function unreadDot(item) {
    return Number(item.read_status) === 0 ? '<span class="unread-dot" aria-label="未读"></span>' : "";
  }

  function card(item, lead) {
    var v = itemVisual(item);
    var read = Number(item.read_status) === 1;
    var active = String(state.reader.id) === String(item.id);
    return '<button type="button" class="card' + (lead ? " lead" : "") + (read ? " is-read" : "") +
      (active ? " is-active" : "") + '" data-action="detail" data-id="' + attr(item.id) + '"' +
      ' aria-label="阅读：' + attr(item.title || "未命名内容") + '">' +
      '<div class="card-cover" style="background:' + v.cover + '"><span class="cover-letter">' + esc(v.letter) + "</span>" +
      (v.score != null ? '<span class="score-chip">' + esc(v.score.toFixed(1)) + "</span>" : "") + "</div>" +
      '<div class="card-body">' +
        '<div class="card-kicker">' + unreadDot(item) + esc(v.label) + "</div>" +
        '<div class="card-title">' + esc(item.title || "未命名内容") + "</div>" +
        '<div class="card-summary">' + esc(item.ai_summary || item.summary || "") + "</div>" +
        '<div class="card-meta">' + esc(fmtDate(item.published_at || item.fetched_at)) +
        (item.category ? " · " + esc(item.category) : "") + "</div>" +
      "</div></button>";
  }

  // listRow is a first-class editorial row, not a data table: monogram cover,
  // kicker, title, summary, and a quiet date / score column.
  function listRow(item) {
    var v = itemVisual(item);
    var read = Number(item.read_status) === 1;
    var active = String(state.reader.id) === String(item.id);
    return '<button type="button" class="item-row' + (read ? " is-read" : "") + (active ? " is-active" : "") +
      '" data-action="detail" data-id="' + attr(item.id) + '"' +
      ' aria-label="阅读：' + attr(item.title || "未命名内容") + '">' +
      '<span class="item-cover" style="background:' + v.cover + '"><span class="cover-letter">' + esc(v.letter) + "</span></span>" +
      '<span class="item-main">' +
        '<span class="item-kicker">' + unreadDot(item) +
          '<span class="item-kicker-text">' + esc(v.label) +
            (item.category ? " · " + esc(item.category) : "") + "</span></span>" +
        '<span class="item-title">' + esc(item.title || "未命名内容") + "</span>" +
        (item.ai_summary || item.summary
          ? '<span class="item-summary">' + esc(item.ai_summary || item.summary) + "</span>"
          : "") +
      "</span>" +
      '<span class="item-side">' +
        '<span class="item-date">' + esc(fmtDate(item.published_at || item.fetched_at)) + "</span>" +
        (v.score != null ? '<span class="item-score">' + esc(v.score.toFixed(1)) + "</span>" : "") +
      "</span></button>";
  }

  function feed(items, leadFirst) {
    if (!items || !items.length) return empty("还没有内容", "添加 RSS 源或连接浏览器扩展后，抓取的内容会显示在这里。");
    if (state.layout === "list") {
      return '<div class="item-list">' + items.map(listRow).join("") + "</div>";
    }
    return '<div class="card-grid">' + items.map(function (it, i) {
      return card(it, leadFirst && i === 0);
    }).join("") + "</div>";
  }

  function itemsUrl(cursor) {
    var p = ["limit=60"];
    var base;
    if (state.filter === "quality") {
      base = "/open/api/ai/items?";
      p.push("score_min=7");
      if (state.aiCategory) p.push("category=" + encodeURIComponent(state.aiCategory));
      if (state.sourceCategory) p.push("source_category=" + encodeURIComponent(state.sourceCategory));
    } else {
      base = "/open/api/items?";
      if (state.filter === "starred") p.push("starred=1");
      if (state.query) p.push("q=" + encodeURIComponent(state.query));
      if (state.category) p.push("category=" + encodeURIComponent(state.category));
      if (state.sourceCategory) p.push("source_category=" + encodeURIComponent(state.sourceCategory));
    }
    if (cursor) p.push("cursor=" + encodeURIComponent(cursor));
    return base + p.join("&");
  }

  async function fetchAll(map) {
    var keys = Object.keys(map);
    var results = await Promise.allSettled(keys.map(function (k) { return map[k]; }));
    var out = {};
    results.forEach(function (r, i) { out[keys[i]] = r.status === "fulfilled" ? r.value : null; });
    return out;
  }

  var loadSeq = 0;
  async function loadView(view) {
    var seq = ++loadSeq;
    var d;
    if (view === "today" || view === "library") {
      d = await fetchAll({
        health: api("/open/api/health"), stats: api("/open/api/stats"),
        items: api(itemsUrl()), browsers: api("/api/browsers"),
        aiCategories: api("/open/api/ai/categories"),
        // Only the newest two reports are needed to preview today's latest one,
        // and this keeps the 今天 page from downloading the whole archive.
        dailyList: api("/open/api/ai/daily/list?limit=2")
      });
    } else if (view === "sources" || view === "news") {
      d = await fetchAll({
        health: api("/open/api/health"), sources: api("/api/sources"), stats: api("/open/api/stats"),
        captures: api("/api/captures/extract?limit=8")
      });
    } else if (view === "ai") {
      d = await fetchAll({
        health: api("/open/api/health"),
        aiSettings: api("/api/ai/settings"), aiStats: api("/open/api/ai/stats"),
        aiCategories: api("/open/api/ai/categories"),
        aiUsage: api("/api/ai/usage?days=7"),
        aiItems: api("/open/api/ai/items?limit=24&score_min=7" +
          (state.aiCategory ? "&category=" + encodeURIComponent(state.aiCategory) : ""))
      });
    } else if (view === "daily") {
      d = await fetchAll({
        health: api("/open/api/health"),
        daily: api("/open/api/ai/daily?date=" + encodeURIComponent(state.dailyDate) + "&type=" + encodeURIComponent(state.dailyType)),
        dailyList: api("/open/api/ai/daily/list?limit=30")
      });
    } else if (view === "devices") {
      d = await fetchAll({ browsers: api("/api/browsers"), health: api("/open/api/health") });
    } else if (view === "settings") {
      d = await fetchAll({
        desktopSettings: api("/api/desktop/settings"), health: api("/open/api/health"),
        aiSettings: api("/api/ai/settings"), aiStats: api("/open/api/ai/stats"),
        aiUsage: api("/api/ai/usage?days=7")
      });
    } else {
      d = await fetchAll({ health: api("/open/api/health") });
    }
    if (seq !== loadSeq) return;

    if (d.health) { state.health = d.health; state.error = ""; }
    else if (view === "today" || view === "library" || view === "devices" || view === "apihub") {
      state.error = "本地服务尚未响应";
    }
    if (d.stats) { state.stats = d.stats; state.sourceCategories = asNames(d.stats.source_categories); }
    if (d.items) {
      var list = d.items.items || [];
      if (state.filter === "quality" && state.query) {
        var q = state.query.toLowerCase();
        list = list.filter(function (it) {
          return ((it.title || "") + " " + (it.ai_summary || it.summary || "")).toLowerCase().indexOf(q) >= 0;
        });
      }
      state.items = list;
      state.itemsCursor = d.items.cursor || d.items.next_cursor || "";
    }
    if (d.browsers) state.browsers = d.browsers.browsers || [];
    if (d.aiCategories) state.aiCategories = d.aiCategories.categories || [];
    if (d.sources) state.sources = Array.isArray(d.sources) ? d.sources : (d.sources.sources || []);
    if (d.captures) state.captures = d.captures.items || [];
    if (d.aiSettings) { state.aiSettings = d.aiSettings.settings || null; state.activeProfileID = null; }
    if (d.aiCatalog && d.aiCatalog.providers) state.aiCatalog = d.aiCatalog;
    if (d.aiUsage && d.aiUsage.summary) state.aiUsage = d.aiUsage.summary;
    if (d.aiStats) state.aiStats = d.aiStats;
    if (d.aiItems) state.aiItems = d.aiItems.items || [];
    if (d.dailyList) state.dailyList = Array.isArray(d.dailyList) ? d.dailyList : (d.dailyList.reports || d.dailyList.items || []);
    if (view === "daily") state.daily = d.daily || null;
    if (d.desktopSettings) {
      state.desktopSettings = d.desktopSettings;
      state.tokenEnabled = !!d.desktopSettings.token_enabled;
      state.external = !!d.desktopSettings.active_external;
    }

    syncStatus();
    render();

    // The catalog can be several MB on first run. Load it after the page is
    // usable so a slow/unreachable models.dev never blocks the settings page.
    if (view === "settings" && !state.aiCatalog) {
      api("/api/ai/catalog").then(function (cr) {
        if (seq !== loadSeq || state.view !== "settings") return;
        if (cr && cr.providers && cr.providers.length) {
          var active = document.activeElement;
          if (active && active.closest && active.closest('form[data-form="ai-settings"]')) {
            state.aiCatalog = cr;
            return;
          }
          state.aiCatalog = cr;
          render();
        }
      }).catch(function () { /* built-in presets remain usable */ });
    }
  }

  async function reloadItems() {
    var d = await fetchAll({ items: api(itemsUrl("")) });
    if (d.items) {
      var list = d.items.items || [];
      if (state.filter === "quality" && state.query) {
        var q = state.query.toLowerCase();
        list = list.filter(function (it) {
          return ((it.title || "") + " " + (it.ai_summary || it.summary || "")).toLowerCase().indexOf(q) >= 0;
        });
      }
      state.items = list;
      state.itemsCursor = d.items.cursor || d.items.next_cursor || "";
      render();
    }
  }

  async function loadMoreItems() {
    if (!state.itemsCursor) return;
    var scroll = $("content").scrollTop;
    var d = await api(itemsUrl(state.itemsCursor));
    var more = d.items || [];
    if (state.filter === "quality" && state.query) {
      var q = state.query.toLowerCase();
      more = more.filter(function (it) {
        return ((it.title || "") + " " + (it.ai_summary || it.summary || "")).toLowerCase().indexOf(q) >= 0;
      });
    }
    state.items = state.items.concat(more);
    state.itemsCursor = d.cursor || d.next_cursor || "";
    render();
    $("content").scrollTop = scroll;
  }

  /* ---------------- Views ---------------- */

  function renderToday() {
    var s = state.stats || {}, h = state.health || {};
    var connected = !!h.browser_connected;
    var report = latestReportForDate(state.dailyList, dateStr(new Date()));

    var html = "";

    // One compact strip instead of four oversized cards.
    html += '<div class="today-stats">' + [
      [s.total_count, "已归档"],
      [s.unread_count, "未读"],
      [s.starred_count, "已收藏"],
      [h.browser_count == null ? "—" : h.browser_count, "已连接浏览器"]
    ].map(function (x) {
      return '<div class="today-stat"><div class="today-stat-value">' + esc(x[0] == null ? "—" : x[0]) +
        '</div><div class="today-stat-label">' + esc(x[1]) + "</div></div>";
    }).join("") + "</div>";

    html += '<div class="today-bar">' +
      '<div class="today-conn">' +
        '<span class="status-dot' + (connected ? " ok" : "") + '"></span>' +
        '<span class="today-conn-text">' + (connected ? "扩展已连接" : "扩展未连接") + "</span>" +
        '<span class="today-conn-addr mono">ws://localhost:' + esc(location.port || "5040") + "/ws_browser</span>" +
        badge(connected ? "在线" : "离线", connected) +
      "</div>" +
      '<div class="today-links">' +
        '<button class="mini" data-action="goto" data-view="sources" type="button">订阅源 ' +
          esc(state.sources.length || asNames(s.source_categories).length) + "</button>" +
        '<button class="mini" data-action="goto" data-view="news" type="button">新闻源</button>' +
        '<button class="mini" data-action="goto" data-view="daily" type="button">AI 日报</button>' +
      "</div></div>";

    if (report) html += todayReportCard(report);

    html += '<h2 class="section-title">最新内容</h2>' + feed(state.items.slice(0, 12), true);
    return html;
  }

  // Pull the first readable paragraph out of a stored daily report so the
  // 今天 page can preview it without rendering the whole document.
  function reportPreviewText(r) {
    try {
      var doc = JSON.parse(r.content || "{}");
      var sections = dailySections(doc);
      if (sections) {
        var keys = Object.keys(sections);
        for (var i = 0; i < keys.length; i++) {
          var sec = sections[keys[i]];
          var items = sec && Array.isArray(sec.items) ? sec.items : [];
          for (var j = 0; j < items.length; j++) {
            var text = items[j] && (items[j].summary || items[j].comment);
            if (text) return String(text);
          }
        }
      }
      if (doc.summary) return String(doc.summary);
    } catch (e) { /* markdown reports have no sections */ }
    return "";
  }

  function todayReportCard(r) {
    var type = r.report_type || "daily";
    var date = r.report_date || r.date || "";
    var preview = reportPreviewText(r);
    return '<div class="today-report">' +
      '<div class="today-report-head">' +
        '<div class="today-report-heading"><span class="eyebrow">今日' + esc(reportTypeLabel(type)) + "</span>" +
          '<div class="today-report-title">' + esc(r.title || (date + " · " + reportTypeLabel(type))) + "</div></div>" +
        '<button class="mini" data-action="daily-open" data-date="' + attr(date) + '" data-type="' + attr(type) + '" type="button">查看日报</button>' +
      "</div>" +
      (preview ? '<div class="today-report-preview">' + esc(preview) + "</div>" : "") +
      '<div class="today-report-meta">生成于 ' + esc(fmtTime(r.generated_at)) +
        (r.total_items ? " · 共 " + esc(r.total_items) + " 条 / " + esc(r.quality_items || 0) + " 条优质" : "") + "</div></div>";
  }

  function chips(items, current, action, key) {
    if (!items || !items.length) return "";
    return '<div class="chips">' + items.map(function (x) {
      var value = typeof x === "string" ? x : x.name;
      var label = typeof x === "string" ? x : (x.name + (x.count != null ? " · " + x.count : ""));
      return '<button class="chip' + (current === value ? " is-active" : "") + '" data-action="' + action +
        '" data-' + key + '="' + attr(value) + '" type="button">' + esc(label) + "</button>";
    }).join("") + "</div>";
  }

  function renderLibrary() {
    var s = state.stats || {};
    var filtered = state.filter !== "all" || !!state.query || !!state.category || !!state.sourceCategory;
    // state.items only holds the current page, so fall back to the library
    // totals for the unfiltered view instead of showing "共 60 条".
    var total = !filtered && s.total_count != null ? s.total_count : state.items.length;
    var desc = filtered
      ? "当前显示 " + state.items.length + " 条 · 来自 RSS、新闻源与浏览器采集"
      : "共 " + total + " 条 · 来自 RSS、新闻源与浏览器采集";
    var html = head(desc);
    html += '<div class="toolbar">' +
      '<div class="segmented">' +
        '<button class="seg' + (state.filter === "all" ? " is-active" : "") + '" data-action="filter" data-value="all" type="button">全部</button>' +
        '<button class="seg' + (state.filter === "starred" ? " is-active" : "") + '" data-action="filter" data-value="starred" type="button">已收藏</button>' +
        '<button class="seg' + (state.filter === "quality" ? " is-active" : "") + '" data-action="filter" data-value="quality" type="button">AI 优质</button>' +
      "</div>" +
      "</div>";

    // Source groups (体育新闻 / 财经新闻 …) are the filter people reach for
    // most, so keep them one click away instead of hiding them behind the nav.
    var srcCats = (s.source_categories || []).slice();
    if (srcCats.length) {
      var srcUnread = s.source_category_unread || {};
      html += '<div class="filter-row"><span class="filter-label">来源</span><div class="chips">' +
        '<button class="chip' + (!state.sourceCategory ? " is-active" : "") + '" data-action="source-all" type="button">全部</button>' +
        srcCats.map(function (name) {
          return '<button class="chip' + (state.sourceCategory === name ? " is-active" : "") +
            '" data-action="source-category" data-value="' + attr(name) + '" type="button">' + esc(name) +
            (srcUnread[name] != null ? ' <span class="chip-count">' + esc(srcUnread[name]) + "</span>" : "") + "</button>";
        }).join("") + "</div></div>";
    }

    var quality = state.filter === "quality";
    var cats = asNames(quality ? state.aiCategories : (state.stats && state.stats.categories));
    if (cats.length) {
      html += '<div class="filter-row"><span class="filter-label">' + (quality ? "AI 分类" : "类型") + "</span>" +
        chips(cats, quality ? state.aiCategory : state.category, quality ? "quality-category" : "category", "value") + "</div>";
    }
    html += '<div style="height:16px"></div>' + feed(state.items, false);
    if (state.itemsCursor) {
      html += '<div class="load-more"><button class="btn" data-action="load-more" type="button">加载更多</button></div>';
    }
    return html;
  }

  function sourceRow(s) {
    var status = s.last_fetch_status || "";
    var ok = status === "success" || status === "partial";
    return '<div class="list-row">' +
      '<div class="list-main">' +
        '<div class="list-title">' + esc(s.name || s.id) + " " + badge(s.type || "rss", s.enabled == 1) + "</div>" +
        '<div class="list-sub mono">' + esc(s.url || "") + "</div>" +
        '<div class="list-meta">分组 ' + esc(s.category || "未分组") + " · 计划 " + esc(s.schedule || "—") +
        " · 上次抓取 " + esc(fmtTime(s.last_fetch_at)) + (status ? " · " + esc(status) : "") + "</div>" +
      "</div>" +
      '<div class="list-actions">' +
        '<button class="mini" data-action="toggle-source" data-id="' + attr(s.id) + '" data-enabled="' + attr(s.enabled) + '" type="button">' + (s.enabled == 1 ? "停用" : "启用") + "</button>" +
        '<button class="mini" data-action="run-source" data-id="' + attr(s.id) + '" type="button">立即抓取</button>' +
        '<button class="mini" data-action="logs-source" data-id="' + attr(s.id) + '" type="button">日志</button>' +
        '<button class="mini" data-action="edit-source" data-id="' + attr(s.id) + '" type="button">编辑</button>' +
        '<button class="mini danger" data-action="delete-source" data-id="' + attr(s.id) + '" type="button">删除</button>' +
      "</div></div>";
  }

  function renderSources() {
    var list = state.sources || [];
    var enabled = list.filter(function (s) { return s.enabled == 1; }).length;
    var groups = {};
    list.forEach(function (s) { var g = s.category || "未分组"; groups[g] = (groups[g] || 0) + 1; });
    var html = head("管理 RSS / API / 网页抓取源，抓取结果会自动进入内容库并触发 AI 分析");
    html += statGrid([
      [list.length, "全部订阅源"], [enabled, "已启用"], [list.length - enabled, "已停用"], [Object.keys(groups).length, "分组"]
    ]);
    html += '<div class="toolbar" style="margin-top:18px">' +
      '<button class="btn primary" data-action="add-source" type="button">添加数据源</button>' +
      '<button class="btn" data-action="run-all-sources" type="button">全部抓取</button>' +
      '<span class="toolbar-note">添加后立即生效，定时任务由 APP 内的调度器管理</span></div>';
    html += '<div class="panel list-panel">';
    html += list.length ? list.map(sourceRow).join("") : empty("还没有订阅源", "点击「添加数据源」新建一个 RSS 或 API 源。");
    html += "</div>";
    html += '<div class="panel"><h3>分组</h3>' +
      (Object.keys(groups).length
        ? Object.keys(groups).map(function (g) {
            return '<div class="row"><span class="row-value">' + esc(g) + "</span>" + badge(groups[g] + " 个源", false) + "</div>";
          }).join("")
        : '<p>暂无分组</p>') + "</div>";
    return html;
  }

  var ADAPTERS = [
    { id: "twitter-search", label: "X / Twitter 搜索", hint: "关键词", path: "/api/twitter/search", field: "query", extra: { limit: 20 } },
    { id: "twitter-timeline", label: "X / Twitter 时间线", hint: "留空即可", path: "/api/twitter/timeline", field: null, extra: { kind: "for_you", limit: 20 } },
    { id: "twitter-likes", label: "X / Twitter 喜欢", hint: "留空即可", path: "/api/twitter/likes", field: null, extra: { limit: 20 } },
    { id: "reddit-subreddit", label: "Reddit 板块", hint: "板块名，如 technology", path: "/api/reddit/subreddit", field: "subreddit", extra: { limit: 20 } },
    { id: "reddit-search", label: "Reddit 搜索", hint: "关键词", path: "/api/reddit/search", field: "query", extra: { limit: 20 } },
    { id: "xhs-search", label: "小红书搜索", hint: "关键词", path: "/api/xiaohongshu/search", field: "query", extra: { limit: 20 } },
    { id: "xhs-note", label: "小红书笔记", hint: "笔记 URL", path: "/api/xiaohongshu/note", field: "url", extra: {} },
    { id: "reddit-thread", label: "Reddit 帖子", hint: "帖子 URL", path: "/api/reddit/thread", field: "url", extra: {} },
    { id: "extract", label: "网页提取为 Markdown", hint: "网页 URL", path: "/api/extract", field: "url", extra: {} }
  ];
  function adapterById(id) {
    for (var i = 0; i < ADAPTERS.length; i++) if (ADAPTERS[i].id === id) return ADAPTERS[i];
    return ADAPTERS[0];
  }

  function renderNews() {
    var groups = {};
    (state.sources || []).forEach(function (s) {
      var g = s.category || "未分组";
      if (!groups[g]) groups[g] = [];
      groups[g].push(s);
    });
    if (state.sourceCategories && state.sourceCategories.length) {
      state.sourceCategories.forEach(function (name) {
        if (name && !groups[name]) groups[name] = [];
      });
    }
    var html = head("按话题分组查看来源，并通过浏览器扩展实时采集平台内容");
    html += '<div class="panel"><h3>来源分组</h3>';
    var names = Object.keys(groups);
    html += names.length
      ? '<div class="group-grid">' + names.map(function (g) {
          return '<button class="group-card" data-action="news-group" data-value="' + attr(g) + '" type="button">' +
            '<div class="group-name">' + esc(g) + "</div>" +
            '<div class="group-count">' + esc(groups[g].length) + " 个源</div>" +
            '<div class="group-list">' + esc(groups[g].slice(0, 3).map(function (s) { return s.name; }).join("、") || "点击查看内容") + "</div></button>";
        }).join("") + "</div>"
      : "<p>尚无分组，先在 RSS 源里给订阅源设置分组。</p>";
    html += "</div>";

    var a = adapterById(state.adapter);
    html += '<div class="panel"><h3>平台采集</h3>' +
      '<p>经 APP 中转下发给 Chrome 扩展，在真实浏览器里带登录态执行。</p>' +
      '<form data-form="adapter" class="form-grid" style="margin-top:14px">' +
        '<label class="field"><span>采集动作</span><select name="adapter">' +
          ADAPTERS.map(function (x) { return '<option value="' + attr(x.id) + '"' + (x.id === state.adapter ? " selected" : "") + ">" + esc(x.label) + "</option>"; }).join("") +
        "</select></label>" +
        '<label class="field"><span>' + esc(a.hint) + '</span><input name="value" type="text" placeholder="' + esc(a.hint) + '" autocomplete="off"></label>' +
        '<div class="field-action"><button class="btn primary" type="submit"' + (state.adapterBusy ? " disabled" : "") + ">" + (state.adapterBusy ? "采集中…" : "执行采集") + "</button></div>" +
      "</form>";
    if (state.adapterResult != null) {
      var text = typeof state.adapterResult === "string" ? state.adapterResult : JSON.stringify(state.adapterResult, null, 2);
      if (text.length > 6000) text = text.slice(0, 6000) + "\n…（结果已截断）";
      html += '<pre class="code-block result-block">' + esc(text) + "</pre>";
    }
    html += "</div>";

    html += '<div class="panel"><h3>最近的网页提取归档</h3>';
    html += state.captures.length
      ? state.captures.map(function (c) {
          return '<div class="row"><div><div class="row-value">' + esc(c.title || c.url) + "</div>" +
            '<div class="row-label mono">' + esc(c.url || "") + "</div></div>" +
            '<span class="badge">' + esc((c.chars || 0) + " 字符") + "</span></div>";
        }).join("")
      : "<p>暂无归档。执行一次「网页提取为 Markdown」后会显示在这里。</p>";
    html += "</div>";
    return html;
  }

  function field(label, name, value, type, extra) {
    return '<label class="field"><span>' + esc(label) + "</span>" +
      '<input name="' + attr(name) + '" type="' + (type || "text") + '" value="' + attr(value == null ? "" : value) + '" ' + (extra || "") + "></label>";
  }
  function switchField(label, name, checked) {
    return '<label class="switch-field"><input type="checkbox" name="' + attr(name) + '"' + (checked ? " checked" : "") + ">" +
      "<span>" + esc(label) + "</span></label>";
  }

  /* ---------------- Settings primitives ---------------- */

  function settingRow(label, valueHtml, desc) {
    return '<div class="setting-row"><div class="setting-row-main">' +
      '<div class="setting-row-label">' + esc(label) + "</div>" +
      (desc ? '<div class="setting-row-desc">' + esc(desc) + "</div>" : "") +
      "</div><div class=\"setting-row-control\">" + valueHtml + "</div></div>";
  }

  function settingSection(index, title, desc, body, aside) {
    return '<section class="panel setting-section" id="sec-' + attr(index) + '">' +
      '<div class="setting-head"><div class="setting-head-main">' +
        '<span class="setting-index">' + esc(index) + "</span>" +
        '<div class="setting-head-text"><h2 class="setting-title">' + esc(title) + "</h2>" +
        (desc ? '<p class="setting-desc">' + esc(desc) + "</p>" : "") + "</div>" +
      "</div>" + (aside ? '<div class="setting-aside">' + aside + "</div>" : "") + "</div>" +
      '<div class="setting-body">' + body + "</div></section>";
  }

  function settingNav(items) {
    return '<div class="settings-nav">' + items.map(function (x) {
      return '<button class="settings-nav-item" data-action="jump" data-target="sec-' + attr(x[0]) + '" type="button">' +
        '<span class="settings-nav-index">' + esc(x[0]) + "</span>" + esc(x[1]) + "</button>";
    }).join("") + "</div>";
  }

  var PROVIDER_PRESETS = [
    { id: "openai", name: "OpenAI", provider: "openai", base_url: "", models: ["gpt-4o-mini", "gpt-4o", "gpt-4.1"] },
    { id: "anthropic", name: "Anthropic", provider: "anthropic", base_url: "", models: ["claude-sonnet-5-5", "claude-opus-5-5", "claude-haiku-5-5"] },
    { id: "deepseek", name: "DeepSeek", provider: "custom", base_url: "https://api.deepseek.com/v1", models: ["deepseek-chat", "deepseek-reasoner"] },
    { id: "gemini", name: "Gemini", provider: "gemini", base_url: "", models: ["gemini-2.0-flash", "gemini-1.5-pro"] },
    { id: "openrouter", name: "OpenRouter", provider: "custom", base_url: "https://openrouter.ai/api/v1", models: ["anthropic/claude-3.5-sonnet", "google/gemini-2.0-flash-001"] },
    { id: "ollama", name: "Ollama", provider: "ollama", base_url: "http://localhost:11434/v1", models: ["qwen3", "llama3.2", "deepseek-r1"] },
    { id: "lmstudio", name: "LM Studio", provider: "lmstudio", base_url: "http://localhost:1234/v1", models: [] },
    { id: "custom", name: "自定义", provider: "custom", base_url: "", models: [] }
  ];

  var PROVIDER_ALIASES = { google: "gemini" };
  var COMMON_PROVIDER_IDS = ["openai", "anthropic", "gemini", "deepseek", "openrouter", "ollama", "lmstudio", "custom", "xai", "mistral", "groq", "qwen"];

  function normalizeBaseURL(url) {
    return String(url || "").trim().replace(/\/+$/, "").toLowerCase();
  }

  // providerList merges the live models.dev catalog over the built-in presets.
  // Branding and known endpoints stay curated; model lists come from the live
  // catalog so the desktop app no longer ships a stale provider table.
  function providerList() {
    var live = (state.aiCatalog && state.aiCatalog.providers) || [];
    if (!live.length) return PROVIDER_PRESETS.slice();
    var out = [];
    var seen = {};
    live.forEach(function (p) {
      var id = PROVIDER_ALIASES[p.id] || p.id;
      var preset = {
        id: id,
        name: p.name || id,
        provider: p.driver || "custom",
        base_url: p.api || "",
        models: (p.models || []).map(function (m) { return m.id; }),
        doc: p.doc || "",
        env: p.env || []
      };
      for (var i = 0; i < PROVIDER_PRESETS.length; i++) {
        var base = PROVIDER_PRESETS[i];
        if (base.id === id || (base.provider === preset.provider && base.base_url && normalizeBaseURL(base.base_url) === normalizeBaseURL(preset.base_url))) {
          preset.name = base.name;
          preset.base_url = base.base_url || preset.base_url;
          if (!preset.models.length) preset.models = base.models;
          break;
        }
      }
      if (!seen[id]) { seen[id] = 1; out.push(preset); }
    });
    PROVIDER_PRESETS.forEach(function (p) {
      if (!seen[p.id]) { seen[p.id] = 1; out.push(p); }
    });
    // Keep the generic custom endpoint at the end so it never looks like the
    // primary recommendation.
    out.sort(function (a, b) {
      if (a.id === "custom") return 1;
      if (b.id === "custom") return -1;
      return String(a.name).localeCompare(String(b.name));
    });
    return out;
  }

  function presetById(id) {
    var list = providerList();
    for (var i = 0; i < list.length; i++) if (list[i].id === id) return list[i];
    return null;
  }

  // activePreset matches the saved settings against the provider list so the
  // matching chip stays highlighted after a reload.
  function activePreset(settings) {
    var provider = String((settings && settings.provider) || "");
    var baseURL = normalizeBaseURL(settings && settings.base_url);
    var list = providerList();
    for (var i = 0; i < list.length; i++) {
      var p = list[i];
      if (p.id === "custom") continue;
      if (p.provider === provider && normalizeBaseURL(p.base_url) === baseURL) return p.id;
    }
    for (var j = 0; j < list.length; j++) {
      if (list[j].provider === provider && !baseURL && !normalizeBaseURL(list[j].base_url)) return list[j].id;
    }
    // A saved custom endpoint may have an empty base URL; keep the generic
    // "custom" preset selected instead of falling back to the first provider.
    if (provider === "custom" || provider) return "custom";
    return "";
  }

  function modelSuggestions(settings) {
    var provider = String((settings && settings.provider) || "");
    var baseURL = normalizeBaseURL(settings && settings.base_url);
    var list = providerList();
    for (var i = 0; i < list.length; i++) {
      var p = list[i];
      if (p.provider === provider && normalizeBaseURL(p.base_url) === baseURL) return p.models;
    }
    for (var j = 0; j < list.length; j++) {
      if (list[j].provider === provider && !baseURL) return list[j].models;
    }
    return [];
  }

  // applyProviderPreset fills the visible form from a catalog entry. The
  // hidden `provider` field keeps the backend driver separate from the catalog
  // id (models.dev has many providers that share the "custom" driver).
  function applyProviderPreset(preset) {
    var aiForm = document.querySelector('form[data-form="ai-settings"]');
    if (!preset || !aiForm) return;
    var presetSel = aiForm.querySelector('[name="provider_preset"]');
    var providerInput = aiForm.querySelector('[name="provider"]');
    var baseInput = aiForm.querySelector('[name="base_url"]');
    var modelInput = aiForm.querySelector('[name="model"]');
    if (presetSel) presetSel.value = preset.id;
    if (providerInput) providerInput.value = preset.provider || "custom";
    if (baseInput) baseInput.value = preset.base_url || "";
    if (modelInput) modelInput.value = preset.models.length ? preset.models[0] : "";
    state.discoveredModels = [];
    var discovered = $("discovered-models");
    if (discovered) discovered.innerHTML = "";
    var list = $("model-options");
    if (list) {
      list.innerHTML = (preset.models || []).map(function (m) {
        return '<option value="' + attr(m) + '"></option>';
      }).join("");
    }
    var chips = document.querySelectorAll('#content [data-action="provider-preset"]');
    for (var i = 0; i < chips.length; i++) {
      chips[i].classList.toggle("is-active", chips[i].dataset.value === preset.id);
    }
  }

  // renderDiscoveredModelsHTML renders the checkbox list returned by the
  // provider's /v1/models endpoint. The model text input stays the source of
  // truth; "应用所选" copies the checked ids into it as a comma-separated list.
  function renderDiscoveredModelsHTML(models, selectedRefs) {
    if (!models || !models.length) return "";
    var selected = {};
    (selectedRefs || []).forEach(function (ref) {
      selected[stripModelRef(ref)] = 1;
    });
    var items = models.map(function (model, index) {
      var id = typeof model === "string" ? model : model.id;
      var name = typeof model === "string" ? model : (model.name || model.id);
      if (!id) return "";
      var checked = selected[id] ? " checked" : "";
      return '<label class="discovered-item" data-search="' + attr((id + " " + name).toLowerCase()) + '">' +
        '<input type="checkbox" name="discovered_model" value="' + attr(id) + '"' + checked + ">" +
        '<span class="discovered-name">' + esc(name) + '</span>' +
        '<span class="discovered-id mono">' + esc(id) + "</span></label>";
    }).join("");
    return '<div class="discovered-head"><span>发现 ' + models.length + ' 个模型，勾选后点「应用所选」</span>' +
      '<button class="mini" data-action="ai-apply-models" type="button">应用所选</button></div>' +
      '<input id="discovered-search" class="date-input discovered-search" placeholder="过滤模型 ID 或名称">' +
      '<div class="discovered-items">' + items + "</div>";
  }

  function applyDiscoveredSelection() {
    var checked = document.querySelectorAll('#discovered-models input[name="discovered_model"]:checked');
    var values = [];
    for (var i = 0; i < checked.length; i++) values.push(checked[i].value);
    var input = document.querySelector('#content form[data-form="ai-settings"] input[name="model"]');
    if (!input) return;
    if (!values.length) { toast("请先勾选至少一个模型", true); return; }
    input.value = values.join(", ");
    toast("已写入 " + values.length + " 个模型，保存后生效");
  }

  var MODEL_PREFIXES = { gemini: "googleai/", openai: "openai/", anthropic: "anthropic/", ollama: "ollama/", vertex: "vertexai/", custom: "custom/" };

  // Genkit needs a namespaced model reference, while users think in plain model
  // ids. The settings page shows the plain id and re-adds the namespace on save,
  // which mirrors the web UI's fromModelRef/toModelRef helpers.
  function stripModelRef(model) {
    var m = String(model || "");
    var lower = m.toLowerCase();
    var known = ["googleai/", "openai/", "anthropic/", "ollama/", "vertexai/", "custom/"];
    for (var i = 0; i < known.length; i++) {
      if (lower.indexOf(known[i]) === 0) return m.slice(known[i].length);
    }
    return m;
  }

  function modelRefFor(provider, model) {
    var m = String(model || "").trim();
    var prefix = MODEL_PREFIXES[String(provider || "").toLowerCase()] || "";
    if (!prefix || !m) return m;
    if (m.toLowerCase().indexOf(prefix) === 0) return m;
    var slash = m.indexOf("/");
    if (slash > 0) {
      var head = m.slice(0, slash).toLowerCase();
      if (head === "googleai" || head === "openai" || head === "anthropic" || head === "ollama" || head === "vertexai" || head === "custom") m = m.slice(slash + 1);
    }
    return prefix + m;
  }

  function strategyLabel(value) {
    return { single: "单模型", "round-robin": "轮询", failover: "故障转移" }[value] || value || "单模型";
  }

  function providerLabel(value) {
    for (var i = 0; i < PROVIDER_PRESETS.length; i++) {
      if (PROVIDER_PRESETS[i].provider === value) return PROVIDER_PRESETS[i].name;
    }
    return value || "自定义";
  }

  // currentProfile picks the profile being edited: the one the user selected,
  // otherwise the active one from the server. A "__new" selection means the
  // form starts empty and saving appends a profile.
  function currentProfile(ai) {
    if (state.activeProfileID === "__new") return null;
    var list = (ai && ai.profiles) || [];
    var id = state.activeProfileID || (ai && ai.active_profile_id) || "";
    for (var i = 0; i < list.length; i++) if (list[i].id === id) return list[i];
    return list.length ? list[0] : null;
  }

  function profileName(p, settings) {
    if (!p) return "新配置";
    return p.name || providerLabel(p.provider) || settings.model || p.id;
  }

  function morningEveningSummary(s) {
    var parts = [];
    if (s.morning_report_enabled) parts.push("早报 " + (s.morning_report_time || "08:00"));
    if (s.evening_report_enabled) parts.push("晚报 " + (s.evening_report_time || "20:00"));
    return parts.length ? esc(parts.join(" · ")) : '<span class="setting-muted">未启用</span>';
  }

  function renderAi() {
    var s = state.aiSettings || {};
    var st = state.aiStats || {};
    var tab = state.aiTab === "overview" ? "overview" : "items";
    var html = head("每条新内容自动进入分析队列，按质量评分筛选优质内容与日报素材");
    html += '<div class="toolbar"><div class="segmented">' +
      '<button class="seg' + (tab === "items" ? " is-active" : "") + '" data-action="ai-tab" data-value="items" type="button">优质内容</button>' +
      '<button class="seg' + (tab === "overview" ? " is-active" : "") + '" data-action="ai-tab" data-value="overview" type="button">概览与配置</button>' +
      "</div></div>";

    if (tab === "overview") {
      html += statGrid([
        [st.total_processed == null ? "—" : st.total_processed, "已分析"],
        [st.total_pending == null ? "—" : st.total_pending, "待处理"],
        [st.average_score == null ? "—" : Number(st.average_score).toFixed(1), "平均评分"],
        [st.queue_length == null ? "—" : st.queue_length, "队列长度"]
      ]);
      var usage = state.aiUsage || {};
      html += '<div class="panel" style="margin-top:18px"><h3>用量 · 近 ' + esc(usage.days || 7) + " 天</h3>" +
        '<div class="row"><span class="row-label">调用</span><span class="row-value">' + esc(usage.total_requests || 0) + " 次" +
          (usage.failed_requests ? " · " + esc(usage.failed_requests) + " 次失败" : "") + "</span></div>" +
        '<div class="row"><span class="row-label">Tokens</span><span class="row-value mono">' +
          esc(Number(usage.prompt_tokens || 0).toLocaleString()) + " 输入 / " +
          esc(Number(usage.completion_tokens || 0).toLocaleString()) + " 输出</span></div>" +
        '<div class="row"><span class="row-label">平均延迟</span><span class="row-value mono">' +
          esc(Math.round(usage.avg_latency_ms || 0)) + " ms</span></div></div>";
      html += '<div class="toolbar" style="margin-top:18px">' +
        '<button class="btn" data-action="ai-test" type="button">测试模型连通性</button>' +
        '<button class="btn" data-action="ai-eval" type="button">启动全量评测</button>' +
        '<button class="btn" data-action="goto" data-view="settings" type="button">模型设置</button>' +
        '<span class="toolbar-note">模型、提示词与日报定时已移到「设置」页</span></div>';

      html += '<div class="panel"><h3>当前分析配置</h3>' +
        '<div class="row"><span class="row-label">AI 分析</span><span class="row-value">' +
          (s.enabled ? badge("已启用", true) : badge("未启用", false)) + "</span></div>" +
        settingRow("服务商 / 模型", esc(s.provider || "未配置") + " · " + esc(s.model || "未配置")) +
        settingRow("Base URL", '<span class="mono">' + esc(s.base_url || "服务商默认") + "</span>") +
        settingRow("优质阈值", esc(s.quality_threshold == null ? 7 : s.quality_threshold) + " 分以上") +
        settingRow("多模型策略", esc(strategyLabel(s.strategy))) +
        settingRow("早晚报", morningEveningSummary(s)) +
        '<p class="hint">改动请到「设置 → AI 模型」，保存后立即对新的分析任务生效。</p></div>';
      return html;
    }

    if (state.aiCategories && state.aiCategories.length) {
      html += chips(asNames(state.aiCategories), state.aiCategory, "ai-category", "value");
      html += '<div style="height:14px"></div>';
    }
    if (!state.aiItems.length) html += empty("暂无优质内容", "AI 分析完成后，评分达到阈值的条目会显示在这里。");
    else html += '<div class="ai-list">' + state.aiItems.map(function (it) {
      return '<div class="ai-row' + (String(state.reader.id) === String(it.id) ? " is-active" : "") +
        '" data-action="detail" data-id="' + attr(it.id) + '" role="button" tabindex="0" aria-label="阅读：' + attr(it.title || "未命名内容") + '">' +
        '<div class="ai-score">' + esc(it.quality_score == null ? "—" : Number(it.quality_score).toFixed(1)) + "</div>" +
        '<div class="ai-main"><div class="ai-title">' + esc(it.title || "未命名内容") + "</div>" +
        '<div class="ai-summary">' + esc(it.ai_summary || it.summary || "") + "</div>" +
        '<div class="ai-meta">' + esc(it.ai_category || it.category || "未分类") + " · " + esc(it.origin_source || "") +
          (it.ai_processed_at ? " · 分析于 " + esc(fmtTime(it.ai_processed_at)) : "") + "</div></div>" +
        '<button class="mini" data-action="reanalyze" data-id="' + attr(it.id) + '" type="button">重新分析</button>' +
        "</div>";
    }).join("") + "</div>";
    return html;
  }

  function dailySections(data) {
    if (!data) return null;
    if (Array.isArray(data.sections)) {
      var obj = {};
      data.sections.forEach(function (s, i) { obj["section_" + (i + 1)] = s; });
      return obj;
    }
    return data.sections && typeof data.sections === "object" ? data.sections : null;
  }

  var REPORT_TYPE_LABELS = { daily: "日报", morning: "早报", evening: "晚报" };
  function reportTypeLabel(type) { return REPORT_TYPE_LABELS[type] || type || "日报"; }
  function reportsForDate(list, date) {
    return (list || []).filter(function (r) { return (r.report_date || r.date || "") === date; });
  }
  function latestReportForDate(list, date) {
    var same = reportsForDate(list, date);
    if (!same.length) return null;
    return same.slice().sort(function (a, b) {
      return new Date(b.generated_at || 0) - new Date(a.generated_at || 0);
    })[0];
  }

  function renderDaily() {
    var d = state.daily || {};
    var html = head("按日期查看自动生成的每日总结、早报与晚报");
    html += '<div class="toolbar daily-toolbar">' +
      '<input class="date-input" type="date" id="daily-date" value="' + attr(state.dailyDate) + '">' +
      '<select class="select-input" id="daily-type">' +
        [["daily", "日报"], ["morning", "早报"], ["evening", "晚报"]].map(function (x) {
          return '<option value="' + x[0] + '"' + (state.dailyType === x[0] ? " selected" : "") + ">" + x[1] + "</option>";
        }).join("") + "</select>" +
      '<button class="btn primary" data-action="daily-generate" type="button">生成日报</button>' +
      '<button class="btn" data-action="daily-prev" type="button">前一天</button>' +
      '<button class="btn" data-action="daily-next" type="button">后一天</button>' +
      '<button class="btn" data-action="daily-week" type="button">补生成近 7 天</button>' +
      "</div>";

    var sections = dailySections(d);
    if (d.format === "markdown") {
      html += '<div class="panel"><h3>' + esc(d.title || "日报") + "</h3>" +
        (d.html_content
          ? '<div class="detail-content">' + safeHtml(d.html_content) + "</div>"
          : '<pre class="code-block report-block">' + esc(d.content || "") + "</pre>") + "</div>";
    } else if (sections && Object.keys(sections).length) {
      html += '<div class="report-head"><div class="eyebrow">' + esc(state.dailyType === "daily" ? "DAILY BRIEF" : state.dailyType.toUpperCase()) + "</div>" +
        '<h2 class="report-title">' + esc(d.title || "Grabby 日报") + "</h2>" +
        '<div class="report-meta">' + esc(d.date || d.report_date || state.dailyDate) +
        (d.editor ? " · " + esc(d.editor) : "") +
        (d.total_items != null ? " · " + esc(d.total_items) + " 条内容 / " + esc(d.quality_items || 0) + " 条优质" : "") + "</div></div>";
      html += Object.keys(sections).map(function (key) {
        var sec = sections[key] || {};
        var items = Array.isArray(sec.items) ? sec.items : [];
        return '<div class="report-section"><h3>' + esc(sec.title || key) + "</h3>" +
          items.map(function (it) {
            var link = it.link || it.url || "";
            return '<div class="report-item">' +
              '<div class="report-item-top">' +
                (link ? '<a href="' + attr(link) + '" target="_blank" rel="noopener" class="report-link">' + esc(it.title || "未命名") + "</a>"
                      : '<span class="report-link">' + esc(it.title || "未命名") + "</span>") +
                (it.score != null ? badge(String(it.score), false) : "") + "</div>" +
              (it.summary ? '<div class="report-summary">' + esc(it.summary) + "</div>" : "") +
              (it.comment ? '<div class="report-comment">' + esc(it.comment) + "</div>" : "") +
              (it.source ? '<div class="report-source">' + esc(it.source) + "</div>" : "") +
              "</div>";
          }).join("") + "</div>";
      }).join("");
    } else {
      // The user may be looking at a type that was never generated for this
      // date (e.g. the default 日报 while only 早报/晚报 exist). Say so instead
      // of claiming the whole day is empty, and link the reports that do exist.
      var typeName = reportTypeLabel(state.dailyType);
      var others = reportsForDate(state.dailyList, state.dailyDate).filter(function (r) {
        return (r.report_type || "daily") !== state.dailyType;
      });
      if (others.length) {
        html += '<div class="panel"><h3>' + esc(state.dailyDate + " 还没有" + typeName) + "</h3>" +
          '<p class="hint">该日期已生成 ' + esc(others.map(function (r) { return reportTypeLabel(r.report_type || "daily"); }).join(" / ")) + "，可直接查看。</p>" +
          others.map(function (r) {
            var rt = r.report_type || "daily";
            return '<div class="row"><div><div class="row-value">' + esc(r.title || (state.dailyDate + " · " + reportTypeLabel(rt))) + "</div>" +
              '<div class="row-label">' + esc(reportTypeLabel(rt)) + " · 生成于 " + esc(fmtTime(r.generated_at)) + "</div></div>" +
              '<button class="mini" data-action="daily-open" data-date="' + attr(state.dailyDate) + '" data-type="' + attr(rt) + '" type="button">查看</button></div>';
          }).join("") + "</div>";
      } else {
        html += '<div class="panel">' + empty(state.dailyDate + " 暂无日报", "点击「生成日报」在后台生成，稍后会自动刷新。") + "</div>";
      }
    }

    html += '<h2 class="section-title">历史日报</h2><div class="panel">';
    html += state.dailyList.length
      ? state.dailyList.map(function (r) {
          var date = r.report_date || r.date || "";
          var type = r.report_type || "daily";
          return '<div class="row"><div><div class="row-value">' + esc(r.title || (date + " · " + reportTypeLabel(type))) + "</div>" +
            '<div class="row-label">' + esc(date) + " · " + esc(reportTypeLabel(type)) +
            (r.generated_at ? " · 生成于 " + esc(fmtTime(r.generated_at)) : "") + "</div></div>" +
            '<button class="mini" data-action="daily-open" data-date="' + attr(date) + '" data-type="' + attr(type) + '" type="button">查看</button></div>';
        }).join("")
      : "<p>暂无历史日报</p>";
    html += "</div>";
    html += '<div class="panel"><h3>订阅日报 RSS</h3>' +
      '<div class="row"><span class="row-value mono">' + esc(location.origin) + '/open/api/ai/daily/rss?limit=20</span>' +
      '<button class="copy-btn" data-copy="' + attr(location.origin + "/open/api/ai/daily/rss?limit=20") + '" type="button">复制 RSS 地址</button></div></div>';
    return html;
  }

  function renderDevices() {
    var list = state.browsers || [];
    var html = head("扩展在真实浏览器里带登录态执行抓取，服务端只负责调度");
    html += '<div class="panel"><h3>已注册设备</h3>';
    if (!list.length) html += empty("暂无设备", "在扩展设置中填写 WebSocket 地址并连接。");
    else html += list.map(function (b) {
      return '<div class="list-row"><div class="list-main">' +
        '<div class="list-title">' + esc(b.name || "未命名") + " " + badge(b.banned ? "已禁用" : (b.online ? "在线" : "离线"), !b.banned && b.online) + "</div>" +
        '<div class="list-sub mono">' + esc(b.conn_id || "") + "</div></div>" +
        '<div class="list-actions">' +
          '<button class="mini" data-action="device-kick" data-id="' + attr(b.conn_id) + '" type="button">断开</button>' +
          (b.banned
            ? '<button class="mini" data-action="device-unban" data-id="' + attr(b.conn_id) + '" type="button">解禁</button>'
            : '<button class="mini danger" data-action="device-ban" data-id="' + attr(b.conn_id) + '" type="button">禁用</button>') +
        "</div></div>";
    }).join("");
    html += "</div>";
    html += '<div class="panel"><h3>扩展连接信息</h3>' +
      '<div class="row"><span class="row-label">WebSocket 地址</span><span class="row-value mono">ws://localhost:' + esc(location.port || "5040") + "/ws_browser</span></div>" +
      '<div class="row"><span class="row-label">注册接口</span><span class="row-value mono">/api/browsers/register</span></div>' +
      '<div class="row"><span class="row-label">登录态</span><span class="row-value">由浏览器侧保证，服务端不保存</span></div></div>';
    return html;
  }

  function renderApiHub() {
    var base = location.origin;
    var tokenHeader = state.tokenEnabled ? ' \\\n  -H "X-API-Key: <YOUR_TOKEN>"' : "";
    var extract = "curl -X POST " + base + "/api/extract" + tokenHeader + ' \\\n  -H "Content-Type: application/json" \\\n  -d \'{"url":"https://example.com"}\'';
    var twitter = "curl -X POST " + base + "/api/twitter/search" + tokenHeader + ' \\\n  -H "Content-Type: application/json" \\\n  -d \'{"query":"grabby","limit":20}\'';
    var html = head("这个 APP 本身就是中转服务：外部请求进来，由它经 WebSocket 下发给浏览器扩展");
    html += '<div class="panel"><h3>服务地址</h3>' +
      '<div class="row"><span class="row-label">Base URL</span><span class="row-value mono">' + esc(base) + "</span></div>" +
      '<div class="row"><span class="row-label">MCP</span><span class="row-value mono">' + esc(base) + "/mcp</span></div>" +
      '<div class="row"><span class="row-label">鉴权</span><span class="row-value">' +
      (state.tokenEnabled ? badge("已启用 Token", true) : badge("本机免密 · 默认", true)) + "</span></div>" +
      '<div class="row"><span class="row-label">监听范围</span><span class="row-value">' +
      (state.external ? "0.0.0.0 · 可被局域网访问" : "127.0.0.1 · 仅本机") + "</span></div>" +
      '<div style="margin-top:14px"><button class="copy-btn" data-copy="' + attr(base) + '" type="button">复制 Base URL</button></div></div>';
    html += '<div class="panel"><h3>中转链路</h3><div class="flow">' +
      '<span class="flow-node">外部 API 调用</span><span class="flow-arrow">→</span>' +
      '<span class="flow-node">Grabby APP</span><span class="flow-arrow">→</span>' +
      '<span class="flow-node">WebSocket /ws_browser</span><span class="flow-arrow">→</span>' +
      '<span class="flow-node">Chrome 扩展</span><span class="flow-arrow">→</span>' +
      '<span class="flow-node">目标网页</span></div>' +
      '<p class="hint">关闭窗口后 APP 会继续在托盘运行，API 服务不中断。</p></div>';
    html += '<div class="panel"><h3>主要端点</h3><div class="endpoint-list">' +
      [
        ["GET", "/open/api/items", "内容列表（免鉴权）"], ["GET", "/open/api/stats", "统计"],
        ["GET", "/open/api/ai/items", "AI 优质内容"], ["GET", "/open/api/ai/daily", "AI 日报"],
        ["GET", "/api/sources", "订阅源列表"], ["POST", "/api/sources", "新增订阅源"],
        ["POST", "/api/sources/:id/run", "立即抓取"], ["POST", "/api/extract", "网页提取"],
        ["POST", "/api/twitter/search", "X 搜索"], ["POST", "/api/ai/daily/generate", "生成日报"],
        ["POST", "/api/ai/start_eval", "全量 AI 评测"], ["GET", "/api/browsers", "设备列表"]
      ].map(function (e) {
        return '<div class="endpoint"><span class="method">' + esc(e[0]) + "</span>" +
          '<span class="mono">' + esc(e[1]) + "</span><span class=\"row-label\">" + esc(e[2]) + "</span></div>";
      }).join("") + "</div></div>";
    html += '<div class="panel"><h3>提取网页为 Markdown</h3>' +
      '<pre class="code-block">' + esc(extract) + "</pre>" +
      '<div style="margin-top:10px"><button class="copy-btn" data-copy="' + attr(extract) + '" type="button">复制命令</button></div></div>';
    html += '<div class="panel"><h3>调用平台适配器（示例）</h3><pre class="code-block">' + esc(twitter) + "</pre></div>";
    if (!state.tokenEnabled && state.external) {
      html += '<div class="panel"><h3>安全提示</h3><p>当前对外开放但未设置 API Token，请到「服务设置」页设置一个。</p></div>';
    }
    return html;
  }

  function randomToken() {
    var bytes = new Uint8Array(16);
    (window.crypto || window.msCrypto).getRandomValues(bytes);
    return Array.prototype.map.call(bytes, function (b) {
      return ("0" + b.toString(16)).slice(-2);
    }).join("");
  }

  function renderSettings() {
    var s = state.desktopSettings || {};
    var h = state.health || {};
    var ai = state.aiSettings || {};
    var st = state.aiStats || {};
    var running = h.status === "ok";
    var tokenOn = !!s.token_enabled;
    var token = s.api_token || "";
    var port = s.port || location.port || "5040";
    var host = s.host || "127.0.0.1";
    var base = location.origin;
    var curl = "curl " + base + "/open/api/health" +
      (tokenOn ? ' \\\n  -H "X-API-Key: ' + token + '"' : "");
    var profiles = ai.profiles || [];
    var prof = currentProfile(ai);
    // A new profile starts from an empty form (keeping the provider) instead of
    // copying the active one, so the user cannot accidentally duplicate a key.
    var effective = prof || (state.activeProfileID === "__new"
      ? { provider: ai.provider || "custom", model: "", base_url: "", api_key: "" }
      : ai);
    var activePresetID = activePreset(effective);
    var suggestions = modelSuggestions(effective);
    var effectiveModels = (effective.models && effective.models.length)
      ? effective.models
      : (effective.model ? [effective.model] : []);
    var modelValues = effectiveModels.map(stripModelRef).join(", ");
    var discoveredHTML = renderDiscoveredModelsHTML(state.discoveredModels || [], effectiveModels);

    var html = head("模型、提示词、日报定时与本地服务都在这里，保存后立即生效");

    html += settingNav([
      ["01", "服务状态"], ["02", "AI 模型"], ["03", "提示词"],
      ["04", "日报定时"], ["05", "API 与安全"], ["06", "数据与文件"]
    ]);

    /* 01 服务状态 */
    html += settingSection("01", "服务状态", "HTTP / WebSocket / MCP 服务随 APP 启动；关窗后留在托盘继续运行，从托盘「退出」才真正结束", 
      settingRow("进程", running ? badge("运行中", true) : badge("未响应", false)) +
      settingRow("监听地址", '<span class="mono">' + esc(host) + ":" + esc(port) + "</span>") +
      settingRow("API Base URL", '<span class="mono">' + esc(base) + "</span>") +
      settingRow("本机请求", badge("始终免密", true), "APP 界面、CLI、MCP 与扩展都走 loopback，不会要求登录") +
      settingRow("API Token", tokenOn ? badge("已启用", false) : badge("未启用 · 默认", true)) +
      settingRow("扩展 WebSocket", '<span class="mono">ws://localhost:' + esc(port) + "/ws_browser</span>")
    );

    if (s.port_shifted) {
      html += '<div class="panel notice" style="margin-top:14px"><h3>端口已顺延</h3>' +
        '<p>默认的 ' + esc(5040) + ' 被其他程序占用，APP 自动改用 ' + esc(port) + "。如果那是你在终端启动的 Grabby 服务，" +
        "请退出它后重启本 APP，并把扩展里的 WebSocket 地址改成新端口。</p></div>";
    }

    /* 02–04 共用一次保存 */
    html += '<form data-form="ai-settings">';

    var providers = providerList();
    var commonProviders = providers.filter(function (p) { return COMMON_PROVIDER_IDS.indexOf(p.id) >= 0; });
    if (commonProviders.length < 8) {
      providers.forEach(function (p) {
        if (commonProviders.length >= 8) return;
        if (p.id === "custom") return;
        if (commonProviders.indexOf(p) < 0) commonProviders.push(p);
      });
    }
    var presetChips = commonProviders.map(function (p) {
      return '<button class="chip' + (activePresetID === p.id ? " is-active" : "") +
        '" data-action="provider-preset" data-value="' + attr(p.id) + '" type="button">' + esc(p.name) + "</button>";
    }).join("");

    var providerOptions = providers.map(function (p) {
      return '<option value="' + attr(p.id) + '" data-driver="' + attr(p.provider) + '"' +
        (activePresetID === p.id ? " selected" : "") + ">" + esc(p.name) + "</option>";
    }).join("");

    var catalogLabel = (state.aiCatalog && state.aiCatalog.providers && state.aiCatalog.providers.length)
      ? "models.dev · " + state.aiCatalog.providers.length + " 个服务商"
      : "内置目录（点「刷新目录」获取 models.dev 实时数据）";

    var profileOptions = profiles.map(function (p) {
      return '<option value="' + attr(p.id) + '"' + (prof && prof.id === p.id ? " selected" : "") + ">" +
        esc(profileName(p, ai) + (p.disabled ? "（已停用）" : "")) + "</option>";
    }).join("");
    if (state.activeProfileID === "__new" || !profiles.length) {
      profileOptions = '<option value="__new"' + (state.activeProfileID === "__new" || !profiles.length ? " selected" : "") + ">新配置</option>" + profileOptions;
    }

    html += settingSection("02", "AI 模型", "选择要使用的配置档并编辑它；同一个配置档可以放多个模型做轮询", 
      '<div class="setting-switch" style="margin-top:0">' + switchField("启用 AI 分析", "enabled", !!ai.enabled) + "</div>" +
      '<div class="profile-row">' +
        '<label class="field"><span>当前配置档</span><select name="profile_id">' + profileOptions + "</select></label>" +
        '<button class="btn" data-action="new-profile" type="button">新建配置档</button>' +
        '<button class="btn" data-action="ai-import" type="button">导入已有配置</button>' +
      "</div>" +
      '<div class="preset-block"><div class="preset-label">常用服务商（填入下方字段）</div><div class="chips">' + presetChips + "</div>" +
        '<div class="catalog-row"><span class="hint">' + esc(catalogLabel) + "</span>" +
        '<button class="mini" data-action="ai-catalog-refresh" type="button">刷新目录</button></div></div>' +
      '<div class="form-grid" style="margin-top:16px">' +
        '<label class="field"><span>服务商</span><select name="provider_preset">' + providerOptions + "</select>" +
          '<input type="hidden" name="provider" value="' + attr(effective.provider || "custom") + '"></label>' +
        '<label class="field"><span>模型（多个用逗号分隔）</span><div class="token-row">' +
          '<input name="model" list="model-options" value="' + attr(modelValues) + '" placeholder="例如 gpt-4o-mini，多个用逗号分隔">' +
          '<button class="btn" data-action="ai-discover-models" type="button">获取模型</button>' +
        "</div></label>" +
        field("Base URL（可选）", "base_url", effective.base_url || "", "text", 'placeholder="留空使用服务商默认地址"') +
        '<label class="field"><span>API Key</span><div class="token-row">' +
          '<input name="api_key" type="password" value="" placeholder="' + (effective.api_key ? "已配置，留空表示不修改" : "粘贴 API Key") + '" autocomplete="off">' +
          '<button class="btn" data-action="toggle-key" type="button">显示</button>' +
        "</div></label>" +
        field("每分钟请求上限", "requests_per_minute", (prof && prof.requests_per_minute) || 10, "number", 'min="1" max="600"') +
        field("优质阈值（1–10 整数）", "quality_threshold", ai.quality_threshold == null ? 7 : ai.quality_threshold, "number", 'step="1" min="1" max="10"') +
        '<label class="field"><span>多模型策略</span><select name="strategy">' +
          [["single", "单模型"], ["round-robin", "轮询"], ["failover", "故障转移"]].map(function (x) {
            return '<option value="' + x[0] + '"' + ((ai.strategy || "single") === x[0] ? " selected" : "") + ">" + x[1] + "</option>";
          }).join("") + "</select></label>" +
      "</div>" +
      '<div id="discovered-models" class="discovered-list">' + discoveredHTML + "</div>" +
      '<datalist id="model-options">' + suggestions.map(function (m) {
        return '<option value="' + attr(m) + '"></option>';
      }).join("") + "</datalist>" +
      '<p class="hint">保存时会把改动写回选中的配置档并设为当前使用；阈值达到后会进入「AI 优质」与日报素材。' +
      "API Key 存在本地数据库，界面不回显。多模型策略需要配置档里有多个模型才会生效。</p>"
    );

    html += settingSection("03", "提示词", "决定单条内容分析与日报生成的输出风格，留空则使用内置提示词",
      '<label class="field block"><span>系统提示词（内容分析）</span>' +
        '<textarea name="system_prompt" rows="5">' + esc(ai.system_prompt || "") + "</textarea></label>" +
      '<p class="hint">可用变量：{{.Title}} {{.Summary}} {{.Content}}</p>' +
      '<label class="field block" style="margin-top:16px"><span>日报提示词</span>' +
        '<textarea name="daily_prompt" rows="5">' + esc(ai.daily_prompt || "") + "</textarea></label>" +
      '<p class="hint">可用变量：{{.Count}} {{.FeedText}} {{.Date}}</p>'
    );

    html += settingSection("04", "日报定时", "到点自动生成早报 / 晚报；定时改动需重启 APP 后才生效",
      '<div class="form-grid">' +
        switchField("启用早报", "morning_report_enabled", !!ai.morning_report_enabled) +
        field("早报时间", "morning_report_time", ai.morning_report_time || "08:00", "time") +
        switchField("启用晚报", "evening_report_enabled", !!ai.evening_report_enabled) +
        field("晚报时间", "evening_report_time", ai.evening_report_time || "20:00", "time") +
      "</div>" +
      '<div class="form-actions"><button class="btn primary" type="submit">保存 AI 设置</button>' +
      '<button class="btn" data-action="ai-test" type="button">测试模型连通性</button>' +
      '<span class="toolbar-note">' + (ai.enabled ? "当前已启用" : "当前未启用") +
        " · 累计已分析 " + esc(st.total_processed == null ? 0 : st.total_processed) + " 条</span></div>"
    );

    html += "</form>";

    /* 05 API 与安全 */
    html += '<form data-form="desktop-settings">';
    html += settingSection("05", "API 与安全", "默认只监听本机且不需要 Token；对外提供 API 时才需要设置 Token",
      '<div class="setting-switch" style="margin-top:0">' + switchField("启用 API Token", "token_enabled", tokenOn) + "</div>" +
      '<label class="field block" style="margin-top:14px"><span>Token 值</span><div class="token-row">' +
        '<input name="api_token" type="password" value="' + attr(token) + '" placeholder="留空表示不启用" autocomplete="off"' + (tokenOn ? "" : " disabled") + ">" +
        '<button class="btn" data-action="toggle-token" type="button">显示</button>' +
        '<button class="btn" data-action="generate-token" type="button">生成</button>' +
      "</div></label>" +
      '<div class="setting-switch">' + switchField("允许局域网访问（需重启 APP）", "external_enabled", !!s.external_enabled) + "</div>" +
      '<p class="hint">本机 loopback 请求始终免密，所以开启后 APP 自身、CLI 与扩展都不受影响；' +
      "远端 HTTP 需带 X-API-Key，远端 WebSocket 需带 ?token=。Token 保存后立即生效，监听范围要重启 APP 才改变。" +
      (s.env_forced_external ? "当前监听范围由启动时的 GRABBY_DESKTOP_HOST 指定。" : "") + "</p>" +
      '<div class="form-actions"><button class="btn primary" type="submit">保存服务设置</button>' +
      '<span class="toolbar-note">保存后 APP 内操作不会被自己设置的 Token 锁住</span></div>'
    );
    html += "</form>";

    /* 06 数据与文件 */
    html += settingSection("06", "数据与文件", "订阅源、内容库与设置文件都保存在同一个数据目录里",
      settingRow("数据目录", '<span class="mono">' + esc(s.data_dir || "—") + "</span>") +
      settingRow("数据库", '<span class="mono">grabby.db</span>', "与 Web 版共用同一个库时内容天然一致") +
      settingRow("APP 设置", '<span class="mono">desktop-settings.json</span>', "包含 API Token，权限 0600") +
      settingRow("配置文件来源", '<span class="mono">.env</span>', "APP 不读取其中的 GRABBY_ADMIN_KEY / GRABBY_API_TOKEN") +
      '<p class="hint">想换一个库，可在启动前设置 GRABBY_DESKTOP_DATA_DIR。Web 版与命令行的配置不受这里影响。</p>' +
      '<div class="form-actions" style="margin-top:16px">' +
        '<button class="btn primary" data-action="config-export" type="button">导出配置备份</button>' +
        '<button class="btn" data-action="config-import" type="button">导入配置备份</button>' +
        '<span class="toolbar-note">包含 AI 设置与数据源，不含内容库与抓取日志</span>' +
      "</div>" +
      '<input type="file" id="config-import-file" accept="application/json,.json" hidden>' +
      '<p class="hint">备份文件包含明文 API Key，请妥善保管。导入时 AI 配置整体覆盖并立即生效；' +
      "数据源按 ID 新增或更新，不会删除本地已有数据源。</p>" +
      '<div style="margin-top:16px"><div class="preset-label">调用示例</div>' +
      '<pre class="code-block">' + esc(curl) + "</pre>" +
      '<div style="margin-top:10px"><button class="copy-btn" data-copy="' + attr(curl) + '" type="button">复制命令</button></div></div>'
    );

    return html;
  }

  function syncViewChrome() {
    var isLibrary = state.view === "library";
    $("search-wrap").hidden = !isLibrary;
    $("search-input").placeholder = isLibrary ? "搜索标题、摘要、来源" : "搜索内容";
    var showLayout = state.view === "today" || isLibrary;
    $("view-switch").hidden = !showLayout;
    var btns = document.querySelectorAll(".view-switch-btn");
    for (var i = 0; i < btns.length; i++) {
      var on = btns[i].dataset.layout === state.layout;
      btns[i].classList.toggle("is-active", on);
      btns[i].setAttribute("aria-pressed", on ? "true" : "false");
    }
  }

  function render(options) {
    syncViewChrome();
    syncTopbarHead();
    var nav = document.querySelectorAll(".nav-item");
    for (var i = 0; i < nav.length; i++) nav[i].classList.toggle("is-active", nav[i].dataset.view === state.view);
    var keepScroll = !!(options && options.keepScroll);
    var scrollTop = keepScroll ? $("content").scrollTop : 0;
    var html;
    try {
      var fn = {
        library: renderLibrary, sources: renderSources, news: renderNews, ai: renderAi,
        daily: renderDaily, devices: renderDevices, apihub: renderApiHub,
        settings: renderSettings
      }[state.view];
      html = fn ? fn() : renderToday();
    } catch (e) {
      html = '<div class="empty"><div class="empty-title">渲染失败</div>' + esc(e.message) + "</div>";
    }
    $("content").innerHTML = html;
    $("content").scrollTop = scrollTop;
  }

  function setLayout(layout) {
    if (layout !== "grid" && layout !== "list") return;
    if (state.layout === layout) return;
    state.layout = layout;
    try { localStorage.setItem("grabby_layout", layout); } catch (e) { /* ignore */ }
    syncViewChrome();
    render({ keepScroll: true });
  }

  function syncStatus() {
    var h = state.health || {};
    if (h.status === "ok") {
      $("status-dot").className = "status-dot " + (h.browser_connected ? "ok" : "");
      $("service-status").textContent = h.browser_connected ? "扩展已连接" : "服务运行中";
      $("service-sub").textContent = "API " + location.host + (h.browser_count ? " · " + h.browser_count + " 台设备" : "");
    } else {
      $("status-dot").className = "status-dot off";
      $("service-status").textContent = "服务未就绪";
      $("service-sub").textContent = state.error || "正在连接本地服务…";
    }
  }

  /* ---------------- Dialogs ---------------- */

  function sourceForm(s) {
    var edit = !!s;
    s = s || {};
    var body = '<form data-form="source" data-edit="' + (edit ? "1" : "0") + '">' +
      '<div class="form-grid">' +
        field("ID" + (edit ? "（不可修改）" : "（唯一标识）"), "id", s.id || "", "text", edit ? "readonly" : 'placeholder="例如 tech-rss"') +
        field("名称", "name", s.name || "", "text", 'placeholder="例如 科技新闻"') +
        '<label class="field"><span>类型</span><select name="type">' +
          ["rss", "api", "web_scrape"].map(function (t) {
            return '<option value="' + t + '"' + ((s.type || "rss") === t ? " selected" : "") + ">" + t + "</option>";
          }).join("") + "</select></label>" +
        field("分组", "category", s.category || "", "text", 'placeholder="例如 AI / 科技新闻"') +
        field("URL", "url", s.url || "", "text", 'placeholder="https://example.com/feed.xml"') +
        field("抓取计划（cron）", "schedule", s.schedule || "0 */2 * * *") +
        '<label class="field"><span>默认内容类型</span><select name="default_category">' +
          ["auto", "article", "tweet", "paper", "project"].map(function (t) {
            return '<option value="' + t + '"' + ((s.default_category || "auto") === t ? " selected" : "") + ">" + t + "</option>";
          }).join("") + "</select></label>" +
        '<label class="field"><span>状态</span><select name="enabled">' +
          '<option value="1"' + (!edit || s.enabled == 1 ? " selected" : "") + ">启用</option>" +
          '<option value="0"' + (edit && s.enabled != 1 ? " selected" : "") + ">停用</option></select></label>" +
      "</div>" +
      '<label class="field block"><span>高级配置（JSON，可留空）</span><textarea name="config" rows="3" placeholder="{}">' + esc(s.config || "") + "</textarea></label>" +
      '<div class="form-actions"><button class="btn primary" type="submit">保存</button>' +
      '<button class="btn" data-action="dialog-close" type="button">取消</button></div></form>';
    openDialog(edit ? "编辑订阅源" : "添加数据源", body, "", true);
  }

  function safeHtml(html) {
    var doc = new DOMParser().parseFromString(String(html || ""), "text/html");
    var blocked = doc.querySelectorAll("script,style,iframe,object,embed,link,meta,form,input,button,svg,math,video,audio");
    for (var i = 0; i < blocked.length; i++) {
      if (blocked[i].parentNode) blocked[i].parentNode.removeChild(blocked[i]);
    }
    var all = doc.body.querySelectorAll("*");
    for (var j = 0; j < all.length; j++) {
      var el = all[j];
      var attrs = Array.prototype.slice.call(el.attributes);
      attrs.forEach(function (a) {
        var name = a.name.toLowerCase(), val = a.value.trim().toLowerCase();
        if (name.indexOf("on") === 0 || name === "style" ||
            ((name === "href" || name === "src") && (val.indexOf("javascript:") === 0 || val.indexOf("data:") === 0))) {
          el.removeAttribute(a.name);
        }
      });
      if (el.tagName === "A") { el.setAttribute("target", "_blank"); el.setAttribute("rel", "noopener noreferrer"); }
    }
    return doc.body.innerHTML;
  }

  /* ---------------- In-app reader ---------------- */

  function findItemById(id) {
    var key = String(id);
    var i;
    for (i = 0; i < state.items.length; i++) if (String(state.items[i].id) === key) return state.items[i];
    for (i = 0; i < state.aiItems.length; i++) if (String(state.aiItems[i].id) === key) return state.aiItems[i];
    if (state.reader.item && String(state.reader.item.id) === key) return state.reader.item;
    return null;
  }

  function readerList() {
    var key = String(state.reader.id || "");
    var i;
    for (i = 0; i < state.items.length; i++) if (String(state.items[i].id) === key) return state.items;
    for (i = 0; i < state.aiItems.length; i++) if (String(state.aiItems[i].id) === key) return state.aiItems;
    return state.reader.item ? [state.reader.item] : [];
  }

  function plainText(html) {
    var raw = String(html == null ? "" : html);
    if (!raw) return "";
    try {
      return new DOMParser().parseFromString(raw, "text/html").body.textContent || "";
    } catch (e) {
      return raw.replace(/<[^>]*>/g, " ");
    }
  }

  function richText(value) {
    var s = String(value == null ? "" : value).trim();
    if (!s) return "";
    if (/<[a-z][\s\S]*>/i.test(s)) return safeHtml(s);
    return "<p>" + esc(s) + "</p>";
  }

  function readingMinutes(html) {
    var text = plainText(html).replace(/&[a-z#0-9]+;/gi, " ");
    var cjk = (text.match(/[㐀-鿿぀-ヿ가-힯]/g) || []).length;
    var words = text.replace(/[㐀-鿿぀-ヿ가-힯]/g, " ").trim().split(/\s+/).filter(Boolean).length;
    return Math.max(1, Math.round(cjk / 430 + words / 220));
  }

  function fmtReaderDate(value) {
    if (!value) return "时间未知";
    var d = new Date(value);
    if (isNaN(d.getTime())) return String(value).slice(0, 16);
    return d.toLocaleString("zh-CN", { year: "numeric", month: "long", day: "numeric", hour: "2-digit", minute: "2-digit" });
  }

  function readerSkeleton(item) {
    var header = '<div class="sk-line sm"></div><div class="sk-line xl"></div><div class="sk-line lg"></div>';
    if (item && item.title) {
      var v = itemVisual(item);
      header = '<div class="reader-kicker">' +
        '<span class="source-mark" style="background:' + v.cover + '">' + esc(v.letter) + "</span>" +
        esc(v.label) + "</div>" +
        '<h1 class="reader-title">' + esc(item.title) + "</h1>" +
        '<div class="reader-meta"><span>' + esc(fmtReaderDate(item.published_at || item.fetched_at)) + "</span></div>";
    }
    return '<div class="reader-skeleton">' + header +
      '<div class="sk-block"></div>' +
      '<div class="sk-line w90"></div><div class="sk-line w80"></div><div class="sk-line w70"></div>' +
      '<div class="sk-line w90"></div><div class="sk-line w60"></div>' +
      '<div class="sk-block" style="height:64px"></div>' +
      '<div class="sk-line w80"></div><div class="sk-line w70"></div></div>';
  }

  function readerHeaderHTML(it, an, score) {
    var v = itemVisual(it);
    var meta = "";
    var time = it.published_at || it.fetched_at;
    if (time) meta += "<span>" + esc(fmtReaderDate(time)) + "</span>";
    if (score != null) {
      meta += (meta ? '<span class="dot"></span>' : "") + "<span>AI 评分 " + esc(score.toFixed(1)) + "</span>";
    }
    var chips = "";
    if (it.category) chips += '<span class="chip-cat">' + esc(it.category) + "</span>";
    if (it.ai_category) {
      chips += '<span class="chip-cat">' + esc(it.ai_category) +
        (it.ai_subcategory ? " · " + esc(it.ai_subcategory) : "") + "</span>";
    }
    return '<header class="reader-header">' +
      '<div class="reader-kicker">' +
        '<span class="source-mark" style="background:' + v.cover + '">' + esc(v.letter) + "</span>" +
        '<span>' + esc(v.label) + "</span>" + chips +
      "</div>" +
      '<h1 class="reader-title">' + esc(it.title || "未命名内容") + "</h1>" +
      '<div class="reader-meta">' + meta +
        (meta ? '<span class="dot"></span>' : "") +
        "<span>约 " + esc(readingMinutes(it._html || it.content || it.summary)) + " 分钟阅读</span>" +
      "</div></header>";
  }

  function readerAnalysisHTML(it, an, score) {
    if (!an || (an.quality_score == null && !an.ai_summary && !an.ai_comment)) return "";
    var tags = String(an.ai_tags || "").split(/[,，]/).map(function (t) { return t.trim(); }).filter(Boolean);
    return '<section class="reader-ai">' +
      '<div class="reader-ai-head">' +
        '<div class="reader-ai-label">' +
          '<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M10 2.6l1.7 4.3 4.5.4-3.4 2.9 1 4.4-3.8-2.3-3.8 2.3 1-4.4L3.8 7.3l4.5-.4z"/></svg>' +
          "AI 精读</div>" +
        (score != null ? '<div class="reader-score" style="--score:' + esc(score) + '"><b>' + esc(score.toFixed(1)) + "</b></div>" : "") +
      "</div>" +
      (an.ai_summary ? '<p class="reader-ai-summary">' + esc(an.ai_summary) + "</p>" : "") +
      (an.ai_comment ? '<p class="reader-ai-comment">' + esc(an.ai_comment) + "</p>" : "") +
      (tags.length ? '<div class="reader-tags">' + tags.map(function (t) {
        return '<span class="reader-tag">' + esc(t) + "</span>";
      }).join("") + "</div>" : "") +
      (an.model_used || an.processed_at
        ? '<div class="reader-ai-foot">' + (an.model_used ? "分析模型 " + esc(an.model_used) : "") +
          (an.processed_at ? (an.model_used ? " · " : "") + "分析于 " + esc(fmtTime(an.processed_at)) : "") + "</div>"
        : "") +
      "</section>";
  }

  function readerHTML(it, an) {
    var scoreRaw = an && an.quality_score != null ? Number(an.quality_score)
      : (it.quality_score != null ? Number(it.quality_score) : null);
    var score = scoreRaw != null && !isNaN(scoreRaw) ? scoreRaw : null;
    var v = itemVisual(it);
    var contentHTML = safeHtml(it._html || "");
    var contentText = plainText(contentHTML).replace(/\s+/g, " ").trim();
    var summaryHTML = richText(it._summaryHtml || it.summary || "");
    var summaryText = plainText(summaryHTML).replace(/\s+/g, " ").trim();
    var showSummary = !!summaryText && !(an && an.ai_summary);

    var body;
    if (contentText && summaryText && contentText === summaryText) {
      body = '<div class="reader-content">' + contentHTML + "</div>";
      summaryHTML = "";
      showSummary = false;
    } else if (contentText) {
      body = '<div class="reader-content">' + contentHTML + "</div>";
    } else {
      body = '<div class="reader-fallback"><strong>这篇内容还没有抓取到正文</strong>' +
        "Grabby 保存了摘要和来源链接。你可以在浏览器中打开原始页面查看完整内容。" +
        (it.url ? '<div style="margin-top:9px"><a href="' + attr(it.url) + '" target="_blank" rel="noopener">在浏览器打开原文</a></div>' : "") +
        "</div>";
    }

    return '<article class="reader-article">' +
      readerHeaderHTML(it, an, score) +
      readerAnalysisHTML(it, an, score) +
      (showSummary ? '<div class="reader-summary">' + summaryHTML + "</div>" : "") +
      (contentText ? '<div class="reader-divider">正文</div>' : "") +
      body +
      '<footer class="reader-footer">' +
        '<span class="reader-footer-source">来源 ' + esc(v.label) +
          (it.fetched_at ? " · 收录于 " + esc(fmtTime(it.fetched_at)) : "") + "</span>" +
        (it.url
          ? '<a href="' + attr(it.url) + '" target="_blank" rel="noopener">在浏览器打开' +
            '<svg viewBox="0 0 20 20" aria-hidden="true"><path d="M8 4H4.5A1.5 1.5 0 0 0 3 5.5v10A1.5 1.5 0 0 0 4.5 17h10a1.5 1.5 0 0 0 1.5-1.5V12M11 3h6v6M17 3l-8 8"/></svg></a>'
          : "") +
      "</footer></article>";
  }

  function updateReaderStar() {
    var btn = $("reader-star");
    var it = state.reader.item;
    var on = it && Number(it.starred) === 1;
    btn.setAttribute("aria-pressed", on ? "true" : "false");
    btn.setAttribute("aria-label", on ? "取消收藏" : "收藏");
    btn.title = on ? "取消收藏" : "收藏";
  }

  function updateReaderNav() {
    var list = readerList();
    var idx = -1;
    for (var i = 0; i < list.length; i++) if (String(list[i].id) === String(state.reader.id)) { idx = i; break; }
    $("reader-prev").disabled = idx <= 0;
    $("reader-next").disabled = idx < 0 || idx >= list.length - 1;
  }

  function updateReaderProgress() {
    var scroll = $("reader-scroll");
    var max = scroll.scrollHeight - scroll.clientHeight;
    var pct = max > 0 ? Math.min(100, Math.max(0, (scroll.scrollTop / max) * 100)) : 0;
    $("reader-progress-bar").style.width = pct.toFixed(2) + "%";
    $("reader-bar").classList.toggle("is-scrolled", scroll.scrollTop > 4);
  }

  function markActiveInFeed(id) {
    var nodes = $("content").querySelectorAll('[data-action="detail"]');
    for (var i = 0; i < nodes.length; i++) {
      var on = String(nodes[i].dataset.id) === String(id);
      nodes[i].classList.toggle("is-active", on);
      if (on) nodes[i].setAttribute("aria-current", "true");
      else nodes[i].removeAttribute("aria-current");
    }
  }

  function updateFeedItemState(id) {
    var it = findItemById(id);
    if (!it) return;
    var read = Number(it.read_status) === 1;
    var nodes = $("content").querySelectorAll('[data-action="detail"]');
    for (var i = 0; i < nodes.length; i++) {
      if (String(nodes[i].dataset.id) !== String(id)) continue;
      nodes[i].classList.toggle("is-read", read);
      if (read) {
        var dot = nodes[i].querySelector(".unread-dot");
        if (dot && dot.parentNode) dot.parentNode.removeChild(dot);
      }
    }
  }

  function markReaderItemRead(it) {
    if (!it || Number(it.read_status) === 1) return;
    it.read_status = 1;
    var local = findItemById(it.id);
    if (local && local !== it) local.read_status = 1;
    if (state.stats && state.stats.unread_count > 0) state.stats.unread_count -= 1;
    updateFeedItemState(it.id);
    post("/api/items/" + encodeURIComponent(it.id) + "/read", { read_status: 1 }).catch(function () { /* non-critical */ });
  }

  function renderReader() {
    var it = state.reader.item;
    if (!it) return;
    var an = state.reader.analysis || null;
    $("reader-kicker").textContent = it.origin_source || it.source_category || "阅读";
    $("reader-body").innerHTML = readerHTML(it, an);
    $("reader-scroll").scrollTop = 0;
    updateReaderStar();
    updateReaderNav();
    updateReaderProgress();
  }

  function renderReaderError(message) {
    $("reader-body").innerHTML = '<div class="reader-error"><h3>无法载入这篇内容</h3>' +
      "<p>" + esc(message || "服务暂时没有响应，请稍后重试。") + "</p>" +
      '<button class="btn primary" data-action="reader-retry" data-id="' + attr(state.reader.id) + '" type="button">重试</button></div>';
    updateReaderNav();
  }

  async function openReader(id, trigger) {
    if (id == null) return;
    var key = String(id);
    if (trigger) state.readerLastFocus = trigger;
    else if (!state.reader.open) state.readerLastFocus = document.activeElement;

    var known = findItemById(key);
    state.reader.id = key;
    state.reader.item = known || null;
    state.reader.analysis = null;
    state.reader.loading = true;
    state.reader.error = "";
    state.reader.open = true;
    var seq = ++state.readerSeq;

    $("workspace").classList.add("is-reading");
    $("reader").setAttribute("aria-hidden", "false");
    $("reader-progress-bar").style.width = "0";
    $("reader-kicker").textContent = known ? (known.origin_source || known.source_category || "阅读") : "阅读";
    $("reader-body").innerHTML = readerSkeleton(known);
    updateReaderStar();
    updateReaderNav();
    markActiveInFeed(key);
    try { $("reader").focus({ preventScroll: true }); } catch (e) { $("reader").focus(); }

    try {
      var d = await fetchAll({
        item: api("/open/api/items/" + encodeURIComponent(key)),
        analysis: api("/open/api/ai/analysis/" + encodeURIComponent(key))
      });
      if (seq !== state.readerSeq) return;
      var it = (d.item && d.item.item) || null;
      if (!it) throw new Error("这个条目可能已被删除。");
      it._html = (d.item && d.item.html_content) || "";
      it._summaryHtml = (d.item && d.item.summary_html) || "";
      state.reader.item = it;
      state.reader.analysis = (d.analysis && d.analysis.analysis) || null;
      state.reader.loading = false;
      var local = findItemById(it.id);
      if (local && local !== it) {
        local.starred = it.starred;
        local.read_status = it.read_status;
      }
      renderReader();
      markReaderItemRead(it);
    } catch (err) {
      if (seq !== state.readerSeq) return;
      state.reader.loading = false;
      state.reader.error = (err && err.message) || "无法读取该条目";
      renderReaderError(state.reader.error);
    }
  }

  function navigateReader(delta) {
    var list = readerList();
    if (!list.length) return;
    var idx = -1;
    for (var i = 0; i < list.length; i++) if (String(list[i].id) === String(state.reader.id)) { idx = i; break; }
    var next = idx + delta;
    if (next < 0 || next >= list.length) return;
    openReader(list[next].id);
  }

  function closeReader(options) {
    options = options || {};
    if (!state.reader.open) return;
    var lastId = state.reader.id;
    var lastFocus = state.readerLastFocus;
    state.reader.open = false;
    state.reader.id = null;
    state.reader.item = null;
    state.reader.analysis = null;
    state.reader.error = "";
    state.reader.loading = false;
    state.readerLastFocus = null;
    state.readerSeq++;

    $("workspace").classList.remove("is-reading");
    $("reader").setAttribute("aria-hidden", "true");
    var active = $("content").querySelectorAll('[aria-current="true"]');
    for (var i = 0; i < active.length; i++) {
      active[i].classList.remove("is-active");
      active[i].removeAttribute("aria-current");
    }

    if (options.restoreFocus) {
      var target = lastFocus && document.contains(lastFocus) ? lastFocus : null;
      if (!target && lastId != null) {
        target = $("content").querySelector('[data-action="detail"][data-id="' + lastId + '"]');
      }
      if (target) {
        try { target.focus({ preventScroll: true }); } catch (e) { target.focus(); }
      }
    }

    // Keep the article visible during the closing animation, then release it.
    window.setTimeout(function () {
      if (!state.reader.open) {
        $("reader-body").innerHTML = "";
        $("reader-progress-bar").style.width = "0";
        $("reader-kicker").textContent = "阅读";
      }
    }, 620);
  }

  $("reader-close").addEventListener("click", function () { closeReader({ restoreFocus: true }); });
  $("reader-scrim").addEventListener("click", function () { closeReader({ restoreFocus: true }); });
  $("reader-prev").addEventListener("click", function () { navigateReader(-1); });
  $("reader-next").addEventListener("click", function () { navigateReader(1); });
  $("reader-scroll").addEventListener("scroll", updateReaderProgress, { passive: true });
  $("reader-star").addEventListener("click", async function () {
    var it = state.reader.item;
    if (!it) return;
    var next = Number(it.starred) === 1 ? 0 : 1;
    it.starred = next;
    var local = findItemById(it.id);
    if (local && local !== it) local.starred = next;
    updateReaderStar();
    try {
      var res = await post("/api/items/" + encodeURIComponent(it.id) + "/star", { starred: next });
      if (res && res.error && res.error !== "") throw new Error(res.error);
      toast(next ? "已收藏" : "已取消收藏");
    } catch (err) {
      it.starred = next ? 0 : 1;
      if (local && local !== it) local.starred = it.starred;
      updateReaderStar();
      toast("操作失败，请重试", true);
    }
  });

  async function openLogs(id) {
    var d = await api("/api/logs?source_id=" + encodeURIComponent(id));
    var logs = Array.isArray(d) ? d : (d.logs || []);
    var body = logs.length
      ? '<div class="log-list">' + logs.slice(0, 30).map(function (l) {
          return '<div class="row"><div><div class="row-value">' + esc(l.status || "-") + "</div>" +
            '<div class="row-label">' + esc(fmtTime(l.started_at)) + " · 发现 " + esc(l.items_found || 0) +
            " · 入库 " + esc(l.items_added || 0) + "</div>" +
            (l.error_message ? '<div class="log-error">' + esc(l.error_message) + "</div>" : "") +
            "</div>" + badge(l.status === "success" ? "成功" : (l.status === "error" ? "失败" : l.status || ""), l.status === "success") + "</div>";
        }).join("") + "</div>"
      : empty("暂无抓取日志", "执行一次抓取后会记录在这里。");
    openDialog("抓取日志 · " + id, body, "", true);
  }

  function confirmDialog(title, message, action, data) {
    openDialog(title, "<p>" + esc(message) + "</p>",
      '<button class="btn danger-btn" data-action="' + action + '" data-id="' + attr(data) + '" type="button">确认删除</button>' +
      '<button class="btn" data-action="dialog-close" type="button">取消</button>');
  }

  // Read a local backup file and show a preview dialog before anything is
  // written. Validation mirrors the server so obvious mistakes never round-trip.
  async function importConfigBackup(file) {
    if (file.size > 16 * 1024 * 1024) { toast("备份文件过大（上限 16 MB）", true); return; }
    var text;
    try { text = await file.text(); } catch (e) { toast("读取文件失败，请重新选择", true); return; }
    var doc;
    try { doc = JSON.parse(text); } catch (e) { toast("无法解析该文件，请确认是 Grabby 导出的备份", true); return; }
    if (!doc || typeof doc !== "object" || Array.isArray(doc)) { toast("无法解析该文件", true); return; }
    if (doc.app && doc.app !== "grabby") { toast("这不是 Grabby 的配置文件", true); return; }
    if (typeof doc.version === "number" && doc.version > 1) { toast("备份来自更新版本，请先升级 Grabby", true); return; }
    var hasAI = !!doc.ai && typeof doc.ai === "object";
    var sourceCount = Array.isArray(doc.sources) ? doc.sources.length : 0;
    if (!hasAI && !sourceCount) { toast("备份中没有可导入的 AI 配置或数据源", true); return; }

    var profileCount = hasAI && Array.isArray(doc.ai.profiles) ? doc.ai.profiles.length : 0;
    var exportedAt = doc.exported_at ? fmtTime(doc.exported_at) : "未知";
    var summary =
      "<p><b>文件：</b>" + esc(file.name) + "<br>" +
      "<b>导出时间：</b>" + esc(exportedAt) + "<br>" +
      "<b>AI 配置：</b>" + (hasAI ? esc(profileCount ? profileCount + " 个配置档案" : "有（含提示词与日报定时）") : "无") + "<br>" +
      "<b>数据源：</b>" + (sourceCount ? esc(sourceCount + " 个") : "无") + "</p>" +
      "<p class=\"hint\">导入会覆盖当前 AI 配置并立即生效；数据源按 ID 新增或更新，不影响本地已有数据源与已抓取内容。备份文件包含明文 API Key。</p>";
    state.pendingImport = doc;
    openDialog("导入配置备份", summary,
      '<button class="btn primary" data-action="config-import-confirm" type="button">确认导入</button>' +
      '<button class="btn" data-action="config-import-cancel" type="button">取消</button>', true);
  }

  document.addEventListener("change", function (e) {
    var input = e.target;
    if (!input || input.id !== "config-import-file") return;
    var file = input.files && input.files[0];
    input.value = "";
    if (file) importConfigBackup(file);
  });

  /* ---------------- Actions ---------------- */

  function goto(view) {
    closeReader({ restoreFocus: false });
    state.view = view;
    render();
    loadView(view);
  }

  function sourceById(id) {
    for (var i = 0; i < state.sources.length; i++) if (state.sources[i].id === id) return state.sources[i];
    return null;
  }

  document.getElementById("nav").addEventListener("click", function (e) {
    var btn = e.target.closest(".nav-item");
    if (btn) goto(btn.dataset.view);
  });

  $("refresh-btn").addEventListener("click", function () { loadView(state.view); });

  var searchTimer = null;
  $("search-input").addEventListener("input", function (e) {
    state.query = e.target.value;
    clearTimeout(searchTimer);
    searchTimer = setTimeout(function () {
      if (state.view === "library") reloadItems();
    }, 300);
  });

  document.addEventListener("input", function (e) {
    if (e.target.id !== "discovered-search") return;
    var term = String(e.target.value || "").toLowerCase();
    var items = document.querySelectorAll("#discovered-models .discovered-item");
    for (var i = 0; i < items.length; i++) {
      items[i].style.display = (items[i].dataset.search || "").indexOf(term) >= 0 ? "" : "none";
    }
  });

  $("dialog-modal").addEventListener("click", function (e) {
    if (e.target === $("dialog-modal")) closeDialog();
  });

  document.addEventListener("submit", async function (e) {
    var form = e.target.closest("form[data-form]");
    if (!form) return;
    e.preventDefault();
    var kind = form.dataset.form;
    var fd = new FormData(form);
    if (state.busy) return;
    state.busy = true;
    try {
      if (kind === "source") {
        var payload = {
          id: String(fd.get("id") || "").trim(),
          name: String(fd.get("name") || "").trim(),
          type: String(fd.get("type") || "rss"),
          url: String(fd.get("url") || "").trim(),
          schedule: String(fd.get("schedule") || "0 */2 * * *").trim(),
          category: String(fd.get("category") || "").trim(),
          default_category: String(fd.get("default_category") || "auto"),
          config: String(fd.get("config") || "").trim() || "{}"
        };
        if (!payload.id || !payload.name || !payload.url) { toast("ID、名称和 URL 为必填项", true); return; }
        var r = form.dataset.edit === "1" ? await put("/api/sources/" + encodeURIComponent(payload.id), payload)
                                         : await post("/api/sources", payload);
        if (r && r.error) { toast(r.error, true); return; }
        var wanted = fd.get("enabled") === "0" ? 0 : 1;
        var existing = form.dataset.edit === "1" ? sourceById(payload.id) : null;
        var current = existing ? (existing.enabled == 1 ? 1 : 0) : 1;
        if (wanted !== current) {
          await post("/api/sources/" + encodeURIComponent(payload.id) + "/toggle", { enabled: wanted });
        }
        closeDialog();
        toast("订阅源已保存");
        await loadView("sources");
      } else if (kind === "ai-settings") {
        var next = Object.assign({}, state.aiSettings || {});
        var provider = String(fd.get("provider") || "").trim();
        var presetID = String(fd.get("provider_preset") || "").trim();
        var chosenPreset = presetById(presetID);
        var chosenName = chosenPreset ? chosenPreset.name : providerLabel(provider);
        var rawModelInput = String(fd.get("model") || "").trim();
        var baseURL = String(fd.get("base_url") || "").trim();
        var key = String(fd.get("api_key") || "").trim();
        var rpm = Math.max(1, Math.round(Number(fd.get("requests_per_minute") || 10)));
        if (!rawModelInput) { toast("请填写模型 ID", true); return; }
        var seenModels = {};
        var models = rawModelInput.split(/[,，\n]/).map(function (m) { return m.trim(); }).filter(function (m) {
          if (!m) return false;
          var ref = modelRefFor(provider, m);
          var lower = ref.toLowerCase();
          if (seenModels[lower]) return false;
          seenModels[lower] = 1;
          return true;
        }).map(function (m) { return modelRefFor(provider, m); });
        if (!models.length) { toast("请填写模型 ID", true); return; }
        var model = models[0];

        next.enabled = fd.get("enabled") === "on";
        next.quality_threshold = Math.round(Number(fd.get("quality_threshold") || 7));
        next.strategy = String(fd.get("strategy") || "single");
        next.system_prompt = String(fd.get("system_prompt") || "");
        next.daily_prompt = String(fd.get("daily_prompt") || "");
        next.morning_report_enabled = fd.get("morning_report_enabled") === "on";
        next.evening_report_enabled = fd.get("evening_report_enabled") === "on";
        next.morning_report_time = String(fd.get("morning_report_time") || "08:30");
        next.evening_report_time = String(fd.get("evening_report_time") || "22:30");

        // The backend mirrors the active profile onto the top-level provider /
        // model / base_url / api_key fields, so edits must land in a profile or
        // they would be silently reverted.
        var list = (next.profiles || []).slice();
        var targetID = String(fd.get("profile_id") || "");
        var target = null;
        if (targetID && targetID !== "__new") {
          for (var pi = 0; pi < list.length; pi++) if (list[pi].id === targetID) target = list[pi];
        }
        if (!target) {
          var n = list.length + 1;
          target = { id: "profile-" + n, name: chosenName, priority: n, requests_per_minute: rpm };
          while (list.some(function (x) { return x.id === target.id; })) {
            n += 1;
            target.id = "profile-" + n;
            target.priority = n;
          }
          list.push(target);
        }
        // The text input now carries the complete model list (comma-separated),
        // so save it as-is instead of trying to merge it with stale extras.
        target.provider = provider;
        target.model = model;
        target.models = models;
        target.base_url = baseURL;
        target.requests_per_minute = rpm;
        if (key) target.api_key = key;
        if (!target.name) target.name = chosenName;

        next.profiles = list;
        next.active_profile_id = target.id;
        next.provider = provider;
        next.model = model;
        next.models = models;
        next.base_url = baseURL;
        if (key) next.api_key = key;
        state.activeProfileID = target.id;

        var sr = await post("/api/ai/settings", next);
        if (sr && sr.error) { toast(sr.error, true); return; }
        toast(next.enabled ? "AI 设置已保存" : "AI 设置已保存（分析当前为停用）");
        await loadView(state.view);
      } else if (kind === "desktop-settings") {
        var tokenOn = fd.get("token_enabled") === "on";
        var newToken = tokenOn ? String(fd.get("api_token") || "").trim() : "";
        var wantExternal = fd.get("external_enabled") === "on";
        if (tokenOn && !newToken) { toast("请填写或生成一个 Token", true); return; }
        if (wantExternal && !newToken) { toast("开放局域网访问必须先设置 API Token", true); return; }
        var ds = await post("/api/desktop/settings", {
          api_token: newToken,
          external_enabled: wantExternal
        });
        if (ds && ds.error) { toast(ds.error, true); return; }
        state.desktopSettings = ds;
        state.tokenEnabled = !!ds.token_enabled;
        state.external = !!ds.active_external;
        toast(ds.restart_required ? "已保存 · 对外监听需重启 APP 生效" : "设置已保存，立即生效");
        await loadView(state.view);
      } else if (kind === "adapter") {
        state.adapter = String(fd.get("adapter") || state.adapter);
        var a = adapterById(state.adapter);
        var value = String(fd.get("value") || "").trim();
        var body = Object.assign({}, a.extra || {});
        if (a.field) {
          if (!value) { toast("请输入" + a.hint, true); return; }
          body[a.field] = value;
        }
        state.adapterBusy = true;
        render();
        var ar = await post(a.path, body);
        state.adapterResult = ar;
        state.adapterBusy = false;
        render();
        toast(ar && (ar.error || ar.detail) ? "采集失败：" + (ar.error || ar.detail) : "采集完成", !!(ar && (ar.error || ar.detail)));
        await loadView("news");
      }
    } catch (err) {
      if (err.message !== "unauthorized") toast("操作失败：" + err.message, true);
      state.adapterBusy = false;
      render();
    } finally {
      state.busy = false;
    }
  });

  document.addEventListener("change", function (e) {
    if (e.target.id === "daily-date" && e.target.value) { state.dailyDate = e.target.value; loadView("daily"); }
    if (e.target.id === "daily-type") { state.dailyType = e.target.value; loadView("daily"); }
    if (e.target.name === "adapter") { state.adapter = e.target.value; render(); }
    if (e.target.name === "profile_id") { state.activeProfileID = e.target.value; state.discoveredModels = []; render(); }
    if (e.target.name === "provider_preset") {
      var chosenProvider = presetById(e.target.value);
      if (chosenProvider) applyProviderPreset(chosenProvider);
    }
    if (e.target.name === "token_enabled") {
      var form = e.target.closest("form");
      var input = form && form.querySelector('input[name="api_token"]');
      if (input) {
        input.disabled = !e.target.checked;
        if (e.target.checked && !input.value) input.value = randomToken();
      }
    }
  });

  document.addEventListener("click", async function (e) {
    var copyBtn = e.target.closest && e.target.closest(".copy-btn");
    if (copyBtn) {
      var copyText = copyBtn.dataset.copy || "";
      var done = function () {
        var old = copyBtn.textContent;
        copyBtn.textContent = "已复制";
        setTimeout(function () { copyBtn.textContent = old; }, 1400);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(copyText).then(done, function () { fallbackCopy(copyText, done); });
      else fallbackCopy(copyText, done);
      return;
    }
    var target = e.target.closest("[data-action]");
    if (!target) return;
    var action = target.dataset.action;
    var id = target.dataset.id;
    try {
      if (action === "goto") { goto(target.dataset.view); return; }
      if (action === "dialog-close") { closeDialog(); return; }
      if (action === "jump") {
        var section = document.getElementById(target.dataset.target);
        if (section) section.scrollIntoView({ behavior: "smooth", block: "start" });
        return;
      }
      if (action === "toggle-key") {
        var keyInput = document.querySelector('#content input[name="api_key"]');
        if (keyInput) {
          if (keyInput.type === "password") {
            if (!keyInput.value && state.aiSettings && state.aiSettings.api_key) {
              keyInput.value = state.aiSettings.api_key;
            }
            keyInput.type = "text";
            target.textContent = "隐藏";
          } else {
            keyInput.type = "password";
            target.textContent = "显示";
          }
        }
        return;
      }
      if (action === "new-profile") {
        state.activeProfileID = "__new";
        render();
        var newForm = document.querySelector('form[data-form="ai-settings"]');
        if (newForm) {
          var modelField = newForm.querySelector('[name="model"]');
          if (modelField) modelField.focus();
        }
        toast("新配置档：填好服务商与模型后保存");
        return;
      }
      if (action === "provider-preset") {
        var preset = presetById(target.dataset.value);
        if (preset) {
          applyProviderPreset(preset);
          toast(preset.base_url ? "已套用 " + preset.name + " 预设，填入 API Key 后保存" : "已套用 " + preset.name + " 预设");
        }
        return;
      }
      if (action === "ai-catalog-refresh") {
        toast("正在刷新模型目录…");
        var cr = await api("/api/ai/catalog?refresh=1");
        if (cr && cr.providers && cr.providers.length) {
          state.aiCatalog = cr;
          toast("模型目录已更新（" + cr.providers.length + " 个服务商）");
          render();
        } else {
          toast(cr && cr.error ? cr.error : "刷新目录失败", true);
        }
        return;
      }
      if (action === "ai-discover-models") {
        var discoverForm = document.querySelector('form[data-form="ai-settings"]');
        if (!discoverForm) return;
        var df = new FormData(discoverForm);
        var discoverProvider = String(df.get("provider") || "");
        var discoverBase = String(df.get("base_url") || "");
        var discoverKey = String(df.get("api_key") || "").trim();
        var discoverProfile = String(df.get("profile_id") || "");
        if (discoverProfile === "__new") discoverProfile = "";
        toast("正在获取模型列表…");
        var dr = await post("/api/ai/models/discover", {
          provider: discoverProvider,
          base_url: discoverBase,
          api_key: discoverKey,
          profile_id: discoverProfile
        });
        if (dr && dr.success && dr.models && dr.models.length) {
          state.discoveredModels = dr.models;
          var modelInput = discoverForm.querySelector('[name="model"]');
          var currentModels = modelInput ? modelInput.value.split(/[,，\n]/).map(function (m) { return m.trim(); }).filter(Boolean) : [];
          var discoveredBox = $("discovered-models");
          if (discoveredBox) discoveredBox.innerHTML = renderDiscoveredModelsHTML(dr.models, currentModels);
          var modelList = $("model-options");
          if (modelList) {
            modelList.innerHTML = dr.models.map(function (m) {
              return '<option value="' + attr(m.id) + '"></option>';
            }).join("");
          }
          toast("发现 " + dr.models.length + " 个模型，勾选后点「应用所选」");
        } else {
          toast(dr && dr.error ? dr.error : "获取模型失败", true);
        }
        return;
      }
      if (action === "ai-apply-models") { applyDiscoveredSelection(); return; }
      if (action === "ai-import") {
        toast("正在扫描本机配置…");
        var ir = await post("/api/ai/import", { source: "auto", apply: true });
        if (ir && ir.success) {
          toast(ir.imported > 0 ? "已导入 " + ir.imported + " 个配置档" : "没有发现新的配置", false);
          if (ir.imported > 0) await loadView("settings");
        } else {
          toast(ir && ir.error ? ir.error : "导入失败", true);
        }
        return;
      }
      if (action === "toggle-token") {
        var tokenInput = document.querySelector('#content input[name="api_token"]');
        if (tokenInput) {
          tokenInput.type = tokenInput.type === "password" ? "text" : "password";
          target.textContent = tokenInput.type === "password" ? "显示" : "隐藏";
        }
        return;
      }
      if (action === "generate-token") {
        var genInput = document.querySelector('#content input[name="api_token"]');
        if (genInput) {
          genInput.value = randomToken();
          genInput.type = "text";
          var toggle = document.querySelector('#content [data-action="toggle-token"]');
          if (toggle) toggle.textContent = "隐藏";
        }
        return;
      }
      if (action === "config-export") {
        toast("正在导出配置…");
        var exportRes = await fetch("/api/config/export", { credentials: "same-origin" });
        if (!exportRes.ok) {
          var exportErr = await exportRes.text();
          toast("导出失败：" + (exportErr || ("HTTP " + exportRes.status)), true);
          return;
        }
        var exportBlob = await exportRes.blob();
        var disposition = exportRes.headers.get("Content-Disposition") || "";
        var nameMatch = /filename="?([^";]+)"?/.exec(disposition);
        var exportName = (nameMatch && nameMatch[1]) || "grabby-config.json";
        var exportUrl = URL.createObjectURL(exportBlob);
        var exportAnchor = document.createElement("a");
        exportAnchor.href = exportUrl;
        exportAnchor.download = exportName;
        document.body.appendChild(exportAnchor);
        exportAnchor.click();
        exportAnchor.remove();
        setTimeout(function () { URL.revokeObjectURL(exportUrl); }, 1000);
        toast("备份已下载，请妥善保管");
        return;
      }
      if (action === "config-import") {
        var importInput = $("config-import-file");
        if (importInput) importInput.click();
        return;
      }
      if (action === "config-import-cancel") {
        state.pendingImport = null;
        closeDialog();
        return;
      }
      if (action === "config-import-confirm") {
        if (!state.pendingImport) { closeDialog(); toast("没有待导入的备份", true); return; }
        var importRes2 = await post("/api/config/import", { data: state.pendingImport });
        if (importRes2 && importRes2.success) {
          var parts = [];
          if (importRes2.ai_imported) parts.push("AI 配置已恢复");
          parts.push("数据源 新增 " + (importRes2.sources_created || 0) + " / 更新 " + (importRes2.sources_updated || 0));
          if (importRes2.warnings && importRes2.warnings.length) parts.push(importRes2.warnings.join("；"));
          state.pendingImport = null;
          closeDialog();
          toast("导入完成：" + parts.join(" · "));
          await loadView("settings");
        } else {
          toast((importRes2 && (importRes2.detail || importRes2.error)) || "导入失败", true);
        }
        return;
      }
      if (action === "layout") { setLayout(target.dataset.layout); return; }
      if (action === "detail") { await openReader(id, target); return; }
      if (action === "reader-retry") { await openReader(id || state.reader.id); return; }
      if (action === "filter") { state.filter = target.dataset.value; await reloadItems(); return; }
      if (action === "load-more") { await loadMoreItems(); return; }
      if (action === "category") {
        state.category = state.category === target.dataset.value ? "" : target.dataset.value;
        await reloadItems(); return;
      }
      if (action === "news-group") {
        state.sourceCategory = target.dataset.value;
        state.category = ""; state.filter = "all";
        state.view = "library"; render(); await reloadItems(); return;
      }
      if (action === "clear-group") { state.sourceCategory = ""; await reloadItems(); return; }
      if (action === "source-category") {
        state.sourceCategory = state.sourceCategory === target.dataset.value ? "" : target.dataset.value;
        await reloadItems(); return;
      }
      if (action === "source-all") { state.sourceCategory = ""; await reloadItems(); return; }
      if (action === "quality-category") {
        state.aiCategory = state.aiCategory === target.dataset.value ? "" : target.dataset.value;
        await reloadItems(); return;
      }
      if (action === "ai-tab") { state.aiTab = target.dataset.value === "overview" ? "overview" : "items"; render(); return; }
      if (action === "ai-category") { state.aiCategory = state.aiCategory === target.dataset.value ? "" : target.dataset.value; await loadView("ai"); return; }
      if (action === "star") {
        var next = target.dataset.starred == 1 ? 0 : 1;
        await post("/api/items/" + encodeURIComponent(id) + "/star", { starred: next });
        toast(next ? "已收藏" : "已取消收藏"); closeDialog(); await loadView(state.view); return;
      }
      if (action === "read") {
        await post("/api/items/" + encodeURIComponent(id) + "/read", { read_status: 1 });
        toast("已标记已读"); closeDialog(); await loadView(state.view); return;
      }
      if (action === "reanalyze") {
        e.stopPropagation();
        var rr = await post("/api/ai/reanalyze/" + encodeURIComponent(id));
        toast(rr && rr.error ? "重分析失败：" + rr.error : "已重新分析", !!(rr && rr.error));
        closeDialog(); await loadView(state.view); return;
      }
      if (action === "add-source") { sourceForm(null); return; }
      if (action === "edit-source") { var s = sourceById(id); if (s) sourceForm(s); return; }
      if (action === "delete-source") { confirmDialog("删除订阅源", "确定删除「" + id + "」？相关抓取计划会一并移除。", "delete-source-confirm", id); return; }
      if (action === "delete-source-confirm") {
        await del("/api/sources/" + encodeURIComponent(id));
        closeDialog(); toast("已删除"); await loadView("sources"); return;
      }
      if (action === "toggle-source") {
        var nextEnabled = target.dataset.enabled == 1 ? 0 : 1;
        await post("/api/sources/" + encodeURIComponent(id) + "/toggle", { enabled: nextEnabled });
        toast(nextEnabled ? "已启用" : "已停用"); await loadView("sources"); return;
      }
      if (action === "run-source" || action === "run-all-sources") {
        if (action === "run-all-sources") {
          var enabled = state.sources.filter(function (x) { return x.enabled == 1; });
          if (!enabled.length) { toast("没有已启用的订阅源", true); return; }
          toast("正在抓取 " + enabled.length + " 个源…");
          var rs = await Promise.allSettled(enabled.map(function (x) { return post("/api/sources/" + encodeURIComponent(x.id) + "/run"); }));
          var added = rs.reduce(function (n, r) { return n + (r.status === "fulfilled" && r.value ? (r.value.items_added || 0) : 0); }, 0);
          toast("抓取完成，新增 " + added + " 条内容");
        } else {
          var r1 = await post("/api/sources/" + encodeURIComponent(id) + "/run");
          toast(r1 && r1.error ? "抓取失败：" + r1.error : "抓取完成，新增 " + (r1.items_added || 0) + " 条", !!(r1 && r1.error));
        }
        await loadView("sources"); return;
      }
      if (action === "logs-source") { await openLogs(id); return; }
      if (action === "ai-test") {
        var aiState = state.aiSettings || {};
        var currentProf = currentProfile(aiState);
        var savedKey = (currentProf && currentProf.api_key) || aiState.api_key || "";
        var testForm = document.querySelector('form[data-form="ai-settings"]');
        var testPayload = {};
        if (testForm) {
          var tf = new FormData(testForm);
          var testProvider = String(tf.get("provider") || "");
          var testModel = String(tf.get("model") || "").split(/[,，\n]/)[0].trim();
          if (!testModel) { toast("请先选择或填写模型 ID", true); return; }
          var testKey = String(tf.get("api_key") || "").trim() || savedKey;
          var testModelRef = modelRefFor(testProvider, testModel);
          testPayload = {
            profile: {
              id: (currentProf && currentProf.id) || "default",
              name: (currentProf && currentProf.name) || providerLabel(testProvider),
              provider: testProvider,
              api_key: testKey,
              model: testModelRef,
              models: [testModelRef],
              base_url: String(tf.get("base_url") || ""),
              requests_per_minute: Math.max(1, Math.round(Number(tf.get("requests_per_minute") || 10))),
              disabled: false
            },
            system_prompt: String(tf.get("system_prompt") || "")
          };
        }
        toast("正在测试当前表单的模型配置…");
        var tr = await post("/api/ai/test", testPayload);
        var txt = typeof tr === "string" ? tr : JSON.stringify(tr, null, 2);
        openDialog("模型连通性测试", '<pre class="code-block result-block">' + esc(txt.slice(0, 6000)) + "</pre>", "", true); return;
      }
      if (action === "ai-eval") {
        var er = await post("/api/ai/start_eval", {});
        toast(er && er.error ? er.error : "全量评测已在后台启动", !!(er && er.error));
        await loadView("ai"); return;
      }
      if (action === "daily-generate" || action === "daily-week") {
        var dates = action === "daily-week"
          ? Array.from({ length: 7 }, function (_, i) { var d = new Date(); d.setDate(d.getDate() - i); return dateStr(d); })
          : [state.dailyDate];
        var gr = await post("/api/ai/daily/generate", { date: dates[0], report_type: state.dailyType });
        if (dates.length > 1) {
          for (var i = 1; i < dates.length; i++) await post("/api/ai/daily/generate", { date: dates[i], report_type: state.dailyType });
        }
        toast(gr && gr.error ? "生成失败：" + gr.error : "已启动生成，稍后自动刷新", !!(gr && gr.error));
        setTimeout(function () { if (state.view === "daily") loadView("daily"); }, 3500); return;
      }
      if (action === "daily-prev" || action === "daily-next") {
        var d0 = new Date(state.dailyDate + "T00:00:00");
        d0.setDate(d0.getDate() + (action === "daily-next" ? 1 : -1));
        state.dailyDate = dateStr(d0); await loadView("daily"); return;
      }
      if (action === "daily-open") {
        state.dailyDate = target.dataset.date; state.dailyType = target.dataset.type || "daily";
        state.view = "daily";
        await loadView("daily"); return;
      }
      if (action === "device-kick" || action === "device-ban" || action === "device-unban") {
        var path = "/api/browsers/" + action.replace("device-", "");
        await post(path, { conn_id: id });
        toast("设备操作已完成"); await loadView("devices"); return;
      }
    } catch (err) {
      if (err.message !== "unauthorized") toast("操作失败：" + err.message, true);
    }
  });

  // Keyboard reading controls. Ignore typing contexts so J/K remain printable
  // characters inside forms and search.
  document.addEventListener("keydown", function (e) {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    var el = e.target;
    if (el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA" || el.tagName === "SELECT" || el.isContentEditable)) return;
    var dialogOpen = !$("dialog-modal").classList.contains("hidden");

    if (e.key === "Escape") {
      if (state.reader.open) { e.preventDefault(); closeReader({ restoreFocus: true }); return; }
      if (dialogOpen) { closeDialog(); return; }
      return;
    }
    if (dialogOpen) return;

    if (state.reader.open && (e.key === "j" || e.key === "J")) { e.preventDefault(); navigateReader(1); return; }
    if (state.reader.open && (e.key === "k" || e.key === "K")) { e.preventDefault(); navigateReader(-1); return; }
    if (e.key === "v" || e.key === "V") {
      if (state.view === "today" || state.view === "library") {
        e.preventDefault();
        setLayout(state.layout === "grid" ? "list" : "grid");
      }
      return;
    }
    if (e.key === "/" && state.view === "library") {
      e.preventDefault();
      $("search-input").focus();
      return;
    }
    if ((e.key === "Enter" || e.key === " ") && el && el.closest) {
      var row = el.closest('[data-action="detail"][role="button"]');
      if (row) { e.preventDefault(); row.click(); }
    }
  });

  function fallbackCopy(text, done) {
    var ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); done(); } catch (err) { /* clipboard unavailable */ }
    document.body.removeChild(ta);
  }

  // Poll the health endpoint so the sidebar footer and the 今天 dashboard agree
  // on the extension state. The dashboard is otherwise rendered once when the
  // view is entered, so an extension that connects afterwards would leave
  // "0 已连接浏览器 / 未连接" on screen while the footer already says 已连接.
  async function refreshHealth() {
    var h;
    try { h = await api("/open/api/health"); } catch (e) { return; }
    var prev = state.health || {};
    var wasOffline = prev.status !== "ok" || !!state.error;
    state.health = h;
    state.error = "";
    syncStatus();
    var changed = wasOffline ||
      !!prev.browser_connected !== !!h.browser_connected ||
      (Number(prev.browser_count) || 0) !== (Number(h.browser_count) || 0);
    if (changed && state.view === "today") {
      var scroll = $("content").scrollTop;
      render();
      $("content").scrollTop = scroll;
    }
  }

  async function init() {
    var session = null;
    try { session = await api("/api/auth/session"); } catch (e) { /* server not ready */ }
    if (!desktopMode && session && session.auth_required && !session.authenticated) showLogin();
    await loadView(state.view);
    setInterval(refreshHealth, 15000);
    window.addEventListener("focus", refreshHealth);
  }

  init();
})();
