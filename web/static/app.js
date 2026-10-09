// Clipboard and share buttons. They ship hidden and are shown only when the
// browser has the API, so without JS the plain textarea and field remain.
(() => {
  const canPaste = !!navigator.clipboard?.readText;
  const canShare = !!navigator.share;
  const canCopy = !!navigator.clipboard?.writeText;

  function reveal(root) {
    root.querySelectorAll('[data-paste]').forEach(b => { b.hidden = !canPaste; });
    root.querySelectorAll('[data-share]').forEach(b => {
      if (!canShare) b.textContent = b.dataset.copyLabel;
      b.hidden = !canShare && !canCopy;
    });
    pushState(root);
  }

  // Paste and submit: one tap from the game's share button to the board.
  async function paste(button) {
    const form = button.form;
    const text = form.querySelector('textarea');
    try {
      text.value = (await navigator.clipboard.readText()).trim();
    } catch { /* refused: fall back to a manual paste */ }
    if (text.value) form.requestSubmit();
    else text.focus();
  }

  async function share(button) {
    const url = button.dataset.share;
    if (canShare) {
      try { await navigator.share({ url }); } catch { /* cancelled */ }
      return;
    }
    try {
      await navigator.clipboard.writeText(url);
    } catch { // refused: select the invite field for a manual copy
      button.closest('section')?.querySelector('input[readonly]')?.select();
      return;
    }
    const label = button.textContent;
    button.textContent = button.dataset.copied;
    setTimeout(() => { button.textContent = label; }, 2000);
  }

  // Notifications: the permission prompt only follows a tap on the button.
  const canPush = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;

  function b64url(s) {
    const b = atob(s.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - s.length % 4) % 4));
    return Uint8Array.from(b, c => c.charCodeAt(0));
  }

  // pushState shows exactly one of the section's parts. It can run more than
  // once per page (load and htmx:load), so it sets every part, never just one.
  async function pushState(root) {
    const box = root.querySelector('[data-push]');
    if (!box) return;
    let part = '[data-push-install]';
    if (canPush && Notification.permission === 'denied') part = '[data-push-blocked]';
    else if (canPush) {
      const reg = await navigator.serviceWorker.ready;
      const sub = await reg.pushManager.getSubscription();
      part = sub ? '[data-push-settings]' : '[data-push-key]';
      // The device may have been subscribed under another account (or the
      // server lost the row): re-register it to whoever is logged in now.
      if (sub) fetch('/push/subscribe', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sub) });
    }
    for (const el of box.querySelectorAll('[data-push-install], [data-push-blocked], [data-push-key], [data-push-settings]')) {
      el.hidden = !el.matches(part);
    }
  }

  async function pushOn(button) {
    try {
      await subscribe(button);
    } catch { // the push service or the server refused: say so, keep the button
      button.closest('[data-push]').querySelector('[data-push-error]').hidden = false;
    }
  }

  async function subscribe(button) {
    if (await Notification.requestPermission() !== 'granted') {
      button.hidden = true;
      button.closest('[data-push]').querySelector('[data-push-blocked]').hidden = false;
      return;
    }
    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: b64url(button.dataset.pushKey) });
    const resp = await fetch('/push/subscribe', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(sub) });
    if (!resp.ok) throw new Error(resp.statusText);
    location.reload();
  }

  async function pushOff() {
    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.getSubscription();
    if (sub) {
      await fetch('/push/unsubscribe', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ endpoint: sub.endpoint }) });
      await sub.unsubscribe();
    }
    location.reload();
  }

  document.addEventListener('click', e => {
    const b = e.target.closest('[data-paste], [data-share], [data-push-key], [data-push-off]');
    if (!b) return;
    if (b.matches('[data-paste]')) paste(b);
    else if (b.matches('[data-share]')) share(b);
    else if (b.matches('[data-push-key]')) pushOn(b);
    else pushOff();
  });
  // htmx:load fires for the first page and for every swapped-in fragment.
  document.addEventListener('htmx:load', e => reveal(e.target));
  reveal(document);
})();
