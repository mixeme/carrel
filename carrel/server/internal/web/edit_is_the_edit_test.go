package web_test

import (
	"net/url"
	"slices"
	"sort"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/web"
	"carrel.dev/carrel/server/view"
)

// S-7.3, the owner's decision of 11.10.2026: a save is the edit and nothing
// else. The patch model keeps what a patch does not name and the re-read
// says what the server lost; what neither saw is a patch that is itself not
// what the person did — the audit of 11.10.2026 found such places by reading
// (docs/plan/scans/silent-choices.md, group Б). Here they are found by
// saving: a record another client could have written, a form opened and one
// thing changed in it, and the lines of the record before and after.

// editedForm is what every edit page answers with.
type editedForm struct {
	Notices []view.Notice `json:"notices"`
	Form    view.Form     `json:"form"`
}

// changedLines are the names of the content lines that differ between two
// bodies, each once, the stamps of a write aside.
func changedLines(before, after string) []string {
	count := map[string]int{}
	lines := func(doc string) []string {
		return strings.Split(strings.ReplaceAll(unfolded(doc), "\r\n", "\n"), "\n")
	}
	for _, l := range lines(after) {
		count[l]++
	}
	names := map[string]bool{}
	name := func(l string) string {
		if i := strings.IndexAny(l, ";:"); i >= 0 {
			l = l[:i]
		}
		return l
	}
	for _, l := range lines(before) {
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
		case "DTSTAMP", "LAST-MODIFIED", "REV", "":
		default:
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// oneEdit opens the form of rec, sets what set names, saves, and answers
// with the record's body before and after. A save that is refused is a
// failure of the test. The stand hands a seeded record out in its own
// spelling (go-vcard's, go-ical's), so the record is first saved once with
// prime — a field no case edits — and «before» is what Carrel wrote then:
// the lines as it read them.
func oneEdit(t *testing.T, f davFixture, rec, file string, prime, set url.Values) (before, after string) {
	t.Helper()
	save := func(set url.Values) string {
		t.Helper()
		form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form[k] = v
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, file)
	}
	before = save(prime)
	return before, save(set)
}

const hardCard = "BEGIN:VCARD\nVERSION:3.0\nUID:hard\nFN:Ada Lovelace\nN:Lovelace;Ada;Augusta;Countess;PhD\n" +
	"NICKNAME:Ada\nORG:Analytical;Engines\nORG:Royal Society\nTITLE:Mathematician\nTITLE:Countess\n" +
	"item1.TEL;TYPE=WORK,CELL,PREF:+44 20 1\nitem1.X-ABLabel:Desk\nTEL;TYPE=HOME:+44 20 2\n" +
	"EMAIL;TYPE=INTERNET,WORK:ada@example.com\nNOTE:First line\\nsecond line\\n\\n\nCATEGORIES:math,poetry\nX-CUSTOM:keep\nEND:VCARD\n"

// A card: every edit of one field changes the lines of that field and no
// other, and of a property that stands twice — the value the form showed.
func TestACardSaveIsTheEdit(t *testing.T) {
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
	}{
		// Б12: the note ends in line ends the form does not post back.
		{"another field than the note", url.Values{"nickname": {"Countess of Lovelace"}}, []string{"NICKNAME"},
			[]string{"NOTE:First line\\nsecond line\\n\\n"}},
		// Б4: the second TITLE and the second ORG are not shown and stay.
		{"a title of two", url.Values{"title": {"Analyst"}}, []string{"TITLE"}, []string{"TITLE:Analyst", "TITLE:Countess"}},
		{"an organisation of two", url.Values{"org": {"Analytical Engines Ltd"}}, []string{"ORG"},
			[]string{"ORG:Analytical Engines Ltd", "ORG:Royal Society"}},
		// Б10: a label chosen replaces the label shown, and what stands
		// beside it stays; the group ties Apple's own label to the line.
		{"the label of a phone", url.Values{"tel.0.type": {"home"}}, []string{"item1.TEL"},
			[]string{"item1.TEL;TYPE=home,CELL,PREF:+44 20 1", "item1.X-ABLABEL:Desk"}},
		{"the number of a phone", url.Values{"tel.0": {"+44 20 9"}}, []string{"item1.TEL"},
			[]string{"item1.TEL;TYPE=WORK,CELL,PREF:+44 20 9", "item1.X-ABLABEL:Desk", "TEL;TYPE=HOME:+44 20 2"}},
		{"a note rewritten", url.Values{"note": {"Another"}}, []string{"NOTE"}, []string{"NOTE:Another"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := contactsFixture(t, false, hardCard)
			before, after := oneEdit(t, f, f.path("contacts")+"/hard", "hard.vcf", url.Values{"bday": {"10.12.1815"}}, tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, tc.holds...)
		})
	}
}

// An event written by its length, with a room among its attendees and a
// description that ends in a line end.
func hardEvent() string {
	return ics("BEGIN:VEVENT\nUID:hard\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDURATION:PT1H\n" +
		"SUMMARY:Sync\nDESCRIPTION:Agenda\\n\nATTENDEE;CN=Room 4;CUTYPE=ROOM:urn:uuid:room-4\n" +
		"ATTENDEE;CN=Bob:mailto:bob@example.com\nRRULE:FREQ=WEEKLY;BYDAY=MO;WKST=SU\nEND:VEVENT")
}

func TestAnEventSaveIsTheEdit(t *testing.T) {
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
	}{
		{"the summary", url.Values{"summary": {"Sync, weekly"}}, []string{"SUMMARY"}, []string{"DESCRIPTION:Agenda\\n", "DURATION:PT1H"}},
		// S-11: DURATION stays DURATION. Moved whole, the event keeps its
		// length and the line is not written; an end alone is a new length.
		{"start and end an hour later", url.Values{"start.time": {"11:00"}, "end.time": {"12:00"}}, []string{"DTSTART"},
			[]string{"DTSTART;TZID=Europe/Moscow:20260112T110000", "DURATION:PT1H"}},
		{"the end alone", url.Values{"end.time": {"12:30"}}, []string{"DURATION"}, []string{"DURATION:PT2H30M"}},
		// Б5: the room is no mail address, so the field does not show it —
		// and a list changed by the field does not take it away.
		{"one more attendee", url.Values{"attendee": {"mailto:bob@example.com"}, "attendee.add": {"carol@example.com"}}, []string{"ATTENDEE"},
			[]string{"ATTENDEE;CN=Room 4;CUTYPE=ROOM:urn:uuid:room-4", "ATTENDEE;CN=Bob:mailto:bob@example.com", "mailto:carol@example.com"}},
		// What the simple fields do not show of the rule stays in it.
		{"every second week", url.Values{"repeat.interval": {"2"}}, []string{"RRULE"}, []string{"RRULE:FREQ=WEEKLY;BYDAY=MO;WKST=SU;INTERVAL=2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "hard.ics", hardEvent())
			before, after := oneEdit(t, f, f.path("calendar")+"/hard", "hard.ics", url.Values{"location": {"Room 4"}}, tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, tc.holds...)
			if strings.Contains(after, "DTEND") {
				t.Errorf("an event written by its length gained an end:\n%s", after)
			}
		})
	}
}

// Б18: Done and Status are one state of a task. Changed together they have
// to agree; a save that says both ways writes neither, and says why.
func TestDoneAndStatusAreOneState(t *testing.T) {
	task := ics("BEGIN:VTODO\nUID:t\nDTSTAMP:20260101T000000Z\nSUMMARY:Ship\nSTATUS:COMPLETED\nCOMPLETED:20260105T100000Z\nEND:VTODO")
	rec := "/t"

	f := calendarFixture(t, nil, "t.ics", task)
	_, after := oneEdit(t, f, f.path("tasks")+rec, "t.ics", url.Values{"location": {"Desk"}}, url.Values{"done": {""}, "status": {"IN-PROCESS"}})
	mustContain(t, after, "STATUS:IN-PROCESS")
	if strings.Contains(after, "COMPLETED:") {
		t.Errorf("a task not done keeps the day it was completed:\n%s", after)
	}

	f = calendarFixture(t, nil, "t.ics", task)
	_, after = oneEdit(t, f, f.path("tasks")+rec, "t.ics", url.Values{"location": {"Desk"}}, url.Values{"status": {"CANCELLED"}})
	mustContain(t, after, "STATUS:CANCELLED")
	if strings.Contains(after, "COMPLETED:") {
		t.Errorf("a cancelled task is still done:\n%s", after)
	}

	// Under Status, what does not agree, by the status chosen, and the two
	// ways out (the owner's words of 02.11.2026). The box is read from the
	// status, so the other way round — Done cleared, Completed chosen — is a
	// status the person did not change, and Done decides.
	for _, status := range []struct{ value, label string }{{"IN-PROCESS", "In process"}, {"CANCELLED", "Cancelled"}} {
		f = calendarFixture(t, nil, "t.ics", strings.Replace(task, "STATUS:COMPLETED\nCOMPLETED:20260105T100000Z\n", "", 1))
		before := body(t, f, "t.ics")
		form := formFields(decode[editedForm](t, f.h.do(get(f.path("tasks")+rec+"/edit", f.sess))).Form)
		form.Set("done", "on")
		form.Set("status", status.value)
		page := decode[editedForm](t, f.h.do(postForm(f.path("tasks")+rec+"/edit", f.sess, form)))
		said := ""
		for _, set := range page.Form.Sets {
			for _, row := range set.Rows {
				if row.Name == "status" {
					said = string(row.Error)
				}
			}
		}
		if want := "Done is ticked, but the status is " + status.label + ". Untick Done, or set the status to Completed."; said != want {
			t.Errorf("under Status: %q,\nwant %q", said, want)
		}
		if got := body(t, f, "t.ics"); got != before {
			t.Errorf("a save that says both ways was written:\n%s", got)
		}
	}
}

// A patch that is not the edit is not sent, and the answer says the fault is
// Carrel's (the owner's decision of 11.10.2026: «Не записывать. Показывать
// ошибку Carrel»). The patch is spoiled on its way the only way a test can:
// the person changes the nickname, and the save would take the title too.
func TestASaveThatIsNotTheEditIsNotSent(t *testing.T) {
	f := contactsFixture(t, false, hardCard)
	rec := f.path("contacts") + "/hard"
	before := body(t, f, "hard.vcf")
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("nickname", "Countess of Lovelace")

	restore := web.TamperEdit(model.RemoveProp{Prop: "TITLE"})
	page := decode[editedForm](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	restore()

	if got := body(t, f, "hard.vcf"); got != before {
		t.Fatalf("the save was sent:\n%s", got)
	}
	// The owner's words of 02.11.2026: an internal error of Carrel, said so,
	// what went wrong by the property's name, nothing sent, report it.
	notice := faultOf(t, page.Notices)
	if want := "Saving would have stored the field Title differently from what you entered, and would have changed the property TITLE, which you did not touch. " + faultTail; string(notice.Body) != want {
		t.Errorf("body = %q,\nwant %q", notice.Body, want)
	}
	// The form keeps what was entered, and the record under it is the one
	// the server holds: saved again without the fault, the edit goes through.
	form = formFields(page.Form)
	if form.Get("nickname") != "Countess of Lovelace" || form.Get("title") != "Mathematician" {
		t.Errorf("form after the fault: nickname %q, title %q", form.Get("nickname"), form.Get("title"))
	}
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("saved again: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "hard.vcf"), "NICKNAME:Countess of Lovelace", "TITLE:Mathematician", "TITLE:Countess")
}

// The same for a record that would not read back as entered: the person
// asks for one summary and the patch writes another.
func TestASaveThatDoesNotReadBackIsNotSent(t *testing.T) {
	f := calendarFixture(t, nil, "hard.ics", hardEvent())
	rec := f.path("calendar") + "/hard"
	before := body(t, f, "hard.ics")
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("summary", "Sync, weekly")

	restore := web.TamperEdit(model.SetTextAt{Prop: "SUMMARY", Index: 0, Value: "Something else"})
	page := decode[editedForm](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	restore()

	if got := body(t, f, "hard.ics"); got != before {
		t.Fatalf("the save was sent:\n%s", got)
	}
	// What went wrong by the field's name, as the form calls it.
	notice := faultOf(t, page.Notices)
	if !strings.HasPrefix(string(notice.Body), "Saving would have stored the field Summary differently from what you entered") ||
		!strings.HasSuffix(string(notice.Body), faultTail) {
		t.Errorf("body = %q", notice.Body)
	}
}

// faultTail is what every fault notice ends with: what became of the record
// and the entry, and what to do.
const faultTail = "Nothing was sent; the record is unchanged and the form keeps your entry. Please report this with the details below."

// faultOf is the notice of an internal error of Carrel among notices.
func faultOf(t *testing.T, notices []view.Notice) view.Notice {
	t.Helper()
	for _, n := range notices {
		if n.Title == "Internal Carrel error — the edit was not saved" {
			if n.Role != view.NoticeAlert {
				t.Errorf("the fault is no alarm: %+v", n)
			}
			return n
		}
	}
	t.Fatalf("notices = %+v", notices)
	return view.Notice{}
}
