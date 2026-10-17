package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// C-14.3: the states of the conflict screen, each as its frame (Typecase's
// note 77): whose edit is not known (s-conflict-6), deleted on the server
// (-7), a combination the event cannot hold (-8), the write refused (-9),
// the source unavailable (-10), a note still to move (-11).
func TestFixtureConflictStates(t *testing.T) {
	const anna = "4f2b-9c31-a0e7"
	annaAt := davtest.AddressBookHome + "kontakty/" + anna + ".vcf"
	// annaOpened is Анна Ковалёва as the card was opened (s-conflict-1, Рв6):
	// without her work phone, her note and her file-as line, under another
	// display name and address.
	annaOpened := func(t *testing.T) string {
		base := strings.Replace(setRecord(t, "nextcloud-home/kontakty.vcf", anna), "FN:Анна Ковалёва\n", "FN:Ковалёва Анна\n", 1)
		base = strings.Replace(base, "TEL;TYPE=WORK:+7 495 120-45-00\n", "", 1)
		base = strings.Replace(base, "EMAIL;TYPE=WORK:a.kovaleva@example.org\n", "EMAIL;TYPE=WORK:anna@slovo.example\n", 1)
		base = regexp.MustCompile(`(?m)^NOTE:.*\n( .*\n)*`).ReplaceAllString(base, "")
		return strings.Replace(base, "X-EVOLUTION-FILE-AS:Ковалёва\\, Анна\n", "", 1)
	}
	// annaMoved is what the other client made of her meanwhile.
	annaMoved := func(base string) string {
		server := strings.Replace(base, "FN:Ковалёва Анна\n", "FN:Анна Ковалёва\n", 1)
		server = strings.Replace(server, "EMAIL;TYPE=WORK:anna@slovo.example\n", "EMAIL;TYPE=WORK:anna.kovaleva@slovo.example\n", 1)
		server = strings.Replace(server, "X-ABLABEL:", "X-EVOLUTION-FILE-AS:Ковалёва\\, Анна\nX-ABLABEL:", 1)
		return strings.Replace(server, "REV:20260814T090412Z\n", "REV:20260810T140200Z\n", 1)
	}
	// annaEdit opens her card's form on the version opened, lets meanwhile
	// happen, and saves the edit of s-conflict-1 (note, if withNote) over
	// what is there then. It returns the card's address.
	annaEdit := func(t *testing.T, app *mockupApp, meanwhile func(form url.Values, base string), withNote bool) string {
		t.Helper()
		combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		rec := app.listPath(t, "contacts", "kontakty") + "/" + anna
		base := annaOpened(t)
		app.servers["nextcloud"].Seed(annaAt, base)
		form := formFields(decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
		meanwhile(form, base)
		form.Set("fn", "Анна Ковалёва")
		form.Set("tel.1", "+7 495 120-45-00")
		form.Set("tel.1.type", "work")
		form.Set("email.0", "a.kovaleva@example.org")
		if withNote {
			form.Set("note", "Пишет по вторникам, отвечает в тот же день.")
		}
		if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 || resp.Header().Get("Location") != rec+"/conflict" {
			t.Fatalf("save: %d %s, want the conflict", resp.Code, resp.Header().Get("Location"))
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		return rec
	}
	moved := func(app *mockupApp) func(url.Values, string) {
		return func(_ url.Values, base string) { app.servers["nextcloud"].Seed(annaAt, annaMoved(base)) }
	}

	t.Run("unknown", func(t *testing.T) {
		// The version the form was opened with is no longer held (Б43):
		// every field that differs from the server's is a card, nothing
		// chosen (s-conflict-6).
		app := newMockupApp(t)
		rec := annaEdit(t, app, func(form url.Values, base string) {
			app.servers["nextcloud"].Seed(annaAt, annaMoved(base))
			form.Set("etag", "a-version-nobody-holds")
		}, true)
		app.compare(t, "app-conflict/unknown.json", app.h.do(get(rec+"/conflict", app.sess)))
	})
	t.Run("deleted", func(t *testing.T) {
		// Another client deleted her while she was edited (Р-67,
		// s-conflict-7).
		app := newMockupApp(t)
		rec := annaEdit(t, app, func(url.Values, string) { app.servers["nextcloud"].Unseed(annaAt) }, false)
		app.compare(t, "app-conflict/deleted.json", app.h.do(get(rec+"/conflict", app.sess)))
	})
	t.Run("write-refused", func(t *testing.T) {
		// The combination as chosen beforehand, and the server out of room
		// (s-conflict-9).
		app := newMockupApp(t)
		rec := annaEdit(t, app, moved(app), true)
		page := decode[view.ConflictPage](t, app.h.do(get(rec+"/conflict", app.sess)))
		app.servers["nextcloud"].Refuse("PUT", 507, "insufficient storage")
		defer app.servers["nextcloud"].Accept("PUT")
		app.compare(t, "app-conflict/write-refused.json", app.h.do(postForm(page.Form.Action, app.sess, formFields(*page.Form))))
	})
	t.Run("unavailable", func(t *testing.T) {
		// The book stopped answering before the screen was opened
		// (s-conflict-10).
		app := newMockupApp(t)
		rec := annaEdit(t, app, moved(app), true)
		refuseReads(app.servers["nextcloud"])
		defer acceptReads(app.servers["nextcloud"])
		app.compare(t, "app-conflict/unavailable.json", app.h.do(get(rec+"/conflict", app.sess)))
	})
	t.Run("refused", func(t *testing.T) {
		// «Забрать документы»: both sides moved its start and its end and
		// named it alike; the start of the edit with the end of the server
		// ends before it starts (s-conflict-8).
		app := newMockupApp(t)
		openList(t, app, "/app/calendar?from=2026-08-10")
		const uid = "1d55-90fe-3c27"
		at := davtest.CalendarHome + "lichnyj/" + uid + ".ics"
		base := setRecord(t, "nextcloud-home/lichnyj.ics", uid)
		app.servers["nextcloud"].Seed(at, base)
		rec := app.listPath(t, "calendar", "lichnyj") + "/" + uid
		form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
		server := strings.Replace(base, "SUMMARY:Забрать документы\n", "SUMMARY:Забрать документы в МФЦ\n", 1)
		server = strings.Replace(server, ":20260811T180000\n", ":20260811T171500\n", 1)
		server = strings.Replace(server, ":20260811T190000\n", ":20260811T173000\n", 1)
		app.servers["nextcloud"].Seed(at, server)
		form.Set("summary", "Забрать документы в МФЦ")
		form.Set("start.time", "18:30")
		form.Set("end.time", "19:30")
		if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 {
			t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		page := decode[view.ConflictPage](t, app.h.do(get(rec+"/conflict", app.sess)))
		chosen := formFields(*page.Form)
		for _, r := range page.Rows {
			side := view.ChoiceMine
			if r.Property == "DTEND" {
				side = view.ChoiceTheirs
			}
			chosen.Set(r.Name, string(side))
		}
		app.compare(t, "app-conflict-event/refused.json", app.h.do(postForm(page.Form.Action, app.sess, chosen)))
	})
	t.Run("moving", func(t *testing.T) {
		// The conflict of s-conflict-5, the edit also moving the note to
		// «Дневник» (s-conflict-11).
		app := newMockupApp(t)
		openList(t, app, "/app/notes")
		rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
		doc := decode[view.NotePage](t, app.h.do(get(rec+"/edit", app.sess))).Doc
		save, err := url.Parse(actionHref(t, doc.Bar, "record.save"))
		if err != nil {
			t.Fatal(err)
		}
		form := url.Values{
			"summary":     {"Заметка со встречи 10.08 — дополнено"},
			"description": {"Договорились: демо в пятницу, релиз в понедельник."},
			"categories":  {"встречи, релиз, итоги"},
		}
		for _, row := range doc.Fields {
			switch {
			case row.Name == "" || row.Name == "summary" || row.Name == "description" || row.Name == "categories":
			case row.Name == "notebook":
				for _, o := range row.Options {
					if o.Label == "Дневник" || strings.HasSuffix(string(o.Label), "Дневник") {
						form.Set(row.Name, o.Value)
					}
				}
			case row.Multiple:
				for _, o := range row.Options {
					if o.Selected {
						form.Add(row.Name, o.Value)
					}
				}
			default:
				form.Set(row.Name, string(row.Value))
			}
		}
		if form.Get("notebook") == "" {
			t.Fatal("no notebook «Дневник» to move to")
		}
		for k, v := range save.Query() {
			form[k] = v
		}
		server := setRecord(t, "nextcloud-home/zapisi.ics", "5c81-77aa-0d64")
		server = strings.Replace(server, "SUMMARY:Заметка со встречи 10.08\n", "SUMMARY:Встреча 10.08\n", 1)
		server = strings.Replace(server, "CATEGORIES:встречи,релиз\n", "CATEGORIES:встречи,релиз,итоги\n", 1)
		server = regexp.MustCompile(`(?m)^DESCRIPTION:.*\n( .*\n)*`).ReplaceAllString(server, "DESCRIPTION:Договорились: демо в пятницу.\n")
		app.servers["nextcloud"].Seed(davtest.CalendarHome+"zapisi/5c81-77aa-0d64.ics", server)
		if resp := app.h.do(postForm(save.EscapedPath(), app.sess, form)); resp.Code != 303 {
			t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-conflict-note/moving.json", app.h.do(get(rec+"/conflict", app.sess)))
	})
}
