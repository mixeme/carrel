package web_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 25 (the owner's decisions of 29–30.09.2026): every list
// that draws Columns has the menu — tasks (s-tasks-12), notes (s-notes-23),
// files (s-files-20), users (s-admin-21), the audit log (s-admin-23) and a
// person's timeline (s-contacts-25). Source stands only in the combined
// view's menu: one collection names its source in its head. Sorting
// follows the columns: a sort shows its column, and taking away the column
// the list is sorted by puts it back in its default order, which the page
// says (s-tasks-13). The frames are read here, not copied.

// listBar is what every list page with a bar answers: its columns and
// segments, its notices, and the poll a combined view runs.
type listBar struct {
	Bar struct {
		Columns  *view.Columns `json:"columns"`
		Segments []view.Action `json:"segments"`
	} `json:"bar"`
	Notices []view.Notice `json:"notices"`
	Poll    *view.PollBar `json:"poll"`
}

// barOf reads a list with the cookies given, and lets a poll it started
// finish: a session runs only so many at once.
func barOf(t *testing.T, h *harness, sess *session.Session, path string, cookies ...*http.Cookie) listBar {
	t.Helper()
	req := get(path, sess)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := h.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
	}
	var out listBar
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Poll != nil && out.Poll.TaskID != "" && out.Poll.Running {
		waitPoll(t, h, sess, out.Poll.TaskID, nil)
	}
	return out
}

// postColumns sends the Columns form and returns where it leads and the
// cookie it sets.
func postColumns(t *testing.T, h *harness, sess *session.Session, action string, cols []string, cookies ...*http.Cookie) (string, []*http.Cookie) {
	t.Helper()
	req := postForm(action, sess, url.Values{"col": cols})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := h.do(req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST %s %q: %d %s", action, cols, rec.Code, rec.Body.String())
	}
	return rec.Header().Get("Location"), rec.Result().Cookies()
}

// sameColumnsMenu compares a server's menu with a frame's, source left out where
// the list is one collection's.
func sameColumnsMenu(t *testing.T, what string, got *view.Columns, frame string, dropSource bool) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: no Columns (%s draws them)", what, frame)
	}
	head, _, items, foot := frameColumnsMenuOf(t, frame)
	got = beyondFrame(t, what, got, frame)
	want := []frameColumn{}
	for _, it := range items {
		if dropSource && it.ID == "source" {
			continue
		}
		want = append(want, it)
	}
	sameColumns(t, what, shownColumns(got), want)
	var buttons []string
	for _, a := range got.Form.Actions {
		b := string(a.Label)
		if a.ID == "columns.default" {
			b += " =default"
		}
		buttons = append(buttons, b)
	}
	if string(got.Label) != head || got.Form.Method != http.MethodPost || strings.Join(buttons, " | ") != strings.Join(foot, " | ") {
		t.Errorf("%s: label %q, method %s, buttons %q; want (%s) %q, POST, %q", what, got.Label, got.Form.Method, buttons, frame, head, foot)
	}
}

// frameAction is the address a frame's Columns form posts to.
func frameAction(t *testing.T, frame string) string {
	t.Helper()
	_, action, _, _ := frameColumnsMenuOf(t, frame)
	return action
}

func formPath(c *view.Columns) string {
	if c == nil {
		return ""
	}
	p, _, _ := strings.Cut(c.Form.Action, "?")
	return p
}

// asMenu writes the columns a frame draws in the ids of its menu: the
// frames class the first cell tc-col--name whatever the menu calls it.
func asMenu(order []string, first string, drop ...string) []string {
	var out []string
	for _, c := range order {
		switch {
		case c == "name":
			out = append(out, first)
		case slices.Contains(drop, c):
		default:
			out = append(out, c)
		}
	}
	return out
}

var frameOverdue = regexp.MustCompile(`<div class="tc-row tc-row--cols is-overdue">.*?tc-col--name">([^<]*)<`)

// frameDay reads «14 Aug» or «10 Aug 11:02» of a frame as a day of 2026.
func frameDay(t *testing.T, v string) (time.Time, bool) {
	t.Helper()
	for _, layout := range []string{"2 Jan 2006 15:04", "2 Jan 2006"} {
		day, rest, _ := strings.Cut(v, " ")
		month, clock, _ := strings.Cut(rest, " ")
		s := day + " " + month + " 2026"
		if clock != "" {
			s += " " + clock
		}
		if at, err := time.Parse(layout, s); err == nil {
			return at, clock != ""
		}
	}
	t.Fatalf("a day %q: read the frame again", v)
	return time.Time{}, false
}

func undashed(v string) string { return strings.TrimSpace(strings.TrimPrefix(v, "—")) }

// frameTodo is a task built from the cells of s-tasks-13.
func frameTodo(t *testing.T, uid string, cells map[string]string) string {
	t.Helper()
	b := "BEGIN:VTODO\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:" + cells["name"] + "\n"
	if v := undashed(cells["due"]); v != "" {
		at, _ := frameDay(t, v)
		b += "DUE;VALUE=DATE:" + at.Format("20060102") + "\n"
	}
	if v := undashed(cells["complete"]); v != "" {
		b += "PERCENT-COMPLETE:" + strings.TrimSuffix(v, "%") + "\n"
	}
	if v := undashed(cells["priority"]); v != "" {
		b += "PRIORITY:" + v + "\n"
	}
	// «Needs action» is also what a task without STATUS is.
	if v := cells["status"]; v != "" && v != "Needs action" {
		b += "STATUS:" + strings.ToUpper(strings.ReplaceAll(v, " ", "-")) + "\n"
	}
	if v := undashed(cells["changed"]); v != "" {
		at, _ := frameDay(t, v)
		b += "LAST-MODIFIED:" + at.Format("20060102T150405Z") + "\n"
	}
	return ics(b + "END:VTODO")
}

// frameJournal is a note built from the cells of s-notes-24.
func frameJournal(t *testing.T, uid string, cells map[string]string) string {
	t.Helper()
	b := "BEGIN:VJOURNAL\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:" + cells["name"] + "\n"
	b += "DESCRIPTION:" + strings.NewReplacer(",", "\\,", ";", "\\;").Replace(strings.TrimSuffix(cells["excerpt"], "…")) + "\n"
	if v := undashed(cells["date"]); v != "" {
		at, _ := frameDay(t, v)
		b += "DTSTART;VALUE=DATE:" + at.Format("20060102") + "\n"
	}
	if v := undashed(cells["changed"]); v != "" {
		at, _ := frameDay(t, v)
		b += "LAST-MODIFIED:" + at.Format("20060102T150405Z") + "\n"
	}
	return ics(b + "END:VJOURNAL")
}

func TestTaskAndNoteColumnsAreTheFrames(t *testing.T) {
	order13, rows13 := frameColsRows(t, "s-tasks-13")
	order24, rows24 := frameColsRows(t, "s-notes-24")
	srv := searchServer(t)
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: homeCal, Name: "Home"})
	n := 0
	for _, cells := range rows13 {
		n++
		srv.Seed(fmt.Sprintf("%st%d.ics", searchWork, n), frameTodo(t, fmt.Sprintf("t%d", n), cells))
	}
	for _, cells := range rows24 {
		n++
		srv.Seed(fmt.Sprintf("%sn%d.ics", homeCal, n), frameJournal(t, fmt.Sprintf("n%d", n), cells))
	}
	f := connectFixture(t, srv, searchWork, nil)
	// s-tasks-13 is drawn on a day after 8 Aug, which is late, and before
	// 14 Aug, which is not.
	f.h.srv.SetClock(time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC))
	work, home := url.PathEscape(searchWork), url.PathEscape(homeCal)
	for _, seg := range []string{"tasks", "notes"} {
		tickRail(t, f.h, f.sess, seg, "/app/"+seg+"/"+f.id+"/"+work, "/app/"+seg+"/"+f.id+"/"+home)
	}
	tasks, notes := "/app/tasks/"+f.id+"/"+work, "/app/notes/"+f.id+"/"+home

	// The menus until a choice: the combined view's is the frame's, Source
	// in it; one list's is the same without Source.
	for _, c := range []struct{ frame, all, one string }{
		{"s-tasks-12", "/app/tasks", tasks},
		{"s-notes-23", "/app/notes", notes},
	} {
		combined := barOf(t, f.h, f.sess, c.all).Bar.Columns
		sameColumnsMenu(t, c.all, combined, c.frame, false)
		if got, want := formPath(combined), frameAction(t, c.frame); got != want {
			t.Errorf("%s posts to %q, want (%s) %q", c.all, got, c.frame, want)
		}
		one := barOf(t, f.h, f.sess, c.one).Bar.Columns
		sameColumnsMenu(t, c.one, one, c.frame, true)
		if got := formPath(one); got != c.one+"/columns" {
			t.Errorf("%s posts to %q", c.one, got)
		}
	}

	// s-tasks-13: one list, Tags taken away and Priority and Status added,
	// sorted by «Recently changed» — which shows Changed without its being
	// ticked. The rows carry every column, in the frame's words.
	_, taskCookie := postColumns(t, f.h, f.sess, tasks+"/columns", asMenu(order13, "title", "name", "changed"))
	if len(taskCookie) != 1 || taskCookie[0].Name != "carrel_cols_tasks" {
		t.Fatalf("the tasks' choice is kept as %+v", taskCookie)
	}
	req := get(tasks+"?show=all&sort=changed", f.sess)
	req.AddCookie(taskCookie[0])
	list := decode[view.TasksPage](t, f.h.do(req))
	if got, want := columnsOn(list.Bar.Columns), asMenu(order13, "title"); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("columns on = %q, want (s-tasks-13) %q", got, want)
	}
	late := map[string]bool{}
	for _, m := range frameOverdue.FindAllStringSubmatch(mockupFrame(t, "s-tasks-13"), -1) {
		late[frameText(m[1])] = true
	}
	if len(late) == 0 {
		t.Fatal("s-tasks-13 draws no late task: read the frame again")
	}
	seen := 0
	for _, g := range list.Groups {
		for _, r := range g.Rows {
			want := rows13[string(r.Summary)]
			if want == nil {
				continue
			}
			seen++
			complete := ""
			if r.Percent != nil {
				complete = fmt.Sprintf("%d%%", *r.Percent)
			}
			got := map[string]string{"due": string(r.Due), "complete": complete, "priority": string(r.Priority),
				"status": string(r.Status), "changed": string(r.Changed)}
			for k, v := range got {
				if w := undashed(want[k]); v != w {
					t.Errorf("%s: %s = %q, want %q", r.Summary, k, v, w)
				}
			}
			if r.Overdue != late[string(r.Summary)] {
				t.Errorf("%s: overdue %v, the frame says %v", r.Summary, r.Overdue, late[string(r.Summary)])
			}
			if r.Tick == nil {
				t.Errorf("%s: no tick before the mark", r.Summary)
			}
		}
	}
	if seen != len(rows13) {
		t.Errorf("%d rows of s-tasks-13 in the list, want %d", seen, len(rows13))
	}

	// «By priority» shows Priority.
	if got := columnsOn(barOf(t, f.h, f.sess, tasks+"?sort=priority").Bar.Columns); !slices.Contains(got, "priority") {
		t.Errorf("sorted by priority, the columns on are %q", got)
	}
	// Taking away the column the list is sorted by: the default order, and
	// the page says so. Nothing of it stays in the addresses the page gives.
	back, _ := postColumns(t, f.h, f.sess, tasks+"/columns?show=all&sort=priority", []string{"due"}, taskCookie...)
	if back != tasks+"?show=all&unsorted=priority" {
		t.Fatalf("taking Priority away leads to %q", back)
	}
	_, taskCookie = postColumns(t, f.h, f.sess, tasks+"/columns", []string{"due"})
	after := barOf(t, f.h, f.sess, back, taskCookie...)
	var told bool
	for _, nt := range after.Notices {
		told = told || nt.Role == view.NoticeInfo && strings.Contains(string(nt.Body), "Priority") && strings.Contains(string(nt.Body), "By due")
	}
	if !told {
		t.Errorf("the page does not say the order went back to By due: %+v", after.Notices)
	}
	for _, a := range after.Bar.Segments {
		if a.ID == "sort.due" && !a.On {
			t.Errorf("By due is not on: %+v", a)
		}
		if strings.Contains(a.Href, "unsorted") {
			t.Errorf("segment %s keeps the notice: %q", a.ID, a.Href)
		}
	}
	if strings.Contains(after.Bar.Columns.Form.Action, "unsorted") {
		t.Errorf("the Columns form keeps the notice: %q", after.Bar.Columns.Form.Action)
	}
	if got := columnsOn(after.Bar.Columns); strings.Join(got, " ") != "title due" {
		t.Errorf("columns on = %q, want title due", got)
	}
	// A column kept, the sort is kept.
	if back, _ := postColumns(t, f.h, f.sess, tasks+"/columns?sort=changed", []string{"due", "changed"}); back != tasks+"?sort=changed" {
		t.Errorf("a choice that keeps Changed leads to %q", back)
	}
	// «Reset to defaults» takes Priority away too.
	if back, _ := postColumns(t, f.h, f.sess, tasks+"/columns?sort=priority", []string{"default"}); back != tasks+"?unsorted=priority" {
		t.Errorf("reset while sorted by priority leads to %q", back)
	}

	// s-notes-24: the combined view, Tags taken away and Changed added.
	_, noteCookie := postColumns(t, f.h, f.sess, "/app/notes/columns", asMenu(order24, "title", "name"))
	if len(noteCookie) != 1 || noteCookie[0].Name != "carrel_cols_notes" {
		t.Fatalf("the notes' choice is kept as %+v", noteCookie)
	}
	all := barOf(t, f.h, f.sess, "/app/notes", noteCookie...)
	if got, want := columnsOn(all.Bar.Columns), asMenu(order24, "title"); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("columns on = %q, want (s-notes-24) %q", got, want)
	}
	req = get("/app/notes?task="+all.Poll.TaskID, f.sess)
	req.AddCookie(noteCookie[0])
	notesPage := decode[view.NotesPage](t, f.h.do(req))
	seen = 0
	for _, g := range notesPage.Groups {
		for _, r := range g.Rows {
			want := rows24[string(r.Title)]
			if want == nil {
				continue
			}
			seen++
			if string(r.Date) != want["date"] || string(r.Changed) != want["changed"] || r.Stamp == "" {
				t.Errorf("%s: date %q, changed %q, stamp %q; want %q, %q and a source", r.Title, r.Date, r.Changed, r.Stamp, want["date"], want["changed"])
			}
			if !strings.HasPrefix(string(r.Excerpt), strings.TrimSuffix(want["excerpt"], "…")) {
				t.Errorf("%s: excerpt %q, want %q", r.Title, r.Excerpt, want["excerpt"])
			}
		}
	}
	if seen != len(rows24) {
		t.Errorf("%d rows of s-notes-24 in the list, want %d", seen, len(rows24))
	}
	// «Oldest» sorts by Date and shows it.
	_, noDate := postColumns(t, f.h, f.sess, notes+"/columns", []string{"excerpt"})
	if got := columnsOn(barOf(t, f.h, f.sess, notes+"?sort=oldest", noDate...).Bar.Columns); strings.Join(got, " ") != "title excerpt date" {
		t.Errorf("sorted oldest first, the columns on are %q, want title excerpt date", got)
	}
}

// s-files-20: every column is on by default; the form posts from the
// folder it was opened in and comes back to it.
func TestFileColumnsAreTheFrame(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"sub/a.txt", "hello")
	root := "/app/files/" + f.id + "/" + url.PathEscape(filesDocs) + "/"
	sub := root + "sub/"
	shape := regexp.MustCompile(`^/app/files/[^/]+/[^/]+/columns/?$`)
	page := barOf(t, f.h, f.sess, root)
	sameColumnsMenu(t, "files", page.Bar.Columns, "s-files-20", false)
	if a := formPath(page.Bar.Columns); !shape.MatchString(a) || !shape.MatchString(frameAction(t, "s-files-20")) {
		t.Errorf("the files' form posts to %q; the frame's to %q", a, frameAction(t, "s-files-20"))
	}
	inSub := barOf(t, f.h, f.sess, sub+"?sort=changed")
	if a := inSub.Bar.Columns.Form.Action; a != root+"columns/sub/?sort=changed" {
		t.Errorf("in a folder the form posts to %q", a)
	}
	back, cookie := postColumns(t, f.h, f.sess, inSub.Bar.Columns.Form.Action, []string{"changed", "type"})
	if back != sub+"?sort=changed" || len(cookie) != 1 || cookie[0].Name != "carrel_cols_files" {
		t.Fatalf("the choice leads to %q with %+v", back, cookie)
	}
	if got := columnsOn(barOf(t, f.h, f.sess, sub, cookie...).Bar.Columns); strings.Join(got, " ") != "name changed type" {
		t.Errorf("columns on = %q, want name changed type", got)
	}
	// Size is not shown; sorting by it shows it.
	if got := columnsOn(barOf(t, f.h, f.sess, sub+"?sort=size", cookie...).Bar.Columns); strings.Join(got, " ") != "name changed type size" {
		t.Errorf("sorted by size, the columns on are %q", got)
	}
	back, _ = postColumns(t, f.h, f.sess, root+"columns/sub/?sort=size", []string{"changed"}, cookie...)
	if back != sub+"?unsorted=size" {
		t.Errorf("taking Size away leads to %q", back)
	}
}

// s-admin-21, s-admin-22, s-admin-23: the menus of the administration, at
// their addresses; Address is the confirmed address, nothing without one.
func TestUserAndAuditColumnsAreTheFrames(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("owner", "password123")
	sess := h.login("owner", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	bob := h.login("bob", "bobpass123")
	h.confirmAddress("owner", "owner@example.org")
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(bobID)
		u.Email, u.EmailConfirmed = "bob@example.org", false
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	users := decode[view.AdminUsersPage](t, h.do(get("/admin/", sess)))
	sameColumnsMenu(t, "users", users.Bar.Columns, "s-admin-21", false)
	if got, want := formPath(users.Bar.Columns), frameAction(t, "s-admin-21"); got != want {
		t.Errorf("users post to %q, want %q", got, want)
	}
	addresses := map[string]string{}
	for _, r := range users.Rows {
		addresses[string(r.Login)] = string(r.Address)
	}
	if addresses["owner"] != "owner@example.org" || addresses["bob"] != "" {
		t.Errorf("addresses = %v: the confirmed one, and none for an address not confirmed", addresses)
	}
	// s-admin-22: the columns its table heads, in their order.
	_, _, items, _ := frameColumnsMenuOf(t, "s-admin-21")
	byLabel := map[string]string{}
	for _, it := range items {
		byLabel[it.Label] = it.ID
	}
	var heads []string
	for _, m := range regexp.MustCompile(`<th[^>]*>([^<]*)</th>`).FindAllStringSubmatch(mockupFrame(t, "s-admin-22"), -1) {
		if l := frameText(m[1]); l != "" {
			if byLabel[l] == "" {
				t.Fatalf("s-admin-22 heads %q, which s-admin-21 does not offer: read the frames again", l)
			}
			heads = append(heads, byLabel[l])
		}
	}
	back, cookie := postColumns(t, h, sess, "/admin/columns", heads[1:])
	if back != "/admin/" || len(cookie) != 1 || cookie[0].Name != "carrel_cols_users" {
		t.Fatalf("the users' choice leads to %q with %+v", back, cookie)
	}
	if got := columnsOn(barOf(t, h, sess, "/admin/", cookie...).Bar.Columns); strings.Join(got, " ") != strings.Join(heads, " ") {
		t.Errorf("columns on = %q, want (s-admin-22) %q", got, heads)
	}

	audit := barOf(t, h, sess, "/admin/audit?show=security")
	sameColumnsMenu(t, "audit", audit.Bar.Columns, "s-admin-23", false)
	if got, want := formPath(audit.Bar.Columns), frameAction(t, "s-admin-23"); got != want {
		t.Errorf("the audit log posts to %q, want %q", got, want)
	}
	back, cookie = postColumns(t, h, sess, audit.Bar.Columns.Form.Action, []string{"detail", "action"})
	if back != "/admin/audit?show=security" || len(cookie) != 1 || cookie[0].Name != "carrel_cols_audit" {
		t.Fatalf("the audit's choice leads to %q with %+v", back, cookie)
	}
	if got := columnsOn(barOf(t, h, sess, "/admin/audit", cookie...).Bar.Columns); strings.Join(got, " ") != "when detail action" {
		t.Errorf("columns on = %q", got)
	}

	// The menus of the administration are an administrator's.
	for _, a := range []string{"/admin/columns", "/admin/audit/columns"} {
		if rec := h.do(postForm(a, bob, url.Values{"col": {"role"}})); rec.Code == http.StatusSeeOther {
			t.Errorf("POST %s by a user: %d", a, rec.Code)
		}
	}
}

// s-contacts-25, s-contacts-26: the timeline's menu; why a record is there
// stands after its title and is not a column to take away; the date is a
// list's day (C-10).
func TestTimelineColumnsAreTheFrame(t *testing.T) {
	now := time.Now()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ann.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ann\nFN:Anna Kovaleva\nEMAIL:anna@example.com\nEND:VCARD\n")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:"+icsDay(now.AddDate(0, 0, -3))+
		"\nDTEND:"+icsDay(now.AddDate(0, 0, -3).Add(time.Hour))+"\nSUMMARY:Review with Anna\nATTENDEE;PARTSTAT=ACCEPTED:mailto:anna@example.com\nEND:VEVENT"))
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	card := f.path("contacts") + "/ann"

	page := barOf(t, f.h, f.sess, card+"/timeline")
	sameColumnsMenu(t, "timeline", page.Bar.Columns, "s-contacts-25", false)
	shape := regexp.MustCompile(`^/app/contacts/[^/]+/[^/]+/[^/]+/timeline/columns$`)
	if a := formPath(page.Bar.Columns); a != card+"/timeline/columns" || !shape.MatchString(frameAction(t, "s-contacts-25")) {
		t.Errorf("the timeline posts to %q; the frame's to %q", a, frameAction(t, "s-contacts-25"))
	}
	rows := timelineRows(decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess))))
	r, ok := rows["Review with Anna"]
	if !ok || r.Kind != view.KindEvent || string(r.Why) != "attendee · accepted" ||
		!regexp.MustCompile(`^\d{1,2} [A-Z][a-z]{2}( \d{4})?$`).MatchString(string(r.When)) {
		t.Errorf("row = %+v (found %v): a list's day, «10 Aug» (s-contacts-26)", r, ok)
	}

	order, _ := frameColsRows(t, "s-contacts-26")
	want := asMenu(order, "title", "detail")
	back, cookie := postColumns(t, f.h, f.sess, card+"/timeline/columns?kind=event", want[1:])
	if back != card+"/timeline?kind=event" || len(cookie) != 1 || cookie[0].Name != "carrel_cols_timeline" {
		t.Fatalf("the timeline's choice leads to %q with %+v", back, cookie)
	}
	if got := columnsOn(barOf(t, f.h, f.sess, card+"/timeline", cookie...).Bar.Columns); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("columns on = %q, want (s-contacts-26) %q", got, want)
	}
}

// beyondFrames are the columns a menu offers that its frame does not draw
// yet: what one kind's list had and another's lacked (scans/kinds-parity.md).
// Each is asked of the frame — Attendees after Р3: task 181. Where of a task
// (Р10, task 170) is drawn since note 69: last, off; a note's Status (Р13,
// task 178) is content of the kind the owner does not take (note 69).
var beyondFrames = map[string][]string{"s-tasks-12": {"attendees"}, "s-notes-23": {"attendees"}}

// beyondFrame is the menu without the columns beyondFrames adds, each of
// which it must offer.
func beyondFrame(t *testing.T, what string, got *view.Columns, frame string) *view.Columns {
	t.Helper()
	if got == nil || len(beyondFrames[frame]) == 0 {
		return got
	}
	out := *got
	out.Items = nil
	for _, it := range got.Items {
		if !slices.Contains(beyondFrames[frame], it.ID) {
			out.Items = append(out.Items, it)
		}
	}
	if len(got.Items)-len(out.Items) != len(beyondFrames[frame]) {
		t.Errorf("%s: columns %+v, want %v beside the frame's", what, got.Items, beyondFrames[frame])
	}
	return &out
}
