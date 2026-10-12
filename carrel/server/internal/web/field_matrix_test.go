package web_test

import (
	"net/url"
	"slices"
	"testing"
)

// The matrix «every field × a hard record» for the kinds the seed of
// v0.2.188 (edit_is_the_edit_test.go) left out: a task and an event with
// what another client writes into them. One field is changed, and only its
// lines change.

func matrixTask() string {
	return ics("BEGIN:VTODO\nUID:hard\nDTSTAMP:20260101T000000Z\nSUMMARY:Ship\nDESCRIPTION:Steps\\n\\n\n" +
		"DTSTART;TZID=Europe/Moscow:20260110T090000\nDUE;TZID=Europe/Moscow:20260112T180000\n" +
		"PRIORITY:3\nPERCENT-COMPLETE:40\nSTATUS:IN-PROCESS\nCATEGORIES:Smith\\, John,urgent\nCATEGORIES:q1\nLOCATION:Dock\\, gate 4\n" +
		"RELATED-TO;RELTYPE=PARENT:parent-1\nRELATED-TO;RELTYPE=SIBLING:sib-1\nX-APPLE-SORT-ORDER:12\nURL:https://example.com/t\n" +
		"BEGIN:VALARM\nACTION:AUDIO\nTRIGGER;RELATED=END:-PT30M\nEND:VALARM\nEND:VTODO")
}

func matrixEvent() string {
	return ics("BEGIN:VEVENT\nUID:hard\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260112T100000\nDTEND;TZID=Europe/Moscow:20260112T110000\n" +
		"SUMMARY:Sync\nLOCATION:Room\\; 4\nGEO:55.7;37.6\nSTATUS:TENTATIVE\nTRANSP:TRANSPARENT\nCLASS:CONFIDENTIAL\n" +
		"CATEGORIES:Smith\\, John,c\nURL:https://example.com/e\nSEQUENCE:4\nX-MICROSOFT-CDO-BUSYSTATUS:FREE\nEND:VEVENT")
}

func TestATaskSaveIsTheEdit(t *testing.T) {
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
	}{
		{"the summary", url.Values{"summary": {"Ship it"}}, []string{"SUMMARY"}, []string{"SUMMARY:Ship it", "X-APPLE-SORT-ORDER:12"}},
		{"the place", url.Values{"location": {"Dock, gate 5"}}, []string{"LOCATION"}, []string{"LOCATION:Dock\\, gate 5"}},
		{"the due time", url.Values{"due.time": {"16:00"}}, []string{"DUE"}, []string{"DUE;TZID=Europe/Moscow:20260112T190000", "DTSTART;TZID=Europe/Moscow:20260110T090000"}},
		// Б11: a tag with a comma of its own is one tag before and after.
		{"one more tag", url.Values{"categories": {"Smith, John, urgent, q1, later"}}, []string{"CATEGORIES"},
			[]string{"CATEGORIES:Smith\\, John,urgent,q1,later"}},
		{"a tag taken away", url.Values{"categories": {"Smith, John, q1"}}, []string{"CATEGORIES"}, []string{"CATEGORIES:Smith\\, John,q1"}},
		// A relation kept keeps what kind of relation it is.
		{"one relation fewer", url.Values{"related": {"parent-1"}}, []string{"RELATED-TO"}, []string{"RELATED-TO;RELTYPE=PARENT:parent-1"}},
		{"the priority", url.Values{"priority": {"1"}}, []string{"PRIORITY"}, []string{"PRIORITY:1", "PERCENT-COMPLETE:40"}},
		{"the percent", url.Values{"percent": {"60"}}, []string{"PERCENT-COMPLETE"}, []string{"STATUS:IN-PROCESS"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "hard.ics", matrixTask())
			before, after := oneEdit(t, f, f.path("tasks")+"/hard", "hard.ics", url.Values{"description": {"Steps, primed"}}, tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, append(tc.holds, "ACTION:AUDIO", "TRIGGER;RELATED=END:-PT30M", "URL:https://example.com/t")...)
		})
	}
}

func TestAnEventOfAnotherClientSaveIsTheEdit(t *testing.T) {
	cases := []struct {
		name    string
		set     url.Values
		changed []string
		holds   []string
	}{
		{"the place", url.Values{"location": {"Room; 5"}}, []string{"LOCATION"}, []string{"LOCATION:Room\\; 5", "GEO:55.7;37.6"}},
		{"one more tag", url.Values{"categories": {"Smith, John, c, d"}}, []string{"CATEGORIES"}, []string{"CATEGORIES:Smith\\, John,c,d"}},
		{"the status", url.Values{"status": {"CONFIRMED"}}, []string{"STATUS"}, []string{"STATUS:CONFIRMED", "TRANSP:TRANSPARENT", "CLASS:CONFIDENTIAL"}},
		{"the start", url.Values{"start.time": {"09:30"}}, []string{"DTSTART"}, []string{"DTSTART;TZID=Europe/Moscow:20260112T093000", "SEQUENCE:4"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := calendarFixture(t, nil, "hard.ics", matrixEvent())
			before, after := oneEdit(t, f, f.path("calendar")+"/hard", "hard.ics", url.Values{"description": {"primed"}}, tc.set)
			if got := changedLines(before, after); !slices.Equal(got, tc.changed) {
				t.Errorf("lines changed: %v, want %v\n%s", got, tc.changed, after)
			}
			mustContain(t, after, append(tc.holds, "X-MICROSOFT-CDO-BUSYSTATUS:FREE", "URL:https://example.com/e")...)
		})
	}
}
