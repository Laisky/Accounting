/*
 * sw.js — minimal service worker for the Accounting PWA.
 * Caches STATIC same-origin assets only; API responses (/api/*) are never cached so
 * bookkeeping data is always fetched fresh.
 */
const CACHE = 'accounting-static-v1';
const STATIC_RE = /\.(?:js|css|svg|png|ico|woff2?)$/;

self.addEventListener('install', () => {
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((key) => key !== CACHE).map((key) => caches.delete(key))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener('fetch', (event) => {
  const { request } = event;
  const url = new URL(request.url);
  if (request.method !== 'GET' || url.origin !== self.location.origin) {
    return;
  }
  // Never cache API traffic.
  if (url.pathname.startsWith('/api')) {
    return;
  }
  if (!url.pathname.startsWith('/assets/') && !STATIC_RE.test(url.pathname)) {
    return;
  }
  event.respondWith(
    caches.open(CACHE).then(async (cache) => {
      const cached = await cache.match(request);
      if (cached) {
        return cached;
      }
      const response = await fetch(request);
      if (response.ok) {
        cache.put(request, response.clone());
      }
      return response;
    }),
  );
});
