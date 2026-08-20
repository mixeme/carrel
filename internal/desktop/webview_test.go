// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareWebviewProfile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := filepath.Join(t.TempDir(), "webview")
	p := Paths{WebviewDataDir: dir}
	if err := prepareWebviewProfile(p); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsDir() {
		t.Fatalf("%q is not a directory", dir)
	}
}
