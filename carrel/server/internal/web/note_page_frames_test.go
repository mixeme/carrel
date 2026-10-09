package web_test

import (
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The page of a note as s-notes-2 and s-notes-20 draw it (the client's scan
// of the notes, groups Г and Д): Edit with its pencil, Export Markdown
// without an icon and Print over the text; Source with the note's UID; the
// related event and task, read before; the file inside it and the link to
// the recording, with where that lies; the same note in another notebook.
// The edit keeps the column — the attachments with their place, the copy —
// and says what Carrel does not know in one line; a new note is the same
// page with nothing to delete.
func TestNotePageIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-notes-2")
	for _, want := range []string{`<span class="tc-badge">Documents / carrel</span>`, `<span class="tc-badge">in the note</span>`, `<div class="tc-rubric">Source</div>`} {
		if !strings.Contains(frame, want) {
			t.Fatalf("s-notes-2 draws no %s: read the frame again", want)
		}
	}
	app := newMockupApp(t)
	linkMockupMeeting(t, app)
	attachMockupRecording(t, app)
	openList(t, app, "/app/calendar?from=2026-08-10")
	openList(t, app, "/app/tasks")
	openList(t, app, "/app/notes")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "notes", "zapisi")
	rec := list + "/5c81-77aa-0d64"

	page := decode[view.NotePage](t, app.h.do(get(rec, app.sess)))
	edit := actionByID(page.Doc.Bar, "record.edit")
	export := actionByID(page.Doc.Bar, "note.export")
	if edit == nil || edit.Icon != "i-pencil" || export == nil || export.Icon != "" || actionByID(page.Doc.Bar, "record.print") == nil {
		t.Errorf("bar = %+v, want Edit with its pencil, Export Markdown bare, Print (s-notes-2)", page.Doc.Bar)
	}
	if page.Source.UID != "5c81-77aa-0d64" {
		t.Errorf("source = %+v, want the note's UID (s-notes-2)", page.Source)
	}
	var related []string
	for _, r := range page.Meta.Related {
		related = append(related, string(r.Title)+" · "+string(r.Kind))
	}
	if !slices.Equal(related, []string{"Синк-ап по релизу · event", "Дописать раздел про кэш · task"}) {
		t.Errorf("related = %q", related)
	}
	checkAttach := func(rows []view.AttachRow) {
		t.Helper()
		if len(rows) != 2 || !rows[0].Inline || rows[0].Size != "412 KB" || rows[0].Place != "" ||
			rows[1].Inline || rows[1].Size != "24 MB" || rows[1].Place != "Documents / carrel" || rows[1].Href == "" {
			t.Errorf("attach = %+v", rows)
		}
	}
	checkAttach(page.Meta.Attach)
	if len(page.Meta.Linked) != 1 || page.Meta.Linked[0].Value != "Text differ" {
		t.Errorf("linked = %+v", page.Meta.Linked)
	}

	ed := decode[view.NotePage](t, app.h.do(get(rec+"/edit", app.sess)))
	checkAttach(ed.Meta.Attach)
	if len(ed.Meta.Linked) != 1 || !strings.Contains(runsText(ed.Doc.Foreign), "X-JTX-COLOR and two more") || ed.Source.UID != "5c81-77aa-0d64" {
		t.Errorf("edit = linked %+v, foreign %q, source %+v", ed.Meta.Linked, ed.Doc.Foreign, ed.Source)
	}

	fresh := decode[view.NotePage](t, app.h.do(get(list+"/new", app.sess)))
	if fresh.Doc.Form == nil || fresh.Doc.Mode != view.NoteEditing || slices.ContainsFunc(fresh.Meta.Actions, func(a view.Action) bool { return a.ID == "record.delete" }) {
		t.Errorf("new note = %+v", fresh.Doc)
	}
}
