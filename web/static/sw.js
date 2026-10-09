// No offline cache: the service worker exists for installability and push.
self.addEventListener('fetch', () => {});

self.addEventListener('push', e => {
  const m = e.data ? e.data.json() : {};
  e.waitUntil(self.registration.showNotification(m.title || 'compete', {
    body: m.body, icon: '/static/icon-192.png', data: { url: m.url || '/' },
  }));
});

// Tapping a notification focuses an open compete tab, else opens one.
self.addEventListener('notificationclick', e => {
  e.notification.close();
  const url = e.notification.data.url;
  e.waitUntil(clients.matchAll({ type: 'window' }).then(ws => {
    for (const w of ws) if ('focus' in w) return w.navigate(url).then(c => (c || w).focus());
    return clients.openWindow(url);
  }));
});
