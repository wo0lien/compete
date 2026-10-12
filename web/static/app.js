// Clipboard and share buttons. They ship hidden and are shown only when the
// browser has the API, so without JS the plain textarea and field remain.
(() => {
  const canPaste = !!navigator.clipboard?.readText;
  const canShare = !!navigator.share;
  const canCopy = !!navigator.clipboard?.writeText;

  function reveal(root) {
    root.querySelectorAll('[data-paste]').forEach(b => { b.hidden = !canPaste; });
    root.querySelectorAll('[data-share]').forEach(b => { b.hidden = !canShare; });
    root.querySelectorAll('[data-copy]').forEach(b => { b.hidden = !canCopy; });
    pushState(root);
    root.querySelectorAll('[data-variants]').forEach(card => {
      card.querySelector('[role=tablist]').hidden = false;
      card.classList.add('tabbed');
      showVariant(card, card.querySelector('[aria-selected=true]').dataset.variant);
    });
  }

  // A card holding several variants of one puzzle shows one at a time.
  function showVariant(card, i) {
    card.querySelectorAll('[data-variant]').forEach(b => {
      const on = b.dataset.variant === i;
      b.setAttribute('aria-selected', on);
      b.classList.toggle('on', on);
    });
    card.querySelectorAll('[data-panel]').forEach(p => { p.hidden = p.dataset.panel !== i; });
  }

  // Paste fills the box; the member checks it and taps Submit.
  async function paste(button) {
    const text = button.form.querySelector('textarea');
    try {
      text.value = (await navigator.clipboard.readText()).trim();
    } catch { /* refused: fall back to a manual paste */ }
    if (!text.value) text.focus();
  }

  async function share(button) {
    try { await navigator.share({ url: button.dataset.share }); } catch { /* cancelled */ }
  }

  async function copy(button) {
    try {
      await navigator.clipboard.writeText(button.dataset.copy);
    } catch { // refused: select the link for a manual copy
      button.parentElement.querySelector('input[readonly]')?.select();
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
  // once per page (load and every htmx swap), so it sets every part, never just one.
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

  // busy dims the button and shows its data-busy label until done.
  function busy(button, on) {
    button.setAttribute('aria-busy', on);
    if (on) [button.dataset.label, button.textContent] = [button.textContent, button.dataset.busy || button.textContent];
    else button.textContent = button.dataset.label;
  }

  async function pushOn(button) {
    busy(button, true);
    try {
      await subscribe(button);
    } catch { // the push service or the server refused: say so, keep the button
      button.closest('[data-push]').querySelector('[data-push-error]').hidden = false;
    }
    busy(button, false);
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

  async function pushOff(button) {
    busy(button, true);
    const reg = await navigator.serviceWorker.ready;
    const sub = await reg.pushManager.getSubscription();
    if (sub) {
      await fetch('/push/unsubscribe', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ endpoint: sub.endpoint }) });
      await sub.unsubscribe();
    }
    location.reload();
  }

  document.addEventListener('click', e => {
    const b = e.target.closest('[data-paste], [data-share], [data-copy], [data-push-key], [data-push-off], [data-dialog], [data-close], [data-variant]');
    if (!b) return;
    if (b.matches('[data-variant]')) { showVariant(b.closest('[data-variants]'), b.dataset.variant); return; }
    // confirm sheets: without JS the #id link and dialog:target show the same sheet
    if (b.matches('[data-dialog]')) { e.preventDefault(); document.querySelector(b.getAttribute('href')).showModal(); return; }
    if (b.matches('[data-close]')) { // shown by :target instead (#id in the URL): let href="#" clear it
      const d = b.closest('dialog');
      if (d.open) { e.preventDefault(); d.close(); }
      return;
    }
    if (b.matches('[data-paste]')) paste(b);
    else if (b.matches('[data-share]')) share(b);
    else if (b.matches('[data-copy]')) copy(b);
    else if (b.matches('[data-push-key]')) pushOn(b);
    else pushOff(b);
  });
  // htmx 4 has no htmx:load: re-reveal after every swap (boosted links and forms).
  document.addEventListener('htmx:after:swap', () => reveal(document));
  reveal(document);
})();
