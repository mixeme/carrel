package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

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
		map[string]string{"summary": "Call", "start.date": "2031-03-11", "start.time": "09:30", "end.date": "", "end.time": ""}))
	mustContain(t, made("all day, full form", newForm,
		map[string]string{"summary": "Holiday", "start.date": "2031-03-11", "start.allday": "on", "end.date": "", "end.time": ""}),
		"DTSTART;VALUE=DATE:20310311", "DTEND;VALUE=DATE:20310312")
	quick := f.path("calendar") + "/new/quick"
	// The quick sheet gives a timed event without an end an hour (Б31):
	// it is written with an end all the same.
	mustContain(t, made("timed, quick sheet", quick,
		map[string]string{"summary": "Call", "start.date": "2031-03-12", "start.time": "09:30"}),
		"DTSTART:20310312T093000Z", "DTEND:20310312T103000Z")
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

// s-agenda-11 (from-typecase note 28, task 53): the calendar the server made
// with less than was asked says so over its lists as the frame writes it —
// the title, the text with the calendar's name, and «New calendar», which
// opens the sheet of a new calendar on this server with what was not taken
// ticked.
func TestTheNoticeOfWhatWasNotTakenIsTheFrame(t *testing.T) {
	frame, _, _ := strings.Cut(mockupFrame(t, "s-agenda-11"), "<figcaption")
	m := regexp.MustCompile(`(?s)<div class="tc-msg"><b>([^<]*)</b>(.*?)</div>`).FindStringSubmatch(frame)
	if m == nil {
		t.Fatal("s-agenda-11 draws no notice: read the frame again")
	}
	var buttons []string
	for _, b := range frameButton.FindAllStringSubmatch(m[2], -1) {
		buttons = append(buttons, frameText(b[1]))
	}
	body := frameText(frameButton.ReplaceAllString(m[2], ""))
	named := regexp.MustCompile(`«([^»]*)»`).FindStringSubmatch(body)
	if len(buttons) == 0 || named == nil {
		t.Fatalf("s-agenda-11 draws no button or no calendar's name: %q", m[2])
	}

	f := calendarFixture(t, nil)
	f.srv.CreateKeepsFirstComponent = true
	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {named[1]}, "components": {"VEVENT", "VTODO"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.AgendaPage](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))
	if len(page.Notices) != 1 {
		t.Fatalf("notices = %+v, want one", page.Notices)
	}
	n := page.Notices[0]
	if string(n.Title) != frameText(m[1]) || string(n.Body) != body || n.Role != view.NoticeInfo {
		t.Errorf("notice = %q / %q (%s), the frame %q / %q", n.Title, n.Body, n.Role, frameText(m[1]), body)
	}
	if got := labels(n.Actions); !equalStrings(got, buttons) {
		t.Fatalf("notice actions = %q, the frame %q", got, buttons)
	}
	a := n.Actions[0]
	if a.Method != "GET" || !a.Enabled || a.Primary {
		t.Errorf("New calendar = %+v: a plain button opening a sheet", a)
	}
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get(a.Href, f.sess)))
	if sheet.Kind != view.KindEvent || sheet.Target.Chosen != f.id {
		t.Errorf("sheet = %+v, want a new calendar on this server", sheet)
	}
	parts := formRow(sheet.Form, "components")
	var ticked []string
	for _, o := range parts.Options {
		if o.Selected {
			ticked = append(ticked, o.Value)
		}
	}
	if !equalStrings(ticked, []string{"VTODO"}) {
		t.Errorf("ticked = %q, want only what the server did not take", ticked)
	}
	if kind := formRow(sheet.Form, "kind"); kind != nil {
		t.Errorf("the sheet asks what it is: %+v", kind)
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

// docs/servers.md, SV-55: iCloud keeps one resource per UID across the whole
// account, and a PUT of a record whose UID lies in another calendar is 412
// under any name. «Copy here» inside such an account gives the copy a UID of
// its own (owner's decision 30.09.2026): for other apps it is another record,
// and the page says so with a link to the copy. The session remembers the
// server, so the next copy there is not sent with the old UID first, and the
// sheet says so before it starts.
func TestACopyGetsAUIDOfItsOwnWhereTheAccountHoldsOneRecordPerUID(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work, home := davtest.CalendarHome+"work/", davtest.CalendarHome+"home/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: home, Name: "Home"})
	srv.Seed(work+"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nX-CUSTOM:keep\nEND:VEVENT"))
	srv.Seed(work+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260116T100000Z\nDTEND:20260116T110000Z\nSUMMARY:Call\nEND:VEVENT"))
	srv.OneUIDPerAccount = true
	f := connectFixture(t, srv, work, nil)
	original, _, _ := srv.Object(work + "meet.ics")

	sheetOf := func(uid string) view.MoveSheet {
		t.Helper()
		q := url.Values{"copy": {"1"}, "to": {f.id}, "into": {home}}
		return decode[view.MoveSheet](t, f.h.do(get(f.path("calendar")+"/"+uid+"/move?"+q.Encode(), f.sess)))
	}
	copyOf := func(uid, summary string) string {
		t.Helper()
		page := decode[view.EventPage](t, f.h.do(postForm(confirmOf(t, sheetOf(uid)).Href, f.sess, url.Values{})))
		var opens string
		for _, n := range page.Notices {
			if string(n.Title) != "Copied, with a UID of its own" {
				continue
			}
			for _, a := range n.Actions {
				if a.ID == "record.open" {
					opens = a.Href
				}
			}
		}
		prefix := "/app/calendar/" + f.id + "/" + url.PathEscape(home) + "/"
		if !strings.HasPrefix(opens, prefix) || opens == prefix+uid {
			t.Fatalf("copy of %s: the notice leads to %q, want the copy under a UID of its own in Home; notices %+v", uid, opens, page.Notices)
		}
		if got := decode[view.EventPage](t, f.h.do(get(opens, f.sess))); string(got.Summary) != summary {
			t.Errorf("the copy the notice opens = %q, notices %+v", got.Summary, got.Notices)
		}
		return strings.TrimPrefix(opens, prefix)
	}

	fresh := copyOf("meet", "Review")
	taken := srv.UIDsTaken()
	if taken == 0 {
		t.Fatal("the stand refused nothing: the guard does not look at what it names")
	}
	paths := srv.Paths(home)
	if len(paths) != 1 {
		t.Fatalf("Home holds %v, want the one copy", paths)
	}
	copied, _, _ := srv.Object(paths[0])
	mustContain(t, unfolded(copied), "UID:"+fresh, "SUMMARY:Review", "X-CUSTOM:keep")
	if strings.Contains(copied, "UID:meet") {
		t.Errorf("the copy kept the UID the account holds already:\n%s", copied)
	}
	if got, _, _ := srv.Object(work + "meet.ics"); got != original {
		t.Error("the original changed")
	}

	// The account is known now: the sheet says it before the start, and the
	// next copy goes out with its own UID at once.
	if w := plainText(sheetOf("call").Warning); !strings.Contains(w, "a UID of its own") {
		t.Errorf("sheet warning = %q, want it to say the copy gets a UID of its own", w)
	}
	copyOf("call", "Call")
	if n := srv.UIDsTaken(); n != taken {
		t.Errorf("PUTs refused = %d after the first %d: the account is not remembered", n, taken)
	}
}

// docs/servers.md, SV-55, the import: a file whose record carries a UID
// another calendar of the same iCloud account holds is refused 412 under any
// name. The owner's rule for a copy (30.09.2026) holds for an import too: the
// record is written with a UID of its own, the report's row says why, and a
// record of the same file that points at it follows it there.
func TestAnImportGetsAUIDOfItsOwnWhereTheAccountHoldsOneRecordPerUID(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work, home := davtest.CalendarHome+"work/", davtest.CalendarHome+"home/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: home, Name: "Home"})
	srv.Seed(work+"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nEND:VEVENT"))
	srv.OneUIDPerAccount = true
	f := connectFixture(t, srv, work, nil)

	file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:meet\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260115T100000Z\r\nDTEND:20260115T110000Z\r\nSUMMARY:Review\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:after\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260116T100000Z\r\nDTEND:20260116T110000Z\r\nSUMMARY:Follow-up\r\nRELATED-TO:meet\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	into := "/app/calendar/" + f.id + "/" + url.PathEscape(home) + "/import"
	page := decode[view.ImportPage](t, f.h.do(get(into, f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "work.ics", []byte(file))))
	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if page.Report == nil {
		t.Fatalf("no report: %+v", page.Notices)
	}
	if srv.UIDsTaken() == 0 {
		t.Fatal("the stand refused nothing: the guard does not look at what it names")
	}
	if page.Report.Created != 2 || page.Report.Renamed != 1 {
		t.Fatalf("report = %+v", page.Report)
	}
	// «With a new UID» says why (s-io-9; Typecase's note 111 p. 7).
	if len(page.Report.NewUIDs) != 1 || !strings.HasPrefix(string(page.Report.NewUIDs[0]), "Review") ||
		!strings.HasSuffix(string(page.Report.NewUIDs[0]), " · the account holds this UID in another collection") {
		t.Errorf("new UIDs = %q", page.Report.NewUIDs)
	}
	paths := srv.Paths(home)
	if len(paths) != 2 {
		t.Fatalf("Home holds %v, want both records", paths)
	}
	var review, after string
	for _, p := range paths {
		body, _, _ := srv.Object(p)
		switch {
		case strings.Contains(body, "SUMMARY:Review"):
			review = body
		case strings.Contains(body, "SUMMARY:Follow-up"):
			after = body
		}
	}
	if review == "" || strings.Contains(review, "UID:meet") {
		t.Fatalf("the imported Review kept the UID the account holds:\n%s", review)
	}
	fresh := strings.TrimSpace(strings.SplitN(strings.SplitN(unfolded(review), "UID:", 2)[1], "\n", 2)[0])
	if !strings.Contains(unfolded(after), "RELATED-TO:"+fresh) {
		t.Errorf("the follow-up still points at the UID the import could not keep (want %s):\n%s", fresh, after)
	}
	if got, _, _ := srv.Object(work + "meet.ics"); !strings.Contains(got, "UID:meet") {
		t.Error("the event in Work changed")
	}
}
