// Carrel UI helpers — no inline scripts (CSP).
document.addEventListener('click', function (e) {
    var refresh = e.target.closest('[data-refresh]');
    if (refresh) {
        window.location.reload();
        return;
    }

    var printBtn = e.target.closest('[data-print]');
    if (printBtn) {
        window.print();
        return;
    }

    var btn = e.target.closest('[data-copy]');
    if (!btn) return;
    var id = btn.getAttribute('data-copy');
    var el = document.getElementById(id);
    if (!el) return;
    el.select();
    el.setSelectionRange(0, 99999);
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(el.value);
    }
});

document.addEventListener('change', function (e) {
    var cb = e.target.closest('[data-print-photos]');
    if (!cb) return;
    document.body.classList.toggle('print-with-photos', cb.checked);
});

// Progress delivery (§16): the panel is streamed when the browser and the
// network allow it, and falls back to polling by itself when the stream will not
// open. The fallback fetch returns a fragment that carries its own poller, so
// nothing further has to be switched on here.
document.addEventListener('htmx:sseError', function (e) {
    var panel = e.target.closest('[data-sse-panel]');
    if (!panel || panel.dataset.pollFallback === 'on') return;
    panel.dataset.pollFallback = 'on';
    panel.removeAttribute('sse-connect');
    panel.removeAttribute('sse-swap');
    var url = panel.getAttribute('data-poll-url');
    if (!url || !window.htmx) return;
    window.htmx.ajax('GET', url, { target: panel, swap: 'innerHTML' });
});
