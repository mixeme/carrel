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
