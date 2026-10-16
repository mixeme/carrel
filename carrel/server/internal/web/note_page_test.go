package web_test

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// noteShelf is the ground of from-typecase note 9: Notes with four notes a
// day apart, Diary beside it, and a read-only Shared, on one server.
type noteShelf struct {
	davFixture
	diary, shared string
}

var shelfTitles = []string{"First", "Second", "Third", "Fourth"}

func newNoteShelf(t *testing.T) noteShelf {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notes := davtest.CalendarHome + "notes/"
	diary := davtest.CalendarHome + "diary/"
	shared := davtest.CalendarHome + "shared/"
	srv.AddCalendar(caldav.Calendar{Path: notes, Name: "Notes"})
	srv.AddCalendar(caldav.Calendar{Path: diary, Name: "Diary"})
	srv.AddCalendar(caldav.Calendar{Path: shared, Name: "Shared", ReadOnly: true})
	for i, title := range shelfTitles {
		uid := "n" + string(rune('1'+i))
		srv.Seed(notes+uid+".ics", ics("BEGIN:VJOURNAL\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:2026010"+string(rune('1'+i))+"T090000Z\nSUMMARY:"+title+"\nDESCRIPTION:Text of "+title+"\n"+
			"CATEGORIES:work\nX-JTX-COLOR:-16711936\nX-JTX-PINNED:1\nX-OTHER:kept\nEND:VJOURNAL"))
	}
	return noteShelf{davFixture: connectFixture(t, srv, notes, nil), diary: diary, shared: shared}
}

func (f noteShelf) notebook(col string) string {
	return "/app/notes/" + f.id + "/" + url.PathEscape(col)
}

var (
	frameFieldLabel = regexp.MustCompile(`<div class="tc-field[^"]*">\s*<label>([^<]+)</label>`)
	frameTreeRow    = regexp.MustCompile(`<div class="([^"]*)tc-tree__row">(.*?)</div>`)
	frameNavItem    = regexp.MustCompile(`(?s)<a class="[^"]*tc-nav__item[^"]*"[^>]*>(.*?)</a>`)
	// frameNavShort is the short label a nav item carries for the phone's
	// bottom panel («Back», from-typecase note 27); the rail's is the full
	// one beside it.
	frameNavShort = regexp.MustCompile(`<span class="tc-nav__short">[^<]*</span>`)
	frameDocNote  = regexp.MustCompile(`(?s)<div class="tc-doc__note">(.*?)</div>`)
)

// frameNeighbours is how many neighbours the frame's rail draws before and
// after the open note.
func frameNeighbours(t *testing.T, id string) (before, after int) {
	t.Helper()
	frame := mockupFrame(t, id)
	at := strings.Index(frame, ">Neighbours<")
	if at < 0 {
		t.Fatalf("%s draws no Neighbours: read the frame again", id)
	}
	rows := frameTreeRow.FindAllStringSubmatch(frame[at:], -1)
	on := -1
	for i, r := range rows {
		if strings.Contains(r[1], "is-on") {
			on = i
		}
	}
	if on < 0 {
		t.Fatalf("%s: no neighbour is the open note", id)
	}
	return on, len(rows) - on - 1
}

func neighbourLabels(rail view.Rail) (labels []string, on string) {
	for _, a := range rail.Neighbours {
		labels = append(labels, string(a.Label))
		if a.On {
			on = string(a.Label)
		}
	}
	return labels, on
}

// from-typecase notes 9 and 18, s-notes-2 and s-notes-20: the rail of a
// note's page, read and edited, lists its neighbours in the order of the
// notebook's list and marks the open one. The frames open a note inside the
// list and draw as many before it as after it — one on each side (s-notes-2,
// s-notes-3, s-notes-20); the newest has nothing before it. The
// neighbours are all of the open notebook — a note of another one, however
// close its day, is not among them. They come from the list the session
// holds: a notebook not loaded has none, and nothing is asked for them.
func TestNotePageRailHasNeighbours(t *testing.T) {
	f := newNoteShelf(t)
	list := f.path("notes")
	// Diary holds a note between Third and Fourth.
	f.srv.Seed(f.diary+"d1.ics", ics("BEGIN:VJOURNAL\nUID:d1\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART:20260103T120000Z\nSUMMARY:Elsewhere\nEND:VJOURNAL"))

	cold := decode[view.NotePage](t, f.h.do(get(list+"/n3", f.sess)))
	if len(cold.Rail.Neighbours) != 0 {
		t.Errorf("neighbours of a notebook never loaded = %+v", cold.Rail.Neighbours)
	}

	_ = f.h.do(get(list, f.sess))
	_ = f.h.do(get(f.notebook(f.diary), f.sess))
	span, after := frameNeighbours(t, "s-notes-20")
	for _, id := range []string{"s-notes-2", "s-notes-3"} {
		if b, a := frameNeighbours(t, id); b != span || a != after {
			t.Fatalf("%s and s-notes-20 draw different neighbours: read the frames again", id)
		}
	}
	// Since Typecase's note 44 the frames open the newest note of its
	// notebook — the neighbours are the notes of that notebook, and «Записи»
	// of the set has two —, so they draw none before it and one after: the
	// newest has nothing before it. One on a side is what they show.
	if span > after || after < 1 {
		t.Fatalf("s-notes-20 draws %d before and %d after the open note: read the frame again", span, after)
	}
	span = after
	newest := []string{"Fourth", "Third", "Second", "First"}
	for _, open := range []struct{ uid, title string }{{"n4", "Fourth"}, {"n3", "Third"}, {"n1", "First"}} {
		at := slices.Index(newest, open.title)
		want := newest[max(0, at-span):min(len(newest), at+span+1)]
		rec := list + "/" + open.uid
		for _, one := range []struct {
			what string
			rail view.Rail
		}{
			{"reading", decode[view.NotePage](t, f.h.do(get(rec, f.sess))).Rail},
			{"editing", decode[view.NotePage](t, f.h.do(get(rec+"/edit", f.sess))).Rail},
		} {
			got, on := neighbourLabels(one.rail)
			if !equalStrings(got, want) || on != open.title {
				t.Errorf("%s %s: neighbours = %q (on %q), want %q", one.what, open.title, got, on, want)
			}
			for _, a := range one.rail.Neighbours {
				if a.Href == "" || a.Icon != "i-note" {
					t.Errorf("%s: neighbour %+v opens nothing or has no icon", one.what, a)
				}
			}
		}
	}
}

type notePageFragment struct {
	Target string        `json:"target"`
	Body   view.NotePage `json:"body"`
}

// from-typecase note 9, s-notes-20: the edit of a note is its page in edit —
// the shell with «Back to list» and the neighbours, the text column a form in
// the frame's order (Notebook, Date, Time, Tags, Related to), and the column
// beside it without Tags and Related, which are fields now. A fragment of
// the address answers the same page (C-7.1).
func TestNoteEditIsThePage(t *testing.T) {
	f := newNoteShelf(t)
	list := f.path("notes")
	rec := list + "/n3"
	_ = f.h.do(get(list, f.sess))
	frame := mockupFrame(t, "s-notes-20")

	for _, req := range []struct {
		what string
		page view.NotePage
	}{
		{"page", decode[view.NotePage](t, f.h.do(get(rec+"/edit", f.sess)))},
		{"fragment", decode[notePageFragment](t, f.h.do(fragment(get(rec+"/edit", f.sess), "main"))).Body},
	} {
		p := req.page
		if p.Doc.Mode != view.NoteEditing || p.UID != "n3" || p.Doc.Title != "Third" {
			t.Fatalf("%s: doc = %+v", req.what, p.Doc)
		}
		nav := frameNavItem.FindStringSubmatch(frame)
		if nav == nil || len(p.Rail.Nav) != 1 || string(p.Rail.Nav[0].Label) != frameText(frameNavShort.ReplaceAllString(nav[1], "")) {
			t.Errorf("%s: nav = %+v, want the frame's %q", req.what, p.Rail.Nav, frameText(frameNavShort.ReplaceAllString(nav[1], "")))
		}
		if len(p.Rail.Sections) == 0 {
			t.Errorf("%s: the rail has no notebooks", req.what)
		}
		var want, got []string
		for _, m := range frameFieldLabel.FindAllStringSubmatch(frame, -1) {
			want = append(want, m[1])
		}
		// The link after Tags, «optional» while empty (Р8 of
		// scans/kinds-parity.md; note 71, task 172). No status: content of
		// the kind (note 69, task 176).
		for _, r := range p.Doc.Fields {
			if r.Name == "status" {
				t.Errorf("%s: a Status among %+v", req.what, p.Doc.Fields)
			}
			if r.Name == "url" && r.Value == "" && r.Placeholder != "optional" {
				t.Errorf("%s: the empty link's sample = %q, want «optional»", req.what, r.Placeholder)
			}
			got = append(got, string(r.Label))
		}
		if !equalStrings(got, want) {
			t.Errorf("%s: fields = %q, want the frame's %q", req.what, got, want)
		}
		if len(p.Meta.Tags) != 0 || len(p.Meta.Related) != 0 {
			t.Errorf("%s: Tags and Related stay in the column while they are fields: %+v", req.what, p.Meta)
		}
		if actionByID(p.Meta.Actions, "attach.add") == nil || actionByID(p.Meta.Actions, "record.delete") == nil {
			t.Errorf("%s: the column keeps Attach and Delete note: %+v", req.what, p.Meta.Actions)
		}
		if p.Source.Stamp == "" {
			t.Errorf("%s: no source", req.what)
		}
		// The shelf's notes carry three properties Carrel does not know, as
		// the frame's does: the line under the text is the frame's, ready.
		m := frameDocNote.FindStringSubmatch(frame)
		if m == nil {
			t.Fatalf("%s: the frame draws no line under the text: read the frame again", req.what)
		}
		if runsText(p.Doc.Foreign) != frameText(m[1]) {
			t.Errorf("%s: the line of properties Carrel does not know = %q, the frame draws %v", req.what, p.Doc.Foreign, m)
		}
		// The property's name is monospaced, as the frame sets it.
		if name := regexp.MustCompile(`<span class="tc-mono">([^<]*)</span>`).FindStringSubmatch(m[1]); name == nil ||
			len(p.Doc.Foreign) != 3 || p.Doc.Foreign[1] != (view.Run{Text: view.Text(name[1]), Mark: view.RunMono}) {
			t.Errorf("%s: the line's pieces = %+v, the frame sets %v monospaced", req.what, p.Doc.Foreign, name)
		}
	}
}

// s-notes-15: the Notebook field offers the notebooks that can be written,
// the note's own chosen; a read-only one is shown and cannot be chosen, with
// the reason. Saving with another notebook saves the edit and moves the note
// there (S-18); a notebook the note cannot go to keeps it where it is, the
// edit saved, and says so.
func TestNoteEditNotebookMoves(t *testing.T) {
	f := newNoteShelf(t)
	list := f.path("notes")
	_ = f.h.do(get(list, f.sess))

	edit := func(uid string) (view.NotePage, view.FormRow) {
		t.Helper()
		p := decode[view.NotePage](t, f.h.do(get(list+"/"+uid+"/edit", f.sess)))
		for _, r := range p.Doc.Fields {
			if r.Label == "Notebook" {
				return p, r
			}
		}
		t.Fatalf("no Notebook in %+v", p.Doc.Fields)
		return p, view.FormRow{}
	}
	option := func(row view.FormRow, label string) view.Option {
		t.Helper()
		for _, o := range row.Options {
			if strings.HasSuffix(string(o.Label), " · "+label) {
				return o
			}
		}
		t.Fatalf("no notebook %q in %+v", label, row.Options)
		return view.Option{}
	}
	save := func(p view.NotePage, set url.Values) (int, string, []byte) {
		t.Helper()
		form := url.Values{"summary": {string(p.Doc.Title)}, "description": {string(p.Doc.Body)}}
		for _, r := range p.Doc.Fields {
			form.Set(r.Name, string(r.Value))
		}
		for k, v := range set {
			form[k] = v
		}
		u, _ := url.Parse(actionHref(t, p.Doc.Bar, "record.save"))
		for k, v := range u.Query() {
			form[k] = v
		}
		rec := f.h.do(postForm(u.EscapedPath(), f.sess, form))
		return rec.Code, rec.Header().Get("Location"), rec.Body.Bytes()
	}

	p, row := edit("n2")
	if row.Kind != view.InputSelect {
		t.Fatalf("Notebook is %q, not a choice", row.Kind)
	}
	own, diary, shared := option(row, "Notes"), option(row, "Diary"), option(row, "Shared")
	if !own.Selected || own.Disabled || diary.Selected || diary.Disabled || own.Colour == "" {
		t.Errorf("options = %+v", row.Options)
	}
	if !shared.Disabled || shared.Reason == "" || shared.Selected {
		t.Errorf("a read-only notebook is shown closed with its reason: %+v", shared)
	}
	if row.Value != text(own.Value) {
		t.Errorf("value %q is not the own notebook %q", row.Value, own.Value)
	}

	code, where, _ := save(p, url.Values{"summary": {"Second, moved"}, row.Name: {diary.Value}})
	if code != 303 || !strings.HasPrefix(where, f.notebook(f.diary)+"/n2") {
		t.Fatalf("save into Diary: %d %s", code, where)
	}
	if _, _, ok := f.srv.Object(f.col + "n2.ics"); ok {
		t.Error("the note stayed in Notes")
	}
	moved, _, ok := f.srv.Object(f.diary + "n2.ics")
	if !ok {
		t.Fatal("the note is not in Diary")
	}
	mustContain(t, unfolded(moved), "SUMMARY:Second\\, moved", "X-JTX-COLOR:-16711936")

	p, row = edit("n1")
	code, _, answer := save(p, url.Values{"summary": {"First, kept"}, row.Name: {option(row, "Shared").Value}})
	if code != 200 {
		t.Fatalf("save into a read-only notebook: %d", code)
	}
	mustContain(t, body(t, f.davFixture, "n1.ics"), "SUMMARY:First\\, kept")
	var page view.NotePage
	if err := json.Unmarshal(answer, &page); err != nil {
		t.Fatalf("decode: %v\n%s", err, answer)
	}
	if !hasNotice(page.Notices, "Saved, but not moved") || page.Doc.Title != "First, kept" {
		t.Errorf("the answer does not say the edit was saved and the note stayed: %+v", page.Notices)
	}
}

// shelfSave posts a note's edit with the fields it shows, set over them.
func shelfSave(t *testing.T, f noteShelf, p view.NotePage, set url.Values) *httptest.ResponseRecorder {
	t.Helper()
	form := url.Values{"summary": {string(p.Doc.Title)}, "description": {string(p.Doc.Body)}}
	for _, r := range p.Doc.Fields {
		form.Set(r.Name, string(r.Value))
	}
	for k, v := range set {
		form[k] = v
	}
	u, _ := url.Parse(actionHref(t, p.Doc.Bar, "record.save"))
	for k, v := range u.Query() {
		form[k] = v
	}
	return f.h.do(postForm(u.EscapedPath(), f.sess, form))
}

// S-8 and S-18: a save into another notebook that the server refuses by
// version keeps the Notebook with the rest of the edit. The conflict screen
// says the note has not moved yet; writing the person's edit there moves it.
func TestNoteMoveSurvivesAConflict(t *testing.T) {
	f := newNoteShelf(t)
	list := f.path("notes")
	_ = f.h.do(get(list, f.sess))
	p := decode[view.NotePage](t, f.h.do(get(list+"/n2/edit", f.sess)))
	diary := f.notebook(f.diary)

	// Another client writes the note meanwhile.
	f.srv.Seed(f.col+"n2.ics", ics("BEGIN:VJOURNAL\nUID:n2\nDTSTAMP:20260101T000000Z\nDTSTART:20260102T090000Z\n"+
		"SUMMARY:Second\nDESCRIPTION:Changed elsewhere\nEND:VJOURNAL"))
	rec := shelfSave(t, f, p, url.Values{"summary": {"Second, mine"}, "notebook": {diary}})
	if rec.Code != 303 || rec.Header().Get("Location") != list+"/n2/conflict" {
		t.Fatalf("save: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	page := decode[view.ConflictPage](t, f.h.do(get(list+"/n2/conflict", f.sess)))
	if !hasNoticeBody(page.Notices, "Diary") {
		t.Errorf("the conflict screen does not say the note is still to move: %+v", page.Notices)
	}
	if resp := f.h.do(postForm(actionHref(t, page.Actions, "conflict.mine"), f.sess, url.Values{})); resp.Code != 303 ||
		!strings.HasPrefix(resp.Header().Get("Location"), diary+"/n2") {
		t.Fatalf("use my edit: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	moved, _, ok := f.srv.Object(f.diary + "n2.ics")
	if !ok {
		t.Fatal("the note did not move after the conflict")
	}
	mustContain(t, unfolded(moved), "SUMMARY:Second\\, mine")
}

// S-7.1, S-16: the loss report of a note's edit is its edit page again, and
// its column is the edit's: since Typecase's Р-58 (from-typecase note 57,
// s-notes-20) neither names the same note in other notebooks — the edit
// writes this copy alone. (Until 14.10.2026 both kept that block.)
func TestNoteLossReportIsTheEdit(t *testing.T) {
	f := newNoteShelf(t)
	f.srv.Seed(f.diary+"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:First\nEND:VJOURNAL"))
	key, _ := f.sess.DataKey()
	if err := f.h.users.EditDuplicates(f.sess.AccountID(), key, "duplicates.link", "", func(d *sealed.Duplicates) error {
		d.Linked = []sealed.LinkedGroup{{ID: "g-1", Kind: view.KindNote, Members: []sealed.RecordRef{
			{AccountID: f.id, Col: f.col, UID: "n1"}, {AccountID: f.id, Col: f.diary, UID: "n1"}}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	list := f.path("notes")
	p := decode[view.NotePage](t, f.h.do(get(list+"/n1/edit", f.sess)))
	if len(p.Meta.Linked) != 0 {
		t.Errorf("the edit names the other copies: %+v", p.Meta.Linked)
	}
	f.srv.Drop = []string{"X-OTHER"}
	lost := decode[view.NotePage](t, shelfSave(t, f, p, url.Values{"summary": {"First, renamed"}}))
	if lost.Loss == nil {
		t.Fatalf("no loss report: %+v", lost.Notices)
	}
	if len(lost.Meta.Linked) != 0 || len(lost.Meta.Attach) != len(p.Meta.Attach) {
		t.Errorf("the loss report's column = %+v, the edit's %+v", lost.Meta, p.Meta)
	}
}

// s-notes-2: the reading page says when the note was written, the time
// with the day, as the frame does.
func TestNoteReadingSaysTheTime(t *testing.T) {
	if !regexp.MustCompile(`tc-head__sub[^>]*>.*<span class="tc-mono">[^<]*\d\d:\d\d</span>`).MatchString(mockupFrame(t, "s-notes-2")) {
		t.Skip("s-notes-2 draws no time: read the frame again")
	}
	f := newNoteShelf(t)
	p := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n3", f.sess)))
	if !regexp.MustCompile(`\d\d:\d\d`).MatchString(string(p.Meta.Date)) {
		t.Errorf("meta date = %q, want the day and the time", p.Meta.Date)
	}
}
