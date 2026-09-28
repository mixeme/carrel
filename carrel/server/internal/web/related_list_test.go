package web_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

var (
	frameRelMenu = regexp.MustCompile(`(?s)<label>Related to</label>.*?<div class="tc-menu is-pop[^"]*">(.*?)</div>`)
	frameRelItem = regexp.MustCompile(`(?s)(<span class="tc-menu__sep"></span>\s*)?<button class="tc-menu__item[^"]*" type="button" role="menuitem">(.*?)</button>`)
	frameRelMeta = regexp.MustCompile(`(?s)<span class="tc-push[^"]*">(.*?)</span>`)
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
		{"Trip", "event · 5 January", "diary"}, {"Pack", "task · 5 January", "diary"},
		{"Fourth", "note · 4 January", "notes"}, {"Third", "note · 3 January", "notes"},
		{"Second", "note · 2 January", "notes"}, {"First", "note · 1 January", "notes"},
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
	for _, r := range note.Fields {
		if strings.HasPrefix(r.Name, "related.") {
			t.Errorf("note: a second field %q beside Related to", r.Name)
		}
	}
	noteWants := slices.Clone(wants[:3])
	noteWants = append(noteWants, want{"Tickets", "task · 3 January", "diary"}, wants[4], wants[5])
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
