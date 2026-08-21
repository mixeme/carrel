// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "embed"

//go:embed static/*
var StaticFS embed.FS

//go:embed template/*
var TemplateFS embed.FS

// ComponentFS holds the component library of internal/web/component: the
// markup of each primitive in tmpl/ and its styles in css/, side by side.
// Kept apart from TemplateFS and StaticFS on purpose — a screen reaches a
// component through {{template}}, never by copying its markup, and the gates
// in internal/web/handler can only say so while the two live apart.
//
//go:embed component/*
var ComponentFS embed.FS
