// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

// noteMarkdown renders note bodies in read mode. Raw HTML in the source stays
// escaped: a note from someone else's server must not become executable markup.
var noteMarkdown = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
)

// RenderNoteHTML turns a note description into HTML for read mode. The source
// string is not modified; unknown syntax is left visible through the parser.
func RenderNoteHTML(src string) ([]byte, error) {
	var buf bytes.Buffer
	if err := noteMarkdown.Convert([]byte(src), &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
