package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// from-typecase note 3: the task form is laid out in rubrics as the event's
// is — When, Progress, Other — and both forms name and order their fields as
// s-tasks-3 and s-agenda-3 do. The frame is read here: rubric by rubric, the
// labels of its fields.

var (
	frameFormPart = regexp.MustCompile(`(?s)<div class="tc-rubric">(.*?)</div>|<label>(.*?)</label>(?:\s*<button class="tc-btn"[^>]*>(.*?)</button>)?|<button class="tc-check"[^>]*>(.*?)</button>`)
	frameSegOpt   = regexp.MustCompile(`(?s)<button class="[^"]*tc-seg__opt[^"]*"[^>]*>(.*?)</button>`)
)

// frameForm is the form of a frame as «rubric / label», in order; a field
// before the first rubric has none. A field labelled with a blank is named
// by its button.
func frameForm(t *testing.T, id string) []string {
	t.Helper()
	frame := mockupFrame(t, id)
	start := strings.Index(frame, `class="tc-form"`)
	if start < 0 {
		t.Fatalf("%s draws no form: the frame changed, read it again", id)
	}
	frame = frame[start:]
	if end := strings.Index(frame, `class="tc-danger"`); end > 0 {
		frame = frame[:end]
	}
	var out []string
	rubric := ""
	for _, m := range frameFormPart.FindAllStringSubmatch(frame, -1) {
		switch {
		case strings.HasPrefix(m[0], `<div`):
			rubric = frameText(m[1])
		case strings.HasPrefix(m[0], `<label`):
			label := frameText(m[2])
			if label == "" {
				label = frameText(m[3])
			}
			out = append(out, rubric+" / "+label)
		default:
			out = append(out, rubric+" / "+frameText(m[4]))
		}
	}
	return out
}

// serverForm is a form the server sent, written the way frameForm writes
// one, without the rows kept beyond the frame.
func serverForm(f view.Form, kept ...string) []string {
	out := []string{}
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			item := string(set.Legend) + " / " + string(row.Label)
			if row.Kind == view.InputHidden || strings.Contains(strings.Join(kept, "\n"), item) {
				continue
			}
			out = append(out, item)
		}
	}
	return out
}

func TestTaskFormIsTheFramesRubrics(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nDUE;VALUE=DATE:20260807\nSTATUS:NEEDS-ACTION\nPRIORITY:1\nEND:VTODO"))
	page := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess)))
	// What the frame does not draw and the owner kept (to-typecase task 13):
	// the start time, the all-day boxes, Done and the reminders; and the UID
	// typed by hand under Related to.
	kept := []string{"When / Start time", "When / All day", "Progress / Done", "Reminders / First", "Other / Or enter a UID by hand"}
	if got, want := serverForm(page.Form, kept...), frameForm(t, "s-tasks-3"); !equalStrings(got, want) {
		t.Errorf("task form =\n%q,\nwant (s-tasks-3)\n%q", got, want)
	}
	legends := []string{}
	for _, set := range page.Form.Sets {
		legends = append(legends, string(set.Legend))
	}
	if want := []string{"", "When", "Progress", "Reminders", "Other"}; !equalStrings(legends, want) {
		t.Errorf("task rubrics = %q, want %q", legends, want)
	}
}

func TestEventFormIsTheFramesRubrics(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=Europe/Moscow:20260810T100000\n"+
			"DTEND;TZID=Europe/Moscow:20260810T110000\nSUMMARY:Sync-up\nLOCATION:Room 2\nSTATUS:CONFIRMED\n"+
			"RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z\n"+
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"))
	page := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess)))
	if got, want := serverForm(page.Form), frameForm(t, "s-agenda-3"); !equalStrings(got, want) {
		t.Errorf("event form =\n%q,\nwant (s-agenda-3)\n%q", got, want)
	}

	// On is one row of the days, as the frame draws it, not seven boxes.
	var days []string
	for _, m := range frameSegOpt.FindAllStringSubmatch(mockupFrame(t, "s-agenda-3"), -1) {
		days = append(days, frameText(m[1]))
	}
	on := rowNamed(t, page.Form, "repeat.on")
	if on.Kind != view.InputCheck || !equalStrings(serverList(on.Options), days) {
		t.Errorf("On = %+v, want a check row of %q", on, days)
	}
	if sel := selectedOf(on.Options); len(sel) != 1 || sel[0] != "MO" {
		t.Errorf("On selects %q, want MO", sel)
	}
}
