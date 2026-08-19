// Carrel UI helpers — no inline scripts (CSP).

// Marks the document so CSS can tell the two narrow-screen layouts apart: with
// JavaScript the source rail lives in the slide-out panel, without it the rail
// stays in the page where it can still be reached.
document.documentElement.classList.add('js');

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
            always: ['badges'],
            columns: [
                { id: 'photo', label: 'Photo', width: 'var(--row-lg)' },
                { id: 'name', label: 'Name', width: 'minmax(150px, 1.4fr)', locked: true },
                { id: 'phone', label: 'Phone', width: 'minmax(110px, 1fr)' },
                { id: 'email', label: 'Email', width: 'minmax(110px, 1fr)' },
                { id: 'badges', label: 'Badges', width: 'auto', always: true }
            ],
            lead: [{ id: 'bar', width: '3px' }]
        },
        tasks: {
            locked: 'name',
            pinned: ['done', 'bar'],
            always: [],
            columns: [
                { id: 'name', label: 'Title', width: 'minmax(190px, 2fr)', locked: true },
                { id: 'due', label: 'Due', width: 'minmax(90px, auto)' },
                { id: 'status', label: 'Status', width: 'minmax(70px, auto)' },
                { id: 'progress', label: 'Progress', width: 'minmax(50px, auto)' },
                { id: 'tags', label: 'Tags', width: 'auto' }
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
                { id: 'date', label: 'Date', width: 'minmax(70px, auto)' },
                { id: 'tags', label: 'Tags', width: 'auto' }
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
        var node = document.querySelector('.section-rail');
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
        var node = mount ? mount.querySelector('.section-rail') : null;
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
