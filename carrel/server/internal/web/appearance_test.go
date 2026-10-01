package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

func rowNamed(t *testing.T, f view.Form, name string) view.FormRow {
	t.Helper()
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			if row.Name == name {
				return row
			}
		}
	}
	t.Fatalf("no row %q in %+v", name, f)
	return view.FormRow{}
}

// C-10: the first day of the week and 12 or 24 hours come from the account
// and reach every time the server assembles; the zone stays the instance's.
func TestAccountWeekStartAndClock(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.Timezone = "Europe/Berlin" },
		"series.ics", ics("BEGIN:VEVENT\nUID:series\nDTSTAMP:20260101T000000Z\nDTSTART:20260112T090000Z\nDTEND:20260112T093000Z\n"+
			"RRULE:FREQ=WEEKLY;COUNT=4\nSUMMARY:Standup\nEND:VEVENT"),
		"ny.ics", ics("BEGIN:VEVENT\nUID:ny\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=America/New_York:20260114T100000\n"+
			"DTEND;TZID=America/New_York:20260114T110000\nSUMMARY:Call New York\nEND:VEVENT"),
	)
	const settings = "/app/settings/appearance"

	page := decode[view.AppearancePage](t, f.h.do(get(settings, f.sess)))
	if v := rowNamed(t, page.Account, "weekStart").Value; v != "monday" {
		t.Errorf("weekStart = %q, want Monday before anything is chosen", v)
	}
	if v := rowNamed(t, page.Account, "clock").Value; v != "24" {
		t.Errorf("clock = %q", v)
	}
	if zone := rowNamed(t, page.Instance, "timezone"); zone.Value != "Europe/Berlin" || !zone.ReadOnly {
		t.Errorf("timezone row = %+v", zone)
	}

	rec := f.h.do(postForm(settings, f.sess, url.Values{"weekStart": {"friday"}, "clock": {"24"}}))
	page = decode[view.AppearancePage](t, rec)
	if rowNamed(t, page.Account, "weekStart").Error == "" {
		t.Error("a day outside the list was not refused")
	}
	snap := f.h.st.Snapshot()
	if u := snap.UserByID(f.sess.AccountID()); u.WeekStart != "" || u.Clock12 {
		t.Errorf("a refused form stored %q / %v", u.WeekStart, u.Clock12)
	}

	if rec := f.h.do(postForm(settings, f.sess, url.Values{"weekStart": {"sunday"}, "clock": {"12"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	snap = f.h.st.Snapshot()
	if u := snap.UserByID(f.sess.AccountID()); u.WeekStart != "sunday" || !u.Clock12 {
		t.Errorf("stored %q / %v", u.WeekStart, u.Clock12)
	}

	agenda := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar"), f.sess)))
	if agenda.Bar.Range == nil {
		t.Fatal("no range")
	}
	if from, err := time.Parse("2 January 2006", string(agenda.Bar.Range.From)); err != nil || from.Weekday() != time.Sunday {
		t.Errorf("this week starts %q (%v), want a Sunday", agenda.Bar.Range.From, err)
	}

	agenda = decode[view.AgendaPage](t, f.h.do(get(f.path("calendar")+"?from=2026-01-11", f.sess)))
	want := []string{
		"Monday · 12 January|10:00 AM|Standup|",
		"Wednesday · 14 January|4:00 PM|Call New York|America/New_York",
	}
	if got := agendaLines(agenda); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("week of 11 January:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if agenda.Bar.Range.To != "17 January 2026" {
		t.Errorf("range = %+v", agenda.Bar.Range)
	}

	event := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/series", f.sess)))
	var when string
	for _, fld := range event.Fields {
		if fld.Label == "When" {
			for _, r := range fld.Runs {
				when += string(r.Text)
			}
		}
	}
	// The clock of the person's twelve hours, the day after it (s-agenda-2);
	// the year only when it is not this year's.
	if !strings.HasPrefix(when, "10:00 AM – 10:30 AM, Monday 12 January") {
		t.Errorf("When = %q", when)
	}

	audit := decode[view.AdminAuditPage](t, f.h.do(get("/admin/audit", f.sess)))
	if len(audit.Rows) == 0 || !strings.HasSuffix(string(audit.Rows[0].When), "M") {
		t.Errorf("audit row time = %+v, want the 12-hour clock", audit.Rows)
	}

	// Another person on the same instance keeps Monday and 24 hours.
	snap = f.h.st.Snapshot()
	f.h.invite(snap.UserByID(f.sess.AccountID()).ID, "bob", "password456")
	bob := f.h.login("bob", "password456")
	page = decode[view.AppearancePage](t, f.h.do(get(settings, bob)))
	if rowNamed(t, page.Account, "weekStart").Value != "monday" || rowNamed(t, page.Account, "clock").Value != "24" {
		t.Errorf("bob's settings = %+v", page.Account)
	}
}
