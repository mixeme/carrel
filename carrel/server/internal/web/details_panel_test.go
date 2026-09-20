package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

type detailsFragment struct {
	Target string       `json:"target"`
	Body   view.Details `json:"body"`
}

// C-7.2: the panel beside a list is the zone "details", and nothing
// answered it. The row chosen on a list is asked for at its own address
// with that target, and comes back as the card cut to a panel — found by
// the first fixture comparison (fixtures-tail.md).
func TestRecordAnswersTheDetailsPanel(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nORG:Analytical Engines\nEMAIL:ada@example.com\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	got := decode[detailsFragment](t, f.h.do(fragment(get(rec, f.sess), "details")))
	if got.Target != "details" {
		t.Fatalf("target = %q", got.Target)
	}
	d := got.Body
	if d.Title != "Ada Lovelace" {
		t.Errorf("title = %q", d.Title)
	}
	var labels []string
	for _, fl := range d.Fields {
		labels = append(labels, string(fl.Label))
	}
	if !strings.Contains(strings.Join(labels, ","), "Organisation") {
		t.Errorf("fields = %v", labels)
	}
	if d.Source.Stamp == "" {
		t.Errorf("the panel does not say where the record came from: %+v", d.Source)
	}
	open := actionByID(d.Actions, "record.open")
	if open == nil || open.Href != rec {
		t.Fatalf("actions = %+v, want the way to the whole record first", d.Actions)
	}
	if d.Actions[0].ID != "record.open" {
		t.Errorf("the first action is %q", d.Actions[0].ID)
	}
	if actionByID(d.Actions, "record.edit") == nil {
		t.Errorf("the panel offers no edit: %+v", d.Actions)
	}

	// An event's panel carries the fields of its card.
	c := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nLOCATION:Room 4\nEND:VEVENT"))
	ev := decode[detailsFragment](t, c.h.do(fragment(get(c.path("calendar")+"/meet", c.sess), "details")))
	if ev.Body.Title != "Review" || len(ev.Body.Fields) == 0 {
		t.Errorf("event panel = %+v", ev.Body)
	}
}
