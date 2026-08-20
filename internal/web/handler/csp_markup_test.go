// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

// The CSP of §24.5 carries no 'unsafe-inline'. A style attribute or an inline
// script in a template is therefore not a small liberty — the browser drops it
// and the thing it was doing simply does not happen. This is what let the
// progress bar sit at nought and every collection colour go missing, so it is
// checked on the templates rather than left to the eye.
var (
	styleAttr     = regexp.MustCompile(`\sstyle\s*=\s*"`)
	inlineScript  = regexp.MustCompile(`<script(?:\s[^>]*)?>`)
	scriptWithSrc = regexp.MustCompile(`<script[^>]*\ssrc=`)
)

func TestTemplatesCarryNothingInline(t *testing.T) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	names, err := fs.Glob(templateFS, "*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no templates found")
	}
	for _, name := range names {
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(b)
		if loc := styleAttr.FindStringIndex(body); loc != nil {
			t.Errorf("%s carries a style attribute at byte %d; the CSP drops it — "+
				"use a class, or a data attribute that carrel.js applies", name, loc[0])
		}
		for _, tag := range inlineScript.FindAllString(body, -1) {
			if !scriptWithSrc.MatchString(tag) {
				t.Errorf("%s carries an inline script (%s); the CSP drops it — "+
					"put it in static/ and load it by src", name, tag)
			}
		}
	}
}

// The fan-out of §16 is delivered over SSE, and the extension that makes
// hx-ext="sse" mean anything is a separate file. Without it nothing connects,
// nothing fails either, and the merged view of every section waits for a
// message that is never coming.
func TestBaseLoadsTheStreamExtension(t *testing.T) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	b, err := fs.ReadFile(templateFS, "base.html")
	if err != nil {
		t.Fatalf("read base.html: %v", err)
	}
	body := string(b)
	for _, want := range []string{"static/htmx.min.js", "static/htmx-sse.js", "static/carrel.js", "static/boot.js"} {
		if !strings.Contains(body, want) {
			t.Errorf("base.html does not load %s", want)
		}
	}
	if strings.Index(body, "htmx.min.js") > strings.Index(body, "htmx-sse.js") {
		t.Error("the sse extension is loaded before htmx itself; it registers against the global and would be lost")
	}

	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		t.Fatalf("static FS: %v", err)
	}
	for _, name := range []string{"htmx-sse.js", "boot.js"} {
		if _, err := fs.Stat(staticFS, name); err != nil {
			t.Errorf("static/%s is referenced but not embedded: %v", name, err)
		}
	}

	js, err := fs.ReadFile(staticFS, "carrel.js")
	if err != nil {
		t.Fatalf("read carrel.js: %v", err)
	}
	if !strings.Contains(string(js), "data-swatch") {
		t.Error("carrel.js does not apply the collection colours the templates hand it")
	}
	if !strings.Contains(string(js), "pollFallback") {
		t.Error("carrel.js has no fallback for a stream that never speaks")
	}
}
