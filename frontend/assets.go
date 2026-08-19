// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package frontend holds the Wails onboarding UI. After a mode is chosen the
// webview navigates to the Carrel instance; templates of the web app are not
// duplicated here.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed *.html *.css *.js
var files embed.FS

// Assets returns the embedded shell files for the Wails asset server.
func Assets() fs.FS {
	sub, err := fs.Sub(files, ".")
	if err != nil {
		panic(err)
	}
	return sub
}
