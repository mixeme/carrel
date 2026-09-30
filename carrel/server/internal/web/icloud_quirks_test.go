package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// docs/servers.md, SV-54: iCloud answers a calendar-query with a text-match on
// UID 412 and an empty body, and no event or task opened through the pages:
// «<calendar> is unavailable … REPORT … 412». A record is looked for without
// the filter — at <UID>.ics, then in the component read whole — and the
// session remembers the refusal, so the next record of that calendar is not
// asked with the filter again.
func TestARecordOpensWhereTheServerRefusesAUIDFilter(t *testing.T) {
	f := calendarFixture(t, nil,
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nEND:VEVENT"),
		"their-own-name.ics", ics("BEGIN:VEVENT\nUID:other\nDTSTAMP:20260101T000000Z\nDTSTART:20260116T100000Z\nDTEND:20260116T110000Z\nSUMMARY:Elsewhere\nEND:VEVENT"),
		"todo.ics", ics("BEGIN:VTODO\nUID:todo\nDTSTAMP:20260101T000000Z\nSUMMARY:Pack\nEND:VTODO"))
	f.srv.RefuseUIDTextMatch = true

	if page := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/meet", f.sess))); string(page.Summary) != "Review" {
		t.Fatalf("event page = %q, notices %+v", page.Summary, page.Notices)
	}
	refused := f.srv.UIDMatchesRefused()
	if refused == 0 {
		t.Fatal("the stand refused nothing: the guard does not look at what it names")
	}
	if page := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/other", f.sess))); string(page.Summary) != "Elsewhere" {
		t.Errorf("event under a name of its own = %q, notices %+v", page.Summary, page.Notices)
	}
	if page := decode[view.TaskPage](t, f.h.do(get(f.path("tasks")+"/todo", f.sess))); string(page.Summary) != "Pack" {
		t.Errorf("task page = %q, notices %+v", page.Summary, page.Notices)
	}
	if n := f.srv.UIDMatchesRefused(); n != refused {
		t.Errorf("UID filters refused = %d after the first %d: the refusal is not remembered", n, refused)
	}

	rec := f.path("calendar") + "/meet"
	form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("summary", "Review again")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("edit: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "meet.ics"), "SUMMARY:Review again")
}

// docs/servers.md, SV-53: iCloud answers PUT of an event with DTSTART and
// neither DTEND nor DURATION 404 with an empty body. RFC 5545 §3.6.1 gives
// such an event a length — none for a time, one day for a date — so Carrel
// writes that end out whenever the person gave none: DTEND equal to DTSTART
// for a time, the next day for a date. On the full form, the quick sheet, and
// an edit that clears the end.
func TestAnEventWithoutAnEndIsWrittenWithOne(t *testing.T) {
	f := calendarFixture(t, nil,
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nEND:VEVENT"))
	f.srv.RefuseEventWithoutEnd = true
	made := func(what, route string, set map[string]string) string {
		t.Helper()
		before := f.srv.Paths(f.col)
		var form url.Values
		if strings.HasSuffix(route, "/quick") {
			form = formFields(decode[view.NewSheet](t, f.h.do(get(route, f.sess))).Form)
		} else {
			form = formFields(decode[view.EventEditPage](t, f.h.do(get(route, f.sess))).Form)
		}
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(route, f.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("%s: %d %s", what, resp.Code, resp.Body.String())
		}
		for _, p := range f.srv.Paths(f.col) {
			if !slices.Contains(before, p) {
				b, _, _ := f.srv.Object(p)
				return unfolded(b)
			}
		}
		t.Fatalf("%s made nothing", what)
		return ""
	}
	sameEnd := func(what, got string) {
		t.Helper()
		start, end := lineOf(got, "DTSTART"), lineOf(got, "DTEND")
		if start == "" || strings.TrimPrefix(end, "DTEND") != strings.TrimPrefix(start, "DTSTART") {
			t.Errorf("%s: %q and %q, want the end equal to the start\n%s", what, start, end, got)
		}
	}

	newForm := f.path("calendar") + "/new"
	sameEnd("timed, full form", made("timed, full form", newForm,
		map[string]string{"summary": "Call", "start.date": "2031-03-11", "start.time": "09:30"}))
	mustContain(t, made("all day, full form", newForm,
		map[string]string{"summary": "Holiday", "start.date": "2031-03-11", "start.allday": "on"}),
		"DTSTART;VALUE=DATE:20310311", "DTEND;VALUE=DATE:20310312")
	quick := f.path("calendar") + "/new/quick"
	sameEnd("timed, quick sheet", made("timed, quick sheet", quick,
		map[string]string{"summary": "Call", "start.date": "2031-03-12", "start.time": "09:30"}))
	mustContain(t, made("all day, quick sheet", quick,
		map[string]string{"summary": "Holiday", "start.date": "2031-03-12", "start.allday": "on"}),
		"DTSTART;VALUE=DATE:20310312", "DTEND;VALUE=DATE:20310313")

	// An end the person clears is the same end as none.
	rec := f.path("calendar") + "/meet"
	form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("end.date", "")
	form.Set("end.time", "")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("clear the end: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "meet.ics")
	sameEnd("end cleared", got)
	mustContain(t, got, "DTSTART:20260115T100000Z")
}

// lineOf is the first content line of a document that starts with the
// property's name followed by its parameters or its value.
func lineOf(doc, prop string) string {
	for _, l := range strings.Split(doc, "\r\n") {
		if strings.HasPrefix(l, prop+":") || strings.HasPrefix(l, prop+";") {
			return l
		}
	}
	return ""
}

// docs/servers.md, SV-52: iCloud answers MKCALENDAR with several components
// 201 and keeps the first, without a word. The catalog takes what the server
// kept (S-12.2); the person who asked for events and tasks is told, on the
// calendar they are sent to, that the tasks need a calendar of their own. A
// set of notes alone is refused outright (500), and that refusal is shown as
// any other.
func TestCreateSaysWhatTheServerDidNotTake(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.CreateKeepsFirstComponent = true

	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {"Trips"}, "components": {"VEVENT", "VTODO"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	path := davtest.CalendarHome + "trips/"
	if entry, ok := catalogOf(t, f).Collection(f.id, path); !ok || strings.Join(entry.Components, " ") != "VEVENT" {
		t.Fatalf("catalog entry = %+v (found %v), want what the server kept", entry, ok)
	}
	page := decode[view.AgendaPage](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))
	if len(page.Notices) != 1 {
		t.Fatalf("notices on the new calendar = %+v, want one", page.Notices)
	}
	n := page.Notices[0]
	if !strings.Contains(string(n.Title), "tasks") || strings.Contains(string(n.Title), "events") ||
		!strings.Contains(string(n.Body), "events only") || !strings.Contains(string(n.Body), "another calendar") {
		t.Errorf("notice on the new calendar = %q / %q, want the tasks the server did not take, what it kept, and another calendar for them", n.Title, n.Body)
	}

	// The calendar the server took all of says nothing.
	rec = f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {"Plain"}, "components": {"VEVENT"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create plain: %d %s", rec.Code, rec.Body.String())
	}
	if page := decode[view.AgendaPage](t, f.h.do(get(rec.Header().Get("Location"), f.sess))); len(page.Notices) != 0 {
		t.Errorf("notices on a calendar made as asked = %+v", page.Notices)
	}

	rec = f.h.do(postForm("/app/collections/new?kind=note", f.sess,
		url.Values{"account": {f.id}, "name": {"Journal"}, "components": {"VJOURNAL"}}))
	sheet := decode[view.CollectionNewSheet](t, rec)
	if sheet.Failure == nil || !strings.Contains(noticeWords(*sheet.Failure), "500") {
		t.Errorf("notes alone: failure %+v, want the server's 500", sheet.Failure)
	}
}

// noticeWords is everything a notice says, its technical block included.
func noticeWords(n view.Notice) string {
	parts := []string{string(n.Title), string(n.Body)}
	if n.Details != nil {
		parts = append(parts, string(n.Details.Summary))
		for _, l := range n.Details.Lines {
			parts = append(parts, string(l))
		}
	}
	return strings.Join(parts, " ")
}
