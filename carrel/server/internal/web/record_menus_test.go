package web_test

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

var (
	// An item that picks a file is the label of its file input (Typecase's
	// task 109): «From this computer» is a label, not a button; «Upload»
	// after the items (tc-nojs) is shown only without a script and sends the
	// file the item picked — the same action, no item of the model's menu.
	frameMenuItem = regexp.MustCompile(`(?s)<span class="tc-menu__sep"></span>|<button class="tc-menu__item[^"]*"[^>]*role="menuitem"[^>]*>(.*?)</button>|<label class="tc-menu__item[^"]*" role="menuitem">(.*?)</label>`)
	frameButton   = regexp.MustCompile(`(?s)<button\b[^>]*>(.*?)</button>`)
	// A button with its menu, closed, is the summary of a details (Typecase's
	// task 60): Attach with its arrow in a rubric (task 71, s-agenda-1).
	frameButtonOrSummary = regexp.MustCompile(`(?s)<(?:button|summary)\b[^>]*>(.*?)</(?:button|summary)>`)
	frameIcon            = regexp.MustCompile(`#(i-[\w-]+)`)
	frameTag             = regexp.MustCompile(`(?s)<svg.*?</svg>|<[^>]+>`)
	// The row a file picked in an edit becomes (Typecase ac658b0,
	// MenuItem.Row) waits in a template inside its item's label: the words
	// of that row («{name}», Remove), not of the item.
	frameTemplate = regexp.MustCompile(`(?s)<template\b.*?</template>`)
)

// frameText is what a piece of a frame says, tags and icons dropped.
func frameText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(frameTag.ReplaceAllString(frameTemplate.ReplaceAllString(s, " "), " "))), " ")
}

// frameMenu is the «⋯» a frame draws open, item by item: the label, the icon
// in brackets, and «— » before an item a line stands above.
func frameMenu(t *testing.T, id string) []string {
	t.Helper()
	return frameMenuIn(t, id, mockupFrame(t, id))
}

// frameMenuIn is frameMenu of a part of the frame id: where the frame draws
// more than one menu (the bar's «⋯» with its twins beside a panel's, Typecase
// ac658b0), the part that holds the one meant.
func frameMenuIn(t *testing.T, id, part string) []string {
	t.Helper()
	var out []string
	line := false
	for _, m := range frameMenuItem.FindAllStringSubmatch(part, -1) {
		if strings.HasPrefix(m[0], "<span") {
			line = true
			continue
		}
		if open, _, _ := strings.Cut(m[0], ">"); strings.Contains(open, "tc-nojs") {
			continue
		}
		body := m[1] + m[2]
		icon := ""
		if i := frameIcon.FindStringSubmatch(body); i != nil {
			icon = i[1]
		}
		item := frameText(body) + " [" + icon + "]"
		if line {
			item, line = "— "+item, false
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		t.Fatalf("%s draws no menu: the frame changed, read it again", id)
	}
	return out
}

// serverMenu is a menu the server sent, written the way frameMenu writes one.
func serverMenu(items []view.MenuItem) []string {
	out := []string{}
	for _, it := range items {
		item := string(it.Label) + " [" + it.Icon + "]"
		if it.Separator {
			item = "— " + item
		}
		out = append(out, item)
	}
	return out
}

func sameMenu(t *testing.T, what string, got []view.MenuItem, frame string, drop ...string) {
	t.Helper()
	want := []string{}
	for _, item := range frameMenu(t, frame) {
		if !strings.Contains(strings.Join(drop, "\n"), item) {
			want = append(want, item)
		}
	}
	if g := serverMenu(got); !equalStrings(g, want) {
		t.Errorf("%s «⋯» = %q,\nwant (%s) %q", what, g, frame, want)
	}
}

// drawn says every action the server shows as a button is a button of the
// frame: by its label, or by its icon where the frame draws the button bare.
func drawn(t *testing.T, what string, actions []view.Action, frame string, skip ...string) {
	t.Helper()
	labels, icons := map[string]bool{}, map[string]bool{}
	for _, m := range frameButtonOrSummary.FindAllStringSubmatch(mockupFrame(t, frame), -1) {
		if label := frameText(m[1]); label != "" {
			labels[label] = true
		} else if i := frameIcon.FindStringSubmatch(m[1]); i != nil {
			icons[i[1]] = true
		}
	}
	for _, a := range actions {
		if strings.Contains(strings.Join(skip, " "), a.ID) {
			continue
		}
		if !labels[string(a.Label)] && !(a.Icon != "" && icons[a.Icon]) {
			t.Errorf("%s: %s «%s» is no button of %s", what, a.ID, a.Label, frame)
		}
	}
}

func ids(actions []view.Action) []string {
	out := []string{}
	for _, a := range actions {
		out = append(out, a.ID)
	}
	return out
}

// from-typecase note 1 (owner's decision 28.09.2026): what stands on a
// record and what is under its «⋯» is data — Actions are the buttons, Menu
// the menu in the frame's order. The menus are read from the frames drawn
// open (s-contacts-8…11, s-agenda-4/5, s-tasks-4/6, s-notes-12/14 and the
// rows of components-1…5, s-files-9/10), labels and icons both.
func TestContactMenusAreTheFrames(t *testing.T) {
	f := contactsFixture(t, false, annaCard)
	card := f.path("contacts") + "/anna"

	page := decode[view.ContactPage](t, f.h.do(get(card, f.sess)))
	sameMenu(t, "card", page.Menu, "s-contacts-10")
	if got := ids(page.Actions); !equalStrings(got, []string{"record.edit", "record.print", "record.delete"}) {
		t.Errorf("card actions = %v", got)
	}
	drawn(t, "card", page.Actions, "s-contacts-6")

	panel := decode[detailsFragment](t, f.h.do(fragment(get(card, f.sess), "details"))).Body
	sameMenu(t, "panel", panel.Menu, "s-contacts-8")
	if got := ids(panel.Actions); !equalStrings(got, []string{"record.edit", "contact.timeline", "record.delete"}) {
		t.Errorf("panel actions = %v", got)
	}
	if a := actionByID(panel.Actions, "contact.timeline"); a == nil || a.Href != card+"/timeline" {
		t.Errorf("All records = %+v, want the timeline", a)
	}
	drawn(t, "panel", panel.Actions, "s-contacts-1")

	timeline := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	sameMenu(t, "timeline card", timeline.Card.Menu, "s-contacts-11")
	if got := ids(timeline.Card.Actions); !equalStrings(got, []string{"timeline.card", "record.edit", "record.delete"}) {
		t.Errorf("timeline card actions = %v", got)
	}
	drawn(t, "timeline card", timeline.Card.Actions, "s-contacts-7")

	list := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	sameMenu(t, "row", list.Groups[0].Rows[0].Menu, "components-1")
}

func TestCalendarMenusAreTheFrames(t *testing.T) {
	now := time.Now().UTC()
	at := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.UTC).Format("20060102T150405Z")
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:"+at+"\nSUMMARY:Review\nEND:VEVENT"),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nDESCRIPTION:Text\nEND:VJOURNAL"))

	event := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e1", f.sess)))
	sameMenu(t, "event card", event.Menu, "s-agenda-5")
	// Attach is not drawn on the full card (to-typecase task 9).
	drawn(t, "event card", event.Actions, "s-agenda-2", "attach.add")
	if n := len(event.Actions); n == 0 || event.Actions[0].ID != "record.edit" || event.Actions[n-1].ID != "record.delete" || !event.Actions[n-1].Danger {
		t.Errorf("event actions = %v: Edit first, the deletion last and marked", ids(event.Actions))
	}
	panel := decode[detailsFragment](t, f.h.do(fragment(get(f.path("calendar")+"/e1", f.sess), "details"))).Body
	sameMenu(t, "event panel", panel.Menu, "s-agenda-4")
	if got := ids(panel.Actions); !equalStrings(got, []string{"record.edit", "attach.add", "record.delete"}) {
		t.Errorf("event panel actions = %v", got)
	}
	drawn(t, "event panel", panel.Actions, "s-agenda-1")
	agenda := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar"), f.sess)))
	found := false
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			if r.UID == "e1" {
				found = true
				sameMenu(t, "agenda row", r.Menu, "components-2")
			}
		}
	}
	if !found {
		t.Fatal("the event is not in this week's agenda")
	}

	task := decode[view.TaskPage](t, f.h.do(get(f.path("tasks")+"/t1", f.sess)))
	sameMenu(t, "task card", task.Menu, "s-tasks-6")
	drawn(t, "task card", task.Actions, "s-tasks-2", "attach.add")
	if got := ids(task.Actions); len(got) < 3 || got[0] != "record.edit" || got[1] != "task.done" || got[len(got)-1] != "record.delete" {
		t.Errorf("task actions = %v: Edit, Mark done, …, the deletion last", got)
	}
	panel = decode[detailsFragment](t, f.h.do(fragment(get(f.path("tasks")+"/t1", f.sess), "details"))).Body
	sameMenu(t, "task panel", panel.Menu, "s-tasks-4")
	if got := ids(panel.Actions); !equalStrings(got, []string{"record.edit", "attach.add", "record.delete"}) {
		t.Errorf("task panel actions = %v", got)
	}
	drawn(t, "task panel", panel.Actions, "s-tasks-1")
	tasks := decode[view.TasksPage](t, f.h.do(get(f.path("tasks"), f.sess)))
	row := tasks.Groups[0].Rows[0]
	sameMenu(t, "task row", row.Menu, "components-3")
	if row.Tick == nil || row.Tick.ID != "task.done" {
		t.Errorf("the tick of the row = %+v", row.Tick)
	}

	note := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	// «Export .ics» after Copy, beside the frame (Р20 of
	// scans/kinds-parity.md: the journal whole, as an event's and a task's;
	// Typecase's task 174): the frame's items around it, and it required.
	if g := serverMenu(note.Meta.Menu); len(g) != 4 || g[2] != "Export .ics [i-down]" {
		t.Errorf("note page «⋯» = %q, want Export .ics third", g)
	} else {
		sameMenu(t, "note page", append(append([]view.MenuItem(nil), note.Meta.Menu[:2]...), note.Meta.Menu[3:]...), "s-notes-14")
	}
	// Print over the text since the client's scan of the notes (НГ2); New
	// task before it, every kind's (Р15 of scans/kinds-parity.md) — the
	// frame does not draw it yet, Typecase's task 168.
	if got := ids(note.Doc.Bar); !equalStrings(got, []string{"record.edit", "note.export", "note.task", "record.print"}) {
		t.Errorf("note bar = %v", got)
	}
	drawn(t, "note bar", note.Doc.Bar, "s-notes-2", "note.task")
	if got := ids(note.Meta.Actions); !equalStrings(got, []string{"attach.add", "record.delete"}) {
		t.Errorf("note column = %v", got)
	}
	drawn(t, "note column", note.Meta.Actions, "s-notes-2")
	panel = decode[detailsFragment](t, f.h.do(fragment(get(f.path("notes")+"/n1", f.sess), "details"))).Body
	sameMenu(t, "note panel", panel.Menu, "s-notes-12")
	if got := ids(panel.Actions); !equalStrings(got, []string{"record.edit", "record.open", "attach.add"}) {
		t.Errorf("note panel actions = %v", got)
	}
	// The panel draws no Attach (to-typecase task 9); the page of the note
	// is a bare icon.
	drawn(t, "note panel", panel.Actions, "s-notes-1", "attach.add")
	notes := decode[view.NotesPage](t, f.h.do(get(f.path("notes"), f.sess)))
	sameMenu(t, "note row", notes.Groups[0].Rows[0].Menu, "components-4")
}

func TestFileRowMenusAreTheFrames(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"report.pdf", "0123456789")
	f.srv.SeedFile(filesDocs+"sub/deep.md", "x")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	sameMenu(t, "file row", fileRow(t, page, "report.pdf").Menu, "s-files-9")
	sameMenu(t, "folder row", fileRow(t, page, "sub").Menu, "s-files-10")
	if a := menuAction(fileRow(t, page, "report.pdf").Menu, "file.open"); a == nil || a.Href != f.at("", "report.pdf") {
		t.Errorf("Open of a file = %+v, want its address in the folder", a)
	}
	// The bar over the folder draws no Publish (s-files-1): a folder is
	// published from its row in the folder above.
	if a := actionByID(page.Bar.Actions, "publish"); a != nil {
		t.Errorf("the folder bar publishes: %+v", a)
	}
	// The head of Files is Upload, New folder and Details; its «⋯»
	// (s-files-7) holds Print alone — the same address with print=1, as the
	// lists' Print (the client's scan, ФБ7): no Import, no Export.
	if want := []string{"Print [i-print]"}; !equalStrings(frameMenu(t, "s-files-7"), want) {
		t.Fatalf("s-files-7 = %q, want %q: read the frame again", frameMenu(t, "s-files-7"), want)
	}
	// Between them stand the operations of the selection bar, the buttons
	// of the selection form (ФН5): the bar is drawn while something is
	// selected and, without scripts, always.
	if got := ids(page.Bar.Actions); !equalStrings(got, []string{"file.upload", "folder.new",
		"file.download", "file.rename", "file.move", "file.copy", "file.delete", "files.details", "list.print"}) {
		t.Errorf("files head = %q, want Upload, New folder, the selection's operations, Details and Print", got)
	}
}

// S-19: Export in the menu of one record hands over that record, named
// after it — for every kind, not only a note. A contact asked for with ?uid=
// used to come back as the whole book.
func TestExportOneRecordOfEveryKind(t *testing.T) {
	f := contactsFixture(t, false, annaCard, "BEGIN:VCARD\nVERSION:3.0\nUID:bob\nFN:Bob Other\nEND:VCARD\n")
	page := decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/anna", f.sess)))
	href := actionHref(t, menuActions(page.Menu), "record.export")
	if href != f.path("contacts")+"/export?uid=anna" {
		t.Fatalf("export = %q", href)
	}
	u, _ := url.Parse(href)
	resp := f.h.do(get(u.RequestURI(), f.sess))
	body := resp.Body.String()
	if resp.Code != 200 || !strings.HasPrefix(resp.Header().Get("Content-Type"), "text/vcard") ||
		!strings.Contains(resp.Header().Get("Content-Disposition"), "anna-kovaleva.vcf") {
		t.Fatalf("contact export: %d %v", resp.Code, resp.Header())
	}
	if strings.Count(body, "BEGIN:VCARD") != 1 || strings.Contains(body, "Bob Other") {
		t.Errorf("one contact exported as:\n%s", body)
	}

	c := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nEND:VEVENT"),
		"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260101T000000Z\nDTSTART:20260116T100000Z\nSUMMARY:Not this one\nEND:VEVENT"),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"))
	for _, one := range []struct{ kind, uid, name, not string }{
		{"calendar", "e1", "review.ics", "Not this one"},
		{"tasks", "t1", "slides.ics", "Review"},
	} {
		resp := c.h.do(get(c.path(one.kind)+"/export?uid="+one.uid, c.sess))
		body := resp.Body.String()
		if resp.Code != 200 || !strings.Contains(resp.Header().Get("Content-Disposition"), one.name) {
			t.Errorf("%s export: %d %v", one.kind, resp.Code, resp.Header())
		}
		if strings.Count(body, "BEGIN:VCALENDAR") != 1 || !strings.Contains(body, "UID:"+one.uid) || strings.Contains(body, one.not) {
			t.Errorf("%s exported as:\n%s", one.kind, body)
		}
	}
	if resp := c.h.do(get(c.path("calendar")+"/export?uid=nosuch", c.sess)); resp.Code != 404 {
		t.Errorf("unknown event: %d, want 404", resp.Code)
	}
}

// from-typecase note 2: the edit page of an event and of a task ends in
// its deletion, with the line the frame writes under it (s-agenda-3,
// s-tasks-3). One Save and one Cancel stay in the form.
func TestDeleteOnTheEditPage(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nRRULE:FREQ=WEEKLY\nSUMMARY:Review\nEND:VEVENT"),
		"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Once\nEND:VEVENT"),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"))
	series := regexp.MustCompile(`>Delete event</button>([^<]+)<`).FindStringSubmatch(mockupFrame(t, "s-agenda-3"))
	if series == nil {
		t.Fatal("s-agenda-3 writes no line under Delete event: read the frame again")
	}

	ev := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess)))
	if ev.Delete == nil || ev.Delete.ID != "record.delete" || !ev.Delete.Danger || ev.Delete.Label != "Delete event" || ev.Delete.Method != "POST" {
		t.Fatalf("event edit delete = %+v", ev.Delete)
	}
	if ev.DeleteNote != view.Text(strings.TrimSpace(series[1])) {
		t.Errorf("series line = %q, want %q", ev.DeleteNote, series[1])
	}
	saves := 0
	for _, a := range ev.Form.Actions {
		if a.ID == "record.save" || a.ID == "record.cancel" {
			saves++
		}
	}
	if saves != 2 {
		t.Errorf("form actions = %v, want one Save and one Cancel", ids(ev.Form.Actions))
	}
	once := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e2/edit", f.sess)))
	if once.Delete == nil || once.DeleteNote != "" {
		t.Errorf("a single event: delete %+v, line %q", once.Delete, once.DeleteNote)
	}

	// Under Delete task no line (to-typecase task 8): deleting is one button.
	if line := regexp.MustCompile(`>Delete task</button>([^<]*)<`).FindStringSubmatch(mockupFrame(t, "s-tasks-3")); line == nil || strings.TrimSpace(line[1]) != "" {
		t.Fatalf("s-tasks-3 under Delete task: %q — read the frame again", line)
	}
	task := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess)))
	if task.Delete == nil || task.Delete.Label != "Delete task" || !task.Delete.Danger {
		t.Fatalf("task edit delete = %+v", task.Delete)
	}
	raw := decode[map[string]any](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess)))
	if line, ok := raw["deleteNote"]; ok {
		t.Errorf("task edit sends a line under Delete: %q", line)
	}
	if resp := f.h.do(postHref(f, task.Delete.Href)); resp.Code != 303 || resp.Header().Get("Location") != f.path("tasks") {
		t.Fatalf("delete from the edit page: %d %q", resp.Code, resp.Header().Get("Location"))
	}
	if resp := f.h.do(get(f.path("tasks")+"/t1", f.sess)); resp.Code != 404 {
		t.Errorf("the task is still there: %d", resp.Code)
	}
}

var ellipsisLabel = regexp.MustCompile(`"label":\s*"([^"]*…)"`)

// The mockup's rule of the ellipsis (§6): it tells apart two scenarios of
// one command and nothing else — the one that asks first carries it. There
// are two such pairs in the product, and only «Delete on the server…» is
// the server's; Move, Copy, Rename, Import, Publish a link have one
// scenario each and no sign.
func TestNoEllipsisWhereTheRuleHasNone(t *testing.T) {
	f := contactsFixture(t, false, annaCard)
	files := filesFixture(t, true)
	files.srv.SeedFile(filesDocs+"report.pdf", "0123456789")
	pages := map[string]string{
		"list":        f.h.do(get(f.path("contacts"), f.sess)).Body.String(),
		"card":        f.h.do(get(f.path("contacts")+"/anna", f.sess)).Body.String(),
		"connections": f.h.do(get("/app/settings/connections", f.sess)).Body.String(),
		"account":     f.h.do(get("/app/settings/account", f.sess)).Body.String(),
		"devices":     f.h.do(get("/app/settings/davloom", f.sess)).Body.String(),
		"files":       files.h.do(get(files.at(""), files.sess)).Body.String(),
	}
	for name, body := range pages {
		for _, m := range ellipsisLabel.FindAllStringSubmatch(body, -1) {
			if m[1] != "Delete on the server…" {
				t.Errorf("%s: «%s» carries the ellipsis", name, m[1])
			}
		}
	}
}

// frameSeriesLine is the line the event card writes under Delete event for
// a series (s-agenda-2).
func frameSeriesLine(t *testing.T) string {
	t.Helper()
	m := regexp.MustCompile(`>Delete event</button>([^<]+)<`).FindStringSubmatch(mockupFrame(t, "s-agenda-2"))
	if m == nil {
		t.Fatal("s-agenda-2 writes no line under Delete event: read the frame again")
	}
	return strings.TrimSpace(m[1])
}
