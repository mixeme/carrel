//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// The matrix «every field × a hard record» of the record's audit on the
// servers people connect (docs/plan/server.md, the check; the owner's word of
// 16.11.2026 «Бери полную сверку»). The stand's version
// (internal/web/edit_is_the_edit_test.go, field_matrix_test.go,
// field_matrix_more_test.go) runs on the test server, which spells every
// record in go-vcard's and go-ical's way; here the server spells it its own
// way. The records and the edits are the stand's: a record another client
// wrote is seeded, saved once by Carrel with a field no case edits (prime),
// read — «before» —, saved with one field changed, read — «after». The lines
// that differ name the properties the edit changed; they must be the
// field's. What the server reports not keeping (C-9) is its answer, said in
// the log and not demanded; what it spells differently is the same on both
// sides, since both were written by Carrel and kept by the server.

type matrixCase struct {
	name    string
	set     map[string]string
	changed []string // property names, upper case, with the group: ITEM2.EMAIL
	says    []string // what the record holds after, in any case
	gone    []string // what it no longer holds
}

type matrixRecord struct {
	name  string
	kind  view.Kind
	doc   func(uid string) string
	prime map[string]string
	cases []matrixCase
}

func matrixCal(body string) string {
	return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n" + body + "\nEND:VCALENDAR\n"
}

func matrixRecords() []matrixRecord {
	return []matrixRecord{
		{name: "card", kind: view.KindContact, prime: map[string]string{"bday": "10.12.1815"},
			doc: func(uid string) string {
				return "BEGIN:VCARD\nVERSION:3.0\nUID:" + uid + "\nFN:Ada Lovelace\nN:Lovelace;Ada;Augusta;Countess;PhD\n" +
					"NICKNAME:Ada\nORG:Analytical;Engines\nORG:Royal Society\nTITLE:Mathematician\nTITLE:Countess\n" +
					"item1.TEL;TYPE=WORK,CELL,PREF:+44 20 1\nitem1.X-ABLabel:Desk\nTEL;TYPE=HOME:+44 20 2\n" +
					"EMAIL;TYPE=INTERNET,WORK:ada@example.com\nNOTE:First line\\nsecond line\\n\\n\nCATEGORIES:math,poetry\nX-CUSTOM:keep\nEND:VCARD\n"
			},
			cases: []matrixCase{
				{name: "another field than the note", set: map[string]string{"nickname": "Countess of Lovelace"}, changed: []string{"NICKNAME"}},
				{name: "a title of two", set: map[string]string{"title": "Analyst"}, changed: []string{"TITLE"}, says: []string{"Analyst", "TITLE:Countess"}},
				{name: "an organisation of two", set: map[string]string{"org": "Analytical Engines Ltd"}, changed: []string{"ORG"}, says: []string{"Analytical Engines Ltd", "ORG:Royal Society"}},
				{name: "the label of a phone", set: map[string]string{"tel.0.type": "home"}, changed: []string{"ITEM1.TEL"}, says: []string{"Desk"}},
				{name: "the number of a phone", set: map[string]string{"tel.0": "+44 20 9"}, changed: []string{"ITEM1.TEL"}, says: []string{"+44 20 9", "+44 20 2", "Desk"}},
				{name: "a note rewritten", set: map[string]string{"note": "Another"}, changed: []string{"NOTE"}, says: []string{"NOTE:Another"}},
			}},
		{name: "card beyond the seed", kind: view.KindContact, prime: map[string]string{"nickname": "Ada"},
			doc: func(uid string) string {
				return "BEGIN:VCARD\nVERSION:3.0\nUID:" + uid + "\nFN:Ada Lovelace\nN:Lovelace;Ada;Augusta;Countess;PhD\n" +
					"item2.EMAIL;TYPE=INTERNET,WORK,PREF:ada@example.com\nitem2.X-ABLabel:Office\nEMAIL;TYPE=INTERNET,HOME:ada@home.example\n" +
					"item3.ADR;TYPE=HOME:;Flat 2;12 St James's Square;London;;SW1Y 4JH;UK\nitem3.X-ABADR:gb\n" +
					"URL;TYPE=WORK:https://ada.example\nBDAY;VALUE=DATE:1815-12-10\nIMPP;X-SERVICE-TYPE=Skype:skype:ada\n" +
					"X-SOCIALPROFILE;TYPE=twitter:https://x.example/ada\nCATEGORIES:math,poetry\nX-CUSTOM:keep\nEND:VCARD\n"
			},
			cases: []matrixCase{
				{name: "a labelled mail", set: map[string]string{"email.0": "ada@new.example"}, changed: []string{"ITEM2.EMAIL"}, says: []string{"ada@new.example", "ada@home.example", "Office"}},
				{name: "the label of the second mail", set: map[string]string{"email.1.type": "WORK"}, changed: []string{"EMAIL"}, says: []string{"ada@home.example", "ada@example.com"}},
				{name: "the city", set: map[string]string{"adr.0.city": "Cambridge"}, changed: []string{"ITEM3.ADR"}, says: []string{"Cambridge", "Flat 2"}},
				{name: "the birthday", set: map[string]string{"bday": "11.12.1815"}, changed: []string{"BDAY"}},
				{name: "the web page", set: map[string]string{"url.0": "https://ada.example/new"}, changed: []string{"URL"}, says: []string{"https://ada.example/new"}},
				// U-6.1: the display name made of the name parts follows them.
				{name: "the given name", set: map[string]string{"n.given": "Augusta"}, changed: []string{"FN", "N"}, says: []string{"FN:Augusta Lovelace"}},
				{name: "one more tag", set: map[string]string{"categories": "math, poetry, engines"}, changed: []string{"CATEGORIES"}, says: []string{"engines"}},
			}},
		{name: "event by its length", kind: view.KindEvent, prime: map[string]string{"location": "Room 4"},
			doc: func(uid string) string {
				return matrixCal("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDURATION:PT1H\n" +
					"SUMMARY:Sync\nDESCRIPTION:Agenda\\n\nATTENDEE;CN=Room 4;CUTYPE=ROOM:urn:uuid:room-4\n" +
					"ATTENDEE;CN=Bob:mailto:bob@example.com\nRRULE:FREQ=WEEKLY;BYDAY=MO;WKST=SU\nEND:VEVENT")
			},
			cases: []matrixCase{
				{name: "the summary", set: map[string]string{"summary": "Sync, weekly"}, changed: []string{"SUMMARY"}},
				{name: "start and end an hour later", set: map[string]string{"start.time": "11:00", "end.time": "12:00"}, changed: []string{"DTSTART"}},
				{name: "the end alone", set: map[string]string{"end.time": "12:30"}, changed: []string{"DURATION"}},
				{name: "every second week", set: map[string]string{"repeat.interval": "2"}, changed: []string{"RRULE"}, says: []string{"INTERVAL=2"}},
			}},
		{name: "event of another client", kind: view.KindEvent, prime: map[string]string{"description": "primed"},
			doc: func(uid string) string {
				return matrixCal("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDTEND;TZID=Europe/Moscow:20260112T110000\n" +
					"SUMMARY:Sync\nLOCATION:Room\\; 4\nGEO:55.7;37.6\nSTATUS:TENTATIVE\nTRANSP:TRANSPARENT\nCLASS:CONFIDENTIAL\n" +
					"CATEGORIES:Smith\\, John,c\nURL:https://example.com/e\nSEQUENCE:4\nX-MICROSOFT-CDO-BUSYSTATUS:FREE\nEND:VEVENT")
			},
			cases: []matrixCase{
				{name: "the place", set: map[string]string{"location": "Room; 5"}, changed: []string{"LOCATION"}},
				{name: "one more tag", set: map[string]string{"categories": "Smith, John, c, d"}, changed: []string{"CATEGORIES"}},
				{name: "the status", set: map[string]string{"status": "CONFIRMED"}, changed: []string{"STATUS"}},
				{name: "the start", set: map[string]string{"start.time": "09:30"}, changed: []string{"DTSTART"}},
			}},
		{name: "meeting", kind: view.KindEvent, prime: map[string]string{"description": "primed"},
			doc: func(uid string) string {
				return matrixCal("BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDTEND;TZID=Europe/Moscow:20260112T110000\n" +
					"SUMMARY:Review\n" +
					"ATTENDEE;CN=Bob;PARTSTAT=ACCEPTED;ROLE=REQ-PARTICIPANT;RSVP=TRUE;LANGUAGE=en:mailto:bob@example.com\n" +
					"ATTENDEE;CN=Carol;PARTSTAT=DELEGATED;DELEGATED-TO=\"mailto:dan@example.com\":mailto:carol@example.com\n" +
					"ATTENDEE;CN=Dan;PARTSTAT=TENTATIVE;DELEGATED-FROM=\"mailto:carol@example.com\";X-NUM-GUESTS=1:mailto:dan@example.com\n" +
					"RRULE:FREQ=WEEKLY;BYDAY=MO\nEND:VEVENT")
			},
			cases: []matrixCase{
				{name: "one more attendee", set: map[string]string{"attendee": "mailto:bob@example.com\x00mailto:carol@example.com\x00mailto:dan@example.com", "attendee.add": "eve@example.com"}, changed: []string{"ATTENDEE"}, says: []string{"eve@example.com", "PARTSTAT=ACCEPTED", "X-NUM-GUESTS=1"}},
				// The drawn field posts its line to type into: without it
				// nobody is taken away. Carol is left out of the list.
				{name: "one attendee fewer", set: map[string]string{"attendee": "mailto:bob@example.com\x00mailto:dan@example.com", "attendee.add": ""},
					changed: []string{"ATTENDEE"}, says: []string{"PARTSTAT=ACCEPTED", "X-NUM-GUESTS=1"}, gone: []string{"CN=Carol"}},
				{name: "every second week", set: map[string]string{"repeat.interval": "2"}, changed: []string{"RRULE"}, says: []string{"INTERVAL=2", "PARTSTAT=ACCEPTED"}},
			}},
		{name: "task of another client", kind: view.KindTask, prime: map[string]string{"description": "Steps, primed"},
			doc: func(uid string) string {
				return matrixCal("BEGIN:VTODO\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:Ship\nDESCRIPTION:Steps\\n\\n\n" +
					"DTSTART;TZID=Europe/Moscow:20260110T090000\nDUE;TZID=Europe/Moscow:20260112T180000\n" +
					"PRIORITY:3\nPERCENT-COMPLETE:40\nSTATUS:IN-PROCESS\nCATEGORIES:Smith\\, John,urgent\nCATEGORIES:q1\nLOCATION:Dock\\, gate 4\n" +
					"RELATED-TO;RELTYPE=PARENT:parent-1\nRELATED-TO;RELTYPE=SIBLING:sib-1\nX-APPLE-SORT-ORDER:12\nURL:https://example.com/t\n" +
					"BEGIN:VALARM\nACTION:AUDIO\nTRIGGER;RELATED=END:-PT30M\nEND:VALARM\nEND:VTODO")
			},
			cases: []matrixCase{
				{name: "the summary", set: map[string]string{"summary": "Ship it"}, changed: []string{"SUMMARY"}},
				{name: "the place", set: map[string]string{"location": "Dock, gate 5"}, changed: []string{"LOCATION"}},
				{name: "the due time", set: map[string]string{"due.time": "16:00"}, changed: []string{"DUE"}},
				{name: "a tag taken away", set: map[string]string{"categories": "Smith, John, q1"}, changed: []string{"CATEGORIES"}},
				{name: "one relation fewer", set: map[string]string{"related": "parent-1"}, changed: []string{"RELATED-TO"}, gone: []string{"sib-1"}},
				{name: "the priority", set: map[string]string{"priority": "1"}, changed: []string{"PRIORITY"}},
				{name: "the percent", set: map[string]string{"percent": "60"}, changed: []string{"PERCENT-COMPLETE"}},
			}},
	}
}

// matrixList is the list of a kind in a collection: where its records and
// its new form live.
func matrixList(kind view.Kind, connID, col string) string {
	section := map[view.Kind]string{view.KindContact: "contacts", view.KindEvent: "calendar", view.KindTask: "tasks"}[kind]
	return "/app/" + section + "/" + connID + "/" + url.PathEscape(col)
}

func matrixExt(kind view.Kind) (string, string) {
	if kind == view.KindContact {
		return ".vcf", "text/vcard; charset=utf-8"
	}
	return ".ics", "text/calendar; charset=utf-8"
}

// matrixSave is liveEditDropping for any kind, with a field of several
// values given joined by NUL.
func matrixSave(t *testing.T, app *liveApp, kind view.Kind, rec string, set map[string]string) []string {
	t.Helper()
	// A record just written may be out of the server's REPORT for a moment
	// (SV-62, Yandex): the form is asked again while it answers 404.
	edit := app.get(rec + "/edit")
	for start := time.Now(); edit.Code == http.StatusNotFound && time.Since(start) < settleWait; {
		time.Sleep(500 * time.Millisecond)
		edit = app.get(rec + "/edit")
	}
	var v url.Values
	switch kind {
	case view.KindContact:
		v = liveForm(liveDecode[view.ContactEditPage](t, "edit "+rec, edit).Form)
	case view.KindEvent:
		v = liveForm(liveDecode[view.EventEditPage](t, "edit "+rec, edit).Form)
	default:
		v = liveForm(liveDecode[view.TaskEditPage](t, "edit "+rec, edit).Form)
	}
	for k, val := range set {
		v[k] = strings.Split(val, "\x00")
	}
	rr := app.post(rec+"/edit", v)
	if rr.Code == http.StatusOK {
		var body struct {
			Loss *view.LossReport `json:"loss"`
		}
		var lost []string
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err == nil && body.Loss != nil {
			for _, p := range body.Loss.Properties {
				lost = append(lost, strings.ToUpper(string(p)))
			}
		}
		if body.Loss == nil {
			t.Fatalf("save %v: answered with the form, no loss report:\n%s", set, rr.Body.String())
		}
		t.Logf("save %v: the server kept everything but %v", set, lost)
		return lost
	}
	// A mail address changed is saved, and the screen offers to rewrite
	// the events that name the old one (S-15.4): the save stands.
	loc := redirected(t, "save", rr)
	if strings.HasPrefix(loc, rec+"/conflict") {
		// The record changed on the server between the form and the save:
		// Carrel asks, as it should. Yandex changes a record it was just
		// handed by itself (seen on the matrix of 16.11.2026, once in two
		// runs); the case cannot be measured then.
		t.Skipf("save %v: the record changed on the server under the form, Carrel asks (%s)", set, loc)
	}
	if loc != rec && !strings.HasPrefix(loc, rec+"/rewrite?") {
		t.Fatalf("save %v went to %q", set, loc)
	}
	return nil
}

// matrixLines are the content lines of the record itself — the card, or the
// event or task and what it holds (an alarm) —, unfolded: a time zone the
// server writes beside it is not the record.
func matrixLines(doc string) []string {
	var out []string
	depth, in := 0, false
	for _, l := range strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n") {
		u := strings.ToUpper(l)
		switch {
		case u == "BEGIN:VCARD" || u == "BEGIN:VEVENT" || u == "BEGIN:VTODO":
			if !in {
				in, depth = true, 0
			}
		case strings.HasPrefix(u, "BEGIN:") && in:
			depth++
		case strings.HasPrefix(u, "END:") && in:
			if depth == 0 {
				in = false
				continue
			}
			depth--
		}
		if in && l != "" {
			out = append(out, l)
		}
	}
	return out
}

// bareName is a property's name without its group, upper case: Google
// takes the groups of a card apart and puts some back its own way
// («item1.TEL» comes back «TEL», «URL» as «item2.URL»), and a line is the
// field's whatever group the server gave it.
func bareName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return strings.ToUpper(name)
}

// count is how many lines of the record itself in doc are of prop.
func count(doc, prop string) int {
	n := 0
	for _, l := range matrixLines(doc) {
		if i := strings.IndexAny(l, ";:"); i >= 0 && bareName(l[:i]) == prop {
			n++
		}
	}
	return n
}

// lineOf is the first line of doc whose property is prop.
func lineOf(doc, prop string) string {
	for _, l := range strings.Split(doc, "\n") {
		if u := strings.ToUpper(l); strings.HasPrefix(u, prop+":") || strings.HasPrefix(u, prop+";") {
			return l
		}
	}
	return ""
}

// matrixChanged names the properties whose lines differ between before and
// after, upper case, those a server writes itself on every save left out.
func matrixChanged(before, after string) []string {
	count := map[string]int{}
	for _, l := range matrixLines(after) {
		count[l]++
	}
	names := map[string]bool{}
	name := func(l string) string {
		if i := strings.IndexAny(l, ";:"); i >= 0 {
			l = l[:i]
		}
		return bareName(l)
	}
	for _, l := range matrixLines(before) {
		if count[l] > 0 {
			count[l]--
			continue
		}
		names[name(l)] = true
	}
	for l, n := range count {
		if n > 0 {
			names[name(l)] = true
		}
	}
	var out []string
	for n := range names {
		switch n {
		// The server's own stamps (model.isServerStamp; Mail.ru's
		// X-MOZ-* — SV-36).
		case "DTSTAMP", "LAST-MODIFIED", "REV", "PRODID", "SEQUENCE", "CREATED", "X-MOZ-LASTACK", "X-MOZ-GENERATION", "X-MOZ-SNOOZE-TIME", "":
		default:
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}
