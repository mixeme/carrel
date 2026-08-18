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

    // Empty contact fields stay in the form so a save is still a patch; the
    // button only changes whether they are shown.
    var emptyBtn = e.target.closest('[data-empty-fields]');
    if (emptyBtn) {
        var form = emptyBtn.closest('form');
        if (!form) return;
        var on = form.classList.toggle('show-empty');
        emptyBtn.setAttribute('aria-expanded', on ? 'true' : 'false');
        emptyBtn.textContent = on ? 'Hide empty fields' : 'Show empty fields';
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

// Attaching by paste and by drop (§23.10). Keep's screenshot is one movement, so
// this one has to be too: the picture in the clipboard goes straight to the
// attach endpoint and the page reloads with it in place. There is no separate
// upload dialogue, no choosing a folder — that was settled once in the settings.
//
// It works by handing the file to the form that is already on the card, which
// means it goes through the same CSRF token, the same route and the same
// server-side checks as choosing a file by hand. The only thing this adds is not
// having to save the picture somewhere first.
(function () {
    function attachForm(node) {
        return node && node.closest ? node.closest('[data-attach-form]') : null;
    }

    function cardForm(target) {
        // A paste or a drop can land anywhere on the card — most usefully in the
        // note's textarea — so the form is looked for on the whole card.
        var card = target && target.closest ? target.closest('.note-card, .event-card') : null;
        if (card) {
            var form = card.querySelector('[data-attach-form]');
            if (form) return form;
        }
        return document.querySelector('[data-attach-form]');
    }

    function send(form, file) {
        var input = form.querySelector('[data-attach-input]');
        var hint = form.querySelector('[data-attach-hint]');
        if (!input || !window.DataTransfer) return false;
        // Assigning through a DataTransfer is what lets a file the page was given
        // become the value of a file input, so the normal submit carries it.
        var holder = new DataTransfer();
        holder.items.add(file);
        input.files = holder.files;
        if (hint) hint.textContent = 'Uploading ' + file.name + '…';
        form.submit();
        return true;
    }

    function firstFile(list) {
        if (!list) return null;
        for (var i = 0; i < list.length; i++) {
            var item = list[i];
            if (item.kind === 'file') {
                var file = item.getAsFile();
                if (file) return file;
            } else if (item instanceof File) {
                return item;
            }
        }
        return null;
    }

    document.addEventListener('paste', function (e) {
        if (!e.clipboardData) return;
        var target = e.target;
        var form = cardForm(target);
        if (!form) return;
        var file = firstFile(e.clipboardData.items);
        if (!file) return;
        // Text pasted into the body is text; only a file is intercepted.
        e.preventDefault();
        send(form, file);
    });

    ['dragover', 'dragenter'].forEach(function (name) {
        document.addEventListener(name, function (e) {
            var card = e.target.closest ? e.target.closest('.note-card, .event-card') : null;
            if (!card || !card.querySelector('[data-attach-form]')) return;
            e.preventDefault();
            card.classList.add('is-drop-target');
        });
    });

    document.addEventListener('dragleave', function (e) {
        var card = e.target.closest ? e.target.closest('.note-card, .event-card') : null;
        if (card) card.classList.remove('is-drop-target');
    });

    document.addEventListener('drop', function (e) {
        var card = e.target.closest ? e.target.closest('.note-card, .event-card') : null;
        if (!card) return;
        var form = card.querySelector('[data-attach-form]');
        if (!form || !e.dataTransfer) return;
        card.classList.remove('is-drop-target');
        var file = firstFile(e.dataTransfer.files.length ? e.dataTransfer.files : e.dataTransfer.items);
        if (!file) return;
        e.preventDefault();
        send(form, file);
    });

    // A file chosen by hand submits on its own, so the person does not have to
    // press a second button for something they have already decided.
    document.addEventListener('change', function (e) {
        var input = e.target.closest ? e.target.closest('[data-attach-input]') : null;
        if (!input || !input.files || !input.files.length) return;
        var form = attachForm(input);
        if (form) form.submit();
    });
})();

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

// Details panel (wave 1.4): list rows load a read-only card into #app-details.
// No selection — no panel; closed manually — stays closed; state is per section.
(function () {
    var STORAGE_KEY = 'carrel:details';

    function shell() {
        return document.querySelector('.app-shell');
    }

    function panel() {
        return document.getElementById('app-details');
    }

    function sectionKey() {
        var layout = document.querySelector('[data-detail-section]');
        return layout ? layout.getAttribute('data-detail-section') : '';
    }

    function readAll() {
        try {
            return JSON.parse(localStorage.getItem(STORAGE_KEY) || '{}');
        } catch (e) {
            return {};
        }
    }

    function writeSection(section, data) {
        if (!section) return;
        var all = readAll();
        if (!data || (!data.url && !data.closed)) {
            delete all[section];
        } else {
            all[section] = data;
        }
        localStorage.setItem(STORAGE_KEY, JSON.stringify(all));
    }

    function showPanel() {
        var el = panel();
        var frame = shell();
        if (el) el.hidden = false;
        if (frame) frame.classList.add('has-details');
    }

    function hidePanel(manual) {
        var el = panel();
        var frame = shell();
        if (el) {
            el.hidden = true;
            el.innerHTML = '';
        }
        if (frame) frame.classList.remove('has-details');
        document.querySelectorAll('.list-row.is-selected').forEach(function (row) {
            row.classList.remove('is-selected');
        });
        var section = sectionKey();
        if (!section) return;
        if (manual) {
            writeSection(section, { closed: true });
        } else {
            writeSection(section, null);
        }
    }

    function markSelected(link) {
        document.querySelectorAll('.list-row.is-selected').forEach(function (row) {
            row.classList.remove('is-selected');
        });
        var row = link.closest('.list-row');
        if (row) row.classList.add('is-selected');
    }

    function rememberOpen(url) {
        var section = sectionKey();
        if (section && url) {
            writeSection(section, { url: url, closed: false });
        }
    }

    document.addEventListener('click', function (e) {
        if (e.target.closest('[data-detail-close]')) {
            e.preventDefault();
            hidePanel(true);
            return;
        }
        var link = e.target.closest('a.detail-link[hx-get]');
        if (!link) return;
        markSelected(link);
        rememberOpen(link.getAttribute('hx-get'));
        showPanel();
    });

    document.body.addEventListener('htmx:afterSwap', function (e) {
        if (!e.detail.target || e.detail.target.id !== 'app-details') return;
        showPanel();
        var root = e.detail.target.querySelector('[data-detail-url]');
        if (root) rememberOpen(root.getAttribute('data-detail-url'));
    });

    document.body.addEventListener('htmx:responseError', function (e) {
        if (!e.detail.target || e.detail.target.id !== 'app-details') return;
        hidePanel(false);
    });

    function restore() {
        var section = sectionKey();
        if (!section || !window.htmx) return;
        var state = readAll()[section];
        if (!state || state.closed || !state.url) return;
        var el = panel();
        if (!el) return;
        window.htmx.ajax('GET', state.url, { target: '#app-details', swap: 'innerHTML' }).then(function () {
            var link = document.querySelector('a.detail-link[hx-get="' + CSS.escape(state.url) + '"]');
            if (link) markSelected(link);
            showPanel();
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', restore);
    } else {
        restore();
    }
})();

// Quick note sheet (wave 1.5) and header shortcuts.
(function () {
    function overlay() {
        return document.getElementById('app-overlay');
    }

    function sheetOpen() {
        var el = overlay();
        return el && !el.hidden;
    }

    function showSheet() {
        var el = overlay();
        if (el) el.hidden = false;
        document.body.classList.add('has-sheet');
        var area = document.querySelector('#app-sheet textarea');
        if (area) area.focus();
    }

    function closeSheet() {
        var el = overlay();
        var host = document.getElementById('app-sheet');
        if (el) el.hidden = true;
        if (host) host.innerHTML = '';
        document.body.classList.remove('has-sheet');
        closeCreateMenu();
    }

    function isTypingTarget(el) {
        if (!el || !el.closest) return false;
        var tag = el.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
        return !!el.isContentEditable;
    }

    function quickNoteURL(link) {
        return link.getAttribute('hx-get') || link.getAttribute('href');
    }

    function openQuickNote(link) {
        var url = quickNoteURL(link);
        if (!url) return;
        if (!window.htmx) {
            window.location.href = url;
            return;
        }
        window.htmx.ajax('GET', url, { target: '#app-sheet', swap: 'innerHTML' }).then(showSheet);
    }

    function createMenu() {
        return document.querySelector('.app-create-menu');
    }

    function closeCreateMenu() {
        var menu = createMenu();
        var toggle = document.querySelector('[data-create-menu-toggle]');
        if (menu) menu.hidden = true;
        if (toggle) toggle.setAttribute('aria-expanded', 'false');
    }

    function toggleCreateMenu(btn) {
        var menu = createMenu();
        if (!menu) return;
        var open = menu.hidden;
        menu.hidden = !open;
        btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    }

    document.addEventListener('click', function (e) {
        if (e.target.closest('[data-sheet-close]')) {
            e.preventDefault();
            closeSheet();
            return;
        }
        var quick = e.target.closest('[data-quick-note-open]');
        if (quick) {
            e.preventDefault();
            openQuickNote(quick);
            closeCreateMenu();
            return;
        }
        var toggle = e.target.closest('[data-create-menu-toggle]');
        if (toggle) {
            e.preventDefault();
            toggleCreateMenu(toggle);
            return;
        }
        if (!e.target.closest('.app-create')) {
            closeCreateMenu();
        }
    });

    document.body.addEventListener('htmx:afterSwap', function (e) {
        if (!e.detail.target || e.detail.target.id !== 'app-sheet') return;
        showSheet();
    });

    document.body.addEventListener('htmx:responseError', function (e) {
        if (!e.detail.target || e.detail.target.id !== 'app-sheet') return;
        closeSheet();
    });

    document.addEventListener('keydown', function (e) {
        var typing = isTypingTarget(e.target);
        if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
            var form = e.target.closest && e.target.closest('[data-quick-note-form]');
            if (form) {
                e.preventDefault();
                if (window.htmx && form.hasAttribute('hx-post')) {
                    window.htmx.trigger(form, 'submit');
                } else {
                    form.submit();
                }
            }
            return;
        }
        if (typing) return;
        if (e.key === 'Escape') {
            if (sheetOpen()) {
                e.preventDefault();
                closeSheet();
                return;
            }
            var details = document.getElementById('app-details');
            if (details && !details.hidden && details.innerHTML.trim()) {
                var closeDetail = details.querySelector('[data-detail-close]');
                if (closeDetail) {
                    e.preventDefault();
                    closeDetail.click();
                }
            }
            return;
        }
        if (e.ctrlKey || e.metaKey || e.altKey) return;
        if (e.key === 'n' || e.key === 'N') {
            var note = document.querySelector('[data-quick-note-open]');
            if (note) {
                e.preventDefault();
                openQuickNote(note);
            }
            return;
        }
        if (e.key === '/') {
            e.preventDefault();
            var search = document.getElementById('app-search');
            if (search) search.focus();
            return;
        }
        if (e.key === 'Enter') {
            if (sheetOpen()) return;
            var selected = document.querySelector('.list-row.is-selected a.detail-link');
            if (selected) {
                e.preventDefault();
                window.location.href = selected.getAttribute('href');
            }
            return;
        }
        if (e.key.length === 1) {
            var item = document.querySelector('[data-create-shortcut="' + e.key.toUpperCase() + '"]');
            if (item) {
                e.preventDefault();
                item.click();
            }
        }
    });
})();
