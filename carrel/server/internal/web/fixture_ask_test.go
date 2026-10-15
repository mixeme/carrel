package web_test

import (
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/web"
	"carrel.dev/carrel/server/view"
)

// seedMockupDuty puts into «Работа» the weekly duty at nine on Mondays of
// January and February 2026 that s-agenda-22…-24 ask about, with three of
// its meetings changed by hand: the one of 26 January moved to noon, the
// one of 2 February renamed, the one of 16 February moved to half past two.
// The set's own duty is a day long and has none (rabota.ics): the series
// lives in this test alone.
func seedMockupDuty(t *testing.T, app *mockupApp) string {
	t.Helper()
	const uid = "7c1e-22ab-90f4"
	at := func(day, clock string) string { return ";TZID=Europe/Moscow:2026" + day + "T" + clock + "00" }
	moved := func(was, day, from, to, title string) string {
		return "BEGIN:VEVENT\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID" + at(was, "0900") +
			"\nDTSTART" + at(day, from) + "\nDTEND" + at(day, to) + "\nSUMMARY:" + title + "\nEND:VEVENT\n"
	}
	app.servers["nextcloud"].Seed(davtest.CalendarHome+"rabota/"+uid+".ics", ics("BEGIN:VEVENT\nUID:"+uid+"\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART"+at("0105", "0900")+"\nDTEND"+at("0105", "1000")+"\nRRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20260302T000000Z\n"+
		"SUMMARY:Дежурство: первая линия\nEND:VEVENT\n"+
		moved("0126", "0126", "1200", "1300", "Duty moved")+
		moved("0202", "0202", "0900", "1000", "Дежурство: вторая линия")+
		moved("0216", "0216", "1430", "1530", "Duty moved")))
	return app.listPath(t, "calendar", "rabota") + "/" + uid
}

// C-14.2: the question before a series is saved (А1; s-agenda-22…-24): the
// series cut to end on 19 January leaves its three meetings changed by hand
// without their day; every other week leaves one; kept, and the server
// refusing one of them, the sheet says what was written and what was not.
// Each sheet stands over the edit as typed, the last over the series' card.
func TestFixtureSeriesQuestion(t *testing.T) {
	app := newMockupApp(t)
	rec := seedMockupDuty(t, app)
	openList(t, app, "/app/calendar?from=2026-08-10")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	ask := func(set map[string]string) view.AskSheet {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		got := app.h.do(postForm(rec+"/edit", app.sess, form))
		return decode[view.AskSheet](t, got)
	}
	cut := map[string]string{"repeat.until": "2026-01-19", "repeat.count": ""}
	form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	for k, v := range cut {
		form.Set(k, v)
	}
	app.compare(t, "app-event-ask/normal.json", app.h.do(postForm(rec+"/edit", app.sess, form)))
	form = formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	form.Set("repeat.interval", "2")
	app.compare(t, "app-event-ask/single.json", app.h.do(postForm(rec+"/edit", app.sess, form)))

	sheet := ask(cut)
	msk, _ := time.LoadLocation("Europe/Moscow")
	restore := web.RefuseApart(time.Date(2026, 2, 2, 9, 0, 0, 0, msk), "507 Insufficient Storage")
	part := app.h.do(postForm(askAnswer(t, sheet, "series.keep").Href, app.sess, formFields(sheet.Form)))
	restore()
	app.compare(t, "app-event-ask/in-part.json", part)
}
