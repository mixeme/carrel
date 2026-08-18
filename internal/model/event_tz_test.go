// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"testing"
	"time"
)

func TestSourceTimeZone(t *testing.T) {
	moscow, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	ev := Event{StartTZID: "America/New_York"}
	if got := ev.SourceTimeZone(moscow); got != "America/New_York" {
		t.Fatalf("different zone should show TZID, got %q", got)
	}
	if got := ev.SourceTimeZone(ny); got != "" {
		t.Fatalf("matching zone should hide TZID, got %q", got)
	}
	empty := Event{}
	if got := empty.SourceTimeZone(moscow); got != "" {
		t.Fatalf("empty TZID should hide, got %q", got)
	}
}
