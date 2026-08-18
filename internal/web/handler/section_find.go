// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
)

// sectionFind starts a fan-out on a section home URL and renders the section
// template with merged results (§1.7).
func (s *Server) sectionFind(w http.ResponseWriter, r *http.Request, req findRequest, template string) {
	if req.Mode == modeTime && req.From == "" {
		from, to := defaultUnifiedRange(s.timezone())
		req.From, req.To = from, to
	}
	s.startFind(w, r, req, template)
}
