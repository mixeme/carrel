package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

var (
	frameShortDay = regexp.MustCompile(`^\d{1,2} [A-Z][a-z]{2}$`)
	frameRelMenu  = regexp.MustCompile(`(?s)<label>Related to</label>.*?<div class="tc-menu is-pop[^"]*">(.*?)</div>`)
	frameRelItem  = regexp.MustCompile(`(?s)(<span class="tc-menu__sep"></span>\s*)?<button class="tc-menu__item[^"]*" type="button" role="menuitem">(.*?)</button>`)
	frameRelMeta  = regexp.MustCompile(`(?s)<span class="tc-push[^"]*">(.*?)</span>`)
)

type frameRelated struct {
	label, meta string
	mark, sep   bool
}

// frameRelatedList is the list of the Related to field as s-tasks-10 draws
// it open, item by item.
func frameRelatedList(t *testing.T) []frameRelated {
	t.Helper()
	menu := frameRelMenu.FindStringSubmatch(mockupFrame(t, "s-tasks-10"))
	if menu == nil {
		t.Fatal("s-tasks-10 draws no Related to list: read the frame again")
	}
	var out []frameRelated
	for _, m := range frameRelItem.FindAllStringSubmatch(menu[1], -1) {
		it := frameRelated{sep: m[1] != "", mark: strings.Contains(m[2], "tc-mark")}
		if meta := frameRelMeta.FindStringSubmatch(m[2]); meta != nil {
			it.meta = frameText(meta[1])
		}
		it.label = frameText(frameRelMeta.ReplaceAllString(m[2], ""))
		out = append(out, it)
	}
	return out
}

// from-typecase note 18, s-tasks-10: the list of Related to is the records
// this session has loaded — of every collection, each with its collection's
// mark and «kind · day» — newest first, and last, after a line, «By UID…»:
// the value typed by hand under the same name. No «Nothing» (the pill's
// cross empties the field), no second field for the UID.
// The task's form and the note's are the same field.
func TestRelatedToIsTheFramesList(t *testing.T) {
	frame := frameRelatedList(t)
	if len(frame) < 2 {
		t.Fatalf("s-tasks-10 reads otherwise: %+v", frame)
	}
	byUID, records := frame[len(frame)-1], frame[:len(frame)-1]
	kinds := map[string]bool{}
	for _, r := range records {
		kind, _, ok := strings.Cut(r.meta, " · ")
		if !r.mark || !ok || r.sep {
			t.Fatalf("s-tasks-10: a record %+v without a mark or «kind · day»: read the frame again", r)
		}
		kinds[kind] = true
		// A list says the day the short way, as the columns of a row do
		// (C-10): «12 Aug»; the record says it in full.
		if _, day, _ := strings.Cut(r.meta, " · "); !frameShortDay.MatchString(day) {
			t.Fatalf("s-tasks-10: the day %q is not «12 Aug»: read the frame again", day)
		}
	}
	if !byUID.sep || byUID.mark || byUID.meta != "" || len(kinds) != 3 {
		t.Fatalf("s-tasks-10: the last item %+v, kinds %v: read the frame again", byUID, kinds)
	}

	f := newNoteShelf(t)
	f.h.srv.SetClock(time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC))
	f.srv.Seed(f.diary+"trip.ics", ics("BEGIN:VEVENT\nUID:trip\nDTSTAMP:20260101T000000Z\nDTSTART:20260105T100000Z\nSUMMARY:Trip\nEND:VEVENT"))
	f.srv.Seed(f.diary+"pack.ics", ics("BEGIN:VTODO\nUID:pack\nDTSTAMP:20260101T000000Z\nDUE:20260105T090000Z\nSUMMARY:Pack\nEND:VTODO"))
	f.srv.Seed(f.diary+"tickets.ics", ics("BEGIN:VTODO\nUID:tickets\nDTSTAMP:20260101T000000Z\nDUE:20260103T090000Z\nSUMMARY:Tickets\nEND:VTODO"))
	diaryTasks := "/app/tasks/" + f.id + "/" + strings.TrimPrefix(f.notebook(f.diary), "/app/notes/"+f.id+"/")
	diaryEvents := "/app/calendar/" + f.id + "/" + strings.TrimPrefix(f.notebook(f.diary), "/app/notes/"+f.id+"/")
	_ = f.h.do(get(f.path("notes"), f.sess))
	_ = f.h.do(get(diaryTasks+"?show=all", f.sess))
	_ = f.h.do(get(diaryEvents+"?from=2026-01-05", f.sess))

	edit := diaryTasks + "/tickets/edit"
	related := func() view.FormRow {
		t.Helper()
		return rowNamed(t, decode[view.TaskEditPage](t, f.h.do(get(edit, f.sess))).Form, "related")
	}
	type want struct{ label, meta, col string }
	wants := []want{
		{"Trip", "event · 5 Jan", "diary"}, {"Pack", "task · 5 Jan", "diary"},
		{"Fourth", "note · 4 Jan", "notes"}, {"Third", "note · 3 Jan", "notes"},
		{"Second", "note · 2 Jan", "notes"}, {"First", "note · 1 Jan", "notes"},
	}
	check := func(what string, row view.FormRow, wants []want, selected string) {
		t.Helper()
		if row.Kind != view.InputSelect || string(row.Value) != selected {
			t.Errorf("%s: related = %s %q, want a list holding %q", what, row.Kind, row.Value, selected)
		}
		if len(row.Options) != len(wants)+1 {
			t.Fatalf("%s: options = %+v, want %d records and «By UID…»", what, row.Options, len(wants))
		}
		colours := map[string]view.Colour{}
		for i, w := range wants {
			o := row.Options[i]
			kind, _, _ := strings.Cut(w.meta, " · ")
			if string(o.Label) != w.label || string(o.Meta) != w.meta || !kinds[kind] || o.Colour == "" || o.Separator || o.Typed ||
				o.Selected != (o.Value == selected) {
				t.Errorf("%s: option %d = %+v, want %q «%s» with its collection's mark", what, i, o, w.label, w.meta)
			}
			if c, seen := colours[w.col]; seen && c != o.Colour {
				t.Errorf("%s: %q is marked %q, its collection %q", what, w.label, o.Colour, c)
			}
			colours[w.col] = o.Colour
		}
		if colours["diary"] == colours["notes"] {
			t.Errorf("%s: two collections, one mark %q", what, colours["diary"])
		}
		last := row.Options[len(row.Options)-1]
		if string(last.Label) != byUID.label || !last.Separator || !last.Typed || last.Value != "" || last.Selected || last.Colour != "" || last.Meta != "" {
			t.Errorf("%s: the last option = %+v, the frame draws %q after a line", what, last, byUID.label)
		}
	}
	check("task", related(), wants, "")

	// The note's Related to is the same list, less the note itself.
	note := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n3/edit", f.sess))).Doc
	// The line one more link is typed into is the field's own (FormRow.Add,
	// Typecase's task 142), not a row beside it.
	for _, r := range note.Fields {
		if strings.HasPrefix(r.Name, "related.") {
			t.Errorf("note: a second field %q beside Related to", r.Name)
		}
	}
	noteWants := slices.Clone(wants[:3])
	noteWants = append(noteWants, want{"Tickets", "task · 3 Jan", "diary"}, wants[4], wants[5])
	check("note", docRow(t, note, "related"), noteWants, "")

	save := func(value string) string {
		t.Helper()
		form := formFields(decode[view.TaskEditPage](t, f.h.do(get(edit, f.sess))).Form)
		form.Set("related", value)
		if resp := f.h.do(postForm(edit, f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST related=%q: %d %s", value, resp.Code, resp.Body.String())
		}
		b, _, _ := f.srv.Object(f.diary + "tickets.ics")
		return unfolded(b)
	}
	// Picked from the list, and typed by hand: one name, one value.
	mustContain(t, save("pack"), "RELATED-TO:pack")
	check("task related to Pack", related(), wants, "pack")
	got := save("elsewhere-uid")
	if !strings.Contains(got, "RELATED-TO:elsewhere-uid") || strings.Contains(got, "RELATED-TO:pack") {
		t.Errorf("typed by hand:\n%s", got)
	}
	// A record not loaded is the pill's own: its UID, first, chosen.
	row := related()
	if first := row.Options[0]; string(row.Value) != "elsewhere-uid" || first.Value != "elsewhere-uid" || string(first.Label) != "elsewhere-uid" ||
		!first.Selected || first.Meta != "" || len(row.Options) != len(wants)+2 {
		t.Errorf("a relation outside the loaded lists = %q, %+v", row.Value, row.Options)
	}
	if got := save(""); strings.Contains(got, "RELATED-TO") {
		t.Errorf("an emptied field kept its line:\n%s", got)
	}
}

// from-typecase note 19, s-tasks-11: Related to holds one relation — the
// first RELATED-TO; the others stay as they are. A record not among the
// loaded ones is a pill with its UID and the frame's line under it; a card
// (a note a timeline wrote for a person) is its name and «contact». «By
// UID…» opens a line for the UID in place of the field: its sample and its
// line are the frame's.
func TestRelatedToOutsideTheLoadedIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-tasks-11")
	type drawn struct{ pill, hint, placeholder string }
	var fields []drawn
	for _, m := range regexp.MustCompile(`(?s)<label>Related to</label>(.*?)</div>`).FindAllStringSubmatch(frame, -1) {
		var d drawn
		if p := regexp.MustCompile(`(?s)<span class="tc-tag tc-tag--pill">(.*?)</span>`).FindStringSubmatch(m[1]); p != nil {
			d.pill = frameText(p[1])
		}
		if h := frameAnyHint.FindStringSubmatch(m[1]); h != nil {
			d.hint = frameText(h[1])
		}
		if ph := regexp.MustCompile(`placeholder="([^"]*)"`).FindStringSubmatch(m[1]); ph != nil {
			d.placeholder = ph[1]
		}
		fields = append(fields, d)
	}
	if len(fields) != 3 || fields[0].hint == "" || !strings.HasSuffix(fields[1].pill, " · contact") || fields[1].hint != "" ||
		fields[2].pill != "" || fields[2].placeholder == "" || fields[2].hint == "" {
		t.Fatalf("s-tasks-11 = %+v: read the frame again", fields)
	}

	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"anna.vcf", "BEGIN:VCARD\nVERSION:3.0\nUID:anna\nFN:Анна Ковалёва\nN:Ковалёва;Анна;;;\nEND:VCARD")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VTODO"}})
	srv.Seed(searchWork+"far.ics", ics("BEGIN:VTODO\nUID:far\nDTSTAMP:20260101T000000Z\nSUMMARY:Far\nRELATED-TO:7c1e-40aa-b913\nRELATED-TO:second\nEND:VTODO"))
	srv.Seed(searchWork+"card.ics", ics("BEGIN:VTODO\nUID:card\nDTSTAMP:20260101T000000Z\nSUMMARY:Call\nRELATED-TO:anna\nEND:VTODO"))
	f := connectFixture(t, srv, searchWork, nil)
	_ = f.h.do(get("/app/contacts/"+f.id+"/"+url.PathEscape(searchBook), f.sess))
	_ = f.h.do(get(f.path("tasks")+"?show=all", f.sess))
	related := func(uid string) view.FormRow {
		t.Helper()
		return rowNamed(t, decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/"+uid+"/edit", f.sess))).Form, "related")
	}

	far := related("far")
	if chosen := far.Options[0]; string(far.Value) != "7c1e-40aa-b913" || !chosen.Selected || string(chosen.Label) != "7c1e-40aa-b913" ||
		string(far.Hint) != fields[0].hint {
		t.Errorf("a record not loaded = %q %+v, hint %q; want the pill «7c1e-40aa-b913» and %q", far.Value, chosen, far.Hint, fields[0].hint)
	}
	card := related("card")
	if chosen := card.Options[0]; !chosen.Selected || string(chosen.Label) != "Анна Ковалёва · contact" || chosen.Meta != "" || card.Hint != "" {
		t.Errorf("a card = %+v, hint %q; want the pill %q", chosen, card.Hint, "Анна Ковалёва · contact")
	}
	for what, row := range map[string]view.FormRow{"far": far, "card": card} {
		typed := row.Options[len(row.Options)-1]
		if !typed.Typed || string(typed.Hint) != fields[2].hint || string(row.Placeholder) != fields[2].placeholder {
			t.Errorf("%s: «By UID…» = %+v, sample %q; want the line %q with %q", what, typed, row.Placeholder, fields[2].placeholder, fields[2].hint)
		}
	}

	// The field is every RELATED-TO (the owner's decision of 09.10.2026):
	// the one unticked goes, the one kept stays, the one picked is added.
	form := formFields(decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/far/edit", f.sess))).Form)
	if got := form["related"]; !slices.Equal(got, []string{"7c1e-40aa-b913", "second"}) {
		t.Fatalf("the form's relations = %q, want both of the record", got)
	}
	form["related"] = []string{"second", "card"}
	if rec := f.h.do(postForm(f.path("tasks")+"/far/edit", f.sess, form)); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	b, _, _ := srv.Object(searchWork + "far.ics")
	if got := unfolded(b); !strings.Contains(got, "RELATED-TO:card") || !strings.Contains(got, "RELATED-TO:second") || strings.Contains(got, "7c1e") {
		t.Errorf("saved:\n%s", got)
	}
}

// from-typecase note 19, s-notes-2: a related record in the column of a
// note is its title, its kind and its day the short way of a list — «event
// · 10 Aug» — and a record without a day only its kind.
func TestRelatedRowsSayTheDayShort(t *testing.T) {
	item := regexp.MustCompile(`(?s)<span class="tc-stack__item"><span class="tc-dim--accent">(.*?)</span> <span class="tc-dim">· (.*?)</span></span>`)
	var days []string
	for _, m := range item.FindAllStringSubmatch(frameBlock(t, mockupFrame(t, "s-notes-2"), "Related"), -1) {
		_, day, _ := strings.Cut(frameText(m[2]), " · ")
		days = append(days, day)
	}
	if len(days) != 2 || !frameShortDay.MatchString(days[0]) || days[1] != "" {
		t.Fatalf("s-notes-2 relates %q: read the frame again", days)
	}
	f := newNoteShelf(t)
	f.h.srv.SetClock(time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC))
	f.srv.Seed(f.col+"rel.ics", ics("BEGIN:VJOURNAL\nUID:rel\nDTSTAMP:20260101T000000Z\nDTSTART:20260105T090000Z\n"+
		"SUMMARY:Rel\nRELATED-TO:n2\nRELATED-TO:t0\nEND:VJOURNAL"))
	f.srv.Seed(f.col+"t0.ics", ics("BEGIN:VTODO\nUID:t0\nDTSTAMP:20260101T000000Z\nSUMMARY:Undated\nEND:VTODO"))
	_ = f.h.do(get(f.path("notes"), f.sess))
	_ = f.h.do(get(strings.Replace(f.path("notes"), "/app/notes/", "/app/tasks/", 1)+"?show=all", f.sess))
	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/rel", f.sess)))
	got := map[string]string{}
	for _, r := range page.Meta.Related {
		got[string(r.Title)] = string(r.When)
	}
	if got["Second"] != "2 Jan" || got["Undated"] != "" || len(got) != 2 {
		t.Errorf("related = %+v, want Second on «2 Jan» and Undated without a day", page.Meta.Related)
	}
}
