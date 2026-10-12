package web_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// from-typecase note 3: the task form is laid out in rubrics as the event's
// is — When, Progress, Other — and both forms name and order their fields as
// s-tasks-3 and s-agenda-3 do. The frame is read here: rubric by rubric, the
// labels of its fields.

var (
	frameFormPart = regexp.MustCompile(`(?s)<div class="tc-rubric">(.*?)</div>|<label>(.*?)</label>(?:\s*<button class="tc-(?:btn|check)"[^>]*>(.*?)</button>)?|<button class="tc-check"[^>]*>(.*?)</button>`)
	// An option of a switch that is a row of checkboxes (Seg.Multiple,
	// Typecase's task 72): its value, ticked or not, its words.
	frameSegOpt = regexp.MustCompile(`(?s)<label class="[^"]*tc-seg__opt[^"]*"><input class="tc-seg__box" type="checkbox"[^>]*value="([^"]*)"( checked)?>(.*?)</label>`)
)

// frameForm is the form of a frame as «rubric / label», in order; a field
// before the first rubric has none. A field labelled with a blank is named
// by its button or its box (s-tasks-3: «Add reminder», «Done»).
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
			if row.Kind == view.InputHidden || row.Kind == view.InputRemove || strings.Contains(strings.Join(kept, "\n"), item) {
				continue
			}
			out = append(out, item)
		}
	}
	return out
}

func TestTaskFormIsTheFramesRubrics(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nDUE;VALUE=DATE:20260807\nSTATUS:NEEDS-ACTION\nPRIORITY:1\n"+
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-P1D\nDESCRIPTION:x\nEND:VALARM\nEND:VTODO"))
	page := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess)))
	// One «All day» for the whole task, the start time in When, Done last in
	// Progress, the reminders a rubric of their own (to-typecase task 13).
	// What the frame does not draw: the UID typed by hand under Related to.
	kept := []string{"Other / Or enter a UID by hand"}
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
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:x\nEND:VALARM\n"+
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-P1D\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"))
	page := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess)))
	// «Related to» is every kind's (the owner's word of 12.10.2026) and the
	// event's frame does not draw it yet — Typecase's task 160: until it
	// does, the form is the frame's with that one row after Description.
	want := frameForm(t, "s-agenda-3")
	if at := slices.Index(want, " / Description"); at >= 0 && !slices.Contains(want, " / Related to") {
		want = slices.Insert(want, at+1, " / Related to")
	}
	if got := serverForm(page.Form); !equalStrings(got, want) {
		t.Errorf("event form =\n%q,\nwant (s-agenda-3, and Related to)\n%q", got, want)
	}

	// On is one row of the days, as the frame draws it, not seven boxes: a
	// check row, each day an option sent by its value, the ticked ones as the
	// frame ticks them. The frame's field name (byday) is the frame's, as
	// csrf in components-42: the row is the form's repeat.on.
	var days, values, ticked []string
	for _, m := range frameSegOpt.FindAllStringSubmatch(mockupFrame(t, "s-agenda-3"), -1) {
		days = append(days, frameText(m[3]))
		values = append(values, m[1])
		if m[2] != "" {
			ticked = append(ticked, m[1])
		}
	}
	on := rowNamed(t, page.Form, "repeat.on")
	var sent []string
	for _, o := range on.Options {
		sent = append(sent, o.Value)
	}
	if len(days) != 7 || on.Kind != view.InputCheck || !equalStrings(serverList(on.Options), days) || !equalStrings(sent, values) {
		t.Errorf("On = %+v, want a check row of %q sending %q", on, days, values)
	}
	if sel := selectedOf(on.Options); !equalStrings(sel, ticked) {
		t.Errorf("On selects %q, the frame %q", sel, ticked)
	}
}

// from-typecase note 18 (owner's decision, to-typecase task 13): a task has
// one «All day» (s-tasks-3), not a box at each of its times. Ticked, both
// times go — the start and the due are written as days; cleared, both are
// times again. A save that does not touch the box leaves both as they were,
// even when another client made one a day and the other a time.
func TestTaskAllDayIsOneBox(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nDTSTART:20260805T090000Z\nDUE:20260807T180000Z\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Mixed\nDTSTART;VALUE=DATE:20260805\nDUE:20260807T180000Z\nEND:VTODO"))
	edit := func(uid string) view.TaskEditPage {
		return decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/"+uid+"/edit", f.sess)))
	}
	save := func(uid string, set map[string]string) string {
		t.Helper()
		form := formFields(edit(uid).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(f.path("tasks")+"/"+uid+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, uid+".ics")
	}
	boxes := func(p view.TaskEditPage) []string {
		var out []string
		for _, set := range p.Form.Sets {
			for _, row := range set.Rows {
				if row.Label == "All day" {
					out = append(out, string(set.Legend)+" / "+row.Name+" = "+string(row.Value))
				}
			}
		}
		return out
	}
	if got := boxes(edit("t1")); !equalStrings(got, []string{"When / allday = "}) {
		t.Fatalf("all-day boxes = %q, want one, off, leading When", got)
	}

	got := save("t1", map[string]string{"summary": "Slides v2"})
	mustContain(t, got, "DTSTART:20260805T090000Z", "DUE:20260807T180000Z")

	got = save("t1", map[string]string{"allday": "on"})
	mustContain(t, got, "DTSTART;VALUE=DATE:20260805", "DUE;VALUE=DATE:20260807")
	if strings.Contains(got, "T090000") || strings.Contains(got, "T180000") {
		t.Errorf("a time stayed on an all-day task:\n%s", got)
	}
	page := edit("t1")
	if got := boxes(page); !equalStrings(got, []string{"When / allday = on"}) {
		t.Errorf("after ticking, boxes = %q", got)
	}
	if form := formFields(page.Form); form.Get("start.time") != "" || form.Get("due.time") != "" {
		t.Errorf("times of an all-day task = %q, %q", form.Get("start.time"), form.Get("due.time"))
	}

	got = save("t1", map[string]string{"allday": "", "start.time": "10:00", "due.time": "18:30"})
	if strings.Contains(got, "VALUE=DATE") || !strings.Contains(got, "T100000") || !strings.Contains(got, "T183000") {
		t.Errorf("cleared, the task is not timed again:\n%s", got)
	}

	// Another client's mix: the start is a day, the due a time. The box
	// follows the start; a save that leaves it alone writes neither.
	if got := boxes(edit("t2")); !equalStrings(got, []string{"When / allday = on"}) {
		t.Errorf("mixed task boxes = %q", got)
	}
	got = save("t2", map[string]string{"summary": "Mixed v2"})
	mustContain(t, got, "SUMMARY:Mixed v2", "DTSTART;VALUE=DATE:20260805", "DUE:20260807T180000Z")
}
