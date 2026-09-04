// Service worker: the reason it exists is web push. No caching on purpose —
// a stale shell after a deploy is worse than a network round trip on a village
// Wi-Fi. The page registers it with the Pages base path as scope.
self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", (e) => e.waitUntil(self.clients.claim()));

self.addEventListener("push", (e) => {
  let d = { title: "Mokri Potok", body: "", url: "#/" };
  try { d = { ...d, ...e.data.json() }; } catch { /* plain text or empty */ }
  e.waitUntil(self.registration.showNotification(d.title, {
    body: d.body,
    icon: "icon-192.png",
    badge: "icon-192.png",
    tag: d.kind || "potok",
    data: { url: d.url },
  }));
});

self.addEventListener("notificationclick", (e) => {
  e.notification.close();
  const target = new URL(e.notification.data?.url || "#/", self.registration.scope).href;
  e.waitUntil(self.clients.matchAll({ type: "window", includeUncontrolled: true }).then((list) => {
    for (const c of list) {
      if (c.url.startsWith(self.registration.scope) && "focus" in c) { c.navigate(target); return c.focus(); }
    }
    return self.clients.openWindow(target);
  }));
});
