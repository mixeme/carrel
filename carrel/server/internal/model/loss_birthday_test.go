package model

import "testing"

// SV-56 (the owner's decision 04.11.2026, variant А; found live the same
// day): Google keeps a card as vCard 3.0, and a birthday without a year only
// as Apple writes it — and it writes Apple's marker back inside VALUE:
// BDAY;VALUE=DATE,X-APPLE-OMIT-YEAR=1604:1604-04-13. That is the day that
// was sent, in each of its forms, and calling BDAY lost told a person of a
// loss that had not happened. A day changed, a year added or taken away, is
// still a loss.
func TestADayWithoutAYearInAnotherFormIsNotALoss(t *testing.T) {
	apple := map[string]string{"X-APPLE-OMIT-YEAR": "1604"}
	google := map[string]string{"VALUE": "DATE,X-APPLE-OMIT-YEAR=1604"}
	for _, c := range []struct {
		name       string
		sentText   string
		sentParams map[string]string
		gotText    string
		gotParams  map[string]string
		wantLost   bool
	}{
		{name: "Apple's form kept as Google keeps it", sentText: "1604-04-13", sentParams: apple, gotText: "1604-04-13", gotParams: google},
		{name: "--MMDD kept as Google keeps Apple's form", sentText: "--0413", gotText: "1604-04-13", gotParams: google},
		{name: "--MM-DD kept as Apple writes it", sentText: "--04-13", gotText: "1604-04-13", gotParams: apple},
		{name: "a date with VALUE=DATE added", sentText: "1990-04-13", gotText: "1990-04-13", gotParams: map[string]string{"VALUE": "DATE"}},
		{name: "a date written the other way", sentText: "19900413", gotText: "1990-04-13"},
		{name: "another day", sentText: "1604-04-13", sentParams: apple, gotText: "1604-04-14", gotParams: google, wantLost: true},
		{name: "the marker gone: the year 1604", sentText: "1604-04-13", sentParams: apple, gotText: "1604-04-13", wantLost: true},
		{name: "a year the server made up", sentText: "--0413", gotText: "1990-04-13", wantLost: true},
		{name: "another parameter", sentText: "--0413", gotText: "1604-04-13", gotParams: map[string]string{"VALUE": "DATE,X-APPLE-OMIT-YEAR=1604", "X-A": "1"}, wantLost: true},
	} {
		sent := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{"BDAY": {{Text: c.sentText, Params: c.sentParams}}})
		got := NewObject("/c/u.vcf", `"2"`, map[string][]RawValue{"BDAY": {{Text: c.gotText, Params: c.gotParams}}})
		if loss := CompareAfterPut("google", sent, got, nil); (loss != nil) != c.wantLost {
			t.Errorf("%s: loss %+v, want lost: %v", c.name, loss, c.wantLost)
		}
	}
}

// AppleOmitYear reads the marker in either place it is kept.
func TestAppleOmitYearInEitherPlace(t *testing.T) {
	for _, c := range []struct {
		params map[string]string
		year   string
		folded bool
	}{
		{map[string]string{"X-APPLE-OMIT-YEAR": "1604"}, "1604", false},
		{map[string]string{"VALUE": "DATE,X-APPLE-OMIT-YEAR=1604"}, "1604", true},
		{map[string]string{"VALUE": "DATE"}, "", false},
		{nil, "", false},
	} {
		if year, folded := AppleOmitYear(c.params); year != c.year || folded != c.folded {
			t.Errorf("%v: %q %v, want %q %v", c.params, year, folded, c.year, c.folded)
		}
	}
}
