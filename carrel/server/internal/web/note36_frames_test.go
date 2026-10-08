package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The frames of Typecase 80d1319 (from-typecase notes 35 and 36; the owner's
// decisions Р-38 … Р-40 of 07.10.2026) against what the server answers.

// frameHeadButtons are the words of the buttons of a frame's head, in order,
// and whether each has its arrow.
func frameHeadButtons(t *testing.T, id string) (labels []string, arrow map[string]bool) {
	t.Helper()
	frame := mockupFrame(t, id)
	at := strings.Index(frame, `<div class="tc-main`)
	if at < 0 {
		t.Fatalf("%s has no main column: read the frame again", id)
	}
	acts := regexp.MustCompile(`(?s)<div class="tc-acts">(.*?)</div>`).FindStringSubmatch(frame[at:])
	if acts == nil {
		t.Fatalf("%s has no buttons in its head: read the frame again", id)
	}
	arrow = map[string]bool{}
	for _, m := range regexp.MustCompile(`(?s)<(?:button|a) class="[^"]*tc-btn[^"]*"[^>]*>(.*?)</(?:button|a)>`).FindAllStringSubmatch(acts[1], -1) {
		label := frameText(regexp.MustCompile(`(?s)<svg.*?</svg>`).ReplaceAllString(m[1], ""))
		labels = append(labels, label)
		arrow[label] = strings.Contains(m[1], `#i-chev`)
	}
	return labels, arrow
}

// With nothing ticked the New… button does not depend on the ticks (Р-39):
// its arrow stays — a record can be written into a collection that is not
// ticked — in every section (states-7, -14, -18, -25).
func TestNothingTickedKeepsTheArrowOfNew(t *testing.T) {
	for _, one := range []struct{ frame, segment, label string }{
		{"states-7", "contacts", "New contact"},
		{"states-14", "calendar", "New event"},
		{"states-18", "tasks", "New task"},
		{"states-25", "notes", "New note"},
	} {
		t.Run(one.segment, func(t *testing.T) {
			if _, arrow := frameHeadButtons(t, one.frame); !arrow[one.label] {
				t.Fatalf("%s draws %q without its arrow: read the frame again", one.frame, one.label)
			}
			app := newMockupApp(t)
			if rec := app.h.do(postForm("/app/sources/"+one.segment, app.sess, url.Values{})); rec.Code != 303 {
				t.Fatalf("untick everything: %d", rec.Code)
			}
			page := decode[struct {
				Bar view.ListBar `json:"bar"`
			}](t, app.h.do(get("/app/"+one.segment, app.sess)))
			a := actionByID(page.Bar.Actions, "record.new")
			if a == nil || !a.Enabled || a.Href == "" {
				t.Fatalf("New = %+v, want it enabled", a)
			}
			writable := 0
			for _, it := range a.Arrow {
				if it.Enabled {
					writable++
				}
			}
			if writable == 0 {
				t.Errorf("the arrow of %q with nothing ticked = %+v, want the collections to write into", one.label, a.Arrow)
			}
		})
	}
}

// A list that can only be read has Export and Print in its head, a notebook
// Export — nothing that writes (Р-38: states-4, -23, -30).
func TestReadOnlyListOffersNothingThatWrites(t *testing.T) {
	for _, id := range []string{"states-4", "states-23", "states-30"} {
		labels, _ := frameHeadButtons(t, id)
		for _, l := range labels {
			if strings.HasPrefix(l, "New ") || l == "Import" {
				t.Fatalf("%s draws %q in a read-only list: read the frame again", id, l)
			}
		}
	}
	app := newMockupApp(t)
	for _, one := range []struct{ segment, slug string }{{"contacts", "komanda"}, {"notes", "zapiski-obshchie"}} {
		page := decode[struct {
			Bar view.ListBar `json:"bar"`
		}](t, app.h.do(get(app.listPath(t, one.segment, one.slug), app.sess)))
		var got []string
		for _, a := range page.Bar.Actions {
			got = append(got, a.ID)
			if a.ID == "record.new" || a.ID == "collection.import" {
				t.Errorf("%s: a read-only list offers %s", one.slug, a.ID)
			}
		}
		if actionByID(page.Bar.Actions, "collection.export") == nil {
			t.Errorf("%s: no Export among %q", one.slug, got)
		}
	}
}

// While a poll runs its bar says how long, at its right in mono, in every
// section (Р-39: states-8 «0.8 s», -19, -26; the calendar's — states-13).
func TestRunningPollSaysHowLongInEverySection(t *testing.T) {
	for _, id := range []string{"states-8", "states-19", "states-26"} {
		if m := regexp.MustCompile(`<span class="tc-mono tc-push">\d+\.\d s</span>`).FindString(mockupFrame(t, id)); m == "" {
			t.Fatalf("%s draws no time at the bar's right: read the frame again", id)
		}
	}
	app := newMockupApp(t)
	release := app.servers["radicale · vps"].Stall("REPORT")
	defer release()
	page := decode[view.ContactsListPage](t, app.h.do(get("/app/contacts", app.sess)))
	if page.Poll == nil || !page.Poll.Running {
		t.Fatalf("poll = %+v, want it running", page.Poll)
	}
	if !regexp.MustCompile(`^\d+\.\d s$`).MatchString(string(page.Poll.Aside)) {
		t.Errorf("aside = %q, want a time like «0.8 s» (states-8)", page.Poll.Aside)
	}
	if page.Top.Refresh.Action.Enabled || page.Top.Refresh.Action.Reason == "" {
		t.Errorf("refresh while the poll runs = %+v, want it off with its reason (states-8)", page.Top.Refresh.Action)
	}
}

// One collection whose server did not answer has the sign with the reason in
// its row of the rail, not a dash (states-11, -22, -29).
func TestRefusedListHasTheSignInItsRailRow(t *testing.T) {
	for _, id := range []string{"states-11", "states-22", "states-29"} {
		found := false
		for _, r := range frameTicks(mockupFrame(t, id)) {
			if r.error {
				found = true
				if r.meta != "" || r.hint == "" {
					t.Fatalf("%s: the refused row %q shows %q, hint %q: read the frame again", id, r.label, r.meta, r.hint)
				}
			}
		}
		if !found {
			t.Fatalf("%s draws no refused row in its rail: read the frame again", id)
		}
	}
	for _, one := range []struct{ segment, slug, label string }{
		{"contacts", "kontakty", "Контакты"},
		{"tasks", "dela", "Дела"},
		{"notes", "zapisi", "Записи"},
	} {
		t.Run(one.segment, func(t *testing.T) {
			app := newMockupApp(t)
			app.servers["nextcloud"].Refuse("REPORT", 503, "the server is busy")
			defer app.servers["nextcloud"].Accept("REPORT")
			page := decode[struct {
				Rail view.Rail `json:"rail"`
			}](t, app.h.do(get(app.listPath(t, one.segment, one.slug), app.sess)))
			_, row := railRow(t, page.Rail, one.label)
			if row.State != view.CollectionUnavailable || row.Detail == "" || !row.On {
				t.Errorf("the row of the list that did not answer = %+v, want it unavailable with the reason", row)
			}
		})
	}
}

// A record linked with its copy in another collection says so in the list of
// one collection too: «linked ×2» by its title, both sources its hint
// (s-tasks-15: «Собрать чемодан», s-notes-31: «Что почитать про CRDT») — the
// link is the person's decision (S-16), not a property of the combined view.
func TestOneListSaysARecordIsLinked(t *testing.T) {
	for _, id := range []string{"s-tasks-15", "s-notes-31"} {
		if !strings.Contains(mockupFrame(t, id), `>linked ×2</span>`) {
			t.Fatalf("%s draws no linked row: read the frame again", id)
		}
	}
	t.Run("tasks", func(t *testing.T) {
		app := newMockupApp(t)
		linkMockupTask(t, app)
		page := decode[view.TasksPage](t, app.h.do(get(app.listPath(t, "tasks", "dela"), app.sess)))
		for _, g := range page.Groups {
			for _, r := range g.Rows {
				if r.UID != "aa11-2d09-5f84" {
					if len(r.Badges) != 0 {
						t.Errorf("%s is not linked and has %+v", r.Summary, r.Badges)
					}
					continue
				}
				if len(r.Badges) != 1 || r.Badges[0].Label != "linked ×2" || len(r.Sources) != 2 || r.State != view.RecordLinked {
					t.Errorf("the linked task in its list = badges %+v, sources %q, state %q", r.Badges, r.Sources, r.State)
				}
				return
			}
		}
		t.Error("the list lacks the linked task")
	})
	t.Run("notes", func(t *testing.T) {
		app := newMockupApp(t)
		linkMockupNote(t, app)
		page := decode[view.NotesPage](t, app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess)))
		for _, g := range page.Groups {
			for _, r := range g.Rows {
				if r.UID == "6d92-13ef-b408" {
					if len(r.Badges) != 1 || r.Badges[0].Label != "linked ×2" || len(r.Sources) != 2 {
						t.Errorf("the linked note in its notebook = badges %+v, sources %q", r.Badges, r.Sources)
					}
					return
				}
			}
		}
		t.Error("the notebook lacks the linked note")
	})
}

// The tags of a rail are the section's, not the open list's (the owner's
// decision of 08.10.2026): the rail of one collection carries the set the
// combined view of the section has — the categories of the ticked
// collections the session has read —, each leading to the open list
// filtered by it; and it keeps them when the list itself did not answer
// (s-tasks-15 draws «релиз», a tag of «Работа», in the rail of «Дела»;
// states-22 draws the tags over a list that is unavailable).
func TestRailTagsAreTheSections(t *testing.T) {
	for _, id := range []string{"s-tasks-15", "states-22"} {
		if !strings.Contains(mockupFrame(t, id), `>релиз</`) {
			t.Fatalf("%s draws no «релиз» among the rail's tags: read the frame again", id)
		}
	}
	labels := func(tags []view.Tag) string {
		var out []string
		for _, tg := range tags {
			out = append(out, string(tg.Label))
		}
		return strings.Join(out, ", ")
	}
	app := newMockupApp(t)
	all := combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	want := labels(all.Rail.Tags)
	if !strings.Contains(want, "релиз") || !strings.Contains(want, "дом") {
		t.Fatalf("the combined list's tags = %q: the set changed", want)
	}
	list := app.listPath(t, "tasks", "dela")
	one := decode[view.TasksPage](t, app.h.do(get(list, app.sess)))
	if got := labels(one.Rail.Tags); got != want {
		t.Errorf("the rail of one list has the tags %q, the section's are %q", got, want)
	}
	for _, tg := range one.Rail.Tags {
		if !strings.HasPrefix(tg.Href, list) {
			t.Errorf("the tag %q leads to %q, not to the open list", tg.Label, tg.Href)
		}
	}
	// The session holds what it read (S-4.2) and asks only whether it
	// changed: the stand refuses that too.
	for _, m := range []string{"PROPFIND", "REPORT"} {
		app.servers["nextcloud"].Refuse(m, 503, "the server is busy")
		defer app.servers["nextcloud"].Accept(m)
	}
	refused := decode[view.TasksPage](t, app.h.do(get(list, app.sess)))
	if len(refused.Notices) == 0 {
		t.Fatal("the list answered: the stand did not refuse")
	}
	if got := labels(refused.Rail.Tags); got != want {
		t.Errorf("the rail of a list that did not answer has the tags %q, the section's are %q", got, want)
	}
}
