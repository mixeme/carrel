// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
	"time"
)

func card(t *testing.T, lines ...string) *Object {
	t.Helper()
	body := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:x\r\n" + strings.Join(lines, "\r\n")
	if len(lines) > 0 {
		body += "\r\n"
	}
	return mustParse(t, body+"END:VCARD\r\n")
}

func TestCompareFindsNothingWhenNothingChanged(t *testing.T) {
	sent := card(t, "FN:Ada", "X-CUSTOM:kept")
	stored := card(t, "FN:Ada", "X-CUSTOM:kept")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !loss.Empty() {
		t.Fatalf("loss = %+v, want none", loss)
	}
	if loss.String() != "" {
		t.Errorf("empty loss has a message: %q", loss.String())
	}
}

// TestCompareReportsDroppedProperty is what §8 is about: the server stored the
// contact but kept none of the X- property another client had put there.
func TestCompareReportsDroppedProperty(t *testing.T) {
	sent := card(t, "FN:Ada", "X-CUSTOM:kept", "CATEGORIES:Friends")
	stored := card(t, "FN:Ada")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if got := loss.Names(); len(got) != 2 || got[0] != "CATEGORIES" || got[1] != "X-CUSTOM" {
		t.Fatalf("names = %v, want CATEGORIES and X-CUSTOM", got)
	}
	if msg := loss.String(); !strings.Contains(msg, "without CATEGORIES, X-CUSTOM") {
		t.Errorf("message does not name the properties: %q", msg)
	}
}

func TestCompareReportsFewerInstances(t *testing.T) {
	sent := card(t, "TEL;TYPE=WORK:+1", "TEL;TYPE=CELL:+2")
	stored := card(t, "TEL;TYPE=WORK:+1")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(loss.Reduced) != 1 || loss.Reduced[0] != "TEL" {
		t.Fatalf("reduced = %v, want TEL", loss.Reduced)
	}
	if len(loss.Missing) != 0 {
		t.Errorf("missing = %v, want none", loss.Missing)
	}
}

func TestCompareReportsChangedValueAndParameters(t *testing.T) {
	sent := card(t, "FN:Ada Lovelace", "TEL;TYPE=CELL:+2")
	stored := card(t, "FN:ADA LOVELACE", "TEL;TYPE=CELL:+2")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(loss.Changed) != 1 || loss.Changed[0] != "FN" {
		t.Fatalf("changed = %v, want FN", loss.Changed)
	}

	sent = card(t, "TEL;TYPE=CELL:+2")
	stored = card(t, "TEL:+2")
	loss, err = Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(loss.Changed) != 1 || loss.Changed[0] != "TEL" {
		t.Fatalf("a dropped parameter was not reported: %+v", loss)
	}
}

// TestCompareIgnoresParameterOrder keeps the check honest: a server that writes
// the same parameters in another order has lost nothing.
func TestCompareIgnoresParameterOrder(t *testing.T) {
	sent := card(t, "TEL;TYPE=WORK;TYPE=VOICE:+1")
	stored := card(t, "TEL;TYPE=VOICE;TYPE=WORK:+1")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !loss.Empty() {
		t.Errorf("loss = %+v, want none", loss)
	}
}

// TestCompareIgnoresPropertiesServersOwn keeps REV and PRODID out of the report:
// a server is expected to stamp those, and reporting them would bury the losses
// that matter.
func TestCompareIgnoresPropertiesServersOwn(t *testing.T) {
	sent := card(t, "FN:Ada", "REV:20260101T000000Z", "PRODID:-//Carrel//EN")
	stored := card(t, "FN:Ada", "REV:20260812T101500Z")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !loss.Empty() {
		t.Errorf("loss = %+v, want none", loss)
	}
}

// TestCompareIgnoresAddedProperties: a server adding something of its own is not
// a loss and is not the person's problem.
func TestCompareIgnoresAddedProperties(t *testing.T) {
	sent := card(t, "FN:Ada")
	stored := card(t, "FN:Ada", "X-SERVER-STAMP:1")
	loss, err := Compare(sent, stored)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !loss.Empty() {
		t.Errorf("loss = %+v, want none", loss)
	}
}

func TestCompareRefusesMissingObject(t *testing.T) {
	if _, err := Compare(nil, card(t, "FN:Ada")); err == nil {
		t.Error("Compare(nil, obj) succeeded")
	}
	if _, err := Compare(card(t, "FN:Ada"), nil); err == nil {
		t.Error("Compare(obj, nil) succeeded")
	}
}

// TestRegistryReportsFirstLossOnlyOnce is the aggregation rule of §8: the person
// is told the first time a property goes, and after that it lives in the
// account's details instead of interrupting every save.
func TestRegistryReportsFirstLossOnlyOnce(t *testing.T) {
	clock := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	reg := NewLossRegistry(func() time.Time { return clock })

	loss := PropertyLoss{Missing: []string{"X-CUSTOM"}}
	if !reg.Record("acc-1", loss) {
		t.Fatal("the first loss was not reported")
	}
	clock = clock.Add(time.Minute)
	if reg.Record("acc-1", loss) {
		t.Error("the same loss was reported a second time")
	}
	if reg.Record("acc-1", PropertyLoss{}) {
		t.Error("a clean write was reported as a loss")
	}
	if !reg.Record("acc-1", PropertyLoss{Missing: []string{"CATEGORIES"}}) {
		t.Error("a newly lost property was not reported")
	}

	report := reg.Report("acc-1")
	if report.Writes != 4 || report.LossyWrites != 3 {
		t.Fatalf("writes = %d, lossy = %d, want 4 and 3", report.Writes, report.LossyWrites)
	}
	if len(report.Properties) != 2 {
		t.Fatalf("properties = %+v, want two", report.Properties)
	}
	if report.Properties[0].Name != "X-CUSTOM" || report.Properties[0].Writes != 2 {
		t.Errorf("first entry = %+v, want X-CUSTOM lost twice", report.Properties[0])
	}
	if !report.Properties[0].Systematic {
		t.Error("a property lost by every save is not marked systematic")
	}
	if report.Properties[1].Systematic {
		t.Error("a property lost once is marked systematic")
	}
	if report.FirstSeen != clock.Add(-time.Minute) || report.LastSeen != clock {
		t.Errorf("first = %v, last = %v", report.FirstSeen, report.LastSeen)
	}
	if msg := report.Summary(); !strings.Contains(msg, "X-CUSTOM") || !strings.Contains(msg, "go every time") {
		t.Errorf("summary = %q", msg)
	}
}

// TestRegistryKeepsAccountsApart: one server's habits say nothing about another's.
func TestRegistryKeepsAccountsApart(t *testing.T) {
	reg := NewLossRegistry(nil)
	reg.Record("acc-1", PropertyLoss{Missing: []string{"X-CUSTOM"}})
	if !reg.Report("acc-2").Empty() {
		t.Error("a loss on one account showed up on another")
	}
	if !reg.Record("acc-2", PropertyLoss{Missing: []string{"X-CUSTOM"}}) {
		t.Error("the first loss on the second account was not reported")
	}
}

func TestRegistryForgetsDisconnectedAccount(t *testing.T) {
	reg := NewLossRegistry(nil)
	reg.Record("acc-1", PropertyLoss{Missing: []string{"X-CUSTOM"}})
	reg.Forget("acc-1")
	if !reg.Report("acc-1").Empty() {
		t.Error("Forget left the account's history behind")
	}
}

// TestNilRegistryIsUsable lets a caller run without aggregation instead of
// having to guard every call.
func TestNilRegistryIsUsable(t *testing.T) {
	var reg *LossRegistry
	if reg.Record("acc", PropertyLoss{Missing: []string{"X"}}) {
		t.Error("a nil registry reported a loss")
	}
	if !reg.Report("acc").Empty() {
		t.Error("a nil registry produced a report")
	}
	reg.Forget("acc")
}
