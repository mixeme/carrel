package model

import (
	"testing"
)

func TestCompareAfterPutDetectsLoss(t *testing.T) {
	sent := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"FN":   {values: []propValue{{text: "Ada"}}},
		"NOTE": {values: []propValue{{text: "secret"}}},
	})
	got := NewObjectFromRaw("/c/u.vcf", `"2"`, map[string]propState{
		"FN": {values: []propValue{{text: "Ada"}}},
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
	sent := NewObjectFromRaw("/c/u.vcf", `"1"`, map[string]propState{
		"NOTE": {values: []propValue{{text: "x"}}},
	})
	got := NewObjectFromRaw("/c/u.vcf", `"2"`, map[string]propState{})
	agg := newMapAgg()
	agg.Note("srv1", "NOTE")
	loss := CompareAfterPut("srv1", sent, got, agg)
	if loss == nil || !loss.Aggregated {
		t.Fatalf("repeat loss should be aggregated: %#v", loss)
	}
}

func TestCompareAfterPutIgnoresServerStamps(t *testing.T) {
	sent := NewObjectFromRaw("/c/e.ics", `"1"`, map[string]propState{
		"SUMMARY": {values: []propValue{{text: "Meet"}}},
		"DTSTAMP": {values: []propValue{{text: "20260101T120000Z"}}},
	})
	got := NewObjectFromRaw("/c/e.ics", `"2"`, map[string]propState{
		"SUMMARY": {values: []propValue{{text: "Meet"}}},
		"DTSTAMP": {values: []propValue{{text: "20260102T120000Z"}}},
	})
	if loss := CompareAfterPut("srv1", sent, got, nil); loss != nil {
		t.Fatalf("stamp change should not count as loss: %#v", loss)
	}
}
