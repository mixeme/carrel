// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package frontend holds the minimal Wails shell assets. Onboarding UI lands
// here in a later phase; Remote mode redirects the webview to the Carrel URL.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed index.html
var files embed.FS

// Assets returns the embedded shell files for the Wails asset server.
func Assets() fs.FS {
	sub, err := fs.Sub(files, ".")
	if err != nil {
		panic(err)
	}
	return sub
}
