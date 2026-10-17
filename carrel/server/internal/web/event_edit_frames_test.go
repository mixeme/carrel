package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// formRowOf is the row of a form under a name.
func formRowOf(f view.Form, name string) *view.FormRow {
	for i := range f.Sets {
		for j := range f.Sets[i].Rows {
			if f.Sets[i].Rows[j].Name == name {
				return &f.Sets[i].Rows[j]
			}
		}
	}
	return nil
}

// The form of an event as s-agenda-3 draws it (the client's scan of the
// calendar, group Д, and Г11): the head names the event and its calendar;
// no hints the frame does not draw; Advanced RRULE empty with the rule as
// its sample, and typed only when it is to win; «as sent» by the zone the
// event came with; the reminders are rows of the form; the properties
// Carrel does not know are shown as kept; a new event has its form.
func TestEventEditByItsFrame(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "calendar", "lichnyj")
	rec := list + "/8f1c-4a02-b7d3"
	page := decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess)))

	// The edit does not name its calendar under the title: the choice is in
	// the head (Typecase's note 72, s-agenda-3).
	if page.Name != "Синк-ап по релизу" || page.Subtitle != "" {
		t.Errorf("head = %q, %q", page.Name, page.Subtitle)
	}
	for _, name := range []string{"categories", "repeat.count"} {
		if r := formRowOf(page.Form, name); r == nil || r.Hint != "" {
			t.Errorf("%s = %+v, want no hint", name, r)
		}
	}
	rule := formRowOf(page.Form, "repeat.rule")
	if rule == nil || rule.Value != "" || rule.Placeholder != "FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z" ||
		rule.Hint != "The advanced value takes precedence when set." {
		t.Errorf("Advanced RRULE = %+v", rule)
	}
	if tz := formRowOf(page.Form, "tz"); tz == nil || tz.Value != "Asia/Yekaterinburg" || tz.ValueAside != "as sent" {
		t.Errorf("Time zone = %+v", tz)
	}
	if r := formRowOf(page.Form, "reminder.0"); r == nil || r.Value != "-PT10M" {
		t.Errorf("the first reminder = %+v", r)
	}
	var kept []string
	for _, f := range page.Foreign {
		kept = append(kept, string(f.Label)+" "+string(f.Value))
	}
	if got := strings.Join(kept, ", "); got != "SEQUENCE 2, X-APPLE-TRAVEL-ADVISORY-BEHAVIOR AUTOMATIC, X-MOZ-GENERATION 3" {
		t.Errorf("kept = %s", got)
	}

	// Saving the form as it stands writes nothing about the rule: the empty
	// Advanced RRULE is not a rule.
	form := formFields(page.Form)
	form.Set("summary", "Синк-ап по релизу ·")
	if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	body, _, _ := app.servers["nextcloud"].Object(davtest.CalendarHome + "lichnyj/8f1c-4a02-b7d3.ics")
	if !strings.Contains(unfolded(body), "RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z") {
		t.Errorf("the rule after a save that did not touch it:\n%s", body)
	}

	fresh := decode[view.EventEditPage](t, app.h.do(get(list+"/new", app.sess)))
	if fresh.Name != "New event" || fresh.Subtitle != "" || fresh.Delete != nil {
		t.Errorf("new event head = %q, %q, delete %v", fresh.Name, fresh.Subtitle, fresh.Delete)
	}
}
