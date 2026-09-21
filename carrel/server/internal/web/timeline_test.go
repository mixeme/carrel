package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

func icsDay(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

func timelineRows(p view.ContactTimelinePage) map[string]view.TimelineRow {
	out := map[string]view.TimelineRow{}
	for _, g := range p.Groups {
		for _, r := range g.Rows {
			out[string(r.Title)] = r
		}
	}
	return out
}

// S-15.4: the person's timeline is a search for one person over the ticked
// sources. A record matches by an address of the card in ATTENDEE or
// ORGANIZER, or by RELATED-TO its UID; a file by the person's name in its
// name; nothing by a name in the text. 12 months back to 3 ahead, widened
// back 3 months at a time; newest first, by month, the undated last.
func TestContactTimeline(t *testing.T) {
	now := time.Now()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ann.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ann\nFN:Anna Kovaleva\nEMAIL:anna@example.com\nEMAIL:a.k@work.example\nEND:VCARD\n")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO", "VJOURNAL"}})
	ev := func(uid, extra string, at time.Time) string {
		return ics("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART:" + icsDay(at) + "\nDTEND:" + icsDay(at.Add(time.Hour)) + "\n" + extra + "END:VEVENT")
	}
	srv.Seed(searchWork+"e1.ics", ev("e1", "SUMMARY:Review with Anna\nATTENDEE;PARTSTAT=ACCEPTED:mailto:anna@example.com\n", now.AddDate(0, -1, 0)))
	srv.Seed(searchWork+"e2.ics", ev("e2", "SUMMARY:Planning ahead\nORGANIZER:mailto:A.K@work.example\n", now.AddDate(0, 1, 0)))
	srv.Seed(searchWork+"e3.ics", ev("e3", "SUMMARY:Anna Kovaleva mentioned\nDESCRIPTION:Anna Kovaleva said hello\n", now.AddDate(0, -1, 0)))
	srv.Seed(searchWork+"e4.ics", ev("e4", "SUMMARY:Old meeting\nATTENDEE:mailto:anna@example.com\n", now.AddDate(0, -14, 0)))
	srv.Seed(searchWork+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Send Anna the slides\nRELATED-TO:ann\nEND:VTODO"))
	srv.Seed(searchWork+"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nDTSTART:"+icsDay(now.AddDate(0, -2, 0))+"\nSUMMARY:Call notes\nRELATED-TO:ann\nEND:VJOURNAL"))
	srv.SeedFile(searchDocs+"Anna Kovaleva contract.pdf", "pdf")
	srv.SeedFile(searchDocs+"unrelated.txt", "x")
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess, sealed.Collection{AccountID: f.id, Path: searchDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})
	card := f.path("contacts") + "/ann"

	// The card does not gather the timeline: its inset is empty until the
	// timeline is opened, and More is always there.
	contact := decode[view.ContactPage](t, f.h.do(get(card, f.sess)))
	if len(contact.Timeline) != 0 || contact.More.Href != card+"/timeline" {
		t.Fatalf("before: inset %+v, more %+v", contact.Timeline, contact.More)
	}
	reports := f.srv.CountRequests("REPORT", "")

	page := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	if page.Poll.TaskID == "" || page.Name != "Anna Kovaleva" || page.Card.Title != "Anna Kovaleva" {
		t.Fatalf("page = %+v", page)
	}
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	if f.srv.CountRequests("REPORT", "") == reports {
		t.Error("the timeline asked no server")
	}
	page = decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	rows := timelineRows(page)
	want := map[string]string{
		"Review with Anna":           "attendee · accepted",
		"Planning ahead":             "organiser",
		"Send Anna the slides":       "related to this person",
		"Call notes":                 "related to this person",
		"Anna Kovaleva contract.pdf": "name in the file name",
	}
	for title, why := range want {
		if r, ok := rows[title]; !ok || string(r.Why) != why {
			t.Errorf("row %q = %+v (found %v), want why %q", title, r, ok, why)
		}
	}
	for _, absent := range []string{"Anna Kovaleva mentioned", "Old meeting", "unrelated.txt"} {
		if _, ok := rows[absent]; ok {
			t.Errorf("%q is in the timeline", absent)
		}
	}
	if len(page.Groups) < 3 || page.Groups[len(page.Groups)-1].Rubric != "Without a date" ||
		string(page.Groups[len(page.Groups)-1].Rows[0].Title) != "Send Anna the slides" {
		t.Errorf("groups = %+v", page.Groups)
	}
	if first := page.Groups[0].Rows[0]; string(first.Title) != "Planning ahead" {
		t.Errorf("first row %q, want the newest — the one ahead", first.Title)
	}
	if !strings.Contains(string(page.Note), "Nothing was looked for before") {
		t.Errorf("note = %q", page.Note)
	}
	var segments []string
	for _, a := range page.Bar.Segments {
		segments = append(segments, string(a.Label))
	}
	if strings.Join(segments, "|") != "All · 5|Events · 2|Tasks · 1|Notes · 1|Files · 1" {
		t.Errorf("segments = %v", segments)
	}
	events := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline?show=events", f.sess)))
	if n := len(timelineRows(events)); n != 2 {
		t.Errorf("events segment has %d rows", n)
	}

	// The inset now shows what was opened.
	contact = decode[view.ContactPage](t, f.h.do(get(card, f.sess)))
	if len(contact.Timeline) == 0 {
		t.Error("the inset stays empty after the timeline was opened")
	}

	// Widen the range: three months further back, the old meeting joins.
	var widen *view.Action
	for i := range page.Range.Presets {
		if page.Range.Presets[i].ID == "timeline.widen" {
			widen = &page.Range.Presets[i]
		}
	}
	if widen == nil {
		t.Fatalf("presets = %+v", page.Range.Presets)
	}
	u, _ := url.Parse(widen.Href)
	wide := decode[view.ContactTimelinePage](t, f.h.do(get(u.RequestURI(), f.sess)))
	waitPoll(t, f.h, f.sess, wide.Poll.TaskID, nil)
	wide = decode[view.ContactTimelinePage](t, f.h.do(get(u.RequestURI(), f.sess)))
	if _, ok := timelineRows(wide)["Old meeting"]; !ok {
		t.Errorf("widened by three months, the meeting 14 months back is missing: %+v", wide.Groups)
	}

	// Creating from here relates to the person.
	note := actionByID(page.Actions, "timeline.note")
	if note == nil || !strings.Contains(note.Href, "related=ann") {
		t.Errorf("actions = %+v, want Write a note related to the person", page.Actions)
	}
	event := actionByID(page.Actions, "timeline.event")
	if event == nil || !strings.Contains(event.Href, "attendee=anna%40example.com") {
		t.Errorf("actions = %+v, want New event with the person as attendee", page.Actions)
	}

	// Followed, the sheets carry the person into what is written.
	f.h.do(get(f.path("calendar"), f.sess)) // any collection works as the last used one
	follow := func(a *view.Action) view.NewSheet {
		t.Helper()
		resp := f.h.do(get(a.Href, f.sess))
		for resp.Code == 303 {
			resp = f.h.do(get(resp.Header().Get("Location"), f.sess))
		}
		return decode[view.NewSheet](t, resp)
	}
	sheet := follow(event)
	if len(sheet.Target.Options) == 0 {
		t.Fatalf("event sheet = %+v", sheet)
	}
	quick, _ := url.Parse(sheet.Target.Options[0].ID)
	sheet = decode[view.NewSheet](t, f.h.do(get(strings.TrimSuffix(quick.EscapedPath(), "/new")+"/new/quick?"+quick.RawQuery, f.sess)))
	form := formFields(sheet.Form)
	if form.Get("attendee") != "anna@example.com" {
		t.Fatalf("event quick form = %v, want the person as attendee", form)
	}
	form.Set("summary", "Coffee with Anna")
	form.Set("start.date", now.AddDate(0, 0, 7).Format("2006-01-02"))
	form.Set("start.time", "10:00")
	u, _ = url.Parse(sheet.Form.Action)
	if resp := f.h.do(postForm(u.EscapedPath(), f.sess, form)); resp.Code != 303 {
		t.Fatalf("save event: %d %s", resp.Code, resp.Body.String())
	}
	found := false
	for _, p := range f.srv.Paths(searchWork) {
		b, _, _ := f.srv.Object(p)
		found = found || strings.Contains(unfolded(b), "SUMMARY:Coffee with Anna") && strings.Contains(unfolded(b), "ATTENDEE:mailto:anna@example.com")
	}
	if !found {
		t.Error("the new event does not invite the person")
	}
}

var _ = davtest.CalendarHome

// A person can be in a record without a line of their own. SENT-BY names
// who really sent it — the assistant who put the meeting in for someone
// else — and DELEGATED-FROM names the one who handed participation over,
// who is often struck from the attendee list in the same breath. Reading
// only ORGANIZER and ATTENDEE gave both of them an empty timeline over a
// record that names them (RFC 5545 §3.2.18, §3.2.4, §3.2.5).
func TestTimelineFindsThePersonBehindAnotherLine(t *testing.T) {
	now := time.Now()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"sec.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:sec\nFN:Vera Sokolova\nEMAIL:vera@example.com\nEND:VCARD\n")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	ev := func(uid, extra string, at time.Time) string {
		return ics("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART:" + icsDay(at) + "\nDTEND:" + icsDay(at.Add(time.Hour)) + "\n" + extra + "END:VEVENT")
	}
	// Sent for the boss, who is the organiser and is not Vera.
	srv.Seed(searchWork+"s1.ics", ev("s1", "SUMMARY:Board meeting\nORGANIZER;SENT-BY=\"mailto:VERA@example.com\":mailto:boss@example.com\n", now.AddDate(0, -1, 0)))
	// Vera gave her seat away and was struck from the list: only the line
	// of the person who took it still names her.
	srv.Seed(searchWork+"s2.ics", ev("s2", "SUMMARY:Standup\nATTENDEE;DELEGATED-FROM=\"mailto:vera@example.com\":mailto:pavel@example.com\n", now.AddDate(0, -2, 0)))
	// And the other way round: Vera is on the list but handed it to Pavel.
	srv.Seed(searchWork+"s3.ics", ev("s3", "SUMMARY:Retro\nATTENDEE;DELEGATED-TO=\"mailto:vera@example.com\":mailto:pavel@example.com\n", now.AddDate(0, -3, 0)))
	// Someone else's delegation is not hers.
	srv.Seed(searchWork+"s4.ics", ev("s4", "SUMMARY:Not hers\nATTENDEE;DELEGATED-FROM=\"mailto:someone@example.com\":mailto:pavel@example.com\n", now.AddDate(0, -1, 0)))
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	card := f.path("contacts") + "/sec"

	page := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	rows := timelineRows(decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess))))
	want := map[string]string{
		"Board meeting": "sent for the organiser",
		"Standup":       "delegated this to an attendee",
		"Retro":         "given this by an attendee",
	}
	for title, why := range want {
		if r, ok := rows[title]; !ok || string(r.Why) != why {
			t.Errorf("row %q = %+v (found %v), want why %q", title, r, ok, why)
		}
	}
	if _, ok := rows["Not hers"]; ok {
		t.Errorf("someone else's delegation is in her timeline: %v", rows)
	}
}

// A series is one record with several components, and an occurrence that
// was overridden carries an organiser and attendees of its own. Someone
// added to one Thursday is named nowhere else in the record, so reading
// only the main component left them out of their own timeline (S-15.4).
func TestTimelineReadsEveryComponentOfASeries(t *testing.T) {
	now := time.Now()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"guest.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:guest\nFN:Pavel Orlov\nEMAIL:pavel@example.com\nEND:VCARD\n")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	at := now.AddDate(0, 0, -7)
	// The series names someone else throughout; Pavel is on one occurrence.
	srv.Seed(searchWork+"weekly.ics", ics(
		"BEGIN:VEVENT\nUID:weekly\nDTSTAMP:20260101T000000Z\nDTSTART:"+icsDay(at)+"\nDTEND:"+icsDay(at.Add(time.Hour))+
			"\nSUMMARY:Weekly sync\nRRULE:FREQ=WEEKLY;COUNT=6\nATTENDEE:mailto:someone@example.com\nEND:VEVENT\n"+
			"BEGIN:VEVENT\nUID:weekly\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID:"+icsDay(at)+
			"\nDTSTART:"+icsDay(at)+"\nDTEND:"+icsDay(at.Add(time.Hour))+
			"\nSUMMARY:Weekly sync\nATTENDEE;PARTSTAT=ACCEPTED:mailto:pavel@example.com\nEND:VEVENT"))
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	card := f.path("contacts") + "/guest"

	page := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	rows := timelineRows(decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess))))
	if r, ok := rows["Weekly sync"]; !ok || string(r.Why) != "attendee · accepted" {
		t.Errorf("the occurrence Pavel was added to: %+v (found %v, all %v)", r, ok, rows)
	}
}
