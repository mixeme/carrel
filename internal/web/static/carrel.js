// Carrel UI helpers — no inline scripts (CSP).
document.addEventListener('click', function (e) {
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
