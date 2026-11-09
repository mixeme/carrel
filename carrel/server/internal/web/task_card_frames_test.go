package web_test

import (
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// frameFields are the rows of a frame's field list, «mono» after the label
// of one it sets monospaced (the whole value or its first piece).
func frameFields(t *testing.T, part string) []string {
	t.Helper()
	var out []string
	for _, m := range regexp.MustCompile(`<dt>([^<]+)</dt><dd( class="tc-mono")?>(<span class="tc-mono">)?`).FindAllStringSubmatch(part, -1) {
		out = append(out, m[1]+map[bool]string{true: " mono"}[m[2] != "" || m[3] != ""])
	}
	return out
}

func pageFields(fields []view.Field) []string {
	var out []string
	for _, f := range fields {
		mono := f.Mono || (len(f.Runs) > 0 && f.Runs[0].Mark == view.RunMono)
		out = append(out, string(f.Label)+map[bool]string{true: " mono"}[mono])
	}
	return out
}

// dangerOf is the piece of a phrase in alarm.
func dangerOf(runs view.Runs) string {
	for _, r := range runs {
		if r.Mark == view.RunDanger {
			return string(r.Text)
		}
	}
	return ""
}

// The card of a task as s-tasks-2 draws it (the client's scan of the tasks,
// group Г): under the title where it is and how late — in alarm —; Edit
// with its pencil, Mark done, Print; the fields in the frame's order,
// monospaced where it sets them, the reminders among them; the path and
// the comparison with the server under Source; the same task in another
// list, the related event, the file inside it.
func TestTaskCardIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-tasks-2")
	main := frame[strings.Index(frame, `<div class="tc-main">`):]
	want := frameFields(t, main)
	// «Собрать чемодан» is not late, and the frame says nothing of lateness
	// (Typecase's note 37, task 105).
	if len(want) != 6 || strings.Contains(main, "overdue") {
		t.Fatalf("s-tasks-2 fields = %q: read the frame again", want)
	}
	head := headButtons(t, "s-tasks-2")

	app := newMockupApp(t)
	linkMockupTask(t, app)
	openList(t, app, "/app/calendar?from=2026-08-10")
	openList(t, app, "/app/tasks")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "tasks", "dela")

	raw := app.h.do(get(list+"/aa11-2d09-5f84", app.sess))
	var generic map[string]json.RawMessage
	_ = json.Unmarshal(raw.Body.Bytes(), &generic)
	for _, gone := range []string{"percent", "reminders"} {
		if _, ok := generic[gone]; ok {
			t.Errorf("the card still carries %q beside its fields", gone)
		}
	}
	page := decode[view.TaskPage](t, raw)
	if got := pageFields(page.Fields); !slices.Equal(got, want) {
		t.Errorf("fields = %q, want (s-tasks-2) %q", got, want)
	}
	if sub := frameSubtitle(t, "s-tasks-2"); runsText(page.Subtitle) != sub {
		t.Errorf("subtitle = %+v, want (s-tasks-2) %q", page.Subtitle, sub)
	}
	for _, f := range page.Fields {
		if f.Label == "Due" && dangerOf(f.Runs) != "" {
			t.Errorf("Due of a task that is not late = %+v", f)
		}
	}
	edit := actionByID(page.Actions, "record.edit")
	if edit == nil || edit.Icon != "i-pencil" || !edit.Primary || actionByID(page.Actions, "record.print") == nil || actionByID(page.Actions, "task.done") == nil {
		t.Errorf("actions = %+v, want Edit with its pencil, Mark done, Print (s-tasks-2: %q)", page.Actions, head)
	}
	if page.Source.Path == "" || page.Source.Compared == "" {
		t.Errorf("source = %+v, want its path and what the server says of it", page.Source)
	}
	if len(page.Linked) != 1 || !strings.Contains(string(page.Linked[0].Label), "Работа") {
		t.Errorf("linked = %+v", page.Linked)
	}
	if len(page.Related) != 1 || page.Related[0].Title != "Поездка в Казань" || page.Related[0].Kind != view.KindEvent {
		t.Errorf("related = %+v", page.Related)
	}
	if len(page.Attach) != 1 || !page.Attach[0].Inline || page.Attach[0].Size != "128 KB" {
		t.Errorf("attach = %+v", page.Attach)
	}

	// A task overdue says by how much, in alarm, under its title and by its
	// due date (the rule stays; the frame's task is not late).
	late := decode[view.TaskPage](t, app.h.do(get(list+"/aa10-77bc-0e31", app.sess)))
	if dangerOf(late.Subtitle) != "overdue by 3 days" {
		t.Errorf("subtitle of the late task = %+v", late.Subtitle)
	}
	for _, f := range late.Fields {
		if f.Label == "Due" && !strings.HasSuffix(dangerOf(f.Runs), "overdue by 3 days") {
			t.Errorf("Due of the late task = %+v", f)
		}
	}
}

// The form of a task as s-tasks-3 draws it (the client's scan of the
// tasks, group Д): the task and its list in the head, how late in alarm;
// Source in the form; no hint under Tags; All day switching both times off;
// the reminders, Done and Percent complete only as the form's rows;
// Related to with where its suggestions are asked — which answer by the
// records the session has loaded; a new task is the same form.
func TestTaskEditIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-tasks-3")
	if !strings.Contains(frame, `<span class="tc-badge is-alert">overdue by 3 days</span>`) || !strings.Contains(frame, `<div class="tc-rubric">Source</div>`) {
		t.Fatal("s-tasks-3 draws no lateness or no Source: read the frame again")
	}
	app := newMockupApp(t)
	openList(t, app, "/app/calendar?from=2026-08-10") // «Забрать документы» is loaded
	openList(t, app, "/app/tasks")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "tasks", "dela")
	rec := list + "/aa10-77bc-0e31"

	raw := app.h.do(get(rec+"/edit", app.sess))
	var generic map[string]json.RawMessage
	_ = json.Unmarshal(raw.Body.Bytes(), &generic)
	for _, gone := range []string{"done", "percent", "reminders"} {
		if _, ok := generic[gone]; ok {
			t.Errorf("the form still carries %q beside its rows (ЗД5, ЗД6)", gone)
		}
	}
	page := decode[view.TaskEditPage](t, raw)
	if page.Name != "Отправить документы в МФЦ" || dangerOf(page.Subtitle) != "overdue by 3 days" {
		t.Errorf("head = %q %+v", page.Name, page.Subtitle)
	}
	if page.Source.Path == "" || page.Source.Compared != "not changed on server" || page.Source.LastRead == "" {
		t.Errorf("source = %+v", page.Source)
	}
	rows := map[string]view.FormRow{}
	for _, set := range page.Form.Sets {
		for _, r := range set.Rows {
			rows[r.Name] = r
		}
	}
	if rows["categories"].Hint != "" {
		t.Errorf("Tags has a hint the frame does not draw: %q", rows["categories"].Hint)
	}
	day := rows["allday"]
	if !slices.Equal(strings.Fields(day.Disables), []string{"start.time", "due.time"}) {
		t.Errorf("All day = %+v, want it to switch both times off", day)
	}
	if day.Value == "on" && (!rows["start.time"].ReadOnly || !rows["due.time"].ReadOnly) {
		t.Errorf("All day ticked, the times are not off: %+v %+v", rows["start.time"], rows["due.time"])
	}
	if rows["percent"].Value != "0" || rows["reminder.0"].Value != "-P1D" {
		t.Errorf("percent = %+v, reminder = %+v", rows["percent"], rows["reminder.0"])
	}
	related := rows["related"]
	chosen := slices.IndexFunc(related.Options, func(o view.Option) bool { return o.Selected })
	if related.Suggest == "" || related.Value != "1d55-90fe-3c27" || chosen < 0 || related.Options[chosen].Label != "Забрать документы" {
		t.Fatalf("Related to = %+v", related)
	}
	// Saved as shown, the form writes nothing new: «0» is no percent.
	before, _, _ := app.servers["nextcloud"].Object(davtest.CalendarHome + "dela/aa10-77bc-0e31.ics")
	if resp := app.h.do(postForm(rec+"/edit", app.sess, formFields(page.Form))); resp.Code != 303 {
		t.Fatalf("save as shown: %d %s", resp.Code, resp.Body.String())
	}
	if after, _, _ := app.servers["nextcloud"].Object(davtest.CalendarHome + "dela/aa10-77bc-0e31.ics"); strings.Contains(after, "PERCENT-COMPLETE") && !strings.Contains(before, "PERCENT-COMPLETE") {
		t.Errorf("saving as shown wrote a percent:\n%s", after)
	}

	// The suggestions of Related to: the loaded records whose title holds
	// what was typed, with their mark and «kind · day» (s-tasks-10).
	sug := decode[view.SuggestList](t, app.h.do(get(related.Suggest+"?q="+url.QueryEscape("заб"), app.sess)))
	if len(sug.Items) != 1 || sug.Items[0].Value != "1d55-90fe-3c27" || sug.Items[0].Meta != "event · 11 Aug" || sug.Items[0].Colour == "" ||
		len(sug.Items[0].Name) < 2 || sug.Items[0].Name[0].Mark != view.RunStrong {
		t.Errorf("suggestions = %+v", sug)
	}
	if self := decode[view.SuggestList](t, app.h.do(get(related.Suggest+"?q="+url.QueryEscape("документы"), app.sess))); slices.ContainsFunc(self.Items, func(i view.SuggestItem) bool { return i.Value == "aa10-77bc-0e31" }) {
		t.Errorf("the task suggests itself: %+v", self)
	}

	// A new task: the same form, «New task», nothing under the title — the
	// list is the choice in the head (s-tasks-14, Typecase's note 72).
	fresh := decode[view.TaskEditPage](t, app.h.do(get(list+"/new", app.sess)))
	if fresh.Name != "New task" || len(fresh.Subtitle) != 0 || fresh.Delete != nil || fresh.Source.Path != "" {
		t.Errorf("new task = %q %+v %+v %+v", fresh.Name, fresh.Subtitle, fresh.Delete, fresh.Source)
	}
}
