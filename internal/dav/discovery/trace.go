// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import "time"

// Step is one discovery attempt for diagnostics (§6).
type Step struct {
	Name       string    `json:"name"`
	Detail     string    `json:"detail"`
	StatusCode int       `json:"status_code,omitempty"`
	Target     string    `json:"target,omitempty"`
	At         time.Time `json:"at"`
}

// Trace records which discovery step succeeded or failed.
type Trace struct {
	Steps []Step `json:"steps"`
}

// Add appends a step to the trace.
func (t *Trace) Add(name, detail string, code int, target string) {
	if t == nil {
		return
	}
	t.Steps = append(t.Steps, Step{
		Name:       name,
		Detail:     detail,
		StatusCode: code,
		Target:     target,
		At:         time.Now().UTC(),
	})
}

// Failed reports whether any recorded step looks like a hard failure.
func (t *Trace) Failed() bool {
	if t == nil {
		return false
	}
	for _, s := range t.Steps {
		if s.StatusCode >= 400 || (s.StatusCode == 0 && s.Detail != "" && s.Detail != "ok") {
			return true
		}
	}
	return false
}
