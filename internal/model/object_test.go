// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"strings"
	"testing"
)

// thirdPartyCard is what another client left on the server: X- properties, a
// grouped property, parameters, categories and a photo.
const thirdPartyCard = "BEGIN:VCARD\r\n" +
	"VERSION:3.0\r\n" +
	"UID:11111111-2222-3333-4444-555555555555\r\n" +
	"FN:Ada Lovelace\r\n" +
	"N:Lovelace;Ada;Byron;;\r\n" +
	"ORG:Analytical Engine;Research\r\n" +
	"TEL;TYPE=WORK,VOICE:+44 20 7000 0000\r\n" +
	"TEL;TYPE=CELL;PREF=1:+44 7700 900000\r\n" +
	"EMAIL;TYPE=INTERNET,HOME:ada@example.org\r\n" +
	"CATEGORIES:Mathematics,Friends\r\n" +
	"item1.URL:https://example.org/ada\r\n" +
	"item1.X-ABLabel:homepage\r\n" +
	"X-EVOLUTION-FILE-AS:Lovelace\\, Ada\r\n" +
	"X-CUSTOM-FLAG;TYPE=ODD:kept\r\n" +
	"PHOTO;ENCODING=b;TYPE=JPEG:/9j/4AAQSkZJRgABAQ\r\n" +
	"REV:20260101T000000Z\r\n" +
	"END:VCARD\r\n"

func mustParse(t *testing.T, body string) *Object {
	t.Helper()
	obj, err := ParseVCard("/ab/ada.vcf", `"etag-1"`, []byte(body))
	if err != nil {
		t.Fatalf("ParseVCard: %v", err)
	}
	return obj
}

// TestApplyKeepsForeignProperties is the stage 3 criterion of §21: a contact
// with X- properties keeps every original property after its name is edited.
func TestApplyKeepsForeignProperties(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	before := obj.Properties()

	patch := (&Patch{}).SetText("FN", "Ada King, Countess of Lovelace")
	if err := obj.Apply(patch); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	after := obj.Properties()
	if len(before) != len(after) {
		t.Fatalf("property count changed: %d before, %d after", len(before), len(after))
	}
	for i := range before {
		if before[i].Name != after[i].Name {
			t.Fatalf("property %d: %q before, %q after", i, before[i].Name, after[i].Name)
		}
		if before[i].Name == "FN" {
			continue
		}
		want := signatures(before[i].Values)
		got := signatures(after[i].Values)
		if !sameSignatures(want, got) {
			t.Errorf("%s changed: %v -> %v", before[i].Name, want, got)
		}
	}
	if got := obj.Property("FN")[0].Text; got != "Ada King, Countess of Lovelace" {
		t.Errorf("FN = %q", got)
	}
}

// TestMarshalPreservesEveryProperty checks the serialised form line by line: a
// property lost between parse and marshal would never reach the server again.
func TestMarshalPreservesEveryProperty(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := unfold(string(out))

	for _, want := range []string{
		"VERSION:3.0",
		"UID:11111111-2222-3333-4444-555555555555",
		"N:Lovelace;Ada;Byron;;",
		"ORG:Analytical Engine;Research",
		"CATEGORIES:Mathematics\\,Friends",
		"item1.URL:https://example.org/ada",
		"item1.X-ABLABEL:homepage",
		"X-EVOLUTION-FILE-AS:Lovelace\\, Ada",
		"X-CUSTOM-FLAG;TYPE=ODD:kept",
		"PHOTO;ENCODING=b;TYPE=JPEG:/9j/4AAQSkZJRgABAQ",
		"REV:20260101T000000Z",
	} {
		if !strings.Contains(body, want+"\r\n") {
			t.Errorf("marshalled card has no line %q\n%s", want, body)
		}
	}
	if count := strings.Count(body, "TEL;"); count != 2 {
		t.Errorf("TEL lines = %d, want 2", count)
	}
	if !strings.HasPrefix(body, "BEGIN:VCARD\r\nVERSION:3.0\r\n") {
		t.Errorf("card does not begin with BEGIN and VERSION:\n%s", body)
	}
	if !strings.HasSuffix(body, "END:VCARD\r\n") {
		t.Errorf("card does not end with END:\n%s", body)
	}
}

// TestMarshalNormalisesOnlyEquivalentForms pins the two rewrites a round trip
// does make. Both are equivalent by RFC 6350 §3.3 and §5.6 — property names are
// case-insensitive, and repeated TYPE parameters mean the same as one
// comma-separated list — so no information is lost. Anything beyond these two
// would be, which is why they are named here rather than left to be discovered.
func TestMarshalNormalisesOnlyEquivalentForms(t *testing.T) {
	obj := mustParse(t, "BEGIN:VCARD\r\n"+
		"VERSION:3.0\r\n"+
		"UID:x\r\n"+
		"X-ABLabel:homepage\r\n"+
		"TEL;TYPE=WORK,VOICE:+1\r\n"+
		"END:VCARD\r\n")
	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := unfold(string(out))
	if !strings.Contains(body, "X-ABLABEL:homepage\r\n") {
		t.Errorf("property name was not upper-cased:\n%s", body)
	}
	if !strings.Contains(body, "TEL;TYPE=WORK;TYPE=VOICE:+1\r\n") {
		t.Errorf("TYPE list was not split into repeated parameters:\n%s", body)
	}
}

// TestRoundTripIsStable checks that a card written by Carrel and read back again
// serialises identically, which is what lets Compare treat a difference after a
// PUT as the server's doing rather than the encoder's.
func TestRoundTripIsStable(t *testing.T) {
	first, err := mustParse(t, thirdPartyCard).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	reparsed, err := ParseVCard("/ab/ada.vcf", "", first)
	if err != nil {
		t.Fatalf("ParseVCard: %v", err)
	}
	second, err := reparsed.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("round trip is not stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestPhotoReplacementKeepsVersion is the §21 criterion that a vCard 3.0 stays
// 3.0 after its photo is replaced.
func TestPhotoReplacementKeepsVersion(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	patch := (&Patch{}).Set("PHOTO", Text("/9j/replaced").
		WithParam("ENCODING", "b").
		WithParam("TYPE", "JPEG"))
	if err := obj.Apply(patch); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := obj.Version(); got != "3.0" {
		t.Fatalf("version = %q, want 3.0", got)
	}
	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(out), "VERSION:3.0\r\n") {
		t.Errorf("serialised card is not 3.0:\n%s", out)
	}
	if strings.Contains(string(out), "4.0") {
		t.Errorf("serialised card mentions 4.0:\n%s", out)
	}
}

func TestApplyRefusesProtectedProperties(t *testing.T) {
	for _, name := range []string{"VERSION", "UID", "version", " uid "} {
		obj := mustParse(t, thirdPartyCard)
		patch := (&Patch{}).SetText(name, "tampered")
		if err := obj.Apply(patch); err == nil {
			t.Errorf("Apply(%q) succeeded, want refusal", name)
		}
		if obj.Version() != "3.0" || obj.UID() == "tampered" {
			t.Errorf("Apply(%q) changed the object anyway", name)
		}
	}
}

// TestApplyIsAllOrNothing checks that a patch with one bad operation leaves the
// object untouched rather than half-applied.
func TestApplyIsAllOrNothing(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	patch := (&Patch{}).SetText("NICKNAME", "Ada").SetText("VERSION", "4.0")
	if err := obj.Apply(patch); err == nil {
		t.Fatal("Apply succeeded, want refusal")
	}
	if obj.Has("NICKNAME") {
		t.Error("the accepted half of a rejected patch was applied")
	}
}

func TestApplyRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"", "  ", "BEGIN", "END", "FN:X", "X FN", "TEL\r\nFN"} {
		obj := mustParse(t, thirdPartyCard)
		if err := obj.Apply((&Patch{}).SetText(name, "x")); err == nil {
			t.Errorf("Apply(%q) succeeded, want refusal", name)
		}
	}
}

// TestSetWithoutValueIsRefused keeps the difference between an empty property
// and no property: §11 asks that deleting a photo remove PHOTO rather than write
// it empty.
func TestSetWithoutValueIsRefused(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	if err := obj.Apply((&Patch{}).Set("PHOTO")); err == nil {
		t.Fatal("Set with no value succeeded, want refusal")
	}
	if !obj.Has("PHOTO") {
		t.Error("PHOTO was removed by a refused patch")
	}
}

func TestRemoveDeletesProperty(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	if err := obj.Apply((&Patch{}).Remove("PHOTO")); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if obj.Has("PHOTO") {
		t.Fatal("PHOTO survived Remove")
	}
	out, err := obj.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(out), "PHOTO") {
		t.Errorf("serialised card still mentions PHOTO:\n%s", out)
	}
	if !strings.Contains(string(out), "X-CUSTOM-FLAG;TYPE=ODD:kept") {
		t.Error("removing one property disturbed another")
	}
}

func TestSetReplacesEveryInstance(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	if len(obj.Property("TEL")) != 2 {
		t.Fatalf("fixture has %d TEL, want 2", len(obj.Property("TEL")))
	}
	patch := (&Patch{}).Set("TEL", Text("+44 1234").WithParam("TYPE", "WORK"))
	if err := obj.Apply(patch); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	values := obj.Property("TEL")
	if len(values) != 1 || values[0].Text != "+44 1234" {
		t.Fatalf("TEL = %+v", values)
	}
}

func TestParseRejectsCardWithoutVersion(t *testing.T) {
	_, err := ParseVCard("/ab/x.vcf", "", []byte("BEGIN:VCARD\r\nFN:No Version\r\nEND:VCARD\r\n"))
	if err == nil {
		t.Fatal("a card without VERSION was accepted")
	}
	if !strings.Contains(err.Error(), "VERSION") {
		t.Errorf("error does not name the problem: %v", err)
	}
}

func TestParseRejectsEmptyBody(t *testing.T) {
	if _, err := ParseVCard("/ab/x.vcf", "", []byte("   \r\n")); err == nil {
		t.Fatal("an empty body was accepted")
	}
}

func TestCloneIsIndependent(t *testing.T) {
	obj := mustParse(t, thirdPartyCard)
	clone := obj.Clone()
	if err := clone.Apply((&Patch{}).SetText("FN", "Someone Else").Remove("X-CUSTOM-FLAG")); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := obj.Property("FN")[0].Text; got != "Ada Lovelace" {
		t.Errorf("editing the clone changed the original FN: %q", got)
	}
	if !obj.Has("X-CUSTOM-FLAG") {
		t.Error("editing the clone removed a property from the original")
	}
	// The parameter maps must be copies too, not shared.
	clone.raw["TEL"][0].Params["TYPE"][0] = "MUTATED"
	if obj.Property("TEL")[0].Param("TYPE") == "MUTATED" {
		t.Error("clone shares parameter storage with the original")
	}
}

func TestNewVCardStartsAlmostEmpty(t *testing.T) {
	obj, err := NewVCard("4.0", "abc-123")
	if err != nil {
		t.Fatalf("NewVCard: %v", err)
	}
	if got := obj.Names(); len(got) != 2 {
		t.Fatalf("new card has properties %v, want only VERSION and UID", got)
	}
	if obj.Version() != "4.0" || obj.UID() != "abc-123" {
		t.Fatalf("version = %q, uid = %q", obj.Version(), obj.UID())
	}
	if _, err := obj.Marshal(); err != nil {
		t.Fatalf("Marshal: %v", err)
	}
}

func TestNewVCardRefusesUnwritableVersion(t *testing.T) {
	if _, err := NewVCard("2.1", "abc"); err == nil {
		t.Error("NewVCard accepted version 2.1")
	}
	if _, err := NewVCard("3.0", "  "); err == nil {
		t.Error("NewVCard accepted an empty UID")
	}
}

func TestNewUIDIsUniqueAndShaped(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 64; i++ {
		uid, err := NewUID()
		if err != nil {
			t.Fatalf("NewUID: %v", err)
		}
		if len(uid) != 36 || strings.Count(uid, "-") != 4 {
			t.Fatalf("NewUID = %q, want a UUID shape", uid)
		}
		if seen[uid] {
			t.Fatalf("NewUID repeated %q", uid)
		}
		seen[uid] = true
	}
}

// unfold reverses line folding so a test can look for whole property lines.
func unfold(s string) string { return strings.ReplaceAll(s, "\r\n ", "") }
