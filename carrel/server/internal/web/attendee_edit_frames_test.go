package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// frameTagField is the attendees' field of an edit as a frame draws it: the
// address of its suggestions, the name and value of each tag — «fixed» for
// the organiser's hidden one —, the line typed in and the hint under it.
type frameTagField struct {
	suggest, add, placeholder, label, hint string
	tags                                   []string // «attendee=mailto:…», «fixed attendee=mailto:…»
}

func frameAttendeeField(t *testing.T, id string) frameTagField {
	t.Helper()
	frame := mockupFrame(t, id)
	var f frameTagField
	m := regexp.MustCompile(`<span class="tc-in tc-in--tags tc-in--cols" data-tc-suggest="([^"]*)">`).FindStringSubmatch(frame)
	if m == nil {
		t.Fatalf("%s draws no field of attendees: read the frame again", id)
	}
	f.suggest = m[1]
	for _, tag := range regexp.MustCompile(`<input( class="tc-check__box")? type="(hidden|checkbox)" name="([^"]*)" value="([^"]*)"( checked)?>`).FindAllStringSubmatch(frame, -1) {
		fixed := ""
		if tag[2] == "hidden" {
			fixed = "fixed "
		}
		f.tags = append(f.tags, fixed+tag[3]+"="+tag[4])
	}
	add := regexp.MustCompile(`<input class="tc-in__add" type="text" name="([^"]*)"(?: value="[^"]*")? placeholder="([^"]*)" aria-label="([^"]*)"`).FindStringSubmatch(frame)
	if add == nil {
		t.Fatalf("%s draws no line to type in: read the frame again", id)
	}
	f.add, f.placeholder, f.label = add[1], add[2], add[3]
	if h := regexp.MustCompile(`<span class="tc-field__hint">([^<]*)</span>`).FindStringSubmatch(frame); h != nil {
		f.hint = h[1]
	}
	return f
}

func pageTags(a view.AttendeeField) []string {
	var out []string
	for _, r := range a.Rows {
		fixed := ""
		if r.Fixed {
			fixed = "fixed "
		}
		out = append(out, fixed+a.Name+"="+r.Value)
	}
	return out
}

// The attendees of an edit as task 77 draws them (s-agenda-3, -14, -15; the
// owner's decisions of 01–02.10.2026): the organiser edits them — a field of
// tags in columns, the organiser's fixed and posted hidden, each other a
// ticked box under the same name; a line to type a name or an address in,
// whose suggestions are asked at the address the field names; the hint says
// whether the server sends the invitations. Without scripts the typed words
// go with the form, and the server decides: a person found by the name in
// the ticked books, an address as it is, else an error at the field. An
// invited person sees the tags and why they cannot change them. A form that
// does not draw the field changes no attendee.
func TestAttendeesEditIsTheFrames(t *testing.T) {
	frame := frameAttendeeField(t, "s-agenda-3")
	if len(frame.tags) != 4 || !strings.HasPrefix(frame.tags[0], "fixed attendee=mailto:") || strings.HasPrefix(frame.tags[1], "fixed") {
		t.Fatalf("s-agenda-3's tags = %q: read the frame again", frame.tags)
	}
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")
	rec := list + "/8f1c-4a02-b7d3"
	stored := func() string {
		body, _, _ := app.servers["nextcloud"].Object(davtest.CalendarHome + "lichnyj/8f1c-4a02-b7d3.ics")
		return unfolded(body)
	}

	page := decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess)))
	a := page.Attendees
	want := []string{"fixed attendee=mailto:mix@example.org", "attendee=mailto:a.kovaleva@example.org", "attendee=mailto:a.belov@example.org", "attendee=mailto:v@example.org"}
	if got := pageTags(a); !slices.Equal(got, want) {
		t.Errorf("tags = %q, want %q (as s-agenda-3 draws %q)", got, want, frame.tags)
	}
	if a.Add == nil || a.Add.Name != frame.add || string(a.Add.Placeholder) != frame.placeholder || string(a.Add.Label) != frame.label ||
		a.Suggest != frame.suggest || string(a.Hint) != frame.hint || a.ReadOnly {
		t.Errorf("field = %+v %+v, want %+v", a, a.Add, frame)
	}
	if a.Rows[1].Status != "accepted" || a.Rows[1].Href == "" || a.Rows[3].Href != "" {
		t.Errorf("rows = %+v", a.Rows)
	}

	save := func(attendees []string, typed string) *http.Response {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
		form["attendee"] = attendees
		form.Set("attendee.add", typed)
		return app.h.do(postForm(rec+"/edit", app.sess, form)).Result()
	}
	// Артём unticked, Вера added by her name as «Архив» writes it: since
	// Typecase's note 44 the set holds her twice — «Лебедева Вера» of
	// «Контакты» is the other card —, and «Вера» alone is two people.
	if resp := save([]string{"mailto:mix@example.org", "mailto:a.kovaleva@example.org", "mailto:v@example.org"}, "Вера Лебедева"); resp.StatusCode != 303 {
		t.Fatalf("save: %d", resp.StatusCode)
	}
	got := stored()
	mustContain(t, got, "ORGANIZER;CN=Михаил Енученко:mailto:mix@example.org", "mailto:a.kovaleva@example.org", "mailto:v@example.org",
		"ATTENDEE;CN=Вера Лебедева;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:vera@example.net")
	if strings.Contains(got, "a.belov@example.org") {
		t.Errorf("Артём is still invited:\n%s", got)
	}
	// An address no book holds, as typed.
	if resp := save([]string{"mailto:mix@example.org", "mailto:a.kovaleva@example.org", "mailto:v@example.org", "mailto:vera@example.net"}, "Ivan.Petrov@mail.example"); resp.StatusCode != 303 {
		t.Fatalf("save an address: %d", resp.StatusCode)
	}
	mustContain(t, stored(), "ATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:ivan.petrov@mail.example")
	// A name no book holds: the form back with the error at the line.
	before := stored()
	form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	form["attendee"] = []string{"mailto:mix@example.org"}
	form.Set("attendee.add", "Nobody")
	bad := decode[view.EventEditPage](t, app.h.do(postForm(rec+"/edit", app.sess, form)))
	if bad.Attendees.Add == nil || bad.Attendees.Add.Error == "" || bad.Attendees.Add.Value != "Nobody" || stored() != before {
		t.Errorf("a name no book holds: %+v, stored changed: %v", bad.Attendees.Add, stored() != before)
	}
	// A form that does not draw the field leaves the attendees be.
	plain := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	plain.Set("summary", "Синк-ап по релизу ·")
	if resp := app.h.do(postForm(rec+"/edit", app.sess, plain)); resp.Code != 303 {
		t.Fatalf("plain save: %d", resp.Code)
	}
	mustContain(t, stored(), "mailto:ivan.petrov@mail.example", "mailto:vera@example.net")

	// s-agenda-15: a new event's field is empty, the line and the hint as
	// the edit's.
	fresh := decode[view.EventEditPage](t, app.h.do(get(list+"/new", app.sess)))
	newFrame := frameAttendeeField(t, "s-agenda-15")
	if len(fresh.Attendees.Rows) != 0 || len(newFrame.tags) != 0 || fresh.Attendees.Add == nil || string(fresh.Attendees.Hint) != newFrame.hint {
		t.Errorf("new event's field = %+v, want s-agenda-15's %+v", fresh.Attendees, newFrame)
	}

	// s-agenda-14: invited, the tags without crosses and why.
	invited := regexp.MustCompile(`<span class="tc-field__hint">You are invited by ([^:<]+): only the organiser changes the list\.</span>`).FindStringSubmatch(mockupFrame(t, "s-agenda-14"))
	if invited == nil {
		t.Fatal("s-agenda-14 says no reason: read the frame again")
	}
	app.servers["nextcloud"].Seed(davtest.CalendarHome+"lichnyj/guest.ics", ics("BEGIN:VEVENT\nUID:guest\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART:20260811T100000Z\nDTEND:20260811T110000Z\nSUMMARY:Review\n"+
		"ORGANIZER;CN=Мария Лебедева:mailto:m.lebedeva@example.org\n"+
		"ATTENDEE;CN=Михаил Енученко;PARTSTAT=ACCEPTED:mailto:mix@example.org\nEND:VEVENT"))
	guest := decode[view.EventEditPage](t, app.h.do(get(list+"/guest/edit", app.sess)))
	if !guest.Attendees.ReadOnly || guest.Attendees.Add != nil || string(guest.Attendees.Hint) != "You are invited by Мария Лебедева: only the organiser changes the list." {
		t.Errorf("an invited person's field = %+v (s-agenda-14 names %q)", guest.Attendees, invited[1])
	}
	gform := formFields(guest.Form)
	gform["attendee"] = []string{"mailto:m.lebedeva@example.org"}
	gform.Set("attendee.add", "someone@example.org")
	if resp := app.h.do(postForm(list+"/guest/edit", app.sess, gform)); resp.Code != 303 {
		t.Fatalf("an invited person's save: %d %s", resp.Code, resp.Body.String())
	}
	body, _, _ := app.servers["nextcloud"].Object(davtest.CalendarHome + "lichnyj/guest.ics")
	if strings.Contains(body, "someone@example.org") || !strings.Contains(unfolded(body), "mailto:mix@example.org") {
		t.Errorf("an invited person changed the list:\n%s", body)
	}
}

// The suggestions of the attendees' field (s-agenda-12, -13): what is typed
// is looked for in the ticked address books the session has read, by the
// name and by the address; a row for each address of a person, the name
// with what was found in bold, the address, the book and its mark, and the
// value of the tag it adds. An address no book holds is offered first as
// «Add», with «Not in the ticked address books.»
func TestAttendeeSuggestionsAreTheFrames(t *testing.T) {
	s12 := mockupFrame(t, "s-agenda-12")
	if !regexp.MustCompile(`<span class="tc-menu__label"><b>Ве</b>ра Лебедева</span><span class="tc-mono[^"]*">vera@example.net</span><span class="tc-mono tc-push[^"]*">Архив</span>`).MatchString(s12) {
		t.Fatal("s-agenda-12 does not suggest Вера as it did: read the frame again")
	}
	s13 := mockupFrame(t, "s-agenda-13")
	if !strings.Contains(s13, `Add<span class="tc-mono tc-mono--t11">ivan.petrov@mail.example</span>`) || !strings.Contains(s13, "Not in the ticked address books.") {
		t.Fatal("s-agenda-13 does not offer the address: read the frame again")
	}
	app := newMockupApp(t)
	suggest := func(q string) view.SuggestList {
		t.Helper()
		return decode[view.SuggestList](t, app.h.do(get("/app/contacts/suggest?q="+url.QueryEscape(q), app.sess)))
	}
	if got := suggest("ве"); len(got.Items) != 0 {
		t.Errorf("suggested before any book was read: %+v", got)
	}
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	got := suggest("ве")
	// The set holds Вера twice since Typecase's note 44 — «Вера Лебедева» of
	// «Архив», the row s-agenda-12 draws, and «Лебедева Вера» of «Контакты»,
	// which the frame does not draw yet (to-typecase.md, 150).
	if len(got.Items) != 2 || got.Note != "" || got.Items[1].Email != "v.lebedeva@example.org" || got.Items[1].Book != "Контакты" {
		t.Fatalf("«ве» = %+v, want Вера of «Архив» and her other card of «Контакты»", got)
	}
	vera := got.Items[0]
	if runsText(vera.Name) != "Вера Лебедева" || len(vera.Name) != 2 || vera.Name[0].Text != "Ве" || vera.Name[0].Mark != view.RunStrong ||
		vera.Email != "vera@example.net" || vera.Book != "Архив" || !strings.EqualFold(string(vera.Colour), "#A05A2C") ||
		vera.Value != "mailto:vera@example.net" || vera.Add {
		t.Errorf("Вера = %+v", vera)
	}
	if got := suggest("A.Kovaleva@example.org"); len(got.Items) != 1 || got.Items[0].Add || runsText(got.Items[0].Name) != "Анна Ковалёва" || got.Note != "" {
		t.Errorf("an address a book holds = %+v", got)
	}
	ivan := suggest("ivan.petrov@mail.example")
	if len(ivan.Items) != 1 || !ivan.Items[0].Add || ivan.Items[0].Email != "ivan.petrov@mail.example" || ivan.Items[0].Value != "mailto:ivan.petrov@mail.example" ||
		ivan.Note != "Not in the ticked address books." {
		t.Errorf("an address no book holds = %+v", ivan)
	}
	if got := suggest(""); len(got.Items) != 0 {
		t.Errorf("nothing typed = %+v", got)
	}
	if resp := app.h.do(get("/app/contacts/suggest?q=ve", nil)); resp.Code == http.StatusOK {
		t.Errorf("suggestions without a session: %d", resp.Code)
	}
}
