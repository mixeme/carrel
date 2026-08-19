// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strings"
)

// signOutPath is served by the Wails asset server. The watch script navigates
// here from the Carrel origin after a real sign-out so bindings work again.
const signOutPath = "/__carrel_signout"

const inAppStorageKey = "carrel.desktop.inApp"

func defaultShellOrigin() string {
	if runtime.GOOS == "windows" {
		return "http://wails.localhost/"
	}
	return "wails://wails/"
}

func shellSignOutURL(shellHref string) string {
	raw := strings.TrimSpace(shellHref)
	if raw == "" {
		raw = defaultShellOrigin()
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" {
		u, err = url.Parse(defaultShellOrigin())
		if err != nil {
			return defaultShellOrigin()
		}
	}
	u.Path = signOutPath
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// isAuthSurface is a Carrel credential page: login, first-run setup, register,
// or the recovery note. Sign-out lands on login; first connect does too, and
// must not be treated as a mode reset.
func isAuthSurface(urlPath string) bool {
	urlPath = strings.TrimRight(urlPath, "/")
	if urlPath == "" {
		return false
	}
	base := path.Base(urlPath)
	switch base {
	case "login", "setup", "register", "forgot":
		return true
	}
	return false
}

// isInAppPath is a signed-in Carrel screen. /about and similar public pages
// are excluded so they cannot trip the sign-out watcher.
func isInAppPath(urlPath string) bool {
	urlPath = strings.TrimRight(urlPath, "/")
	if strings.HasSuffix(urlPath, "/app") || strings.Contains(urlPath, "/app/") {
		return true
	}
	if strings.HasSuffix(urlPath, "/admin") || strings.Contains(urlPath, "/admin/") {
		return true
	}
	return false
}

func signOutWatchScript(signOutURL string) string {
	return fmt.Sprintf(`(function(){
  try {
    var href = String(location.href || '');
    if (location.protocol === 'wails:' || /wails\.localhost$/i.test(location.host) || /(?:^|\.)wails$/i.test(location.host)) {
      return;
    }
    var path = location.pathname || '';
    var auth = /\/(login|setup|register|forgot)\/?$/.test(path);
    var inApp = /\/(app|admin)(\/|$)/.test(path);
    var key = %s;
    if (inApp) {
      sessionStorage.setItem(key, '1');
      return;
    }
    if (auth && sessionStorage.getItem(key) === '1') {
      sessionStorage.removeItem(key);
      location.replace(%s);
    }
  } catch (e) {}
})();`, jsString(inAppStorageKey), jsString(signOutURL))
}

func (a *App) assetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == signOutPath {
			sup, running := a.beginSignOut()
			go func() {
				if sup != nil {
					_ = sup.Stop(running)
				}
			}()
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
