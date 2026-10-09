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

  document.addEventListener('click', e => {
    const b = e.target.closest('[data-paste], [data-share]');
    if (!b) return;
    if (b.matches('[data-paste]')) paste(b);
    else share(b);
  });
  // htmx:load fires for the first page and for every swapped-in fragment.
  document.addEventListener('htmx:load', e => reveal(e.target));
  reveal(document);
})();
