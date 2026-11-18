(function () {
  "use strict";
  var html = document.documentElement;
  var root = html.dataset.root || "";

  // Light / dark switch. The choice is remembered in this browser.
  document.getElementById("theme-toggle").addEventListener("click", function () {
    var next = html.dataset.theme === "dark" ? "light" : "dark";
    html.dataset.theme = next;
    try { localStorage.setItem("octavo-theme", next); } catch (e) {}
  });

  // Reader settings, remembered in this browser for every book served from
  // the same site: the "Aa" panel (font, text size, line spacing, page width)
  // and the layout (chapter list, "On this page", zen mode).
  var saved = {};
  try { saved = JSON.parse(localStorage.getItem("octavo-reader") || "{}") || {}; } catch (e) {}
  var appliers = [];
  function store() {
    try {
      if (Object.keys(saved).length) localStorage.setItem("octavo-reader", JSON.stringify(saved));
      else localStorage.removeItem("octavo-reader");
    } catch (e) {}
    appliers.forEach(function (apply) { apply(); });
  }

  // The "Aa" panel. Its Escape handler runs in the capture phase, so that
  // Escape closes the panel first and does not also leave zen mode.
  (function () {
    var toggle = document.getElementById("reader-toggle");
    var panel = document.getElementById("reader-panel");
    if (!toggle || !panel) return; // a custom page template without the menu
    var sizes = [0.85, 0.92, 1, 1.1, 1.2, 1.32, 1.45, 1.6];
    var defaults = { font: html.dataset.defaultFont || "literata", spacing: "normal", width: "normal", size: 1 };

    function apply() {
      ["font", "spacing", "width"].forEach(function (key) {
        if (saved[key]) html.dataset[key] = saved[key]; else delete html.dataset[key];
        panel.querySelectorAll('[data-setting="' + key + '"] button').forEach(function (b) {
          b.setAttribute("aria-pressed", String(b.dataset.value === (saved[key] || defaults[key])));
        });
      });
      var size = saved.size || defaults.size;
      if (saved.size) html.style.setProperty("--text-scale", size); else html.style.removeProperty("--text-scale");
      document.getElementById("reader-size").textContent = Math.round(size * 100) + "%";
      document.getElementById("reader-smaller").disabled = size <= sizes[0];
      document.getElementById("reader-larger").disabled = size >= sizes[sizes.length - 1];
    }
    function step(by) {
      var at = sizes.indexOf(saved.size || defaults.size);
      if (at < 0) at = sizes.indexOf(1);
      saved.size = sizes[Math.max(0, Math.min(sizes.length - 1, at + by))];
      if (saved.size === defaults.size) delete saved.size;
      store();
    }
    panel.querySelectorAll("[data-setting] button").forEach(function (b) {
      b.addEventListener("click", function () {
        var key = b.parentNode.dataset.setting;
        if (b.dataset.value === defaults[key]) delete saved[key]; else saved[key] = b.dataset.value;
        store();
      });
    });
    document.getElementById("reader-smaller").addEventListener("click", function () { step(-1); });
    document.getElementById("reader-larger").addEventListener("click", function () { step(1); });
    document.getElementById("reader-reset").addEventListener("click", function () {
      Object.keys(saved).forEach(function (key) { delete saved[key]; });
      store();
    });

    function open(show) {
      panel.hidden = !show;
      toggle.setAttribute("aria-expanded", String(show));
    }
    toggle.addEventListener("click", function () { open(panel.hidden); });
    document.addEventListener("click", function (e) {
      if (!panel.hidden && !panel.contains(e.target) && e.target !== toggle) open(false);
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && !panel.hidden) { open(false); toggle.focus(); e.preventDefault(); }
    }, true);
    appliers.push(apply);
    apply();
  })();

  // Layout: the chapter list, "On this page", and zen mode, which hides both
  // and quiets the top bar for reading full page. On narrow screens the menu
  // button opens the chapter list over the page instead.
  (function () {
    var narrow = matchMedia("(max-width: 52rem)");
    var zen = document.getElementById("zen-toggle");
    var sidebar = document.getElementById("sidebar");
    function apply() {
      ["sidebar", "toc"].forEach(function (key) {
        if (saved[key] === "hidden") html.dataset[key] = "hidden"; else delete html.dataset[key];
      });
      if (saved.zen) html.dataset.zen = "on"; else delete html.dataset.zen;
      if (zen) zen.setAttribute("aria-pressed", String(!!saved.zen));
      document.querySelectorAll("[data-toggle]").forEach(function (b) {
        b.setAttribute("aria-pressed", String(saved[b.dataset.toggle] !== "hidden"));
      });
      document.querySelectorAll("#reader-layout [data-panel]").forEach(function (b) {
        var key = b.dataset.panel;
        b.setAttribute("aria-pressed", String(key === "zen" ? !!saved.zen : saved[key] !== "hidden" && !saved.zen));
      });
    }
    function toggleZen() {
      if (saved.zen) delete saved.zen; else saved.zen = true;
      if (sidebar) sidebar.classList.remove("open");
      store();
    }
    appliers.push(apply);
    apply();

    var menu = document.getElementById("menu-toggle");
    if (menu && sidebar) menu.addEventListener("click", function () {
      if (narrow.matches) { sidebar.classList.toggle("open"); return; }
      if (saved.sidebar === "hidden") delete saved.sidebar; else saved.sidebar = "hidden";
      store();
    });
    if (zen) zen.addEventListener("click", toggleZen);
    document.querySelectorAll("#reader-layout [data-panel]").forEach(function (b) {
      b.addEventListener("click", function () {
        var key = b.dataset.panel;
        if (key === "zen") { toggleZen(); return; }
        // In zen mode both panels are hidden; asking for one leaves zen mode
        // and shows it.
        if (saved.zen) { delete saved.zen; delete saved[key]; }
        else if (saved[key] === "hidden") delete saved[key];
        else saved[key] = "hidden";
        store();
      });
    });
    // "Expand all" / "Collapse all" for the parts in the chapter list. By
    // default only the part holding this page is open. Expand all is
    // remembered (saved.nav), so the next page opens with every part open
    // too; Collapse all closes every part and goes back to the default.
    var navAll = document.getElementById("nav-all");
    if (navAll && sidebar) {
      var groups = sidebar.querySelectorAll("details");
      var allOpen = function () {
        return Array.prototype.every.call(groups, function (d) { return d.open; });
      };
      var label = function () {
        var open = allOpen();
        var text = open ? "Collapse all parts" : "Expand all parts";
        navAll.setAttribute("aria-label", text);
        navAll.title = text;
        navAll.setAttribute("aria-expanded", String(open));
      };
      var setAll = function (open) {
        groups.forEach(function (d) { d.open = open; });
        label();
      };
      if (saved.nav === "open") setAll(true); else label();
      groups.forEach(function (d) { d.addEventListener("toggle", label); });
      navAll.addEventListener("click", function () {
        var open = !allOpen();
        if (open) saved.nav = "open"; else delete saved.nav;
        setAll(open);
        store();
      });
      navAll.hidden = false;
    }
    // The small button at the top right that shows and hides "On this page".
    document.querySelectorAll("[data-toggle]").forEach(function (b) {
      b.addEventListener("click", function () {
        var key = b.dataset.toggle;
        if (saved[key] === "hidden") delete saved[key]; else saved[key] = "hidden";
        store();
      });
    });
    // Z switches zen mode and Escape leaves it. Not while typing, and not
    // with modifier keys, so the browser's own shortcuts keep working.
    document.addEventListener("keydown", function (e) {
      var t = e.target;
      if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) return;
      if (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName)) return;
      if (e.key === "z" || e.key === "Z") { toggleZen(); e.preventDefault(); }
      else if (e.key === "Escape" && saved.zen) toggleZen();
      else if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
        // The previous or next chapter, unless the page itself scrolls sideways.
        if (document.documentElement.scrollWidth > window.innerWidth) return;
        var link = document.querySelector(e.key === "ArrowLeft" ? ".pager .prev" : ".pager .next");
        if (link) location.href = link.href;
      }
    });
  })();

  // The version menu: the same page in the chosen version, when it has that
  // page, else that version's first page.
  (function () {
    var menu = document.getElementById("versions");
    var home = new URL(root || "./", location.href).href; // this version's folder
    function go(folder) {
      var start = new URL(folder + "index.html", location.href).href;
      var here = location.href.indexOf(home) === 0 ? location.href.slice(home.length) : "";
      var same = new URL(folder + here, location.href).href;
      if (!here || location.protocol === "file:") { location.href = start; return; }
      fetch(same, { method: "HEAD" }).then(function (r) {
        location.href = r.ok ? same : start;
      }, function () { location.href = start; });
    }
    if (menu) menu.addEventListener("change", function () { go(menu.value); });
    document.querySelectorAll("[data-version-link]").forEach(function (a) {
      a.addEventListener("click", function (e) {
        e.preventDefault();
        go(a.getAttribute("href").replace(/index\.html$/, ""));
      });
    });
  })();

  // Offline reading (see sw.js). Service workers need https, or a local
  // address. A book built without offline reading, and the preview, remove
  // one left by an earlier build, so it cannot serve old pages.
  (function () {
    if (!("serviceWorker" in navigator) || !window.isSecureContext) return;
    var scope = new URL(root || "./", location.href).href;
    if (html.dataset.offline !== "on") {
      navigator.serviceWorker.getRegistration(scope).then(function (reg) {
        if (reg && reg.scope === scope) reg.unregister();
      }).catch(function () {});
      return;
    }
    navigator.serviceWorker.register(root + "sw.js", { scope: scope }).catch(function () {});
    var box = document.getElementById("offline");
    var save = document.getElementById("offline-save");
    var status = document.getElementById("offline-status");
    if (!box || !save) return;
    var key = "octavo-offline@" + scope;
    function say(text) { status.textContent = text; }
    try { if (localStorage.getItem(key)) { save.textContent = "Update offline copy"; say("Saved " + localStorage.getItem(key)); } } catch (e) {}
    navigator.serviceWorker.ready.then(function (reg) {
      box.hidden = false;
      save.addEventListener("click", function () {
        save.disabled = true;
        say("Saving…");
        (reg.active || navigator.serviceWorker.controller).postMessage({ type: "save-all" });
      });
    });
    navigator.serviceWorker.addEventListener("message", function (e) {
      var m = e.data || {};
      if (m.type === "save-progress") say(m.done + " of " + m.total + " pages");
      if (m.type === "save-done") {
        save.disabled = false;
        if (m.failed && !m.total) { say("Could not save; try again when online"); return; }
        var day = new Date().toISOString().slice(0, 10);
        try { localStorage.setItem(key, day); } catch (e) {}
        save.textContent = "Update offline copy";
        say(m.failed ? "Saved, except " + m.failed + " pages" : "Saved " + day);
      }
    });
  })();

  // Copy buttons on code blocks.
  document.querySelectorAll("article pre").forEach(function (pre) {
    if (!navigator.clipboard) return;
    var button = document.createElement("button");
    button.type = "button";
    button.className = "copy-button";
    button.textContent = "Copy";
    button.addEventListener("click", function () {
      var code = pre.querySelector("code") || pre;
      navigator.clipboard.writeText(code.textContent).then(function () {
        button.textContent = "Copied";
        setTimeout(function () { button.textContent = "Copy"; }, 1500);
      });
    });
    pre.appendChild(button);
  });

  // Search over search.json, loaded the first time the box is used.
  var input = document.getElementById("search");
  var list = document.getElementById("search-results");
  var index = null;

  function load() {
    if (!index) {
      index = fetch(root + "search.json").then(function (r) { return r.json(); });
    }
    return index;
  }

  function show(pages, query) {
    var words = query.toLowerCase().split(/\s+/).filter(Boolean);
    list.textContent = "";
    if (!words.length) { list.hidden = true; return; }
    var hits = [];
    pages.forEach(function (page) {
      var title = page.title.toLowerCase(), text = page.text.toLowerCase(), score = 0;
      for (var i = 0; i < words.length; i++) {
        var inTitle = title.indexOf(words[i]) >= 0, inText = text.indexOf(words[i]) >= 0;
        if (!inTitle && !inText) return;
        score += (inTitle ? 10 : 0) + (inText ? 1 : 0);
      }
      var at = Math.max(0, text.indexOf(words[0]) - 40);
      hits.push({ page: page, score: score, snippet: (at ? "…" : "") + page.text.slice(at, at + 140) + "…" });
    });
    hits.sort(function (a, b) { return b.score - a.score; });
    hits.slice(0, 8).forEach(function (hit) {
      var item = document.createElement("li"), link = document.createElement("a"), snippet = document.createElement("small");
      link.href = root + hit.page.url;
      link.textContent = hit.page.title;
      snippet.textContent = hit.snippet;
      link.appendChild(snippet);
      item.appendChild(link);
      list.appendChild(item);
    });
    if (!hits.length) {
      var empty = document.createElement("li");
      empty.className = "empty";
      empty.textContent = "No results";
      list.appendChild(empty);
    }
    list.hidden = false;
  }

  input.addEventListener("input", function () {
    load().then(function (pages) { show(pages, input.value); }).catch(function () {
      list.textContent = "";
      var item = document.createElement("li");
      item.className = "empty";
      item.textContent = "Search needs the book to be served over http.";
      list.appendChild(item);
      list.hidden = false;
    });
  });
  input.addEventListener("keydown", function (e) {
    if (e.key === "Escape") { input.value = ""; list.hidden = true; }
  });
  document.addEventListener("click", function (e) {
    if (!e.target.closest(".search")) list.hidden = true;
  });
})();
