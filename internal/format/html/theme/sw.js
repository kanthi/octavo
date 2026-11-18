// Octavo's offline reading: a service worker that keeps the book readable
// without a connection. The browser runs it for this book's folder only.
//
// Pages (and search.json) come from the network when there is one, so
// readers always see the latest text, and from the cache when there is not.
// Every page read is kept. "Save for offline reading" in the chapter list
// keeps every page at once, with its pictures.
//
// Styles, scripts and fonts come from the cache first. They change only with
// a new build, which changes BUILD below; the browser then installs this
// file again and the old copies are dropped.
"use strict";
var BUILD = "{{build}}";
var SCOPE = self.registration.scope;
var ASSETS = "octavo-assets-" + BUILD + "@" + SCOPE;
var PAGES = "octavo-pages@" + SCOPE;
var SHELL = {{shell}};

self.addEventListener("install", function (e) {
  e.waitUntil(caches.open(ASSETS).then(function (c) { return c.addAll(SHELL); }).then(function () {
    return self.skipWaiting();
  }));
});

self.addEventListener("activate", function (e) {
  e.waitUntil(caches.keys().then(function (keys) {
    return Promise.all(keys.filter(function (k) {
      return k.indexOf("octavo-assets-") === 0 && k.slice(-SCOPE.length - 1) === "@" + SCOPE && k !== ASSETS;
    }).map(function (k) { return caches.delete(k); }));
  }).then(function () { return self.clients.claim(); }));
});

function isPage(url, request) {
  return request.mode === "navigate" || /\.(html|json)$/.test(url.pathname) || url.pathname.slice(-1) === "/";
}

function isAsset(url) {
  return url.href.indexOf(SCOPE + "assets/") === 0;
}

// The network first; the saved copy when the network fails.
function networkFirst(request) {
  return caches.open(PAGES).then(function (cache) {
    return fetch(request).then(function (response) {
      if (response.ok) cache.put(request, response.clone());
      return response;
    }, function () {
      return cache.match(request, { ignoreSearch: true }).then(function (hit) {
        // A folder address is saved as its index.html.
        if (!hit && /\/$/.test(new URL(request.url).pathname)) hit = cache.match(new URL("index.html", request.url).href);
        return hit;
      }).then(function (hit) {
        return hit || (request.mode === "navigate" ? offlinePage() : Response.error());
      });
    });
  });
}

// The saved copy at once, while a fresh one is fetched for next time: for
// pictures and your own stylesheets, which can change without a new BUILD.
function staleWhileRevalidate(request) {
  return caches.open(PAGES).then(function (cache) {
    return cache.match(request, { ignoreSearch: true }).then(function (hit) {
      var fresh = fetch(request).then(function (response) {
        if (response.ok) cache.put(request, response.clone());
        return response;
      });
      if (hit) { fresh.catch(function () {}); return hit; }
      return fresh;
    });
  });
}

// The saved copy first; the network for anything not saved yet.
function cacheFirst(request, name) {
  return caches.match(request, { ignoreSearch: true }).then(function (hit) {
    return hit || fetch(request).then(function (response) {
      if (response.ok) {
        var copy = response.clone();
        caches.open(name).then(function (c) { c.put(request, copy); });
      }
      return response;
    });
  });
}

function offlinePage() {
  return new Response(
    '<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">' +
    '<title>Offline</title><body style="font: 1.05rem/1.6 system-ui, sans-serif; text-align: center; padding: 4rem 1rem">' +
    "<h1 style=\"font-size: 1.4rem\">You are offline</h1><p>This page has not been saved for offline reading yet.</p>" +
    '<p><a href="javascript:history.back()">Go back</a></p>',
    { status: 503, headers: { "Content-Type": "text/html; charset=utf-8" } });
}

self.addEventListener("fetch", function (e) {
  var request = e.request, url = new URL(request.url);
  // Only this book's own files; never the preview's live reload, or other sites.
  if (request.method !== "GET" || url.href.indexOf(SCOPE) !== 0 || url.pathname.indexOf("/__octavo/") >= 0) return;
  if (isAsset(url)) e.respondWith(cacheFirst(request, ASSETS));
  else if (isPage(url, request)) e.respondWith(networkFirst(request));
  else e.respondWith(staleWhileRevalidate(request)); // pictures and other files of the book
});

// "Save for offline reading": every page in search.json, and the pictures
// they show. Progress goes back to the page that asked.
self.addEventListener("message", function (e) {
  if (!e.data || e.data.type !== "save-all") return;
  var client = e.source;
  function tell(msg) { if (client) client.postMessage(msg); }
  e.waitUntil(caches.open(PAGES).then(function (cache) {
    return fetch(SCOPE + "search.json", { cache: "no-cache" }).then(function (r) { return r.json(); }).then(function (pages) {
      var urls = pages.map(function (p) { return SCOPE + p.url; }), done = 0, failed = 0, next = 0;
      cache.put(SCOPE + "search.json", new Response(JSON.stringify(pages), { headers: { "Content-Type": "application/json" } }));
      function one(url) {
        return fetch(url, { cache: "no-cache" }).then(function (response) {
          if (!response.ok) throw new Error(response.status);
          return response.clone().text().then(function (html) {
            cache.put(url, response);
            var pictures = [], m, re = /<img[^>]+src="([^"#?]+)/g;
            while ((m = re.exec(html))) {
              var src = new URL(m[1].replace(/&amp;/g, "&"), url).href;
              if (src.indexOf(SCOPE) === 0) pictures.push(src);
            }
            return Promise.all(pictures.map(function (src) {
              return cache.match(src).then(function (hit) { return hit || cache.add(src).catch(function () {}); });
            }));
          });
        }).catch(function () { failed++; }).then(function () {
          done++;
          tell({ type: "save-progress", done: done, total: urls.length });
        });
      }
      // A few at a time, so a big book does not flood the server.
      function worker() {
        if (next >= urls.length) return Promise.resolve();
        return one(urls[next++]).then(worker);
      }
      return Promise.all([worker(), worker(), worker(), worker()]).then(function () {
        tell({ type: "save-done", total: urls.length, failed: failed });
      });
    });
  }).catch(function () { tell({ type: "save-done", total: 0, failed: 1 }); }));
});
