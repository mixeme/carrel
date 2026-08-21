// Carrel UI helpers — no inline scripts (CSP).

// Marks the document so CSS can tell the two narrow-screen layouts apart: with
// JavaScript the source rail lives in the slide-out panel, without it the rail
// stays in the page where it can still be reached.
document.documentElement.classList.add('js');

// Collection colours and the progress fill (§16). The CSP of §24.5 has no
// 'unsafe-inline' for styles, so a style attribute written by a template is
// dropped before it reaches the box; the value travels as a data attribute and
// is applied here instead. Without JavaScript the CSS default — the accent —
// still shows, which is why the rules carry one.
(function () {
    function paint(root) {
        if (!root || !root.querySelectorAll) return;
        var scope = root.nodeType === 1 ? [root] : [];
        scope.concat(Array.prototype.slice.call(root.querySelectorAll('[data-swatch],[data-fill]')))
            .forEach(function (el) {
                if (!el.getAttribute) return;
                var colour = el.getAttribute('data-swatch');
                if (colour) el.style.background = colour;
                var fill = el.getAttribute('data-fill');
                if (fill !== null) el.style.width = fill + '%';
            });
    }

    document.addEventListener('DOMContentLoaded', function () { paint(document); });
    document.addEventListener('htmx:afterSwap', function (e) { paint(e.target); });
    document.addEventListener('htmx:afterSettle', function (e) { paint(e.target); });
    paint(document);
})();

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
    if (btn) {
        var id = btn.getAttribute('data-copy');
        var el = document.getElementById(id);
        if (!el) return;
        el.select();
        el.setSelectionRange(0, 99999);
        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(el.value);
        }
        return;
    }

    var copyURL = e.target.closest('[data-copy-url]');
    if (copyURL && navigator.clipboard && navigator.clipboard.writeText) {
        var link = copyURL.getAttribute('data-copy-url');
        if (link) navigator.clipboard.writeText(link);
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
(function () {
    // How long a panel may show nothing before the stream is written off. A
    // failed connection raises htmx:sseError and is handled at once; a stream
    // that never speaks — no extension, a proxy that buffers, a connection that
    // hangs open — raises nothing at all, and only this timer catches it.
    var GRACE = 3000;

    function running(panel) {
        return !!panel.querySelector('.find-source.is-waiting, .find-source.is-querying');
    }

    function fallback(panel) {
        if (!panel || panel.dataset.pollFallback === 'on') return;
        panel.dataset.pollFallback = 'on';
        panel.removeAttribute('sse-connect');
        panel.removeAttribute('sse-swap');
        var url = panel.getAttribute('data-poll-url');
        if (!url || !window.htmx) return;
        window.htmx.ajax('GET', url, { target: panel, swap: 'innerHTML' });
    }

    document.addEventListener('htmx:sseError', function (e) {
        var panel = e.target.closest('[data-sse-panel]');
        if (panel) fallback(panel);
    });

    function watch() {
        document.querySelectorAll('[data-sse-panel][data-poll-url]').forEach(function (panel) {
            if (!panel.getAttribute('sse-connect') || !running(panel)) return;
            var spoke = false;
            function mark() { spoke = true; }
            panel.addEventListener('htmx:sseMessage', mark);
            panel.addEventListener('htmx:afterSwap', mark);
            window.setTimeout(function () {
                panel.removeEventListener('htmx:sseMessage', mark);
                panel.removeEventListener('htmx:afterSwap', mark);
                if (!spoke && running(panel)) fallback(panel);
            }, GRACE);
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', watch);
    } else {
        watch();
    }
})();

// Details panel (wave 1.4): list rows load a read-only card into #app-details.
// No selection — no panel; closed manually — stays closed; state is per section.
(function () {
    var STORAGE_KEY = 'carrel:details';
    var narrow = window.matchMedia('(max-width: 640px)');

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
        document.querySelectorAll('.m-row.is-on').forEach(function (row) {
            row.classList.remove('is-on');
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
        document.querySelectorAll('.m-row.is-on').forEach(function (row) {
            row.classList.remove('is-on');
        });
        var row = link.closest('.m-row');
        if (row) row.classList.add('is-on');
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
        // §13: no properties panel on a narrow screen. The row opens the
        // record; the htmx request itself is cancelled below.
        if (narrow.matches) return;
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
        if (!section || !window.htmx || narrow.matches) return;
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
        return document.querySelector('[data-create-menu]');
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

    // The ⋯ overflow menu of 2.6.D1: one per page, holding the Import/Export
    // destination pickers a merged view needs (2.6.D2 — there is no single
    // collection to act on until one is chosen). Scoped by the nearest
    // [data-dots-menu] wrapper rather than a single global element, so more
    // than one could exist on a page without colliding.
    function dotsMenuOf(el) {
        var wrap = el.closest('[data-dots-menu]');
        return wrap ? wrap.querySelector('.m-menu') : null;
    }

    // Stage 1: the bar's ⋯ holds the is-2nd items that the container query
    // hid. They move into the menu on open and back to the bar on close, so
    // a sort link or the density toggle is still the same node — not a copy
    // that would desync from the form it belongs to.
    var barMoreHome = typeof WeakMap === 'function' ? new WeakMap() : null;

    function parkBarMore(wrap) {
        var bar = wrap.closest('.m-bar');
        var menu = wrap.querySelector('.m-menu');
        if (!bar || !menu) return;
        Array.prototype.slice.call(bar.children).forEach(function (kid) {
            if (kid === wrap) return;
            if (!kid.classList || !kid.classList.contains('is-2nd')) return;
            if (barMoreHome) barMoreHome.set(kid, { parent: bar, next: kid.nextSibling });
            menu.appendChild(kid);
        });
    }

    function unparkBarMore(menu) {
        var wrap = menu.closest('[data-bar-more]');
        if (!wrap) return;
        Array.prototype.slice.call(menu.children).forEach(function (kid) {
            var home = barMoreHome && barMoreHome.get(kid);
            if (home && home.parent) {
                var next = home.next;
                home.parent.insertBefore(kid, next && next.parentNode === home.parent ? next : wrap);
            } else if (wrap.parentNode) {
                wrap.parentNode.insertBefore(kid, wrap);
            }
        });
    }

    function closeDotsMenus(except) {
        document.querySelectorAll('[data-dots-menu] .m-menu').forEach(function (menu) {
            if (menu === except) return;
            if (!menu.hidden) unparkBarMore(menu);
            menu.hidden = true;
            var wrap = menu.closest('[data-dots-menu]');
            var toggle = wrap && wrap.querySelector('[data-dots-toggle]');
            if (toggle) toggle.setAttribute('aria-expanded', 'false');
        });
    }

    function restoreWideBarMore() {
        document.querySelectorAll('[data-bar-more]').forEach(function (wrap) {
            if (getComputedStyle(wrap).display !== 'none') return;
            var menu = wrap.querySelector('.m-menu');
            if (!menu || menu.hidden) return;
            unparkBarMore(menu);
            menu.hidden = true;
            var toggle = wrap.querySelector('[data-dots-toggle]');
            if (toggle) toggle.setAttribute('aria-expanded', 'false');
        });
    }
    window.addEventListener('resize', restoreWideBarMore);

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
        var dotsToggle = e.target.closest('[data-dots-toggle]');
        if (dotsToggle) {
            e.preventDefault();
            var menu = dotsMenuOf(dotsToggle);
            if (menu) {
                var open = menu.hidden;
                closeDotsMenus(open ? menu : null);
                if (open) {
                    var barWrap = dotsToggle.closest('[data-bar-more]');
                    if (barWrap) parkBarMore(barWrap);
                    menu.hidden = false;
                } else {
                    menu.hidden = true;
                    unparkBarMore(menu);
                }
                dotsToggle.setAttribute('aria-expanded', open ? 'true' : 'false');
            }
            return;
        }
        if (!e.target.closest('[data-dots-menu]')) {
            closeDotsMenus();
        }
    });

    // A destination picked in the ⋯ menu navigates straight there — the
    // <select> stands in for a submit button (2.6.D2).
    document.addEventListener('change', function (e) {
        var select = e.target.closest('[data-dest-select]');
        if (select && select.value && select.form) {
            select.form.submit();
            return;
        }
        // A plain filter select that submits its own form on every change,
        // placeholder option included — unlike [data-dest-select] above,
        // which never submits on the placeholder (2.6.G6).
        var auto = e.target.closest('[data-auto-submit]');
        if (auto && auto.form) {
            auto.form.submit();
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
            var selected = document.querySelector('.m-row.is-on a.detail-link');
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

// Appearance: theme and density in localStorage (wave 1.10).
(function () {
    var THEME_KEY = 'carrel:theme';
    var DENSITY_KEY = 'carrel:density';

    function readTheme() {
        try {
            return localStorage.getItem(THEME_KEY) || 'auto';
        } catch (e) {
            return 'auto';
        }
    }

    function readDensity() {
        try {
            return localStorage.getItem(DENSITY_KEY) || 'comfortable';
        } catch (e) {
            return 'comfortable';
        }
    }

    function applyTheme(value) {
        if (value === 'light' || value === 'dark') {
            document.documentElement.dataset.theme = value;
        } else {
            delete document.documentElement.dataset.theme;
        }
        try {
            localStorage.setItem(THEME_KEY, value || 'auto');
        } catch (e) {}
        document.querySelectorAll('[data-theme-segment]').forEach(function (group) {
            group.querySelectorAll('[data-theme-value]').forEach(function (btn) {
                btn.classList.toggle('is-on', btn.getAttribute('data-theme-value') === (value || 'auto'));
            });
        });
    }

    function applyDensity(value) {
        if (value === 'compact') {
            document.documentElement.dataset.density = 'compact';
        } else {
            delete document.documentElement.dataset.density;
        }
        try {
            localStorage.setItem(DENSITY_KEY, value || 'comfortable');
        } catch (e) {}
        document.querySelectorAll('[data-density-segment]').forEach(function (group) {
            group.querySelectorAll('[data-density-value]').forEach(function (btn) {
                btn.classList.toggle('is-on', btn.getAttribute('data-density-value') === (value || 'comfortable'));
            });
        });
    }

    document.addEventListener('click', function (e) {
        var themeBtn = e.target.closest('[data-theme-value]');
        if (themeBtn && themeBtn.closest('[data-theme-segment]')) {
            e.preventDefault();
            applyTheme(themeBtn.getAttribute('data-theme-value'));
            return;
        }
        var densityBtn = e.target.closest('[data-density-value]');
        if (densityBtn && densityBtn.closest('[data-density-segment]')) {
            e.preventDefault();
            applyDensity(densityBtn.getAttribute('data-density-value'));
        }
    });

    applyTheme(readTheme());
    applyDensity(readDensity());
})();

// User menu in the header (wave 1.10).
(function () {
    function menu() {
        return document.querySelector('.app-user-dropdown');
    }

    function closeMenu() {
        var el = menu();
        var toggle = document.querySelector('[data-user-menu-toggle]');
        if (el) el.hidden = true;
        if (toggle) toggle.setAttribute('aria-expanded', 'false');
    }

    document.addEventListener('click', function (e) {
        var toggle = e.target.closest('[data-user-menu-toggle]');
        if (toggle) {
            e.preventDefault();
            var el = menu();
            if (!el) return;
            var open = el.hidden;
            el.hidden = !open;
            toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
            return;
        }
        if (!e.target.closest('.app-user-menu')) {
            closeMenu();
        }
    });
})();

// Column picker per list/table (wave 1.11).
(function () {
    var STORAGE_PREFIX = 'carrel:columns:';

    var PRESETS = {
        contacts: {
            locked: 'name',
            pinned: ['bar'],
            always: [],
            columns: [
                { id: 'photo', label: 'Photo', width: '28px' },
                { id: 'name', label: 'Name', width: 'minmax(150px, 1.4fr)', locked: true },
                { id: 'phone', label: 'Phone', width: '142px' },
                { id: 'email', label: 'Email', width: 'minmax(150px, 1.1fr)' }
            ],
            lead: [{ id: 'bar', width: '3px' }]
        },
        tasks: {
            locked: 'name',
            pinned: ['done', 'bar'],
            always: [],
            columns: [
                { id: 'name', label: 'Title', width: 'minmax(190px, 2fr)', locked: true },
                { id: 'tags', label: 'Tags', width: 'minmax(90px, auto)' },
                { id: 'due', label: 'Due', width: '84px' },
                { id: 'progress', label: 'Progress', width: '44px' }
            ],
            lead: [
                { id: 'done', width: '14px' },
                { id: 'bar', width: '3px' }
            ]
        },
        notes: {
            locked: 'name',
            pinned: ['bar'],
            always: [],
            columns: [
                { id: 'name', label: 'Title', width: 'minmax(140px, 1.2fr)', locked: true },
                { id: 'excerpt', label: 'Excerpt', width: 'minmax(170px, 2fr)' },
                { id: 'tags', label: 'Tags', width: 'minmax(70px, auto)' },
                { id: 'date', label: 'Date', width: '80px' }
            ],
            lead: [{ id: 'bar', width: '3px' }]
        },
        files: {
            locked: 'name',
            always: ['actions'],
            columns: [
                { id: 'name', label: 'Name', width: 'minmax(220px, 2.4fr)', locked: true },
                { id: 'size', label: 'Size', width: '90px' },
                { id: 'type', label: 'Type', width: '130px' },
                { id: 'changed', label: 'Changed', width: '130px' },
                { id: 'actions', label: 'Actions', width: '60px', always: true }
            ],
            table: true
        },
        'admin-users': {
            locked: 'login',
            always: ['actions'],
            columns: [
                { id: 'login', label: 'Login', locked: true },
                { id: 'role', label: 'Role' },
                { id: 'created', label: 'Created' },
                { id: 'last_login', label: 'Last login' },
                { id: 'dav', label: 'DAV' },
                { id: 'sessions', label: 'Sessions' },
                { id: 'actions', label: 'Actions', always: true }
            ],
            table: true
        },
        'admin-audit': {
            locked: 'when',
            columns: [
                { id: 'when', label: 'When', locked: true },
                { id: 'action', label: 'Action' },
                { id: 'actor', label: 'Actor' },
                { id: 'target', label: 'Target' },
                { id: 'detail', label: 'Detail' }
            ],
            table: true
        }
    };

    function loadState(id, preset) {
        try {
            var raw = localStorage.getItem(STORAGE_PREFIX + id);
            if (raw) {
                return JSON.parse(raw);
            }
        } catch (e) {}
        return {
            order: preset.columns.map(function (c) { return c.id; }),
            visible: preset.columns.reduce(function (acc, col) {
                acc[col.id] = true;
                return acc;
            }, {})
        };
    }

    function saveState(id, state) {
        try {
            localStorage.setItem(STORAGE_PREFIX + id, JSON.stringify(state));
        } catch (e) {}
    }

    function columnMap(preset) {
        var map = {};
        preset.columns.forEach(function (col) {
            map[col.id] = col;
        });
        return map;
    }

    function visibleOrder(state, preset) {
        var cols = columnMap(preset);
        return state.order.filter(function (id) {
            var col = cols[id];
            return col && (col.locked || col.always || state.visible[id] !== false);
        });
    }

    function applyList(root, preset, state) {
        var order = visibleOrder(state, preset);
        var widths = [];
        (preset.lead || []).forEach(function (lead) {
            widths.push(lead.width);
        });
        order.forEach(function (id) {
            var col = columnMap(preset)[id];
            if (col && col.width) {
                widths.push(col.width);
            }
        });
        root.style.setProperty('--list-cols', widths.join(' '));

        var visible = {};
        order.forEach(function (id) { visible[id] = true; });
        preset.columns.forEach(function (col) {
            if (!visible[col.id]) {
                root.querySelectorAll('[data-col="' + col.id + '"]').forEach(function (node) {
                    node.classList.add('is-col-hidden');
                });
            }
        });
        (preset.lead || []).forEach(function (lead) {
            root.querySelectorAll('[data-col="' + lead.id + '"]').forEach(function (node) {
                node.classList.remove('is-col-hidden');
            });
        });
        order.forEach(function (id) {
            root.querySelectorAll('[data-col="' + id + '"]').forEach(function (node) {
                node.classList.remove('is-col-hidden');
            });
        });
    }

    function applyTable(root, preset, state) {
        var order = visibleOrder(state, preset);
        var visible = {};
        order.forEach(function (id) { visible[id] = true; });
        preset.columns.forEach(function (col) {
            var hide = !visible[col.id];
            root.querySelectorAll('[data-col="' + col.id + '"]').forEach(function (node) {
                node.classList.toggle('is-col-hidden', hide);
            });
        });
    }

    function applyColumns(id) {
        var preset = PRESETS[id];
        if (!preset) return;
        var state = loadState(id, preset);
        document.querySelectorAll('[data-columns-root="' + id + '"]').forEach(function (root) {
            preset.columns.forEach(function (col) {
                root.querySelectorAll('[data-col="' + col.id + '"]').forEach(function (node) {
                    node.classList.remove('is-col-hidden');
                });
            });
            if (preset.table) {
                applyTable(root, preset, state);
            } else {
                applyList(root, preset, state);
            }
        });
        document.querySelectorAll('[data-columns-id="' + id + '"] [data-columns-count]').forEach(function (el) {
            el.textContent = String(visibleOrder(state, preset).length);
        });
    }

    function renderMenu(picker, id) {
        var preset = PRESETS[id];
        if (!preset) return;
        var menu = picker.querySelector('[data-columns-menu]');
        if (!menu) return;
        var state = loadState(id, preset);
        var cols = columnMap(preset);
        menu.innerHTML = '';
        var rubric = document.createElement('div');
        rubric.className = 'column-picker-rubric';
        rubric.textContent = 'Columns';
        menu.appendChild(rubric);

        state.order.forEach(function (colId) {
            var col = cols[colId];
            if (!col) return;
            var row = document.createElement('div');
            row.className = 'column-picker-row';
            row.draggable = !col.locked;
            row.dataset.colId = colId;

            var drag = document.createElement('span');
            drag.className = 'column-picker-drag';
            drag.textContent = '≡';
            drag.setAttribute('aria-hidden', 'true');
            row.appendChild(drag);

            var box = document.createElement('span');
            var on = col.locked || col.always || state.visible[colId] !== false;
            box.className = 'column-picker-check' + (on ? '' : ' is-off') + (col.locked ? ' is-locked' : '');
            row.appendChild(box);

            var label = document.createElement('span');
            label.textContent = col.label;
            row.appendChild(label);

            if (col.locked) {
                var always = document.createElement('span');
                always.className = 'column-picker-always';
                always.textContent = 'always';
                row.appendChild(always);
            }

            if (!col.locked && !col.always) {
                row.addEventListener('click', function (e) {
                    if (e.target.closest('.column-picker-drag')) return;
                    state.visible[colId] = state.visible[colId] !== false ? false : true;
                    saveState(id, state);
                    applyColumns(id);
                    renderMenu(picker, id);
                });
            }

            row.addEventListener('dragstart', function (e) {
                if (col.locked) {
                    e.preventDefault();
                    return;
                }
                row.classList.add('is-dragging');
                e.dataTransfer.setData('text/plain', colId);
            });
            row.addEventListener('dragend', function () {
                row.classList.remove('is-dragging');
            });
            row.addEventListener('dragover', function (e) {
                e.preventDefault();
            });
            row.addEventListener('drop', function (e) {
                e.preventDefault();
                var fromId = e.dataTransfer.getData('text/plain');
                if (!fromId || fromId === colId) return;
                var fromIdx = state.order.indexOf(fromId);
                var toIdx = state.order.indexOf(colId);
                if (fromIdx < 0 || toIdx < 0) return;
                state.order.splice(fromIdx, 1);
                state.order.splice(toIdx, 0, fromId);
                saveState(id, state);
                applyColumns(id);
                renderMenu(picker, id);
            });

            menu.appendChild(row);
        });

        var reset = document.createElement('button');
        reset.type = 'button';
        reset.className = 'column-picker-reset';
        reset.textContent = 'Reset to defaults';
        reset.addEventListener('click', function () {
            try {
                localStorage.removeItem(STORAGE_PREFIX + id);
            } catch (e) {}
            applyColumns(id);
            renderMenu(picker, id);
        });
        menu.appendChild(reset);
    }

    function initPickers() {
        document.querySelectorAll('[data-columns-id]').forEach(function (picker) {
            var id = picker.getAttribute('data-columns-id');
            renderMenu(picker, id);
            applyColumns(id);
            var toggle = picker.querySelector('[data-columns-toggle]');
            if (toggle) {
                toggle.addEventListener('click', function (e) {
                    e.preventDefault();
                    e.stopPropagation();
                    var menu = picker.querySelector('[data-columns-menu]');
                    if (!menu) return;
                    var open = menu.hidden;
                    document.querySelectorAll('[data-columns-menu]').forEach(function (other) {
                        other.hidden = true;
                    });
                    menu.hidden = !open;
                    toggle.setAttribute('aria-expanded', open ? 'true' : 'false');
                });
            }
        });
    }

    document.addEventListener('click', function (e) {
        if (!e.target.closest('.column-picker')) {
            document.querySelectorAll('[data-columns-menu]').forEach(function (menu) {
                menu.hidden = true;
            });
            document.querySelectorAll('[data-columns-toggle]').forEach(function (btn) {
                btn.setAttribute('aria-expanded', 'false');
            });
        }
    });

    document.addEventListener('click', function (e) {
        if (e.target.closest('[data-reset-columns]')) {
            Object.keys(PRESETS).forEach(function (id) {
                try {
                    localStorage.removeItem(STORAGE_PREFIX + id);
                } catch (err) {}
                applyColumns(id);
            });
            document.querySelectorAll('[data-columns-id]').forEach(function (picker) {
                renderMenu(picker, picker.getAttribute('data-columns-id'));
            });
        }
    });

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initPickers);
    } else {
        initPickers();
    }
})();

// Wave 1.18 — drag-to-crop photo (§1.18). The canvas overlay lets a person
// drag to pan and scroll to zoom instead of typing numbers. The hidden form
// fields are kept in sync so Update preview sends the right values. On narrow
// screens the canvas is hidden by CSS and the number fields remain.
(function () {
    function initCrop(root) {
        var wrap = root.querySelector('[data-crop-canvas]');
        var img = root.querySelector('[data-crop-img]');
        var frame = root.querySelector('[data-crop-frame]');
        var result = root.querySelector('[data-crop-result]');
        if (!wrap || !img || !frame) return;

        var panXIn = root.querySelector('[data-crop-pan-x]');
        var panYIn = root.querySelector('[data-crop-pan-y]');
        var zoomIn = root.querySelector('[data-crop-zoom]');
        var rotateIn = root.querySelector('[data-crop-rotate]');

        var state = {
            panX: parseFloat(panXIn.value) || 0,
            panY: parseFloat(panYIn.value) || 0,
            zoom: parseFloat(zoomIn.value) || 1,
            rotate: parseInt(rotateIn.value, 10) || 0
        };

        var imgW = 0, imgH = 0;

        function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }

        function sync() {
            state.panX = clamp(state.panX, -1, 1);
            state.panY = clamp(state.panY, -1, 1);
            state.zoom = clamp(state.zoom, 1, 8);
            panXIn.value = state.panX.toFixed(2);
            panYIn.value = state.panY.toFixed(2);
            zoomIn.value = state.zoom.toFixed(1);
            rotateIn.value = String(state.rotate);
            var nums = root.querySelectorAll('[data-crop-num]');
            for (var i = 0; i < nums.length; i++) {
                var key = nums[i].getAttribute('data-crop-num');
                if (key === 'pan_x') nums[i].value = state.panX.toFixed(2);
                else if (key === 'pan_y') nums[i].value = state.panY.toFixed(2);
                else if (key === 'zoom') nums[i].value = state.zoom.toFixed(1);
                else if (key === 'rotate') nums[i].value = String(state.rotate);
            }
            render();
        }

        function render() {
            if (!imgW || !imgH) return;
            var wrapW = wrap.clientWidth, wrapH = wrap.clientHeight;
            var fitScale = Math.min(wrapW / imgW, wrapH / imgH);
            var dispW = imgW * fitScale, dispH = imgH * fitScale;
            img.style.width = dispW + 'px';
            img.style.height = dispH + 'px';
            var ox = (wrapW - dispW) / 2, oy = (wrapH - dispH) / 2;
            img.style.left = ox + 'px';
            img.style.top = oy + 'px';
            img.style.transform = '';

            var side = Math.min(dispW, dispH) / state.zoom;
            var cx = dispW / 2 + state.panX * ((dispW - side) / 2);
            var cy = dispH / 2 + state.panY * ((dispH - side) / 2);
            frame.style.left = (ox + cx - side / 2) + 'px';
            frame.style.top = (oy + cy - side / 2) + 'px';
            frame.style.width = side + 'px';
            frame.style.height = side + 'px';
        }

        img.addEventListener('load', function () {
            imgW = img.naturalWidth;
            imgH = img.naturalHeight;
            render();
        });
        if (img.naturalWidth) {
            imgW = img.naturalWidth;
            imgH = img.naturalHeight;
            render();
        }

        var dragging = false, startX = 0, startY = 0, startPanX = 0, startPanY = 0;

        wrap.addEventListener('pointerdown', function (e) {
            if (e.button && e.button !== 0) return;
            dragging = true;
            startX = e.clientX;
            startY = e.clientY;
            startPanX = state.panX;
            startPanY = state.panY;
            wrap.setPointerCapture(e.pointerId);
            e.preventDefault();
        });

        wrap.addEventListener('pointermove', function (e) {
            if (!dragging || !imgW) return;
            var wrapW = wrap.clientWidth, wrapH = wrap.clientHeight;
            var fitScale = Math.min(wrapW / imgW, wrapH / imgH);
            var dispW = imgW * fitScale, dispH = imgH * fitScale;
            var side = Math.min(dispW, dispH) / state.zoom;
            var rangeX = (dispW - side) / 2;
            var rangeY = (dispH - side) / 2;
            var dx = e.clientX - startX, dy = e.clientY - startY;
            state.panX = rangeX > 0 ? clamp(startPanX + dx / rangeX, -1, 1) : 0;
            state.panY = rangeY > 0 ? clamp(startPanY + dy / rangeY, -1, 1) : 0;
            sync();
        });

        wrap.addEventListener('pointerup', function () { dragging = false; });
        wrap.addEventListener('pointercancel', function () { dragging = false; });

        wrap.addEventListener('wheel', function (e) {
            e.preventDefault();
            state.zoom = clamp(state.zoom - e.deltaY * 0.005, 1, 8);
            sync();
        }, { passive: false });

        root.addEventListener('change', function (e) {
            var num = e.target.closest('[data-crop-num]');
            if (!num) return;
            var key = num.getAttribute('data-crop-num');
            var val = key === 'rotate' ? parseInt(num.value, 10) : parseFloat(num.value);
            if (key === 'pan_x') state.panX = val;
            else if (key === 'pan_y') state.panY = val;
            else if (key === 'zoom') state.zoom = val;
            else if (key === 'rotate') state.rotate = val;
            sync();
        });

        sync();
    }

    function run() {
        document.querySelectorAll('#photo-crop').forEach(initCrop);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', run);
    } else {
        run();
    }

    document.body.addEventListener('htmx:afterSwap', function (e) {
        if (e.detail.target && e.detail.target.id === 'photo-crop') {
            initCrop(e.detail.target);
        }
        var crops = e.detail.target ? e.detail.target.querySelectorAll('#photo-crop') : [];
        crops.forEach(initCrop);
    });
})();
// Wave 1.17 — the slide-out panel on a narrow screen holds the source rail
// (§13 asks for that rail, not the section links, to move out of the way) and
// the section links below it. Without JavaScript the source rail stays in the
// flow of the page, which is why the `js` class gates hiding it.
(function () {
    var narrow = window.matchMedia('(max-width: 640px)');
    var home = null;

    function rail() {
        return document.querySelector('[data-app-rail]');
    }

    function scrim() {
        return document.querySelector('[data-rail-scrim]');
    }

    // The source rail is rendered inside the page, next to the list. On a
    // narrow screen it is borrowed by the panel and put back afterwards, so
    // the wide layout never sees it move.
    function mountSources() {
        var node = document.querySelector('[data-section-rail]');
        var mount = document.querySelector('[data-rail-mount]');
        var title = document.querySelector('[data-rail-title]');
        if (!node || !mount || node.parentNode === mount) return;
        home = { parent: node.parentNode, next: node.nextSibling };
        mount.appendChild(node);
        if (title) title.textContent = 'Sources';
    }

    function unmountSources() {
        var mount = document.querySelector('[data-rail-mount]');
        var title = document.querySelector('[data-rail-title]');
        var node = mount ? mount.querySelector('[data-section-rail]') : null;
        if (node && home) home.parent.insertBefore(node, home.next);
        home = null;
        if (title) title.textContent = 'Menu';
    }

    function openRail() {
        var el = rail();
        if (!el) return;
        mountSources();
        el.classList.add('is-open');
        var back = scrim();
        if (back) back.hidden = false;
        var toggle = document.querySelector('[data-rail-toggle]');
        if (toggle) toggle.setAttribute('aria-expanded', 'true');
        var first = el.querySelector('[data-rail-close]');
        if (first) first.focus();
    }

    function closeRail() {
        var el = rail();
        if (el) el.classList.remove('is-open');
        var back = scrim();
        if (back) back.hidden = true;
        var toggle = document.querySelector('[data-rail-toggle]');
        if (toggle) toggle.setAttribute('aria-expanded', 'false');
        unmountSources();
    }

    function railOpen() {
        var el = rail();
        return !!el && el.classList.contains('is-open');
    }

    document.addEventListener('click', function (e) {
        if (e.target.closest('[data-rail-toggle]')) {
            if (railOpen()) closeRail(); else openRail();
            return;
        }
        if (e.target.closest('[data-rail-scrim]') || e.target.closest('[data-rail-close]')) {
            closeRail();
            return;
        }
        // A section link inside the panel navigates; nothing to put back.
        if (railOpen() && e.target.closest('[data-app-rail] a')) {
            var el = rail();
            if (el) el.classList.remove('is-open');
        }
    });

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape' && railOpen()) {
            e.preventDefault();
            closeRail();
        }
    });

    // Growing past the breakpoint while the panel is open would leave the
    // source rail parked in a nav that is a column again.
    function onWidthChange() {
        if (!narrow.matches && railOpen()) closeRail();
    }

    if (narrow.addEventListener) narrow.addEventListener('change', onWidthChange);
    else if (narrow.addListener) narrow.addListener(onWidthChange);
})();

// Wave 1.17 — no properties panel on a narrow screen: the row opens the record
// itself, which is what its href points at anyway.
(function () {
    var narrow = window.matchMedia('(max-width: 640px)');
    document.body.addEventListener('htmx:beforeRequest', function (e) {
        var el = e.detail && e.detail.elt;
        if (!narrow.matches || !el || !el.classList.contains('detail-link')) return;
        if (el.getAttribute('hx-target') !== '#app-details') return;
        e.preventDefault();
        var href = el.getAttribute('href');
        if (href) window.location.assign(href);
    });
})();

// Wave 2.1 — full-screen note: width, focus, markup, unsaved warning.
(function () {
    var WIDTH_KEY = 'carrel:note-width';
    var doc = document.querySelector('[data-note-doc]');
    var screen = document.querySelector('.note-screen');
    if (!doc) return;

    function readWidth() {
        try {
            return localStorage.getItem(WIDTH_KEY) || 'full';
        } catch (e) {
            return 'full';
        }
    }

    function applyWidth(value) {
        var reading = value === 'reading';
        doc.classList.toggle('is-reading-width', reading);
        document.querySelectorAll('[data-note-width-seg]').forEach(function (seg) {
            seg.querySelectorAll('[data-note-width]').forEach(function (btn) {
                var on = btn.getAttribute('data-note-width') === (reading ? 'reading' : 'full');
                btn.classList.toggle('is-on', on);
            });
        });
        try {
            localStorage.setItem(WIDTH_KEY, reading ? 'reading' : 'full');
        } catch (e) {}
    }

    applyWidth(readWidth());

    document.addEventListener('click', function (e) {
        var widthBtn = e.target.closest('[data-note-width]');
        if (widthBtn) {
            applyWidth(widthBtn.getAttribute('data-note-width'));
            return;
        }
        var focusBtn = e.target.closest('[data-note-focus]');
        if (focusBtn && screen) {
            screen.classList.toggle('is-focus');
            return;
        }
        var metaBtn = e.target.closest('[data-note-meta-toggle]');
        if (metaBtn) {
            doc.classList.toggle('is-meta-hidden');
            return;
        }
        var sourceBtn = e.target.closest('[data-note-source-toggle]');
        if (sourceBtn) {
            doc.classList.toggle('is-source-on');
            return;
        }
        var markup = e.target.closest('[data-note-markup]');
        if (markup) {
            insertMarkup(markup.getAttribute('data-note-markup'));
        }
    });

    function insertMarkup(kind) {
        var area = document.getElementById('description');
        if (!area) return;
        var start = area.selectionStart;
        var end = area.selectionEnd;
        var text = area.value;
        var sel = text.slice(start, end);
        var insert = sel;
        var cursor = start;
        switch (kind) {
        case 'bold':
            insert = '**' + (sel || 'text') + '**';
            cursor = start + 2 + (sel || 'text').length + 2;
            break;
        case 'italic':
            insert = '_' + (sel || 'text') + '_';
            cursor = start + 1 + (sel || 'text').length + 1;
            break;
        case 'h2':
            insert = '## ' + (sel || 'Heading');
            cursor = start + insert.length;
            break;
        case 'list':
            insert = '- ' + (sel || 'item');
            cursor = start + insert.length;
            break;
        case 'task':
            insert = '- [ ] ' + (sel || 'task');
            cursor = start + insert.length;
            break;
        case 'link':
            insert = '[' + (sel || 'text') + '](url)';
            cursor = start + insert.length - 5;
            break;
        }
        area.value = text.slice(0, start) + insert + text.slice(end);
        area.focus();
        area.setSelectionRange(cursor, cursor);
        area.dispatchEvent(new Event('input', { bubbles: true }));
    }

    var form = document.querySelector('[data-note-form]');
    if (form) {
        var snapshot = formSnapshot(form);
        form.addEventListener('input', function () {
            form.dataset.dirty = formSnapshot(form) !== snapshot ? '1' : '';
        });
        form.addEventListener('submit', function () {
            form.dataset.dirty = '';
        });
        window.addEventListener('beforeunload', function (e) {
            if (form.dataset.dirty === '1') {
                e.preventDefault();
                e.returnValue = '';
            }
        });
        document.addEventListener('keydown', function (e) {
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                if (!form.contains(document.activeElement) && document.activeElement !== form) return;
                e.preventDefault();
                form.requestSubmit();
            }
            if (e.key === 'Escape' && screen && screen.classList.contains('is-focus')) {
                e.preventDefault();
                screen.classList.remove('is-focus');
            }
        });
    }

    function formSnapshot(f) {
        var parts = [];
        f.querySelectorAll('input, textarea, select').forEach(function (el) {
            if (!el.name) return;
            if (el.type === 'hidden' && el.name !== 'related') return;
            parts.push(el.name + '=' + el.value);
        });
        return parts.join('&');
    }
})();

// Wave 2.3 — related-to typeahead on the note editor.
(function () {
    var picker = document.querySelector('[data-related-picker]');
    if (!picker) return;

    var input = picker.querySelector('[data-related-input]');
    var chips = picker.querySelector('[data-related-chips]');
    var query = picker.querySelector('[data-related-query]');
    var menu = picker.querySelector('[data-related-menu]');
    var searchURL = picker.getAttribute('data-search-url');
    var excludeUID = picker.getAttribute('data-exclude-uid') || '';
    var timer = 0;
    var active = -1;

    function selectedUIDs() {
        var out = [];
        chips.querySelectorAll('[data-related-chip]').forEach(function (chip) {
            var uid = chip.getAttribute('data-uid');
            if (uid) out.push(uid);
        });
        return out;
    }

    function syncInput() {
        input.value = selectedUIDs().join(', ');
        input.dispatchEvent(new Event('input', { bubbles: true }));
    }

    function addChip(uid, title) {
        uid = (uid || '').trim();
        if (!uid) return;
        if (selectedUIDs().indexOf(uid) >= 0) return;
        var chip = document.createElement('span');
        chip.className = 'related-chip';
        chip.setAttribute('data-related-chip', '');
        chip.setAttribute('data-uid', uid);
        chip.setAttribute('data-title', title || uid);
        var label = document.createElement('span');
        label.className = 'related-chip-label';
        label.textContent = title || uid;
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'related-chip-remove';
        btn.setAttribute('data-related-remove', '');
        btn.setAttribute('aria-label', 'Remove');
        btn.textContent = '×';
        chip.appendChild(label);
        chip.appendChild(btn);
        chips.appendChild(chip);
        syncInput();
    }

    function closeMenu() {
        menu.hidden = true;
        query.setAttribute('aria-expanded', 'false');
        active = -1;
        menu.innerHTML = '';
    }

    function kindLabel(kind) {
        switch (kind) {
        case 'event': return 'Event';
        case 'task': return 'Task';
        case 'note': return 'Note';
        case 'contact': return 'Contact';
        default: return kind;
        }
    }

    function renderMenu(items, q) {
        menu.innerHTML = '';
        items.forEach(function (item, index) {
            var row = document.createElement('button');
            row.type = 'button';
            row.className = 'related-picker-option';
            row.setAttribute('role', 'option');
            row.setAttribute('data-related-option', '');
            row.setAttribute('data-uid', item.uid);
            row.setAttribute('data-title', item.title);
            row.innerHTML = '<span class="related-picker-kind">' + kindLabel(item.kind) + '</span>' +
                '<span class="related-picker-title">' + escapeHTML(item.title) + '</span>' +
                (item.meta ? '<span class="related-picker-meta">' + escapeHTML(item.meta) + '</span>' : '');
            row.addEventListener('mousedown', function (e) {
                e.preventDefault();
            });
            row.addEventListener('click', function () {
                addChip(item.uid, item.title);
                query.value = '';
                closeMenu();
                query.focus();
            });
            menu.appendChild(row);
        });
        if (q) {
            var sep = document.createElement('div');
            sep.className = 'related-picker-sep';
            sep.setAttribute('role', 'separator');
            menu.appendChild(sep);
            var manual = document.createElement('button');
            manual.type = 'button';
            manual.className = 'related-picker-option is-manual';
            manual.setAttribute('role', 'option');
            manual.setAttribute('data-related-manual', '');
            manual.textContent = 'Enter UID by hand…';
            manual.addEventListener('mousedown', function (e) {
                e.preventDefault();
            });
            manual.addEventListener('click', function () {
                addChip(q, q);
                query.value = '';
                closeMenu();
                query.focus();
            });
            menu.appendChild(manual);
        }
        menu.hidden = items.length === 0 && !q;
        query.setAttribute('aria-expanded', menu.hidden ? 'false' : 'true');
        active = -1;
    }

    function escapeHTML(text) {
        return String(text)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;');
    }

    function fetchMatches(q) {
        if (!searchURL || !q) {
            closeMenu();
            return;
        }
        var url = searchURL + '?q=' + encodeURIComponent(q);
        if (excludeUID) url += '&exclude=' + encodeURIComponent(excludeUID);
        fetch(url, { credentials: 'same-origin', headers: { Accept: 'application/json' } })
            .then(function (res) { return res.ok ? res.json() : []; })
            .then(function (items) {
                if (query.value.trim() !== q) return;
                var selected = selectedUIDs();
                var filtered = (items || []).filter(function (item) {
                    return selected.indexOf(item.uid) < 0;
                });
                renderMenu(filtered, q);
            })
            .catch(function () {
                renderMenu([], q);
            });
    }

    query.addEventListener('input', function () {
        var q = query.value.trim();
        clearTimeout(timer);
        if (!q) {
            closeMenu();
            return;
        }
        timer = setTimeout(function () {
            fetchMatches(q);
        }, 200);
    });

    query.addEventListener('keydown', function (e) {
        var options = menu.querySelectorAll('[data-related-option], [data-related-manual]');
        if (e.key === 'ArrowDown') {
            if (menu.hidden) {
                fetchMatches(query.value.trim());
                return;
            }
            e.preventDefault();
            active = Math.min(active + 1, options.length - 1);
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            active = Math.max(active - 1, 0);
        } else if (e.key === 'Enter') {
            if (active >= 0 && options[active]) {
                e.preventDefault();
                options[active].click();
            }
            return;
        } else if (e.key === 'Escape') {
            closeMenu();
            return;
        } else {
            return;
        }
        options.forEach(function (opt, i) {
            opt.classList.toggle('is-active', i === active);
        });
    });

    picker.addEventListener('click', function (e) {
        var remove = e.target.closest('[data-related-remove]');
        if (remove) {
            var chip = remove.closest('[data-related-chip]');
            if (chip) chip.remove();
            syncInput();
        }
    });

    document.addEventListener('click', function (e) {
        if (!picker.contains(e.target)) closeMenu();
    });
})();

// Wave 2.4 — file manager: selection, ops bar, filter, properties panel.
(function () {
    var browse = document.querySelector('[data-files-browse]');
    if (!browse) return;

    var STORAGE_KEY = 'carrel:files-props';
    var layout = document.querySelector('[data-files-layout]');
    var list = browse.querySelector('[data-files-list]');
    var grid = browse.querySelector('[data-files-grid]');
    var opsBar = browse.querySelector('[data-files-ops]');
    var opsCount = browse.querySelector('[data-files-ops-count]');
    var opsSize = browse.querySelector('[data-files-ops-size]');
    var filterInput = browse.querySelector('[data-files-filter]');
    var toolbar = browse.querySelector('[data-files-toolbar]');
    var deleteForm = browse.querySelector('[data-files-delete-form]');
    var moveForm = browse.querySelector('[data-files-move-form]');
    var copyForm = browse.querySelector('[data-files-copy-form]');
    var renameForm = browse.querySelector('[data-files-rename-form]');
    var moveDialog = browse.querySelector('[data-files-move-dialog]');
    var picker = moveDialog && moveDialog.querySelector('[data-folder-picker]');
    var pickerConfirm = picker && picker.querySelector('[data-picker-confirm]');
    var pickerTitle = picker && picker.querySelector('.folder-picker-title');
    var pickerWarn = picker && picker.querySelector('[data-picker-warn]');
    var pickerSelection = null;
    var pickerMode = 'move';
    var srcAccount = browse.getAttribute('data-account');
    var srcCol = browse.getAttribute('data-col');
    var propsPanel = document.getElementById('files-props');
    var propsBody = propsPanel && propsPanel.querySelector('[data-files-props-body]');
    var propsTitle = propsPanel && propsPanel.querySelector('[data-files-props-title]');
    var readonly = browse.getAttribute('data-readonly') === '1';
    var narrow = window.matchMedia('(max-width: 640px)');

    function rows() {
        return Array.prototype.slice.call(browse.querySelectorAll('[data-file-row]'));
    }

    function rowChecks() {
        return Array.prototype.slice.call(browse.querySelectorAll('[data-files-check]'));
    }

    function syncRowSelection(rel, on) {
        browse.querySelectorAll('[data-file-row][data-rel="' + rel + '"]').forEach(function (row) {
            row.classList.toggle('is-selected', on);
            var cb = row.querySelector('[data-files-check]');
            if (cb) cb.checked = on;
        });
    }

    function selectedRows() {
        var seen = {};
        var out = [];
        rows().forEach(function (row) {
            var rel = row.getAttribute('data-rel');
            if (!rel || seen[rel]) return;
            var cb = row.querySelector('[data-files-check]');
            if (cb && cb.checked) {
                seen[rel] = true;
                out.push(row);
            }
        });
        return out;
    }

    function formatSize(bytes) {
        bytes = Number(bytes) || 0;
        if (bytes < 1024) return bytes + ' B';
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' kB';
        if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
        return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
    }

    function propsClosed() {
        try {
            return localStorage.getItem(STORAGE_KEY) === 'closed';
        } catch (e) {
            return false;
        }
    }

    function setPropsClosed(closed) {
        try {
            if (closed) localStorage.setItem(STORAGE_KEY, 'closed');
            else localStorage.removeItem(STORAGE_KEY);
        } catch (e) { /* ignore */ }
    }

    function showProps() {
        if (!propsPanel || narrow.matches) return;
        propsPanel.hidden = false;
        if (layout) layout.classList.add('has-files-props');
    }

    function hideProps(manual) {
        if (!propsPanel) return;
        propsPanel.hidden = true;
        if (layout) layout.classList.remove('has-files-props');
        if (propsBody) propsBody.innerHTML = '';
        if (manual) setPropsClosed(true);
    }

    function renderProps(row) {
        if (!propsBody || !propsTitle || !row) return;
        var name = row.getAttribute('data-name') || '';
        var isDir = row.getAttribute('data-dir') === 'true';
        propsTitle.textContent = name;
        var html = '';
        if (!isDir && row.getAttribute('data-download')) {
            html += '<div class="files-props-actions"><a class="link" href="' + row.getAttribute('data-download') + '">Download</a></div>';
        }
        html += '<dl class="m-fields">';
        if (!isDir) {
            html += '<dt>Size</dt><dd>' + (row.getAttribute('data-size') ? formatSize(row.getAttribute('data-size')) : '—') + '</dd>';
            html += '<dt>Type</dt><dd>' + (row.getAttribute('data-type') || '—') + '</dd>';
        }
        html += '<dt>Changed</dt><dd>' + (row.getAttribute('data-changed') || '—') + '</dd>';
        html += '<dt>Path</dt><dd>' + (row.getAttribute('data-rel') || '') + '</dd>';
        if (!isDir && row.getAttribute('data-etag')) {
            html += '<dt>ETag</dt><dd>' + row.getAttribute('data-etag') + '</dd>';
        }
        html += '</dl>';
        html += '<p class="hint">Carrel keeps no index of what points at this file. It may be attached to a note or an event — check before deleting.</p>';
        propsBody.innerHTML = html;
    }

    function updateSelection() {
        var sel = selectedRows();
        rows().forEach(function (row) {
            var rel = row.getAttribute('data-rel');
            var cb = row.querySelector('[data-files-check]');
            var on = cb && cb.checked;
            syncRowSelection(rel, on);
        });
        if (sel.length === 0) {
            if (opsBar) opsBar.hidden = true;
            if (toolbar) toolbar.hidden = false;
            hideProps(false);
            return;
        }
        if (opsBar) opsBar.hidden = false;
        if (toolbar) toolbar.hidden = true;
        var total = 0;
        sel.forEach(function (row) {
            total += Number(row.getAttribute('data-size')) || 0;
        });
        if (opsCount) opsCount.textContent = sel.length + (sel.length === 1 ? ' selected' : ' selected');
        if (opsSize) opsSize.textContent = total > 0 ? formatSize(total) : '';
        var renameBtn = browse.querySelector('[data-files-op="rename"]');
        if (renameBtn) renameBtn.disabled = readonly || sel.length !== 1;
        if (sel.length === 1 && !propsClosed() && !narrow.matches) {
            renderProps(sel[0]);
            showProps();
        } else {
            hideProps(false);
        }
        var all = rowChecks();
        var selectAll = list.querySelector('[data-files-select-all]');
        if (selectAll) {
            var checked = all.filter(function (cb) { return cb.checked; }).length;
            selectAll.checked = all.length > 0 && checked === all.length;
            selectAll.indeterminate = checked > 0 && checked < all.length;
        }
    }

    function clearSelection() {
        rowChecks().forEach(function (cb) { cb.checked = false; });
        rows().forEach(function (row) {
            syncRowSelection(row.getAttribute('data-rel'), false);
        });
        updateSelection();
    }

    function applyFilter() {
        var q = (filterInput && filterInput.value || '').trim().toLowerCase();
        rows().forEach(function (row) {
            var name = (row.getAttribute('data-name') || '').toLowerCase();
            row.hidden = q !== '' && name.indexOf(q) < 0;
        });
    }

    list.addEventListener('change', function (e) {
        if (e.target.matches('[data-files-check], [data-files-select-all]')) {
            if (e.target.matches('[data-files-select-all]')) {
                var on = e.target.checked;
                rowChecks().forEach(function (cb) {
                    if (!cb.closest('[data-file-row]').hidden) cb.checked = on;
                });
            }
            updateSelection();
        }
    });

    browse.addEventListener('click', function (e) {
        var tile = e.target.closest('.m-tile[data-file-row]');
        if (tile && !e.target.closest('a')) {
            if (e.detail === 2) {
                var url = tile.getAttribute('data-dir') === 'true' ? tile.getAttribute('data-url') : tile.getAttribute('data-download');
                if (url) window.location.href = url;
                return;
            }
            var rel = tile.getAttribute('data-rel');
            var listRow = list.querySelector('[data-file-row][data-rel="' + rel + '"]');
            var cb = listRow && listRow.querySelector('[data-files-check]');
            if (cb) {
                cb.checked = !cb.checked;
                updateSelection();
            }
        }
    });

    list.addEventListener('click', function (e) {
        if (e.target.closest('a')) return;
        var row = e.target.closest('[data-file-row]');
        if (!row) return;
        var cb = row.querySelector('[data-files-check]');
        if (!cb) return;
        if (!e.target.matches('[data-files-check]')) {
            cb.checked = !cb.checked;
            updateSelection();
        }
    });

    browse.addEventListener('click', function (e) {
        if (e.target.closest('[data-files-clear]')) {
            e.preventDefault();
            clearSelection();
            return;
        }
        if (e.target.closest('[data-files-props-toggle]')) {
            e.preventDefault();
            setPropsClosed(false);
            var sel = selectedRows();
            if (sel.length === 1) {
                renderProps(sel[0]);
                showProps();
            } else if (propsPanel && !propsPanel.hidden) {
                hideProps(true);
            }
            return;
        }
        if (e.target.closest('[data-files-props-close]')) {
            e.preventDefault();
            hideProps(true);
            return;
        }
        var op = e.target.closest('[data-files-op]');
        if (!op || op.disabled) return;
        var action = op.getAttribute('data-files-op');
        var sel = selectedRows();
        if (sel.length === 0) return;
        if (action === 'download') {
            e.preventDefault();
            sel.forEach(function (row) {
                var url = row.getAttribute('data-download');
                if (url) window.open(url, '_blank');
            });
            return;
        }
        if (action === 'delete') {
            e.preventDefault();
            if (readonly || !deleteForm) return;
            if (!window.confirm('Delete ' + sel.length + ' item(s) from the server?')) return;
            deleteForm.querySelectorAll('[data-batch-field]').forEach(function (el) { el.remove(); });
            sel.forEach(function (row) {
                ['target', 'etag'].forEach(function (name) {
                    var input = document.createElement('input');
                    input.type = 'hidden';
                    input.name = name;
                    input.value = row.getAttribute(name === 'target' ? 'data-rel' : 'data-etag') || '';
                    input.setAttribute('data-batch-field', '');
                    deleteForm.appendChild(input);
                });
            });
            deleteForm.submit();
            return;
        }
        if (action === 'rename') {
            e.preventDefault();
            if (readonly || !renameForm || sel.length !== 1) return;
            var current = sel[0].getAttribute('data-name') || '';
            var next = window.prompt('New name', current);
            if (!next || next === current) return;
            renameForm.querySelector('[name="target"]').value = sel[0].getAttribute('data-rel') || '';
            renameForm.querySelector('[name="new_name"]').value = next;
            renameForm.submit();
            return;
        }
        if (action === 'move' || action === 'copy') {
            e.preventDefault();
            if (readonly || !moveDialog) return;
            pickerMode = action;
            if (pickerTitle) pickerTitle.textContent = action === 'copy' ? 'Copy here' : 'Move here';
            if (pickerConfirm) pickerConfirm.textContent = action === 'copy' ? 'Copy here' : 'Move here';
            pickerSelection = null;
            if (pickerConfirm) pickerConfirm.disabled = true;
            if (pickerWarn) pickerWarn.hidden = true;
            picker.querySelectorAll('.m-tree [role="treeitem"].is-on').forEach(function (n) { n.classList.remove('is-on'); });
            moveDialog.showModal();
        }
    });

    function updatePickerWarn() {
        if (!pickerSelection || !pickerWarn) return;
        var cross = pickerSelection.account !== srcAccount || pickerSelection.col !== srcCol;
        pickerWarn.hidden = !cross;
    }

    if (picker) {
        picker.addEventListener('click', function (e) {
            var node = e.target.closest('[data-picker-node]');
            if (!node || node.classList.contains('is-disabled')) return;
            picker.querySelectorAll('.m-tree [role="treeitem"].is-on').forEach(function (n) { n.classList.remove('is-on'); });
            node.classList.add('is-on');
            pickerSelection = {
                account: node.getAttribute('data-account'),
                col: node.getAttribute('data-col'),
                folder: node.getAttribute('data-folder') || ''
            };
            if (pickerConfirm) pickerConfirm.disabled = false;
            updatePickerWarn();
        });
        picker.addEventListener('click', function (e) {
            if (e.target.closest('[data-picker-cancel]')) {
                moveDialog.close();
            }
            if (e.target.closest('[data-picker-confirm]') && pickerSelection) {
                var form = pickerMode === 'copy' ? copyForm : moveForm;
                if (!form) return;
                if (pickerMode === 'move' && pickerWarn && !pickerWarn.hidden) {
                    if (!window.confirm('Files will be downloaded and uploaded through Carrel. Continue?')) return;
                }
                if (pickerMode === 'copy' && pickerWarn && !pickerWarn.hidden) {
                    if (!window.confirm('Files will be copied through Carrel (download then upload). Continue?')) return;
                }
                form.querySelector('[name="dest_account"]').value = pickerSelection.account;
                form.querySelector('[name="dest_col"]').value = pickerSelection.col;
                form.querySelector('[name="dest_folder"]').value = pickerSelection.folder;
                form.querySelectorAll('[data-batch-field]').forEach(function (el) { el.remove(); });
                selectedRows().forEach(function (row) {
                    var input = document.createElement('input');
                    input.type = 'hidden';
                    input.name = 'target';
                    input.value = row.getAttribute('data-rel') || '';
                    input.setAttribute('data-batch-field', '');
                    form.appendChild(input);
                });
                moveDialog.close();
                form.submit();
            }
        });
    }

    if (filterInput) {
        filterInput.addEventListener('input', applyFilter);
    }

    document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape' && propsPanel && !propsPanel.hidden) {
            hideProps(true);
        }
    });
})();

// Wave 2.6 — upload queue with XHR progress and name-collision choices.
(function () {
    var browse = document.querySelector('[data-files-browse]');
    if (!browse || browse.getAttribute('data-readonly') === '1') return;

    var uploadInput = browse.querySelector('[data-files-upload-input]');
    var uploadForm = uploadInput && uploadInput.closest('form');
    var transfers = browse.querySelector('[data-files-transfers]');
    var transfersList = browse.querySelector('[data-files-transfers-list]');
    var transfersLabel = browse.querySelector('[data-files-transfers-label]');
    var csrf = uploadForm && uploadForm.querySelector('[name="csrf_token"]');
    var folder = browse.getAttribute('data-folder') || '';
    var postURL = uploadForm && uploadForm.action;
    var queue = [];
    var active = 0;

    function showTransfers() {
        if (transfers) transfers.hidden = false;
    }

    function updateLabel() {
        if (!transfersLabel) return;
        var waiting = queue.filter(function (q) { return q.state === 'queued' || q.state === 'uploading'; }).length;
        transfersLabel.textContent = waiting > 0 ? 'Uploading ' + (active + 1) + ' of ' + queue.length : 'Uploads';
    }

    function renderItem(item) {
        var li = item.el;
        if (!li) {
            li = document.createElement('li');
            item.el = li;
            transfersList.appendChild(li);
        }
        var status = item.state === 'done' ? 'done' : item.state === 'conflict' ? 'name taken' :
            item.state === 'skipped' ? 'skipped' : item.state === 'error' ? item.error :
                Math.round(item.progress * 100) + '%';
        var html = '<div class="row"><span' + (item.state === 'conflict' || item.state === 'error' ? ' style="color:var(--alert)"' : '') + '>' + item.file.name + '</span><span class="hint">' + status + '</span></div>';
        if (item.state === 'uploading' || item.state === 'done') {
            html += '<div class="bar"><i style="width:' + Math.round(item.progress * 100) + '%"></i></div>';
        }
        if (item.state === 'conflict') {
            html += '<div class="actions"><button type="button" class="link" data-upload-choice="keep-both">Keep both</button>' +
                '<button type="button" class="link" data-upload-choice="replace">Replace</button>' +
                '<button type="button" class="link" data-upload-choice="skip">Skip</button></div>';
        }
        li.innerHTML = html;
        li.querySelectorAll('[data-upload-choice]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                var choice = btn.getAttribute('data-upload-choice');
                if (choice === 'skip') {
                    item.state = 'skipped';
                    renderItem(item);
                    pump();
                    return;
                }
                item.mode = choice;
                item.state = 'queued';
                pump();
            });
        });
    }

    function enqueue(file) {
        var item = { file: file, state: 'queued', progress: 0, mode: '' };
        queue.push(item);
        showTransfers();
        renderItem(item);
        updateLabel();
        pump();
    }

    function pump() {
        if (active >= 1) return;
        var next = queue.find(function (q) { return q.state === 'queued'; });
        if (!next) {
            updateLabel();
            return;
        }
        active++;
        next.state = 'uploading';
        renderItem(next);
        updateLabel();
        var xhr = new XMLHttpRequest();
        xhr.open('POST', postURL);
        xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
        xhr.setRequestHeader('Accept', 'application/json');
        xhr.upload.onprogress = function (ev) {
            if (ev.lengthComputable) {
                next.progress = ev.loaded / ev.total;
                renderItem(next);
            }
        };
        xhr.onload = function () {
            active--;
            var reply = {};
            try { reply = JSON.parse(xhr.responseText || '{}'); } catch (e) { /* ignore */ }
            if (xhr.status === 409 && reply.conflict) {
                next.state = 'conflict';
                renderItem(next);
            } else if (xhr.status >= 200 && xhr.status < 300 && reply.ok) {
                next.state = 'done';
                next.progress = 1;
                renderItem(next);
            } else {
                next.state = 'error';
                next.error = reply.error || 'upload failed';
                renderItem(next);
            }
            updateLabel();
            pump();
        };
        xhr.onerror = function () {
            active--;
            next.state = 'error';
            next.error = 'network error';
            renderItem(next);
            updateLabel();
            pump();
        };
        var body = new FormData();
        body.append('csrf_token', csrf && csrf.value || '');
        body.append('p', folder);
        body.append('file', next.file);
        if (next.mode) body.append('upload_mode', next.mode);
        xhr.send(body);
    }

    if (uploadForm) {
        uploadForm.addEventListener('submit', function (e) {
            if (!uploadInput || !uploadInput.files || !uploadInput.files.length) return;
            e.preventDefault();
            Array.prototype.forEach.call(uploadInput.files, enqueue);
            uploadInput.value = '';
        });
    }

    browse.addEventListener('dragover', function (e) {
        e.preventDefault();
    });
    browse.addEventListener('drop', function (e) {
        e.preventDefault();
        if (!e.dataTransfer || !e.dataTransfer.files) return;
        Array.prototype.forEach.call(e.dataTransfer.files, enqueue);
    });

    var hideBtn = browse.querySelector('[data-files-transfers-hide]');
    if (hideBtn) {
        hideBtn.addEventListener('click', function () {
            if (transfers) transfers.hidden = true;
        });
    }
})();

// Wave 2.7 — list / tiles toggle.
(function () {
    var browse = document.querySelector('[data-files-browse]');
    if (!browse) return;
    var layout = document.querySelector('[data-files-layout]');
    var grid = browse.querySelector('[data-files-grid]');
    var list = browse.querySelector('[data-files-list]');
    var KEY = 'carrel:files-view';

    function setView(mode) {
        var tiles = mode === 'tiles';
        if (layout) layout.classList.toggle('is-tiles', tiles);
        if (grid) grid.hidden = !tiles;
        if (list) list.hidden = tiles;
        browse.querySelectorAll('[data-files-view]').forEach(function (btn) {
            var on = btn.getAttribute('data-files-view') === mode;
            btn.setAttribute('aria-pressed', on ? 'true' : 'false');
        });
        try { localStorage.setItem(KEY, mode); } catch (e) { /* ignore */ }
    }

    var saved = 'list';
    try { saved = localStorage.getItem(KEY) || 'list'; } catch (e) { /* ignore */ }
    setView(saved === 'tiles' ? 'tiles' : 'list');

    browse.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-files-view]');
        if (!btn) return;
        setView(btn.getAttribute('data-files-view'));
    });
})();

// 2.6.B2 — sort the files table by its own columns. Reorders the rows
// already in the DOM; nothing is re-fetched. The "up one folder" row stays
// pinned at the top, same as it does unsorted.
(function () {
    var browse = document.querySelector('[data-files-browse]');
    if (!browse) return;
    var list = browse.querySelector('[data-files-list]');
    if (!list) return;
    var tbody = list.querySelector('tbody');
    var state = { col: null, dir: 1 };

    function sortValue(row, col) {
        if (col === 'size') return Number(row.getAttribute('data-size')) || 0;
        if (col === 'changed') return row.getAttribute('data-changed') || '';
        if (col === 'type') return (row.getAttribute('data-type') || '').toLowerCase();
        return (row.getAttribute('data-name') || '').toLowerCase();
    }

    function applySort(col) {
        if (state.col === col) {
            state.dir = -state.dir;
        } else {
            state.col = col;
            state.dir = 1;
        }
        var rows = Array.prototype.slice.call(tbody.querySelectorAll('[data-file-row]'));
        rows.sort(function (a, b) {
            var va = sortValue(a, col), vb = sortValue(b, col);
            if (va < vb) return -state.dir;
            if (va > vb) return state.dir;
            return 0;
        });
        rows.forEach(function (row) { tbody.appendChild(row); });
        browse.querySelectorAll('[data-files-sort]').forEach(function (btn) {
            btn.classList.toggle('is-on', btn.getAttribute('data-files-sort') === col);
        });
    }

    browse.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-files-sort]');
        if (!btn) return;
        applySort(btn.getAttribute('data-files-sort'));
    });
})();

// 2.6.B6 — Week/Month/Range on the agenda. Week and Month are plain links;
// Range only reveals the existing From/To form already on the page, so
// there is nothing to fetch here either.
(function () {
    document.addEventListener('click', function (e) {
        var btn = e.target.closest('[data-range-toggle]');
        if (!btn) return;
        var scope = btn.closest('[data-filter-scope]');
        if (!scope) return;
        var open = scope.hasAttribute('data-range-open');
        if (open) {
            scope.removeAttribute('data-range-open');
        } else {
            scope.setAttribute('data-range-open', '');
        }
        btn.setAttribute('aria-expanded', open ? 'false' : 'true');
        btn.classList.toggle('is-on', !open);
    });
})();

// 2.6.B1 — filter the page-bar acts on: narrows the rows already on the
// page, entirely in the browser. No request is made, so there is nothing to
// filter without JavaScript and the field stays hidden (see ".list-filter"
// in carrel.css). Each filter input names the ancestor it filters through
// data-filter-scope, and an optional data-filter-group marks a row's day or
// section so an emptied group disappears along with its rows — the agenda
// is the one screen where rows come in named groups.
(function () {
    function applyFilter(input) {
        var scope = input.closest('[data-filter-scope]');
        if (!scope) return;
        var q = input.value.trim().toLowerCase();
        var rows = Array.prototype.slice.call(scope.querySelectorAll('.m-row'));
        var visible = 0;
        rows.forEach(function (row) {
            var match = !q || row.textContent.toLowerCase().indexOf(q) !== -1;
            row.classList.toggle('is-filtered-out', !match);
            if (match) visible++;
        });
        Array.prototype.slice.call(scope.querySelectorAll('[data-filter-group]')).forEach(function (group) {
            var anyVisible = !!group.querySelector('.m-row:not(.is-filtered-out)');
            group.classList.toggle('is-filtered-out', !anyVisible);
        });
        var counter = scope.querySelector('[data-bar-count]');
        if (counter) counter.textContent = visible + (visible === 1 ? ' item' : ' items');
    }

    // 2.6.G8/G10: Open/Done/All on the merged tasks view, from the same rows
    // the filter already counts — no second request, and no distinction from
    // the per-collection view's own server-computed numbers except that
    // these come from whatever the fan-out has loaded so far.
    function updateTaskCounts(scope) {
        var box = scope.querySelector('[data-task-counts]');
        if (!box) return;
        var rows = Array.prototype.slice.call(scope.querySelectorAll('.m-row.is-task'));
        var done = rows.filter(function (r) { return r.classList.contains('is-done'); }).length;
        var all = rows.length;
        var openEl = box.querySelector('[data-task-count="open"]');
        var doneEl = box.querySelector('[data-task-count="done"]');
        var allEl = box.querySelector('[data-task-count="all"]');
        if (openEl) openEl.textContent = 'Open ' + (all - done);
        if (doneEl) doneEl.textContent = 'Done ' + done;
        if (allEl) allEl.textContent = 'All ' + all;
    }

    function refreshScope(scope) {
        Array.prototype.slice.call(scope.querySelectorAll('[data-list-filter]')).forEach(applyFilter);
        updateTaskCounts(scope);
    }

    document.addEventListener('input', function (e) {
        var input = e.target.closest('[data-list-filter]');
        if (input) applyFilter(input);
    });

    // 2.6.G9: #find-panel is replaced whole by every poll tick and every SSE
    // message. htmx:afterSwap was tried first and does not fire reliably for
    // the SSE path in practice — live-checked against a real instance:
    // rows arrived, the counter and task counts stayed at zero. A
    // MutationObserver on the panel itself does not depend on which
    // mechanism changed it, only that it did, so it catches both the same
    // way. Without this, a filled-in filter would go stale the moment a new
    // batch of rows arrived — worse than an emptied field, because the list
    // would silently stop being filtered while still looking like it was.
    // Re-run unconditionally, not only when a query is typed: an empty
    // query still recomputes the counter against whatever just changed
    // (2.6.G8), which is the only way the merged-view counter and task
    // counts stay right as a poll brings more rows in.
    Array.prototype.slice.call(document.querySelectorAll('[data-sse-panel]')).forEach(function (panel) {
        var scope = panel.closest('[data-filter-scope]') || panel;
        new MutationObserver(function () { refreshScope(scope); }).observe(panel, { childList: true, subtree: true });
    });

    // A page can load with #find-panel already populated — a running task ID
    // carried over rather than started fresh — in which case the observer
    // above sees no mutation to react to. carrel.js is loaded with `defer`,
    // so the DOM is already parsed by the time this runs.
    Array.prototype.slice.call(document.querySelectorAll('[data-filter-scope]')).forEach(refreshScope);
})();

// Installed PWA and signed-in shell: show the §13 offline bar when the browser
// loses the network. Session memory stays on screen; nothing is written to disk.
(function () {
    var bar = document.getElementById('app-offline');
    if (!bar) return;
    function sync() {
        bar.hidden = navigator.onLine !== false;
    }
    window.addEventListener('online', sync);
    window.addEventListener('offline', sync);
    sync();
})();
