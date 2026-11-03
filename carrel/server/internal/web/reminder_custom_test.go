package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Typecase's task 156 (s-agenda-19; the owner's decision of 12.10.2026): a
// reminder of its own. «Custom…» stands in the list after the steps and
// shows how long before and in what; a whole day's also at what hour. A
// reminder another client wrote at an offset none of the steps is comes into
// the form as «Custom…» with its numbers, not as one more item; on the card
// it is words. The numbers are read only while «Custom…» is chosen, and
// numbers that make no reminder bring the form back with them open.
func TestCustomReminder(t *testing.T) {
	f := calendarFixture(t, nil,
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\n"+
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:x\nEND:VALARM\n"+
			"BEGIN:VALARM\nACTION:AUDIO\nTRIGGER:-P2D\nEND:VALARM\nEND:VEVENT"),
		"day.ics", ics("BEGIN:VEVENT\nUID:day\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART;VALUE=DATE:20260120\nDTEND;VALUE=DATE:20260121\nSUMMARY:Holiday\n"+
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT15H\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))

	// A step: «Custom…» is offered, its fields stand hidden.
	step := rowNamed(t, page.Form, "reminder.0")
	if step.Value != "-PT10M" || customOption(step) == nil || customOption(step).Selected ||
		customOption(step).Shows != "reminder.0.count reminder.0.unit" {
		t.Errorf("a step's row = %+v", step)
	}
	if !rowNamed(t, page.Form, "reminder.0.count").Hidden || !rowNamed(t, page.Form, "reminder.0.unit").Hidden || hasRow(page.Form, "reminder.0.at") {
		t.Errorf("a step's numbers are not hidden, or a timed event has At")
	}
	// Two days before: «Custom…» chosen, 2, days, the fields open; no item
	// of its own in the list.
	own := rowNamed(t, page.Form, "reminder.1")
	if own.Value != "custom" || !customOption(own).Selected || len(own.Options) != 8 {
		t.Errorf("another client's reminder = %+v", own)
	}
	if c, u := rowNamed(t, page.Form, "reminder.1.count"), rowNamed(t, page.Form, "reminder.1.unit"); c.Value != "2" || c.Hidden || u.Value != "days" || u.Hidden {
		t.Errorf("its numbers: %+v, %+v", c, u)
	}

	// A whole day: the day before at 09:00, and the card says so.
	day := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/day/edit", f.sess)))
	if c, a := rowNamed(t, day.Form, "reminder.0.count"), rowNamed(t, day.Form, "reminder.0.at"); c.Value != "1" || a.Value != "09:00" || a.Hidden ||
		customOption(rowNamed(t, day.Form, "reminder.0")).Shows != "reminder.0.count reminder.0.unit reminder.0.at" {
		t.Errorf("a whole day's own reminder: %+v, %+v", c, a)
	}
	card := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/day", f.sess)))
	if got := fieldValue(card.Fields, "Reminders"); got != "1 day before, 09:00" {
		t.Errorf("the card says %q, want «1 day before, 09:00»", got)
	}

	// The step changed to «Custom…», 3 hours: written. The other client's
	// reminder keeps what it does (ACTION:AUDIO).
	form := formFields(page.Form)
	form.Set("reminder.0", "custom")
	form.Set("reminder.0.count", "3")
	form.Set("reminder.0.unit", "hours")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "meet.ics")
	mustContain(t, got, "TRIGGER:-PT3H", "TRIGGER:-P2D", "ACTION:AUDIO")
	if strings.Contains(got, "-PT10M") {
		t.Errorf("the step is still there:\n%s", got)
	}

	// Without a script: «Custom…» chosen on a row of a step, its numbers
	// sent hidden and empty — nothing is written, the form comes back with
	// them open and says why.
	page = decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	puts := f.srv.CountRequests("PUT", f.col)
	form = formFields(page.Form)
	form.Set("add", "reminder")
	more := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	form = formFields(more.Form)
	form.Set("reminder.2", "custom")
	back := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("an unfinished «Custom…» wrote %d record(s)", n)
	}
	if r := rowNamed(t, back.Form, "reminder.2"); r.Value != "custom" || !customOption(r).Selected || rowNamed(t, back.Form, "reminder.2.count").Hidden {
		t.Errorf("the row came back as %+v", r)
	}
	// The reason stands under the row of «Custom…», not under the first.
	if got := rowNamed(t, back.Form, "reminder.2").Error; got != "How long: enter a whole number, 0 or more." {
		t.Errorf("the row's reason = %q (all: %q)", got, formErrors(back.Form))
	}
	if got := rowNamed(t, back.Form, "reminder.0").Error; got != "" {
		t.Errorf("the first row says %q, though its reminder is a step", got)
	}
}

func customOption(r view.FormRow) *view.Option {
	for i, o := range r.Options {
		if o.Value == "custom" && o.Label == "Custom…" {
			return &r.Options[i]
		}
	}
	return nil
}

func fieldValue(fs []view.Field, label string) string {
	for _, f := range fs {
		if string(f.Label) == label {
			return string(f.Value)
		}
	}
	return ""
}

func formErrors(f view.Form) string {
	var out []string
	for _, e := range f.Errors {
		out = append(out, string(e.Message))
	}
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Error != "" {
				out = append(out, string(r.Error))
			}
		}
	}
	return strings.Join(out, "\n")
}
