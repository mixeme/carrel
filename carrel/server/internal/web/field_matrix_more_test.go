package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The matrix «every field × a hard record», what v0.2.188 and its first
// rounds left out (docs/plan/server.md, the check of the record's audit):
// a card beyond the seed — a labelled mail, a second mail, an address with
// what the form does not show, a birthday, a web page, and lines the form
// has no field for —; attendees with what other clients write beside them;
// the fields of a new record. A second RRULE is the stand's: the test
// server (go-ical) refuses to keep one. One field changed changes its
// own lines; a new record holds no empty line for a field left empty.
//
// The test server writes back every card it is handed in go-vcard's
// spelling, so the line «before» is the card after a first save of another
// field (oneEdit's prime): both sides are spelt alike.

const matrixCard = "BEGIN:VCARD\nVERSION:3.0\nUID:hard\nFN:Ada Lovelace\nN:Lovelace;Ada;Augusta;Countess;PhD\n" +
	"item2.EMAIL;TYPE=INTERNET,WORK,PREF:ada@example.com\nitem2.X-ABLabel:Office\nEMAIL;TYPE=INTERNET,HOME:ada@home.example\n" +
	"item3.ADR;TYPE=HOME:;Flat 2;12 St James's Square;London;;SW1Y 4JH;UK\nitem3.X-ABADR:gb\n" +
	"URL;TYPE=WORK:https://ada.example\nBDAY;VALUE=DATE:1815-12-10\nIMPP;X-SERVICE-TYPE=Skype:skype:ada\n" +
	"X-SOCIALPROFILE;TYPE=twitter:https://x.example/ada\nCATEGORIES:math,poetry\nX-CUSTOM:keep\nEND:VCARD\n"

func TestACardBeyondTheSeedSaveIsTheEdit(t *testing.T) {
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
	}{
		{"a labelled mail", url.Values{"email.0": {"ada@new.example"}}, []string{"item2.EMAIL"},
			[]string{"item2.X-ABLABEL:Office", "ada@new.example", "ada@home.example"}},
		{"the label of the second mail", url.Values{"email.1.type": {"WORK"}}, []string{"EMAIL"},
			[]string{"ada@home.example", "ada@example.com"}},
		// The extended address (Flat 2) has no field: it stays.
		{"the city", url.Values{"adr.0.city": {"Cambridge"}}, []string{"item3.ADR"},
			[]string{";Flat 2;12 St James's Square;Cambridge;;SW1Y 4JH;UK", "item3.X-ABADR:gb"}},
		{"the birthday", url.Values{"bday": {"11.12.1815"}}, []string{"BDAY"}, []string{"1815-12-11"}},
		{"the web page", url.Values{"url.0": {"https://ada.example/new"}}, []string{"URL"}, []string{"https://ada.example/new"}},
		// U-6.1: the display name made of the name parts follows them.
		{"the given name", url.Values{"n.given": {"Augusta"}}, []string{"FN", "N"}, []string{"N:Lovelace;Augusta;Augusta;Countess;PhD", "FN:Augusta Lovelace"}},
		{"one more tag", url.Values{"categories": {"math, poetry, engines"}}, []string{"CATEGORIES"}, []string{"CATEGORIES:math,poetry,engines"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := contactsFixture(t, false, matrixCard)
			before, after := oneEdit(t, f, f.path("contacts")+"/hard", "hard.vcf", url.Values{"nickname": {"Ada"}}, tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, append(tc.holds, "IMPP;X-SERVICE-TYPE=Skype:skype:ada", "X-SOCIALPROFILE;TYPE=twitter:https://x.example/ada", "X-CUSTOM:keep")...)
		})
	}
}

// A web page in a group with Apple's label, as Google keeps one
// («item2.URL» beside «item2.X-ABLabel:Work»; the live matrix on Google,
// 16.11.2026): a new address keeps the group, so the label stays its.
func TestAGroupedWebPageKeepsItsGroup(t *testing.T) {
	card := "BEGIN:VCARD\nVERSION:3.0\nUID:hard\nFN:Ada Lovelace\nN:Lovelace;Ada;;;\n" +
		"item2.URL;TYPE=PREF:https://ada.example\nitem2.X-ABLabel:Work\nEND:VCARD\n"
	f := contactsFixture(t, false, card)
	before, after := oneEdit(t, f, f.path("contacts")+"/hard", "hard.vcf", url.Values{"nickname": {"Ada"}}, url.Values{"url.0": {"https://ada.example/new"}})
	if got := changedLines(before, after); !slices.Equal(got, []string{"item2.URL"}) {
		t.Errorf("lines changed: %v, want [item2.URL]\n%s", got, after)
	}
	mustContain(t, after, "item2.URL", "https://ada.example/new", "item2.X-ABLABEL:Work")
}

// A card as Google keeps it (the live matrix, 16.11.2026): groups taken
// apart, an address marked PREF, Apple's label lines without their group.
// Another field saved is saved.
func TestACardAsGoogleKeepsItSaves(t *testing.T) {
	card := "BEGIN:VCARD\nVERSION:3.0\nN:Lovelace;Ada;Augusta;Countess;PhD\nFN:Countess Ada Augusta Lovelace PhD\nUID:hard\n" +
		"BDAY;VALUE=DATE:1815-12-10\nADR;TYPE=HOME,PREF:;Flat 2;12 St James's Square;London;;SW1Y 4JH;UK\n" +
		"EMAIL;TYPE=WORK,PREF:ada@example.com\nEMAIL;TYPE=HOME:ada@home.example\n" +
		"item1.IMPP;TYPE=PREF;X-SERVICE-TYPE=Skype:skype:ada\nX-CUSTOM:keep\nX-ABLABEL:Office\nX-ABADR:gb\n" +
		"item1.X-ABLabel:Other\nitem2.URL;TYPE=PREF:https://ada.example\nitem2.X-ABLabel:Work\nEND:VCARD\n"
	f := contactsFixture(t, false, card)
	before, after := oneEdit(t, f, f.path("contacts")+"/hard", "hard.vcf", url.Values{"bday": {"11.12.1815"}}, url.Values{"nickname": {"Ada"}})
	if got := changedLines(before, after); !slices.Equal(got, []string{"NICKNAME"}) {
		t.Errorf("lines changed: %v, want [NICKNAME]\n%s", got, after)
	}
}

// Attendees as other clients write them: a reply, a role, a delegate, a
// language. A list changed by the field changes the lines of the people
// changed; the others keep every parameter.
func matrixMeeting() string {
	return ics("BEGIN:VEVENT\nUID:hard\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDTEND;TZID=Europe/Moscow:20260112T110000\n" +
		"SUMMARY:Review\n" +
		"ATTENDEE;CN=Bob;PARTSTAT=ACCEPTED;ROLE=REQ-PARTICIPANT;RSVP=TRUE;LANGUAGE=en:mailto:bob@example.com\n" +
		"ATTENDEE;CN=Carol;PARTSTAT=DELEGATED;DELEGATED-TO=\"mailto:dan@example.com\":mailto:carol@example.com\n" +
		"ATTENDEE;CN=Dan;PARTSTAT=TENTATIVE;DELEGATED-FROM=\"mailto:carol@example.com\";X-NUM-GUESTS=1:mailto:dan@example.com\n" +
		"RRULE:FREQ=WEEKLY;BYDAY=MO\nEND:VEVENT")
}

func TestAttendeesOfAnotherClientSaveIsTheEdit(t *testing.T) {
	// As the test server spells them: parameters in the order of their
	// names.
	bob := "ATTENDEE;CN=Bob;LANGUAGE=en;PARTSTAT=ACCEPTED;ROLE=REQ-PARTICIPANT;RSVP=TRUE:mailto:bob@example.com"
	dan := "ATTENDEE;CN=Dan;DELEGATED-FROM=\"mailto:carol@example.com\";PARTSTAT=TENTATIVE;X-NUM-GUESTS=1:mailto:dan@example.com"
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
		gone    []string
	}{
		{"one more", url.Values{"attendee": {"mailto:bob@example.com", "mailto:carol@example.com", "mailto:dan@example.com"}, "attendee.add": {"eve@example.com"}},
			[]string{"ATTENDEE"}, []string{bob, dan, "mailto:eve@example.com"}, nil},
		// The drawn field posts its line to type into, empty or not: without
		// it nobody is taken away (a form that did not draw the field).
		{"one fewer", url.Values{"attendee": {"mailto:bob@example.com", "mailto:dan@example.com"}, "attendee.add": {""}},
			// Dan keeps naming whom he stands for (DELEGATED-FROM): that is
			// his line, not Carol's.
			[]string{"ATTENDEE"}, []string{bob, dan}, []string{"CN=Carol"}},
		{"every second week", url.Values{"repeat.interval": {"2"}}, []string{"RRULE"},
			[]string{"RRULE:FREQ=WEEKLY;BYDAY=MO;INTERVAL=2", bob, dan}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "hard.ics", matrixMeeting())
			before, after := oneEdit(t, f, f.path("calendar")+"/hard", "hard.ics", url.Values{"description": {"primed"}}, tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, tc.holds...)
			for _, g := range tc.gone {
				if strings.Contains(after, g) {
					t.Errorf("%q stayed:\n%s", g, after)
				}
			}
		})
	}
}

// emptyLines are the content lines of a document with nothing after their
// colon.
func emptyLines(doc string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n") {
		if l != "" && strings.HasSuffix(l, ":") {
			out = append(out, l)
		}
	}
	return out
}

// newOne saves the new form of a list with set over what it shows, and is
// the document it made.
func newOne(t *testing.T, f davFixture, list, ext string, set url.Values) string {
	t.Helper()
	form := formFields(decode[editedForm](t, f.h.do(get(list+"/new", f.sess))).Form)
	for k, v := range set {
		form[k] = v
	}
	resp := f.h.do(postForm(list+"/new", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("new: %d %s", resp.Code, resp.Body.String())
	}
	at := resp.Header().Get("Location")
	return body(t, f, at[strings.LastIndex(at, "/")+1:]+ext)
}

// A new record writes the fields typed, and no line for a field left empty.
func TestANewRecordHoldsWhatWasTyped(t *testing.T) {
	c := contactsFixture(t, false)
	card := newOne(t, c, c.path("contacts"), ".vcf", url.Values{"fn": {"Grace Hopper"}, "email.0": {"grace@example.com"}})
	mustContain(t, card, "FN:Grace Hopper", "grace@example.com")
	if e := emptyLines(card); len(e) > 0 {
		t.Errorf("a new card holds empty lines %v:\n%s", e, card)
	}

	e := calendarFixture(t, nil)
	event := newOne(t, e, e.path("calendar"), ".ics", url.Values{"summary": {"Launch"}, "start.date": {"2026-03-02"}, "start.time": {"10:00"}, "end.date": {"2026-03-02"}, "end.time": {"11:00"}})
	mustContain(t, event, "SUMMARY:Launch")
	if l := emptyLines(event); len(l) > 0 {
		t.Errorf("a new event holds empty lines %v:\n%s", l, event)
	}

	k := calendarFixture(t, nil)
	task := newOne(t, k, k.path("tasks"), ".ics", url.Values{"summary": {"Ship"}})
	mustContain(t, task, "SUMMARY:Ship")
	if l := emptyLines(task); len(l) > 0 {
		t.Errorf("a new task holds empty lines %v:\n%s", l, task)
	}
	_ = view.KindTask
}
