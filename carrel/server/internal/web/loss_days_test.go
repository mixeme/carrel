package web_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Typecase's Р-60 (the owner's answer of 14.10.2026, states-36): a server
// that kept the change of a series but lost a day left out and an occurrence
// moved apart is said in days, not in EXDATE and RECURRENCE-ID — each by its
// start in the series and what became of it. Properties lost besides go in
// their own message, as before (states-17).
func TestLostDaysOfASeriesAreDays(t *testing.T) {
	series := "BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T120000Z\nDTEND:20260810T130000Z\n" +
		"RRULE:FREQ=WEEKLY;COUNT=6\nEXDATE:20260824T120000Z\nSUMMARY:Sync-up\nX-PRIVATE:yes\nEND:VEVENT\n" +
		"BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID:20260831T120000Z\nDTSTART:20260901T150000Z\n" +
		"DTEND:20260901T160000Z\nSUMMARY:Sync-up\nEND:VEVENT"
	if !strings.Contains(mockupFrame(t, "states-36"), "did not keep two days of the series") {
		t.Fatal("states-36 says something else: read the frame again")
	}
	for _, tc := range []struct {
		name  string
		drop  []string
		props []string
		title string
	}{
		{"the days alone", []string{"EXDATE", "RECURRENCE-ID"}, nil,
			"The change was saved, but the server did not keep two days of the series."},
		{"a property too", []string{"EXDATE", "RECURRENCE-ID", "X-PRIVATE"}, []string{"X-PRIVATE"},
			"Nor did it keep two days of the series."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "e.ics", ics(series))
			f.srv.Drop = tc.drop
			rec := f.path("calendar") + "/e"
			form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
			form.Set("summary", "Sync-up, renamed")
			resp := f.h.do(postForm(rec+"/edit", f.sess, form))
			if resp.Code != http.StatusOK {
				t.Fatalf("save: %d, want the loss report", resp.Code)
			}
			page := decode[view.EventEditPage](t, resp)
			if page.Loss == nil || page.Loss.Days == nil {
				t.Fatalf("loss = %+v", page.Loss)
			}
			var props []string
			for _, p := range page.Loss.Properties {
				props = append(props, string(p))
			}
			if !slices.Equal(props, tc.props) {
				t.Errorf("properties = %q, want %q: the days are not named by their properties", props, tc.props)
			}
			days := page.Loss.Days
			body := string(days.Body)
			if string(days.Title) != tc.title || !strings.HasPrefix(body, "Other clients will not see them either: the meeting of 24 August, which was skipped, is back; the meeting of 31 August, which was moved to 1 September, ") ||
				!strings.Contains(body, ", is back on its day at ") || !strings.HasSuffix(body, ".") {
				t.Errorf("days = %q / %q", days.Title, body)
			}
		})
	}
}
