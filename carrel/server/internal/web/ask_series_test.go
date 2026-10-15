package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/web"
	"carrel.dev/carrel/server/view"
)

// A series of six Mondays at ten in Moscow, a day left out on 19 January,
// and two meetings changed by hand: the one of 26 January moved to noon, the
// one of 2 February renamed.
func askSeriesDoc() string {
	return ics("BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDTEND;TZID=Europe/Moscow:20260112T110000\n" +
		"SUMMARY:Duty\nRRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=6\nEXDATE;TZID=Europe/Moscow:20260119T100000\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260126T100000\n" +
		"DTSTART;TZID=Europe/Moscow:20260126T120000\nDTEND;TZID=Europe/Moscow:20260126T130000\nSUMMARY:Duty moved\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:duty\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID;TZID=Europe/Moscow:20260202T100000\n" +
		"DTSTART;TZID=Europe/Moscow:20260202T100000\nDTEND;TZID=Europe/Moscow:20260202T110000\nSUMMARY:Duty second line\nEND:VEVENT")
}

// cutSeries opens the series' edit and cuts it to one Monday.
func cutSeries(t *testing.T) (davFixture, url.Values, string) {
	t.Helper()
	f := calendarFixture(t, nil, "duty.ics", askSeriesDoc())
	rec := f.path("calendar") + "/duty"
	form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("repeat.count", "1")
	form.Set("repeat.rule", "")
	return f, form, rec
}

func askAnswer(t *testing.T, sheet view.AskSheet, id string) view.Action {
	t.Helper()
	for _, a := range sheet.Actions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("no answer %s in %+v", id, sheet.Actions)
	return view.Action{}
}

// А1 (the owner's decision of 11.10.2026; Typecase's note 56, s-agenda-22):
// an edit that cuts the series to one Monday leaves both meetings changed by
// hand without their day. Save asks first, over the edit as typed, and
// writes nothing: Cancel goes back to the edit; Remove writes the series
// without them and without the day left out it no longer has; Keep writes
// each as an event of its own first.
func TestASeriesAsksBeforeItLeavesMeetingsWithoutTheirDay(t *testing.T) {
	t.Cleanup(web.DrawSheets())
	f, form, rec := cutSeries(t)
	before := body(t, f, "duty.ics")
	sheet := decode[view.AskSheet](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	if string(sheet.Title) != "Keep the meetings that were moved by hand?" || !strings.HasSuffix(string(sheet.Subject), "· not saved yet") ||
		len(sheet.Blocks) != 2 || sheet.Blocks[1].Title != "Left without their day · 2" || sheet.Under == nil || sheet.Under.EventEdit == nil {
		t.Fatalf("sheet = %+v", sheet)
	}
	lines := sheet.Blocks[1].Lines
	if lines[0].Text != "26 January, 12:00 — Duty moved" || lines[0].Aside != "— it was the meeting of 26 January, 10:00" ||
		lines[1].Text != "2 February, 10:00 — Duty second line" || lines[1].Aside != "— its title was changed" {
		t.Errorf("left = %+v", lines)
	}
	if body(t, f, "duty.ics") != before {
		t.Fatal("the question wrote")
	}
	if keep := askAnswer(t, sheet, "series.keep"); !keep.Primary || string(keep.Label) != "Keep as events" {
		t.Errorf("keep = %+v", keep)
	}
	again := formFields(sheet.Form)
	if again.Get("repeat.count") != "1" {
		t.Fatalf("the sheet does not carry the edit: %v", again)
	}
	back := decode[view.EventEditPage](t, f.h.do(postForm(askAnswer(t, sheet, "series.cancel").Href, f.sess, again)))
	if formFields(back.Form).Get("repeat.count") != "1" || body(t, f, "duty.ics") != before {
		t.Error("Cancel is not the edit as typed, or it wrote")
	}
	if resp := f.h.do(postForm(askAnswer(t, sheet, "series.remove").Href, f.sess, again)); resp.Code != 303 {
		t.Fatalf("remove: %d %s", resp.Code, resp.Body.String())
	}
	got := unfolded(body(t, f, "duty.ics"))
	if strings.Contains(got, "RECURRENCE-ID") || strings.Contains(got, "EXDATE") || !strings.Contains(got, "COUNT=1") {
		t.Errorf("after Remove:\n%s", got)
	}

	f, form, rec = cutSeries(t)
	sheet = decode[view.AskSheet](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	if resp := f.h.do(postForm(askAnswer(t, sheet, "series.keep").Href, f.sess, formFields(sheet.Form))); resp.Code != 303 {
		t.Fatalf("keep: %d %s", resp.Code, resp.Body.String())
	}
	if got := unfolded(body(t, f, "duty.ics")); strings.Contains(got, "RECURRENCE-ID") {
		t.Errorf("the series still holds them:\n%s", got)
	}
	apart := 0
	for _, p := range f.srv.Paths(f.col) {
		b, _, _ := f.srv.Object(p)
		if strings.Contains(b, "SUMMARY:Duty moved") || strings.Contains(b, "SUMMARY:Duty second line") {
			if strings.Contains(b, "RECURRENCE-ID") || strings.Contains(b, "RRULE") || strings.Contains(unfolded(b), "UID:duty\n") {
				t.Errorf("%s is not an event of its own:\n%s", p, b)
			}
			apart++
		}
	}
	if apart != 2 {
		t.Errorf("%d events apart, want 2", apart)
	}
}

// s-agenda-24: one meeting cannot be written apart. The series is saved,
// the other meeting is an event of its own, and the one refused stays in
// the series as it was; the sheet says what was written, what was not and
// why. Try again writes it apart now.
func TestASeriesSavedInPart(t *testing.T) {
	t.Cleanup(web.DrawSheets())
	f, form, rec := cutSeries(t)
	sheet := decode[view.AskSheet](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	msk, _ := time.LoadLocation("Europe/Moscow")
	restore := web.RefuseApart(time.Date(2026, 2, 2, 10, 0, 0, 0, msk), "507 Insufficient Storage")
	part := decode[view.AskSheet](t, f.h.do(postForm(askAnswer(t, sheet, "series.keep").Href, f.sess, formFields(sheet.Form))))
	restore()
	if !strings.HasSuffix(string(part.Subject), "· saved in part") || len(part.Blocks) != 2 || part.Blocks[1].Title != "Not written · 1" ||
		!strings.Contains(string(part.Blocks[1].Lines[0].Aside), "507 Insufficient Storage") ||
		part.Alert != "The meeting of 2 February was not made a separate event: it stays in the series, with its change, as it was before you saved." {
		t.Fatalf("saved in part = %+v", part)
	}
	got := unfolded(body(t, f, "duty.ics"))
	if !strings.Contains(got, "RECURRENCE-ID;TZID=Europe/Moscow:20260202T100000") || strings.Contains(got, "20260126T100000") || !strings.Contains(got, "COUNT=1") {
		t.Fatalf("the series as saved:\n%s", got)
	}
	retry := askAnswer(t, part, "series.retry")
	if !retry.Primary {
		t.Errorf("Try again = %+v", retry)
	}
	if resp := f.h.do(postForm(retry.Href, f.sess, formFields(part.Form))); resp.Code != 303 {
		t.Fatalf("Try again: %d %s", resp.Code, resp.Body.String())
	}
	if got := unfolded(body(t, f, "duty.ics")); strings.Contains(got, "RECURRENCE-ID") {
		t.Errorf("after Try again the series still holds it:\n%s", got)
	}
}

// Until the client draws sheets (the owner's decision of 15.10.2026) Save
// writes the series as before: the meetings changed by hand stay in it
// loose, and only the day left out it no longer has goes.
func TestASeriesSavesAsBeforeUntilTheClientDrawsSheets(t *testing.T) {
	f, form, rec := cutSeries(t)
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	got := unfolded(body(t, f, "duty.ics"))
	if !strings.Contains(got, "RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000") || strings.Contains(got, "EXDATE") || !strings.Contains(got, "COUNT=1") {
		t.Errorf("saved:\n%s", got)
	}
}
