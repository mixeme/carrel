package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-10: an event showed its status and categories (s-agenda-3) and
// the form could change neither. Status is a choice of RFC 5545 §3.8.1.11
// for an event, and a value another client wrote stays a choice; categories
// are one comma field, each written as a TEXT item. One change, one line.
func TestEventStatusAndCategories(t *testing.T) {
	f := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART:20260115T100000Z\nSUMMARY:Review\nSTATUS:X-HELD\nCATEGORIES:work,review\nX-KEEP:yes\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	editForm := func() (view.EventEditPage, map[string]string) {
		page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
		out := map[string]string{}
		for k, v := range formFields(page.Form) {
			out[k] = v[0]
		}
		return page, out
	}
	save := func(set map[string]string) string {
		t.Helper()
		page, _ := editForm()
		form := formFields(page.Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "meet.ics")
	}

	page, form := editForm()
	if form["status"] != "X-HELD" || form["categories"] != "work, review" {
		t.Fatalf("form = %v", form)
	}
	status := rowNamed(t, page.Form, "status")
	var values []string
	for _, o := range status.Options {
		values = append(values, o.Value)
	}
	if strings.Join(values, " ") != " TENTATIVE CONFIRMED CANCELLED X-HELD" {
		t.Errorf("status options = %v", values)
	}

	got := save(map[string]string{"summary": "Review again"})
	mustContain(t, got, "STATUS:X-HELD", "CATEGORIES:work,review", "X-KEEP:yes")

	got = save(map[string]string{"status": "CANCELLED"})
	mustContain(t, got, "STATUS:CANCELLED", "CATEGORIES:work,review", "SUMMARY:Review again")

	got = save(map[string]string{"categories": "work, a\\b, c;d"})
	mustContain(t, got, `CATEGORIES:work,a\\b,c\;d`, "STATUS:CANCELLED")

	got = save(map[string]string{"status": "", "categories": ""})
	if strings.Contains(got, "STATUS") || strings.Contains(got, "CATEGORIES") {
		t.Errorf("emptied fields left their lines:\n%s", got)
	}

	// What the list offers and nothing else.
	page, _ = editForm()
	form2 := formFields(page.Form)
	form2.Set("status", "DONE")
	bad := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form2)))
	if len(bad.Form.Errors) == 0 || bad.Form.Errors[0].Name != "status" {
		t.Errorf("errors = %+v, want status refused", bad.Form.Errors)
	}
}

// review Р-10, "Пояс события": the zone the event was sent in is shown as
// such ("as sent") and kept by every save that does not change it. Another
// zone rewrites DTSTART and DTEND at the same instants in that zone and
// brings its VTIMEZONE; UTC writes Z and no TZID. The form's own times stay
// in the instance zone (C-10).
func TestEventTimeZone(t *testing.T) {
	f := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART;TZID=Europe/Moscow:20260115T100000\nDTEND;TZID=Europe/Moscow:20260115T110000\nSUMMARY:Review\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	save := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "meet.ics")
	}

	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	zone := rowNamed(t, page.Form, "tz")
	if zone.Value != "Europe/Moscow" || len(zone.Options) < 3 || zone.Options[0].Value != "Europe/Moscow" ||
		zone.Options[0].Label != "Europe/Moscow · as sent" || !zone.Options[0].Selected {
		t.Fatalf("zone row = %+v", zone)
	}
	if form := formFields(page.Form); form.Get("start.time") != "07:00" {
		t.Errorf("start shown at %s, want the instance zone's 07:00", form.Get("start.time"))
	}

	got := save(map[string]string{"summary": "Review again"})
	mustContain(t, got, "DTSTART;TZID=Europe/Moscow:20260115T100000", "DTEND;TZID=Europe/Moscow:20260115T110000")

	got = save(map[string]string{"tz": "Europe/London"})
	mustContain(t, got, "DTSTART;TZID=Europe/London:20260115T070000", "DTEND;TZID=Europe/London:20260115T080000",
		"BEGIN:VTIMEZONE", "TZID:Europe/London", "SUMMARY:Review again")

	got = save(map[string]string{"tz": "UTC"})
	mustContain(t, got, "DTSTART:20260115T070000Z", "DTEND:20260115T080000Z")

	form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("tz", "Mars/Olympus")
	bad := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	if len(bad.Form.Errors) == 0 || bad.Form.Errors[0].Name != "tz" {
		t.Errorf("errors = %+v, want tz refused", bad.Form.Errors)
	}
}
