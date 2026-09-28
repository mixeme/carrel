package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

var (
	framePanelRubric = regexp.MustCompile(`<div class="tc-rubric">([^<]+)`)
	framePanelTerm   = regexp.MustCompile(`<dt>([^<]+)</dt>`)
)

// framePanel is the panel a list frame draws beside the list.
func framePanel(t *testing.T, id string) string {
	t.Helper()
	frame := mockupFrame(t, id)
	at := strings.Index(frame, `data-part="panel"`)
	if at < 0 {
		t.Fatalf("%s draws no panel: read the frame again", id)
	}
	if end := strings.Index(frame[at:], "<figcaption>"); end > 0 {
		return frame[at : at+end]
	}
	return frame[at:]
}

// from-typecase note 9: the panel beside a list is the record for reading,
// as each kind's panel frame draws it. Every rubric of the frame has its
// block in Details and every row of its field list a field — tags as Tags —,
// filled for a record that holds all of them.
func TestPanelsReadAsTheirFrames(t *testing.T) {
	blocks := map[string]func(d view.Details) bool{
		"Description":                  func(d view.Details) bool { return d.Body != "" },
		"Attendees":                    func(d view.Details) bool { return len(d.Attendees) > 0 },
		"Attachments":                  func(d view.Details) bool { return len(d.Attach) > 0 },
		"Related":                      func(d view.Details) bool { return len(d.Related) > 0 },
		"Source":                       func(d view.Details) bool { return d.Source.Stamp != "" },
		"Kept as the server sent them": func(d view.Details) bool { return len(d.Foreign) > 0 },
	}
	check := func(frame string, d view.Details) {
		t.Helper()
		panel := framePanel(t, frame)
		for _, m := range framePanelRubric.FindAllStringSubmatch(panel, -1) {
			rubric := strings.TrimSpace(m[1])
			has, ok := blocks[rubric]
			switch {
			case !ok:
				t.Errorf("%s: the rubric %q has no block in Details", frame, rubric)
			case !has(d):
				t.Errorf("%s: %q is empty: %+v", frame, rubric, d)
			}
		}
		for _, m := range framePanelTerm.FindAllStringSubmatch(panel, -1) {
			term := strings.TrimSpace(m[1])
			if term == "Tags" || term == "Categories" {
				if len(d.Tags) == 0 {
					t.Errorf("%s: no tags: %+v", frame, d)
				}
				continue
			}
			found := false
			for _, f := range d.Fields {
				found = found || string(f.Label) == term
			}
			if !found {
				t.Errorf("%s: no field %q in %+v", frame, term, d.Fields)
			}
		}
		for _, f := range d.Fields {
			if f.Label == "Categories" || f.Label == "Tags" {
				t.Errorf("%s: the tags are Tags, not a field too: %+v", frame, f)
			}
		}
		if !strings.HasPrefix(string(d.Subtitle), string(d.Source.Stamp)) || d.Source.Stamp == "" {
			t.Errorf("%s: subtitle %q does not say where the record is (%q)", frame, d.Subtitle, d.Source.Stamp)
		}
		if strings.Contains(panel, "tc-avatar") != (d.Avatar != nil) {
			t.Errorf("%s: avatar %+v", frame, d.Avatar)
		}
		if strings.Contains(panel, "tc-doc__task") {
			tasks := 0
			for _, b := range d.Rendered {
				for _, c := range b.Children {
					if c.Kind == view.BlockTask {
						tasks++
					}
				}
			}
			if tasks == 0 {
				t.Errorf("%s: the text has no task items: %+v", frame, d.Rendered)
			}
		}
	}
	panelOf := func(h *harness, rec string, f davFixture) view.Details {
		t.Helper()
		return decode[detailsFragment](t, h.do(fragment(get(rec, f.sess), "details"))).Body
	}

	c := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:anna\nFN:Anna Kovaleva\nORG:Slovo\nTITLE:Editor\n"+
		"TEL;TYPE=cell:+7 912\nTEL;TYPE=work:+7 495\nEMAIL;TYPE=work:a@example.org\nBDAY:1987-03-14\n"+
		"CATEGORIES:work,press\nNOTE:Writes on Tuesdays.\nX-FOO:kept\nEND:VCARD\n")
	check("s-contacts-1", panelOf(c.h, c.path("contacts")+"/anna", c))

	attach := "ATTACH;FMTTYPE=text/markdown;FILENAME=list.md:https://files.example.org/list.md\n"
	cal := calendarFixture(t, nil,
		"ev.ics", ics("BEGIN:VEVENT\nUID:ev\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T070000Z\nDTEND:20260810T080000Z\n"+
			"RRULE:FREQ=WEEKLY;BYDAY=MO\nSUMMARY:Sync\nLOCATION:Room 2\nSTATUS:CONFIRMED\nCATEGORIES:release,team\n"+
			"DESCRIPTION:Go over the release.\nATTENDEE;CN=Anna;PARTSTAT=ACCEPTED:mailto:a@example.org\n"+attach+
			"RELATED-TO:nt\nBEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"),
		"td.ics", ics("BEGIN:VTODO\nUID:td\nDTSTAMP:20260101T000000Z\nDUE:20260812T150000Z\nSUMMARY:Pack\nSTATUS:NEEDS-ACTION\n"+
			"PRIORITY:5\nPERCENT-COMPLETE:0\nCATEGORIES:home\nDESCRIPTION:Chargers.\n"+attach+"RELATED-TO:ev\nEND:VTODO"),
		"nt.ics", ics("BEGIN:VJOURNAL\nUID:nt\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T080500Z\nSUMMARY:Meeting notes\n"+
			"DESCRIPTION:We agreed.\\n\\n- [ ] check the changelog\\n- [x] ask for screenshots\nCATEGORIES:meetings\n"+attach+
			"RELATED-TO:ev\nEND:VJOURNAL"))
	ev := panelOf(cal.h, cal.path("calendar")+"/ev", cal)
	check("s-agenda-1", ev)
	if ev.DeleteNote == "" {
		t.Errorf("s-agenda-1: a series says under Delete that it is deleted whole: %+v", ev)
	}
	check("s-tasks-1", panelOf(cal.h, cal.path("tasks")+"/td", cal))
	note := panelOf(cal.h, cal.path("notes")+"/nt", cal)
	check("s-notes-1", note)
	if !strings.Contains(string(note.Subtitle), "2026") {
		t.Errorf("s-notes-1: the subtitle of a note says when it was written: %q", note.Subtitle)
	}
}

type detailsFragment struct {
	Target string       `json:"target"`
	Body   view.Details `json:"body"`
}

// C-7.2: the panel beside a list is the zone "details", and nothing
// answered it. The row chosen on a list is asked for at its own address
// with that target, and comes back as the card cut to a panel — found by
// the first fixture comparison (review Р-13).
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
	// Edit first, as the panel frames draw it (s-contacts-1); the row itself
	// opens the whole record, and no button of the panel repeats it.
	if len(d.Actions) == 0 || d.Actions[0].ID != "record.edit" || d.Actions[0].Href != rec+"/edit" {
		t.Errorf("actions = %+v, want Edit first", d.Actions)
	}
	if actionByID(d.Actions, "record.open") != nil {
		t.Errorf("the panel draws no Open: %+v", d.Actions)
	}

	// An event's panel carries the fields of its card.
	c := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nLOCATION:Room 4\nEND:VEVENT"))
	ev := decode[detailsFragment](t, c.h.do(fragment(get(c.path("calendar")+"/meet", c.sess), "details")))
	if ev.Body.Title != "Review" || len(ev.Body.Fields) == 0 {
		t.Errorf("event panel = %+v", ev.Body)
	}
}
