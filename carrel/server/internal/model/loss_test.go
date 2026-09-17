package model

import (
	"testing"
)

func TestCompareAfterPutDetectsLoss(t *testing.T) {
	sent := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"FN":   {{Text: "Ada"}},
		"NOTE": {{Text: "secret"}},
	})
	got := NewObject("/c/u.vcf", `"2"`, map[string][]RawValue{
		"FN": {{Text: "Ada"}},
	})
	agg := newMapAgg()
	loss := CompareAfterPut("srv1", sent, got, agg)
	if loss == nil || len(loss.Properties) != 1 || loss.Properties[0] != "NOTE" {
		t.Fatalf("loss = %#v", loss)
	}
	if loss.Aggregated {
		t.Fatal("first loss should not be aggregated")
	}
}

func TestCompareAfterPutAggregatesRepeat(t *testing.T) {
	sent := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"NOTE": {{Text: "x"}},
	})
	got := NewObject("/c/u.vcf", `"2"`, map[string][]RawValue{})
	agg := newMapAgg()
	agg.Note("srv1", "NOTE")
	loss := CompareAfterPut("srv1", sent, got, agg)
	if loss == nil || !loss.Aggregated {
		t.Fatalf("repeat loss should be aggregated: %#v", loss)
	}
}

func TestCompareAfterPutIgnoresServerStamps(t *testing.T) {
	sent := NewObject("/c/e.ics", `"1"`, map[string][]RawValue{
		"SUMMARY": {{Text: "Meet"}},
		"DTSTAMP": {{Text: "20260101T120000Z"}},
	})
	got := NewObject("/c/e.ics", `"2"`, map[string][]RawValue{
		"SUMMARY": {{Text: "Meet"}},
		"DTSTAMP": {{Text: "20260102T120000Z"}},
	})
	if loss := CompareAfterPut("srv1", sent, got, nil); loss != nil {
		t.Fatalf("stamp change should not count as loss: %#v", loss)
	}
}

// docs/servers.md, SV-27: a server that hands a structured value back with its
// trailing empty components dropped. "Lovelace;Ada" is the name
// "Lovelace;Ada;;;" — nothing was lost, and a loss report about it would teach
// people to close the report unread. A component that actually changed still
// counts.
func TestCompareIgnoresTrailingEmptyComponents(t *testing.T) {
	sent := NewObject("/c/u.vcf", `"1"`, map[string][]RawValue{
		"FN":  {{Text: "Ada Lovelace"}},
		"N":   {{Text: "Lovelace;Ada;;;"}},
		"ADR": {{Text: ";;Main St;Town;;;"}},
	})
	got := NewObject("/c/u.vcf", `"2"`, map[string][]RawValue{
		"FN":  {{Text: "Ada Lovelace"}},
		"N":   {{Text: "Lovelace;Ada"}},
		"ADR": {{Text: ";;Main St;Town"}},
	})
	if loss := CompareAfterPut("srv1", sent, got, nil); loss != nil {
		t.Fatalf("a shorter structured value is the same value: %#v", loss)
	}

	cut := NewObject("/c/u.vcf", `"2"`, map[string][]RawValue{
		"FN":  {{Text: "Ada Lovelace"}},
		"N":   {{Text: "Lovelace"}},
		"ADR": {{Text: ";;Main St;Town"}},
	})
	loss := CompareAfterPut("srv2", sent, cut, nil)
	if loss == nil || len(loss.Properties) != 1 || loss.Properties[0] != "N" {
		t.Fatalf("a dropped given name is a loss: %#v", loss)
	}

	// A plain text property keeps every semicolon it was given.
	text := NewObject("/c/n.ics", `"1"`, map[string][]RawValue{"SUMMARY": {{Text: "Two;"}}})
	same := NewObject("/c/n.ics", `"2"`, map[string][]RawValue{"SUMMARY": {{Text: "Two"}}})
	if loss := CompareAfterPut("srv3", text, same, nil); loss == nil {
		t.Error("SUMMARY is not structured: a changed value is a loss")
	}
}
