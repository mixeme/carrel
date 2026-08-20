// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Minimal service worker for the PWA shell (§13). Caches only static assets
// and a small offline page — never HTML from /app/, attachments, or probes.
'use strict';

var CACHE = 'carrel-shell-1';

var SHELL_ASSETS = [
    'static/carrel.css',
    'static/boot.js',
    'static/htmx.min.js',
    'static/htmx-sse.js',
    'static/carrel.js',
    'static/icon.svg',
    'static/offline-shell.html',
    'static/fonts/literata-latin.woff2',
    'static/fonts/literata-latin-ext.woff2',
    'static/fonts/literata-cyrillic.woff2',
    'static/fonts/literata-italic-latin.woff2',
    'static/fonts/literata-italic-latin-ext.woff2',
    'static/fonts/literata-italic-cyrillic.woff2',
    'static/fonts/publicsans-latin.woff2',
    'static/fonts/publicsans-latin-ext.woff2'
];

function scopeBase() {
    var path = self.location.pathname;
    var slash = path.lastIndexOf('/');
    return slash >= 0 ? path.slice(0, slash + 1) : '/';
}

function assetURL(relative) {
    return scopeBase() + relative;
}

function isShellAsset(pathname) {
    var base = scopeBase();
    var rel = pathname.indexOf(base) === 0 ? pathname.slice(base.length) : pathname.replace(/^\//, '');
    return rel.indexOf('static/') === 0;
}

self.addEventListener('install', function (event) {
    self.skipWaiting();
    event.waitUntil(
        caches.open(CACHE).then(function (cache) {
            return cache.addAll(SHELL_ASSETS.map(assetURL));
        })
    );
});

self.addEventListener('activate', function (event) {
    event.waitUntil(
        caches.keys().then(function (keys) {
            return Promise.all(
                keys.filter(function (key) { return key !== CACHE; }).map(function (key) {
                    return caches.delete(key);
                })
            );
        }).then(function () { return self.clients.claim(); })
    );
});

self.addEventListener('fetch', function (event) {
    var req = event.request;
    var url = new URL(req.url);
    if (url.origin !== self.location.origin) {
        return;
    }

    if (req.mode === 'navigate') {
        event.respondWith(
            fetch(req).catch(function () {
                return caches.match(assetURL('static/offline-shell.html'));
            })
        );
        return;
    }

    if (!isShellAsset(url.pathname)) {
        return;
    }

    event.respondWith(
        caches.match(req).then(function (cached) {
            var network = fetch(req).then(function (res) {
                if (res && res.ok) {
                    var copy = res.clone();
                    caches.open(CACHE).then(function (cache) {
                        cache.put(req, copy);
                    });
                }
                return res;
            });
            return cached || network;
        })
    );
});
