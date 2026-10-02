package web_test

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// from-typecase note 32, task 93 (the owner's decisions of 02.10.2026): an
// attachment's name is a link in every card — one in the record downloads,
// one in Files opens there (the folder with the file chosen); the card has
// no other action on it. Remove stands on the edit only (s-agenda-3).

var frameAttachLink = regexp.MustCompile(`<a class="tc-dim--accent" href="[^"]*">([^<]+)</a> <span class="tc-dim">[^<]*</span> <span class="tc-badge">`)

func TestAnAttachmentIsALinkAndRemovedOnTheEdit(t *testing.T) {
	names := func(id string) []string {
		var out []string
		for _, m := range frameAttachLink.FindAllStringSubmatch(mockupFrame(t, id), -1) {
			out = append(out, frameText(m[1]))
		}
		return out
	}
	removes := func(id string) int {
		frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
		return strings.Count(frame, `is-danger" type="button">Remove</button>`)
	}
	card, note := names("s-agenda-2"), names("s-notes-2")
	if len(card) != 1 || len(note) != 2 || removes("s-agenda-2") != 0 || removes("s-notes-2") != 0 || removes("s-agenda-3") != 1 {
		t.Fatalf("s-agenda-2 %q, s-notes-2 %q, Remove %d, %d, %d: read the frames again", card, note,
			removes("s-agenda-2"), removes("s-notes-2"), removes("s-agenda-3"))
	}

	app := newMockupApp(t)
	attachMockupRecording(t, app)
	openList(t, app, "/app/calendar?from=2026-08-10")
	openList(t, app, "/app/notes")
	event := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"
	rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"

	cardRows := func(what string, rows []view.AttachRow, want []string) {
		t.Helper()
		if len(rows) != len(want) {
			t.Fatalf("%s: attachments %+v, the frame %q", what, rows, want)
		}
		for i, r := range rows {
			if string(r.Name) != want[i] || r.Href == "" || len(r.Actions) != 0 {
				t.Errorf("%s: %q = href %q, actions %+v; the frame draws %q a link and no action", what, r.Name, r.Href, r.Actions, want[i])
			}
			inFiles := strings.HasPrefix(r.Href, "/app/files/")
			if r.Inline == inFiles || !r.Inline && r.Place == "" {
				t.Errorf("%s: %q (inline %v, %q) goes to %q", what, r.Name, r.Inline, r.Place, r.Href)
			}
			if inFiles {
				page := app.h.do(get(r.Href, app.sess))
				if page.Code != http.StatusOK {
					t.Errorf("%s: %q opens %q: %d", what, r.Name, r.Href, page.Code)
				}
			}
		}
	}
	cardRows("the event", decode[view.EventPage](t, app.h.do(get(event, app.sess))).Attach, card)
	cardRows("the note", decode[view.NotePage](t, app.h.do(get(rec, app.sess))).Meta.Attach, note)

	edit := decode[view.EventEditPage](t, app.h.do(get(event+"/edit", app.sess))).Attach
	if len(edit) != 1 || len(edit[0].Actions) != 1 || edit[0].Actions[0].ID != "attach.remove" || edit[0].Actions[0].Label != "Remove" {
		t.Errorf("the edit of the event: %+v, want Remove (s-agenda-3)", edit)
	}
	noteEdit := decode[view.NotePage](t, app.h.do(get(rec+"/edit", app.sess))).Meta.Attach
	for _, r := range noteEdit {
		if len(r.Actions) != 1 || r.Actions[0].ID != "attach.remove" {
			t.Errorf("the edit of the note: %q has %+v", r.Name, r.Actions)
		}
	}
}
