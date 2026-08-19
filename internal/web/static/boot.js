// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Theme and density, applied before the first paint so a dark setting does not
// flash light. It is a file rather than an inline script because the CSP of
// §24.5 has no 'unsafe-inline' for scripts, and an inline block is simply
// dropped. Loaded without defer for the same reason it exists: it has to run
// before the body is painted.
(function () {
    try {
        var theme = localStorage.getItem('carrel:theme');
        if (theme === 'light' || theme === 'dark') {
            document.documentElement.dataset.theme = theme;
        }
        if (localStorage.getItem('carrel:density') === 'compact') {
            document.documentElement.dataset.density = 'compact';
        }
    } catch (e) {}
})();
