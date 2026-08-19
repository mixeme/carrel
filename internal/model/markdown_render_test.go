// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
)

func TestRenderNoteHTMLSupportsGFM(t *testing.T) {
	src := "## What we do\n\n- [ ] check changelog\n- [x] ship\n\n**bold** and `code`.\n"
	out, err := RenderNoteHTML(src)
	if err != nil {
		t.Fatalf("RenderNoteHTML: %v", err)
	}
	html := string(out)
	for _, want := range []string{"<h2", "<ul", "<strong>bold</strong>", "<code>code</code>"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered HTML lacks %q:\n%s", want, html)
		}
	}
}

func TestRenderNoteHTMLBlocksRawHTML(t *testing.T) {
	src := "<script>alert(1)</script>\n\n**safe**\n"
	out, err := RenderNoteHTML(src)
	if err != nil {
		t.Fatalf("RenderNoteHTML: %v", err)
	}
	html := string(out)
	if strings.Contains(html, "<script>") {
		t.Errorf("raw HTML was not escaped:\n%s", html)
	}
	if !strings.Contains(html, "<strong>safe</strong>") {
		t.Errorf("markdown was not rendered:\n%s", html)
	}
}

func TestRenderNoteHTMLDoesNotChangeSource(t *testing.T) {
	src := "Line one\n\n## Heading\n"
	before := []byte(src)
	if _, err := RenderNoteHTML(src); err != nil {
		t.Fatalf("RenderNoteHTML: %v", err)
	}
	if string(before) != src {
		t.Fatal("rendering modified the source string")
	}
}
