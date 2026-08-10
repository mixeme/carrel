// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "embed"

//go:embed static/*
var StaticFS embed.FS

//go:embed template/*
var TemplateFS embed.FS
