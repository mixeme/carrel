package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

const (
	rewriteWork   = davtest.CalendarHome + "work/"
	rewriteShared = davtest.CalendarHome + "shared/"
	rewriteCard   = "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:ada\r\nFN:Ada Lovelace\r\nEMAIL;TYPE=work:ada@old.example\r\nEND:VCARD\r\n"
)

// meeting is an event of 2099 organised by organiser, with Ada among the
// attendees and her answer on the line.
func meeting(uid, organiser, when string) string {
	return strings.Join([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//T//EN", "BEGIN:VEVENT",
		"UID:" + uid, "DTSTAMP:20260101T000000Z", "DTSTART:" + when, "SUMMARY:" + uid,
		"ORGANIZER;CN=Organiser:mailto:" + organiser,
		"ATTENDEE;CN=Ada Lovelace;PARTSTAT=ACCEPTED;ROLE=REQ-PARTICIPANT:mailto:ADA@old.example",
		"END:VEVENT", "END:VCALENDAR", ""}, "\r\n")
}

// rewriteStand is one account that schedules, with Ada's card, a calendar
// of meetings and a read-only one, both ticked.
type rewriteStand struct {
	davFixture
	card string // the card's address on Carrel
}

func newRewriteStand(t *testing.T) rewriteStand {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.ScheduleOutbox = davtest.Principal + "outbox/"
	srv.UserAddresses = []string{"mailto:alice@example.org"}
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: rewriteWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO"}})
	srv.AddCalendar(caldav.Calendar{Path: rewriteShared, Name: "Shared", SupportedComponentSet: []string{"VEVENT"}, ReadOnly: true})
	srv.Seed(peopleBook+"ada.vcf", rewriteCard)
	srv.Seed(rewriteWork+"mine.ics", meeting("mine", "alice@example.org", "20990105T100000Z"))
	srv.Seed(rewriteWork+"theirs.ics", meeting("theirs", "boss@example.org", "20990106T100000Z"))
	srv.Seed(rewriteWork+"old.ics", meeting("old", "alice@example.org", "20200106T100000Z"))
	srv.Seed(rewriteWork+"other.ics", strings.ReplaceAll(meeting("other", "alice@example.org", "20990107T100000Z"), "ADA@old.example", "bob@example.org"))
	srv.Seed(rewriteShared+"shared.ics", meeting("shared", "boss@example.org", "20990108T100000Z"))
	f := connectFixture(t, srv, peopleBook, nil)
	key, _ := f.sess.DataKey()
	for _, k := range []view.Kind{view.KindEvent, view.KindTask} {
		sel := []sealed.Selection{{Kind: k, AccountID: f.id, Path: rewriteWork}}
		if k == view.KindEvent {
			sel = append(sel, sealed.Selection{Kind: k, AccountID: f.id, Path: rewriteShared})
		}
		if err := f.h.users.SetSelection(f.sess.AccountID(), key, k, sel); err != nil {
			t.Fatal(err)
		}
	}
	return rewriteStand{davFixture: f, card: f.path("contacts") + "/ada"}
}

// save posts the card's edit form with the emails set.
func (s rewriteStand) save(t *testing.T, set func(url.Values)) *http.Response {
	t.Helper()
	form := formFields(decode[view.ContactEditPage](t, s.h.do(get(s.card+"/edit", s.sess))).Form)
	set(form)
	return s.h.do(postForm(s.card+"/edit", s.sess, form)).Result()
}

// S-15.4, decision of 24.09.2026: an address replaced on the card is asked
// about, with what rewriting can set off named before anything is written
// (s-contacts-15); the answer rewrites every record that is not read-only,
// and the card says what came of it (s-contacts-16).
func TestReplacingAnAddressOffersToRewriteIt(t *testing.T) {
	s := newRewriteStand(t)
	resp := s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("save answered %d, want the sheet", resp.StatusCode)
	}
	sheetAt := resp.Header.Get("Location")
	if !strings.HasPrefix(sheetAt, s.card+"/rewrite?") {
		t.Fatalf("after the save: %q, want the rewrite sheet", sheetAt)
	}
	if body, _, _ := s.srv.Object(peopleBook + "ada.vcf"); !strings.Contains(body, "ada@new.example") {
		t.Fatalf("the card was not saved first:\n%s", body)
	}
	sheet := decode[view.RewriteSheet](t, s.h.do(get(sheetAt, s.sess)))
	if sheet.Title != "Rewrite the address in their records?" || sheet.Subject != "Ada Lovelace · saved" {
		t.Errorf("head = %q, %q", sheet.Title, sheet.Subject)
	}
	if len(sheet.Pairs) != 1 || sheet.Pairs[0].From != "ada@old.example" || sheet.Pairs[0].To != "ada@new.example" ||
		sheet.Pairs[0].Note != "4 events and tasks name the old one; they drop out of the person’s timeline" {
		t.Errorf("pairs = %+v", sheet.Pairs)
	}
	want := []view.Field{
		{Label: "3 records on servers that schedule", Value: "the server may send a cancellation to the old address, an invitation to the new one and an update to the other attendees"},
		{Label: "1 meeting organised by others", Value: "the organiser’s server may refuse the change, and its next update may undo it"},
		{Label: "Past meetings", Value: "will show the new address, as if the person had been invited at it"},
	}
	if !sameFields(sheet.Effects, want) {
		t.Errorf("effects = %+v\nwant %+v", sheet.Effects, want)
	}
	if kept := []view.Field{{Label: "1 record in Shared", Value: "read-only; it keeps the old address"}}; !sameFields(sheet.Kept, kept) {
		t.Errorf("kept = %+v", sheet.Kept)
	}
	confirm := rewriteAction(t, sheet.Actions, "contact.rewrite.confirm")
	if confirm.Label != "Rewrite 3 records" || !confirm.Enabled {
		t.Errorf("confirm = %+v", confirm)
	}
	if leave := rewriteAction(t, sheet.Actions, "contact.rewrite.leave"); leave.Label != "Leave as is" {
		t.Errorf("leave = %+v", leave)
	}

	sharedBefore, _, _ := s.srv.Object(rewriteShared + "shared.ics")
	otherBefore, _, _ := s.srv.Object(rewriteWork + "other.ics")
	page := decode[view.ContactPage](t, s.h.do(postForm(confirm.Href, s.sess, url.Values{})))
	var n *view.Notice
	for i := range page.Notices {
		if strings.HasPrefix(string(page.Notices[i].Title), "Rewritten in") {
			n = &page.Notices[i]
		}
	}
	if n == nil {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if n.Title != "Rewritten in 3 records." || n.Role != view.NoticeInfo ||
		n.Body != "1 record in Shared is read-only and keeps the old address." {
		t.Errorf("outcome = %+v", *n)
	}
	for _, name := range []string{"mine", "theirs", "old"} {
		body, _, _ := s.srv.Object(rewriteWork + name + ".ics")
		un := strings.ReplaceAll(body, "\r\n ", "")
		if strings.Contains(strings.ToLower(un), "ada@old.example") || !strings.Contains(un, "mailto:ada@new.example") ||
			!strings.Contains(un, "PARTSTAT=ACCEPTED") || !strings.Contains(un, "ROLE=REQ-PARTICIPANT") {
			t.Errorf("%s was not rewritten whole:\n%s", name, body)
		}
	}
	if after, _, _ := s.srv.Object(rewriteShared + "shared.ics"); after != sharedBefore {
		t.Errorf("a read-only calendar was written:\n%s", after)
	}
	if after, _, _ := s.srv.Object(rewriteWork + "other.ics"); after != otherBefore {
		t.Errorf("a record that does not name her was written:\n%s", after)
	}
	// The plan is spent: the sheet again is the card.
	if rec := s.h.do(get(sheetAt, s.sess)); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != s.card {
		t.Errorf("the sheet after the answer: %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

// What the server refuses and what changed after the scan are named per
// record, and the notice is an alert (s-contacts-16).
func TestARewriteNamesWhatWasRefused(t *testing.T) {
	s := newRewriteStand(t)
	resp := s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	// Changed on the server after the sheet was drawn.
	s.srv.Seed(rewriteWork+"old.ics", strings.Replace(meeting("old", "alice@example.org", "20200106T100000Z"), "SUMMARY:old", "SUMMARY:old moved", 1))
	s.srv.Refuse("PUT", http.StatusForbidden, "")
	page := decode[view.ContactPage](t, s.h.do(postForm(rewriteAction(t, sheet.Actions, "contact.rewrite.confirm").Href, s.sess, url.Values{})))
	var n view.Notice
	for _, x := range page.Notices {
		if strings.HasPrefix(string(x.Title), "Rewritten in") {
			n = x
		}
	}
	if n.Title != "Rewritten in 0 of 3 records." || n.Role != view.NoticeAlert {
		t.Fatalf("outcome = %+v", n)
	}
	for _, part := range []string{
		"«theirs» was refused: its organiser’s server does not take changes to attendees.",
		"«old moved» was not written: it changed after Carrel read it.",
		"«mine» was refused: ",
		"1 record in Shared is read-only and keeps the old address.",
	} {
		if !strings.Contains(string(n.Body), part) {
			t.Errorf("the outcome does not say %q:\n%s", part, n.Body)
		}
	}
}

// Removing an address asks nothing; «Leave as is» leaves everything and
// comes back to the card.
func TestRemovedAddressAsksNothingAndLeaveKeepsAll(t *testing.T) {
	s := newRewriteStand(t)
	resp := s.save(t, func(f url.Values) {
		f.Set("email.0", "")
		f.Set("title", "Countess")
	})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != s.card {
		t.Fatalf("an address removed: %d %q, want the card", resp.StatusCode, resp.Header.Get("Location"))
	}

	s = newRewriteStand(t)
	resp = s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	sheetAt := resp.Header.Get("Location")
	sheet := decode[view.RewriteSheet](t, s.h.do(get(sheetAt, s.sess)))
	before, _, _ := s.srv.Object(rewriteWork + "mine.ics")
	rec := s.h.do(postForm(rewriteAction(t, sheet.Actions, "contact.rewrite.leave").Href, s.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != s.card {
		t.Fatalf("Leave as is: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if after, _, _ := s.srv.Object(rewriteWork + "mine.ics"); after != before {
		t.Error("Leave as is wrote a record")
	}
	if rec := s.h.do(postForm(rewriteAction(t, sheet.Actions, "contact.rewrite.confirm").Href, s.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Errorf("the plan outlived Leave as is: %d", rec.Code)
	}
}

// Several replaced at once: each removed address goes to the card's default
// one — the preferred, else the first — and the sheet names every pair.
func TestSeveralReplacedGoToTheDefaultAddress(t *testing.T) {
	s := newRewriteStand(t)
	s.srv.Seed(peopleBook+"ada.vcf", strings.Replace(rewriteCard, "END:VCARD", "EMAIL;TYPE=home:ada@home.example\r\nEND:VCARD", 1))
	resp := s.save(t, func(f url.Values) {
		f.Set("email.0", "ada@new.example")
		f.Set("email.1", "")
		f.Set("email.2", "ada@second.example")
	})
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	var pairs []string
	for _, p := range sheet.Pairs {
		pairs = append(pairs, string(p.From)+" → "+string(p.To))
	}
	if strings.Join(pairs, "; ") != "ada@home.example → ada@new.example; ada@old.example → ada@new.example" {
		t.Errorf("pairs = %q", pairs)
	}
}

// C-10: the rows of the sheet say what the frame says (s-contacts-15),
// read from the frame — except where the frame writes «she» and «her»: the
// server does not know the person's gender and says «they» (to-typecase 25).
func TestRewriteSheetSaysWhatTheFrameSays(t *testing.T) {
	frame := mockupFrame(t, "s-contacts-15")
	item := regexp.MustCompile(`<span class="tc-stack__item"><b>([^<]*)</b> <span class="tc-dim">— ([^<]*)</span></span>`)
	notes := map[string]bool{}
	for _, m := range item.FindAllStringSubmatch(frame, -1) {
		notes[html.UnescapeString(m[2])] = true
	}
	s := newRewriteStand(t)
	resp := s.save(t, func(f url.Values) { f.Set("email.0", "ada@new.example") })
	sheet := decode[view.RewriteSheet](t, s.h.do(get(resp.Header.Get("Location"), s.sess)))
	for _, f := range append(append([]view.Field(nil), sheet.Effects...), sheet.Kept...) {
		v := string(f.Value)
		if strings.Contains(v, "the person") || strings.HasPrefix(v, "read-only; it") {
			continue // the frame's «she», and the frame's plural of the same row
		}
		if !notes[v] {
			t.Errorf("%q: the frame has no such note; it has %v", v, notes)
		}
	}
	for _, label := range []string{"Leave as is", "Rewrite 12 records"} {
		if !strings.Contains(frame, ">"+label+"<") {
			t.Errorf("the frame has no %q", label)
		}
	}
	if !strings.Contains(frame, "read-only; they keep the old address") {
		t.Error("the frame's read-only row changed")
	}
}

func sameFields(got, want []view.Field) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Label != want[i].Label || got[i].Value != want[i].Value {
			return false
		}
	}
	return true
}

func rewriteAction(t *testing.T, actions []view.Action, id string) view.Action {
	t.Helper()
	for _, a := range actions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no action %q in %+v", id, actions)
	return view.Action{}
}

// S-6, S-15.4: what the principal says of scheduling is kept on the
// connection when it is made, edited and discovered again — a connection
// made before the question learns it at its next edit or re-discovery.
func TestTheConnectionKeepsWhetherItSchedules(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	f := connectFixture(t, srv, work, nil)
	if c := catalogOf(t, f).Connections[0]; c.Scheduling != "no" || len(c.Addresses) != 0 {
		t.Fatalf("added without an outbox: %q %q", c.Scheduling, c.Addresses)
	}

	srv.ScheduleOutbox = davtest.Principal + "outbox/"
	srv.UserAddresses = []string{"mailto:Alice@Example.org"}
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "team/", Name: "Team"})
	n := noticeTitled(rediscoverPage(t, f).Notices, "Collections changed on the server")
	if n == nil {
		t.Fatal("no proposal")
	}
	f.h.do(postHref(f, actionHref(t, n.Actions, "connection.rediscover.apply")))
	if c := catalogOf(t, f).Connections[0]; c.Scheduling != "yes" || strings.Join(c.Addresses, ",") != "alice@example.org" {
		t.Errorf("after Apply: %q %q", c.Scheduling, c.Addresses)
	}

	srv.ScheduleOutbox = ""
	rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"op": {"edit"}, "account": {f.id}, "server": {srv.URL}, "login": {"alice"}, "password": {""},
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("edit: %d", rec.Code)
	}
	if c := catalogOf(t, f).Connections[0]; c.Scheduling != "no" {
		t.Errorf("after Edit: %q", c.Scheduling)
	}
}

// A save the server did not keep whole is reported first, where the change
// was made (C-9); the question about the address is then not asked.
func TestALossComesBeforeTheQuestion(t *testing.T) {
	s := newRewriteStand(t)
	s.srv.Seed(peopleBook+"ada.vcf", strings.Replace(rewriteCard, "END:VCARD", "X-KEPT-HERE:yes\r\nEND:VCARD", 1))
	s.srv.Drop = []string{"X-KEPT-HERE"}
	form := formFields(decode[view.ContactEditPage](t, s.h.do(get(s.card+"/edit", s.sess))).Form)
	form.Set("email.0", "ada@new.example")
	page := decode[view.ContactEditPage](t, s.h.do(postForm(s.card+"/edit", s.sess, form)))
	if page.Loss == nil {
		t.Fatal("no loss report")
	}
	if s.sess.Draft(session.DraftRewrite) != nil {
		t.Error("a plan was kept for a save that ended in a loss report")
	}
}
