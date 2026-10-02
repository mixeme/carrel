package web_test

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

func notesPoll(p view.NotesPage) *view.PollBar { return p.Poll }

// The combined notebook as s-notes-1 draws it (the client's scan of the
// notes, НБ1–НБ6, НБ9, НВ1, НВ2): the line under the title, the month with
// its count, the filter that works without scripts, the row of a linked
// note with how many sources it has, and the note open in the panel
// without scripts, its row on — Edit and the note's page on view, the
// deletion the last item of «⋯». One notebook says the same of itself;
// nothing ticked says what to do.
func TestNotebookIsTheFrame(t *testing.T) {
	shape := regexp.MustCompile(`^\d+ of \d+ notebooks · \d+ notes · VJOURNAL$`)
	if sub := frameSubtitle(t, "s-notes-1"); !shape.MatchString(sub) {
		t.Fatalf("s-notes-1 writes %q under its title: read the frame again", sub)
	}
	frame := mockupFrame(t, "s-notes-1")
	groups := frameGroups(t, frame)
	if len(groups) != 1 || groups[0].rubric != "August 2026" || !strings.Contains(frame, `placeholder="Filter by word"`) {
		t.Fatalf("s-notes-1 groups = %+v: read the frame again", groups)
	}

	app := newMockupApp(t)
	linkMockupNote(t, app)
	app.lastUsed(t, "zapisi", view.KindNote)
	openList(t, app, "/app/calendar?from=2026-08-10") // the panel's Related
	openList(t, app, "/app/tasks")
	first := combined(t, app.h, app.sess, "/app/notes", notesPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	open := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
	page := decode[view.NotesPage](t, app.h.do(get("/app/notes?task="+first.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))

	if !shape.MatchString(string(page.Subtitle)) {
		t.Errorf("subtitle = %q, want the shape of %q", page.Subtitle, frameSubtitle(t, "s-notes-1"))
	}
	if len(page.Groups) != 1 || page.Groups[0].Rubric != "August 2026" || page.Groups[0].Count != len(page.Groups[0].Rows) || page.Groups[0].Count != 3 {
		t.Errorf("groups = %+v", page.Groups)
	}
	if n := actionByID(page.Bar.Actions, "record.new"); n == nil || len(n.Arrow) != 2 || !slices.ContainsFunc(n.Arrow, func(m view.MenuItem) bool { return m.Label == "Записи" && m.Aside == "last used" }) {
		t.Errorf("New note = %+v, want its arrow with «last used» (s-notes-13)", n)
	}
	if page.Bar.FilterForm == nil || page.Bar.FilterForm.Method != "GET" {
		t.Fatalf("filter = %+v, want a GET form", page.Bar.FilterForm)
	}
	var linked, opened *view.NoteRow
	for ri := range page.Groups[0].Rows {
		switch r := &page.Groups[0].Rows[ri]; r.Title {
		case "Что почитать про CRDT":
			linked = r
		case "Заметка со встречи 10.08":
			opened = r
		}
	}
	if linked == nil || linked.Stamp != "2 sources" || len(linked.Sources) != 2 {
		t.Errorf("the linked row = %+v, want «2 sources» and both stamps", linked)
	}
	if opened == nil || !opened.On {
		t.Errorf("the open row = %+v", opened)
	}
	d := page.Details
	if d == nil || d.Close == "" || len(d.Rendered) == 0 || d.Date == "" {
		t.Fatalf("panel = %+v", d)
	}
	if got := []string{d.Actions[0].ID, d.Actions[1].ID}; !slices.Equal(got, []string{"record.edit", "record.open"}) || d.Menu[len(d.Menu)-1].ID != "record.delete" {
		t.Errorf("panel actions = %+v, menu = %+v (s-notes-12)", d.Actions, d.Menu)
	}
	// s-notes-1 draws the event alone; the page draws it and the task
	// (s-notes-2) — the same note, both relations (task 105).
	if len(d.Related) != 2 || d.Related[0].Title != "Синк-ап по релизу" || d.Related[1].Title != "Дописать раздел про кэш" {
		t.Errorf("related = %+v", d.Related)
	}

	// The filter without scripts: the word, sent as the form sends it.
	q := formFields(*page.Bar.FilterForm)
	q.Set("filter", "crdt")
	sifted := decode[view.NotesPage](t, app.h.do(get("/app/notes?"+q.Encode(), app.sess)))
	if len(sifted.Groups) != 1 || len(sifted.Groups[0].Rows) != 1 || sifted.Groups[0].Rows[0].Title != "Что почитать про CRDT" || sifted.Bar.Filter != "crdt" {
		t.Errorf("filtered = %+v", sifted.Groups)
	}

	// One notebook: where, how many and as what.
	one := decode[view.NotesPage](t, app.h.do(get(app.listPath(t, "notes", "zapisi")+"?open="+url.QueryEscape(open), app.sess)))
	if one.Subtitle != "nextcloud · 2 notes · VJOURNAL" || one.Details == nil || one.Bar.FilterForm == nil {
		t.Errorf("one notebook = %q %+v", one.Subtitle, one.Details)
	}

	// Nothing ticked: what to do, and no poll.
	if rec := app.h.do(postForm("/app/sources/notes", app.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("untick everything: %d", rec.Code)
	}
	none := decode[view.NotesPage](t, app.h.do(get("/app/notes", app.sess)))
	if none.Poll != nil || none.Empty == nil || len(none.Empty.Actions) != 1 || none.Empty.Actions[0].ID != "rail.tick.all" {
		t.Errorf("nothing ticked = poll %+v, empty %+v", none.Poll, none.Empty)
	}
}
