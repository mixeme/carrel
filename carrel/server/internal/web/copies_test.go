package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/caldav"
)

// linkedTasks are two task lists, Work and Home, each with a copy of one
// task — the same title and day, another priority —, linked by the
// duplicates screen.
func linkedTasks(t *testing.T) (davFixture, func(col, uid string) string) {
	t.Helper()
	srv := searchServer(t)
	home := davtest.CalendarHome + "home/"
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: home, Name: "Home"})
	srv.Seed(searchWork+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260801T000000Z\nSUMMARY:Pack\nDUE;VALUE=DATE:20260812\nPRIORITY:1\nEND:VTODO"))
	srv.Seed(home+"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260801T000000Z\nSUMMARY:Pack\nDUE;VALUE=DATE:20260812\nPRIORITY:5\nEND:VTODO"))
	f := connectFixture(t, srv, searchWork, nil)
	list := func(col string) string { return "/app/tasks/" + f.id + "/" + url.PathEscape(col) }
	for _, col := range []string{searchWork, home} {
		f.h.do(get(list(col)+"?seg=all", f.sess))
	}
	g := onlyGroup(t, decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess))))
	f.h.do(postForm(actionOf(t, g, "duplicates.link").Href, f.sess, url.Values{}))
	record := func(col, uid string) string { return list(col) + "/" + uid }
	return f, record
}

// copyRow is the row of copies of a sheet.
func copyRow(t *testing.T, s view.CopiesSheet) view.FormRow {
	t.Helper()
	for _, set := range s.Form.Sets {
		for _, r := range set.Rows {
			if r.Name == "copy" {
				return r
			}
		}
	}
	t.Fatalf("the sheet has no copies: %+v", s.Form)
	return view.FormRow{}
}

// frameSheet is what a frame's sheet says: its title, the line under its
// head, its primary button.
func frameSheet(t *testing.T, id string) (title, note, primary string) {
	t.Helper()
	frame := mockupFrame(t, id)
	m := regexp.MustCompile(`(?s)<b class="tc-sheet__title">([^<]*)</b>.*?<div class="tc-sheet__body">\s*<span[^>]*>([^<]*)</span>.*?<button class="tc-btn is-primary" type="submit">([^<]*)</button>`).FindStringSubmatch(frame)
	if m == nil {
		t.Fatalf("%s draws no sheet of copies: read the frame again", id)
	}
	return m[1], m[2], m[3]
}

// The tick of a linked task in the combined list opens «Which copy to mark
// done?» (s-tasks-16; from-typecase note 57, Р-58: the row is one for every
// copy): the copies as boxes, the first ticked, both may be; Mark done marks
// the ticked ones, a copy not ticked stays as it is.
func TestWhichCopyToMarkDone(t *testing.T) {
	f, _ := linkedTasks(t)
	home := davtest.CalendarHome + "home/"
	first := decode[view.TasksPage](t, f.h.do(get("/app/tasks", f.sess)))
	if first.Empty != nil && len(first.Empty.Actions) > 0 {
		f.h.do(postForm(first.Empty.Actions[0].Href, f.sess, url.Values{})) // Tick every list
	}
	all := combined(t, f.h, f.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	var tick *view.Action
	for _, g := range all.Groups {
		for _, r := range g.Rows {
			if r.Summary == "Pack" {
				tick = r.Tick
			}
		}
	}
	if tick == nil || tick.Method != "POST" {
		t.Fatalf("the linked row's tick = %+v in %+v", tick, all.Groups)
	}
	// The client draws the sheet since v0.4.42 (copiesOnPages): the tick
	// posts to it.
	if !strings.Contains(tick.Href, "/done?") {
		t.Errorf("the linked row's tick does not ask which copy: %+v", tick)
	}
	sheet := decode[view.CopiesSheet](t, f.h.do(postForm(tick.Href, f.sess, url.Values{})))
	title, note, primary := frameSheet(t, "s-tasks-16")
	if string(sheet.Title) != title || string(sheet.Note) != "This task stands in two lists. Tick the copy that is done; a copy that is not ticked stays as it is." ||
		!strings.HasPrefix(string(sheet.Subject), "Pack · 2 copies") {
		t.Errorf("sheet = %+v, want s-tasks-16's %q, %q", sheet, title, note)
	}
	row := copyRow(t, sheet)
	if row.Kind != view.InputCheck || len(row.Options) != 2 || !row.Options[0].Selected || row.Options[1].Selected ||
		!strings.HasPrefix(string(row.Options[0].Meta), "priority ") || row.Options[0].Colour == "" ||
		!strings.HasPrefix(string(row.Options[1].Meta), "Priority differ · priority ") {
		t.Errorf("copies = %+v", row)
	}
	var mark *view.Action
	for i, a := range sheet.Actions {
		if a.Primary {
			mark = &sheet.Actions[i]
		}
	}
	if mark == nil || string(mark.Label) != primary || !mark.Submits || sheet.Form.Method != "POST" {
		t.Fatalf("Mark done = %+v, form %s %s", mark, sheet.Form.Method, sheet.Form.Action)
	}
	form := formFields(sheet.Form)
	form["copy"] = []string{row.Options[0].Value, row.Options[1].Value}
	resp := f.h.do(postForm(sheet.Form.Action, f.sess, form))
	if resp.Code != 303 || resp.Header().Get("Location") != "/app/tasks" {
		t.Fatalf("Mark done: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	for _, p := range []string{searchWork + "t1.ics", home + "t2.ics"} {
		if b, _, _ := f.srv.Object(p); !strings.Contains(b, "STATUS:COMPLETED") {
			t.Errorf("%s is not done:\n%s", p, b)
		}
	}
}

// Edit of a linked record opens «Which copy to edit?» first (s-contacts-32,
// one sheet for every kind; note 57): the copies as radios, the opened one
// chosen; Edit this copy opens the form of the copy chosen, and that form
// says, before its fields, that it saves this copy only (s-contacts-31).
func TestWhichCopyToEdit(t *testing.T) {
	f, record := linkedTasks(t)
	home := davtest.CalendarHome + "home/"
	page := decode[view.TaskPage](t, f.h.do(get(record(searchWork, "t1"), f.sess)))
	// The client draws the sheet since v0.4.42 (copiesOnPages): Edit leads
	// to it.
	edit := actionByID(page.Actions, "record.edit")
	if edit == nil || !strings.HasSuffix(edit.Href, "/t1/copies") {
		t.Fatalf("Edit = %+v", edit)
	}
	sheet := decode[view.CopiesSheet](t, f.h.do(get(edit.Href, f.sess)))
	title, _, primary := frameSheet(t, "s-contacts-32")
	if string(sheet.Title) != title || string(sheet.Note) != "This task stands in two lists. Saving changes one copy only; the others stay as they are." {
		t.Errorf("sheet = %+v, want s-contacts-32's %q", sheet, title)
	}
	row := copyRow(t, sheet)
	if row.Kind != view.InputRadio || len(row.Options) != 2 || !row.Options[0].Selected ||
		string(row.Options[0].Meta) != "the copy you opened · priority 1" || string(row.Options[1].Meta) != "Priority differ · priority 5" {
		t.Errorf("copies = %+v", row)
	}
	if sheet.Form.Method != "GET" || !strings.HasSuffix(sheet.Form.Action, "/t1/edit") {
		t.Errorf("form = %s %s", sheet.Form.Method, sheet.Form.Action)
	}
	primaryOK := false
	for _, a := range sheet.Actions {
		primaryOK = primaryOK || (a.Primary && string(a.Label) == primary && a.Submits)
	}
	if !primaryOK {
		t.Errorf("actions = %+v, want %q", sheet.Actions, primary)
	}
	resp := f.h.do(get(sheet.Form.Action+"?copy="+url.QueryEscape(row.Options[1].Value), f.sess))
	to := resp.Header().Get("Location")
	if resp.Code != 303 || !strings.HasPrefix(to, record(home, "t2")+"/edit?copy=") {
		t.Fatalf("Edit this copy: %d %s", resp.Code, to)
	}
	form := decode[view.TaskEditPage](t, f.h.do(get(to, f.sess)))
	said := false
	for _, n := range form.Notices {
		said = said || (n.Role == view.NoticeInfo && strings.HasPrefix(string(n.Body), "Saving changes this copy only — ") &&
			strings.HasSuffix(string(n.Body), "Home. The others stay as they are."))
	}
	if !said {
		t.Errorf("the form does not say which copy it saves: %+v", form.Notices)
	}
	// The same copy chosen opens its own form, which says it as well; the
	// form opened without the sheet does not (s-notes-20).
	own := decode[view.TaskEditPage](t, f.h.do(get(sheet.Form.Action+"?copy="+url.QueryEscape(row.Options[0].Value), f.sess)))
	plain := decode[view.TaskEditPage](t, f.h.do(get(sheet.Form.Action, f.sess)))
	if len(own.Notices) != 1 || len(plain.Notices) != 0 {
		t.Errorf("notices: chosen %+v, plain %+v", own.Notices, plain.Notices)
	}
	// A record in no group opens its form at once.
	f.srv.Seed(searchWork+"t3.ics", ics("BEGIN:VTODO\nUID:t3\nDTSTAMP:20260801T000000Z\nSUMMARY:Alone\nEND:VTODO"))
	alone := decode[view.TaskPage](t, f.h.do(get(record(searchWork, "t3"), f.sess)))
	if e := actionByID(alone.Actions, "record.edit"); e == nil || !strings.HasSuffix(e.Href, "/t3/edit") {
		t.Errorf("an unlinked task's Edit = %+v", e)
	}
	if resp := f.h.do(get(record(searchWork, "t3")+"/copies", f.sess)); resp.Code != 303 || !strings.HasSuffix(resp.Header().Get("Location"), "/t3/edit") {
		t.Errorf("the sheet of a task in no group: %d %s", resp.Code, resp.Header().Get("Location"))
	}
}

// A sheet opened by its address without scripts stands over the screen it
// was opened from (the owner's decision of 15.10.2026; s-contacts-2, the
// Overlay of Typecase): its answer carries that page — back's, else the
// record's or the list's it belongs to. A sheet asked for as a fragment
// carries none.
func TestASheetPageCarriesTheScreenUnderIt(t *testing.T) {
	f, record := linkedTasks(t)
	copies := decode[view.CopiesSheet](t, f.h.do(get(record(searchWork, "t1")+"/copies", f.sess)))
	if copies.Under == nil || copies.Under.Task == nil || copies.Under.Task.Summary != "Pack" {
		t.Errorf("under the copies sheet: %+v", copies.Under)
	}
	move := decode[view.MoveSheet](t, f.h.do(get(record(searchWork, "t1")+"/move?back="+url.QueryEscape("/app/tasks/"+f.id+"/"+url.PathEscape(searchWork)), f.sess)))
	if move.Under == nil || move.Under.Tasks == nil {
		t.Errorf("under the move sheet opened from the list: %+v", move.Under)
	}
	odd := decode[view.MoveSheet](t, f.h.do(get(record(searchWork, "t1")+"/move?back="+url.QueryEscape("https://elsewhere.example/app/x"), f.sess)))
	if odd.Under == nil || odd.Under.Task == nil {
		t.Errorf("a back that is no address of the app is not the record's page: %+v", odd.Under)
	}
	req := get(record(searchWork, "t1")+"/copies", f.sess)
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "sheet")
	if frag := decode[view.CopiesSheet](t, f.h.do(req)); frag.Under != nil {
		t.Errorf("a fragment carries a screen under it: %+v", frag.Under)
	}
}
