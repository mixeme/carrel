package web_test

import (
	"net/url"
	"regexp"
	"slices"
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
	// The tick posts to the sheet.
	if !strings.Contains(tick.Href, "/done?") {
		t.Errorf("the linked row's tick does not ask which copy: %+v", tick)
	}
	sheet := decode[view.CopiesSheet](t, f.h.do(postForm(tick.Href, f.sess, url.Values{})))
	title, note, primary := frameSheet(t, "s-tasks-16")
	// The frame's one sentence (Typecase's note 73).
	if string(sheet.Title) != title || string(sheet.Note) != note ||
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
	// Edit leads to the sheet.
	edit := actionByID(page.Actions, "record.edit")
	if edit == nil || !strings.HasSuffix(edit.Href, "/t1/copies") {
		t.Fatalf("Edit = %+v", edit)
	}
	sheet := decode[view.CopiesSheet](t, f.h.do(get(edit.Href, f.sess)))
	title, _, primary := frameSheet(t, "s-contacts-32")
	// One sentence, as the owner shortened it (Typecase's note 73); how many
	// copies, the head says.
	if string(sheet.Title) != title || string(sheet.Note) != "Saving changes one copy only; the others stay as they are." {
		t.Errorf("sheet = %+v, want s-contacts-32's %q", sheet, title)
	}
	// The same sheet by the group's address, the UID of a copy of it
	// (s-contacts-35: /app/{kind}/linked/{uid}/copies); a UID in no group is
	// not found.
	byUID := f.h.do(get("/app/tasks/linked/t1/copies", f.sess))
	if byUID.Code != 200 || decode[view.CopiesSheet](t, byUID).Subject != sheet.Subject {
		t.Errorf("the sheet by the group's address: %d %s", byUID.Code, byUID.Body)
	}
	if rec := f.h.do(get("/app/tasks/linked/nobody/copies", f.sess)); rec.Code != 404 {
		t.Errorf("a UID in no group: %d", rec.Code)
	}
	// What each copy holds otherwise than the linked record shows
	// (from-typecase note 117 p. 4): the priority of the copy not chosen.
	row := copyRow(t, sheet)
	metas := []string{string(row.Options[0].Meta), string(row.Options[1].Meta)}
	if row.Kind != view.InputRadio || len(row.Options) != 2 || !row.Options[0].Selected ||
		!strings.HasPrefix(metas[0], "the copy you opened") ||
		!(slices.Contains(metas, "the copy you opened · Priority differs") != slices.Contains(metas, "Priority differs")) {
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
	// The form says which copy it saves by its List — the chosen copy's —,
	// and no line «Saving changes this copy only…» (the owner's decision,
	// Typecase's note 72).
	form := decode[view.TaskEditPage](t, f.h.do(get(to, f.sess)))
	chosen := ""
	if r := formRowOf(form.Form, "notebook"); r != nil {
		for _, o := range r.Options {
			if o.Selected {
				chosen = string(o.Label)
			}
		}
	}
	if !strings.HasSuffix(chosen, "Home") || len(form.Notices) != 0 {
		t.Errorf("the form of the chosen copy: List %q, notices %+v", chosen, form.Notices)
	}
	own := decode[view.TaskEditPage](t, f.h.do(get(sheet.Form.Action+"?copy="+url.QueryEscape(row.Options[0].Value), f.sess)))
	plain := decode[view.TaskEditPage](t, f.h.do(get(sheet.Form.Action, f.sess)))
	if len(own.Notices) != 0 || len(plain.Notices) != 0 {
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
	// Cancel goes back to the page under the sheet, not to the record's
	// (Лс30); a link without back is followed from its Referer (Лс31), and
	// the sheet with another collection chosen keeps it.
	list := "/app/tasks/" + f.id + "/" + url.PathEscape(searchWork)
	if c := actionByID(move.Actions, "record.move.cancel"); c == nil || c.Href != list {
		t.Errorf("Cancel of the move sheet opened from the list = %+v", c)
	}
	for _, o := range move.Target.Options {
		if o.Href != "" && !strings.Contains(o.Href, "back="+url.QueryEscape(list)) {
			t.Errorf("the sheet with %s chosen forgets where it was opened: %s", o.Label, o.Href)
		}
	}
	followed := get(record(searchWork, "t1")+"/copies", f.sess)
	followed.Header.Set("Referer", "http://"+followed.Host+list)
	fromList := decode[view.CopiesSheet](t, f.h.do(followed))
	if fromList.Under == nil || fromList.Under.Tasks == nil {
		t.Errorf("a sheet followed from the list stands over: %+v", fromList.Under)
	}
	if c := actionByID(fromList.Actions, "copies.cancel"); c == nil || c.Href != list {
		t.Errorf("Cancel of the copies sheet followed from the list = %+v", c)
	}
	// The quick sheet too (Лс28): over the page New was pressed on.
	quick := decode[view.NewSheet](t, f.h.do(get(list+"/new/quick?back="+url.QueryEscape(list), f.sess)))
	if quick.Under == nil || quick.Under.Tasks == nil {
		t.Errorf("under the quick sheet: %+v", quick.Under)
	}
	req := get(record(searchWork, "t1")+"/copies", f.sess)
	req.Header.Set("Carrel-View", "fragment")
	req.Header.Set("Carrel-Target", "sheet")
	if frag := decode[view.CopiesSheet](t, f.h.do(req)); frag.Under != nil {
		t.Errorf("a fragment carries a screen under it: %+v", frag.Under)
	}
}

// Лс34: Mark done of the sheet with no copy ticked marks nothing and says
// so — the sheet again, nothing ticked, and why — rather than the sheet
// again with the opened copy ticked, as if the tick had been pressed.
func TestMarkDoneWithNoCopyTickedSaysSo(t *testing.T) {
	f, record := linkedTasks(t)
	sheet := decode[view.CopiesSheet](t, f.h.do(postForm(record(searchWork, "t1")+"/done?from=all", f.sess, url.Values{})))
	form := formFields(sheet.Form)
	form.Del("copy")
	again := decode[view.CopiesSheet](t, f.h.do(postForm(sheet.Form.Action, f.sess, form)))
	if len(again.Form.Errors) != 1 || again.Form.Errors[0].Name != "copy" {
		t.Fatalf("errors = %+v", again.Form.Errors)
	}
	for _, o := range copyRow(t, again).Options {
		if o.Selected {
			t.Errorf("%s ticked again", o.Label)
		}
	}
	if b, _, _ := f.srv.Object(searchWork + "t1.ics"); strings.Contains(b, "STATUS:COMPLETED") {
		t.Error("a copy was marked")
	}
}
