const CACHE = "devhive-v1";
const SHELL = ["/", "/index.html", "/style.css", "/app.js", "/manifest.json", "/icon-192.png", "/icon-512.png"];

self.addEventListener("install", e => {
  e.waitUntil(caches.open(CACHE).then(c => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", e => {
  e.waitUntil(
    caches.keys()
      .then(keys => Promise.all(keys.filter(k => k !== CACHE).map(k => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", e => {
  const req = e.request;
  const u = new URL(req.url);
  if (req.method !== "GET" || u.origin !== location.origin) return;
  if (u.pathname.startsWith("/api/") || u.pathname === "/healthz") return;
  e.respondWith(
    caches.match(req).then(hit =>
      hit || fetch(req).then(res => {
        if (res.ok) caches.open(CACHE).then(c => c.put(req, res.clone()));
        return res;
      }).catch(() => caches.match("/index.html"))
    )
  );
});
