package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// from-typecase note 23 (task 29, the owner's decision — whole): the
// Columns menu of a contacts list is a form. Its ticked items go as col in
// their order, «Reset to defaults» sends col=default (components-42); the
// list comes back with the columns chosen, in that order (s-contacts-23,
// -24). The choice is this browser's (U-12, S-9.6). The frames are read
// here: the menu as it stands by default (components-21), the form after a
// choice (components-42), the rows the choice draws.

// frameColumn is an item of a Columns menu as a frame draws it.
type frameColumn struct {
	ID, Label, Aside string
	On, Fixed        bool
}

var (
	frameColumnButton = regexp.MustCompile(`<button class="tc-menu__item" type="button" role="menuitem"><svg[^>]*><use href="#i-drag"/></svg><span class="tc-tickrow__tick( is-off)?"></span>([^<]*)(?:<span class="tc-push[^"]*">([^<]*)</span>)?</button>`)
	frameColumnCheck  = regexp.MustCompile(`<label class="tc-menu__item[^"]*" role="menuitem"><input class="tc-check__box" type="checkbox" name="col" value="([^"]+)"( checked)?( disabled)?><svg[^>]*><use href="#i-drag"/></svg><span class="tc-tickrow__tick( is-off)?"></span>([^<]*)(?:<span class="tc-push[^"]*">([^<]*)</span>)?</label>`)
	frameColumnLabel  = regexp.MustCompile(`<label class="tc-menu__item[^"]*" role="menuitem">`)
	frameColumnsForm  = regexp.MustCompile(`<form class="tc-menu" action="([^"]+)" method="post">`)
	frameColumnsFoot  = regexp.MustCompile(`<button class="tc-btn[^"]*" type="submit"( name="col" value="default")?>([^<]*)</button>`)
	frameColumnsHead  = regexp.MustCompile(`<div class="tc-rubric tc-menu__item[^"]*" role="presentation">([^<]*)</div>`)
	frameColsRow      = regexp.MustCompile(`(?s)<div class="tc-row tc-row--cols[^"]*">(.*?)</div>`)
	// A cell of tags shows the first and counts the rest («работа +1»); all
	// of them are in its hint (Typecase Р-53, note 44).
	frameColsCell = regexp.MustCompile(`<span class="[^"]*tc-col--(\w+)"(?: title="([^"]*)")?>((?:<span class="tc-tag">[^<]*</span>)?(?:<span class="tc-row__more">[^<]*</span>)?|[^<]*)</span>`)
	frameColsTag  = regexp.MustCompile(`<span class="tc-tag">([^<]*)</span>`)
)

// frameDefaultColumns is the menu of components-21: the columns a list
// shows until the person chooses, in their order, the ones off among them.
func frameDefaultColumns(t *testing.T) []frameColumn {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, "components-21"), "<figcaption")
	var out []frameColumn
	for _, m := range frameColumnButton.FindAllStringSubmatch(frame, -1) {
		out = append(out, frameColumn{Label: frameText(m[2]), Aside: frameText(m[3]), On: m[1] == "", Fixed: m[3] != ""})
	}
	if len(out) < 2 {
		t.Fatalf("components-21 = %+v: read the frame again", out)
	}
	return out
}

// frameColumnsMenu is the form of components-42: its heading, its address,
// its items and the buttons of its foot («name=col value=default» after the
// one that resets).
func frameColumnsMenu(t *testing.T) (head, action string, items []frameColumn, foot []string) {
	t.Helper()
	return frameColumnsMenuOf(t, "components-42")
}

// frameColumnsMenuOf is the Columns form a frame draws: components-42 and
// the menus of note 25 (s-tasks-12, s-notes-23, s-files-20, s-admin-21,
// s-admin-23, s-contacts-25).
func frameColumnsMenuOf(t *testing.T, id string) (head, action string, items []frameColumn, foot []string) {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
	if m := frameColumnsHead.FindStringSubmatch(frame); m != nil {
		head = frameText(m[1])
	}
	if m := frameColumnsForm.FindStringSubmatch(frame); m != nil {
		action = m[1]
	}
	for _, m := range frameColumnCheck.FindAllStringSubmatch(frame, -1) {
		if (m[2] != "") == (m[4] != "") {
			t.Fatalf("%s: %q is checked %v, its tick off %v: read the frame again", id, m[1], m[2] != "", m[4] != "")
		}
		items = append(items, frameColumn{ID: m[1], Label: frameText(m[5]), Aside: frameText(m[6]), On: m[2] != "", Fixed: m[3] != ""})
	}
	// Every item is read or the reading stops: an item the pattern missed
	// would leave the menu shorter than drawn, and the server's longer.
	if n := len(frameColumnLabel.FindAllString(frame, -1)); n != len(items) {
		t.Fatalf("%s: %d items drawn, %d read: read the frame again", id, n, len(items))
	}
	for _, m := range frameColumnsFoot.FindAllStringSubmatch(frame, -1) {
		b := frameText(m[2])
		if m[1] != "" {
			b += " =default"
		}
		foot = append(foot, b)
	}
	if head == "" || !strings.HasSuffix(action, "/columns") || len(items) < 2 || len(foot) != 2 {
		t.Fatalf("%s: %q %q %+v %q: read the frame again", id, head, action, items, foot)
	}
	return head, action, items, foot
}

// frameColsRows are the rows a frame draws with chosen columns: the column
// order of its first row, and every row's cells by column (tags joined by
// «, »), keyed by the name.
func frameColsRows(t *testing.T, id string) (order []string, rows map[string]map[string]string) {
	t.Helper()
	rows = map[string]map[string]string{}
	for i, r := range frameColsRow.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		cells := map[string]string{}
		for _, c := range frameColsCell.FindAllStringSubmatch(r[1], -1) {
			v := frameText(c[3])
			if c[1] == "tags" {
				var tags []string
				for _, g := range frameColsTag.FindAllStringSubmatch(c[3], -1) {
					tags = append(tags, frameText(g[1]))
				}
				v = strings.Join(tags, ", ")
				if c[2] != "" {
					v = frameText(c[2])
				}
			}
			cells[c[1]] = v
			if i == 0 {
				order = append(order, c[1])
			}
		}
		rows[cells["name"]] = cells
	}
	if len(order) < 2 || len(rows) < 2 {
		t.Fatalf("%s draws no rows of columns: read the frame again", id)
	}
	return order, rows
}

// shownColumns are the items of the server's menu, written as a frame's.
func shownColumns(c *view.Columns) []frameColumn {
	out := []frameColumn{}
	if c == nil {
		return out
	}
	for _, it := range c.Items {
		out = append(out, frameColumn{ID: it.ID, Label: string(it.Label), Aside: string(it.Aside), On: it.On, Fixed: it.Fixed})
	}
	return out
}

func columnsOn(c *view.Columns) []string {
	out := []string{}
	if c == nil {
		return out
	}
	for _, it := range c.Items {
		if it.On {
			out = append(out, it.ID)
		}
	}
	return out
}

func sameColumns(t *testing.T, what string, got, want []frameColumn) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %+v,\nwant %+v", what, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %+v,\nwant %+v", what, got, want)
			return
		}
	}
}

// frameColsCard is a card built from what the frames draw of a person: the
// cells of s-contacts-23 and -24. A day is written back as vCard writes
// it: «14.03.1987» is 1987-03-14, «02.11» is --1102, «12 Aug» the REV of
// that day this year.
func frameColsCard(t *testing.T, uid string, cells map[string]string) string {
	t.Helper()
	card := "BEGIN:VCARD\nVERSION:3.0\nUID:" + uid + "\nFN:" + cells["name"] + "\n"
	set := func(k string) string { v := cells[k]; return strings.TrimSpace(strings.TrimPrefix(v, "—")) }
	if v := set("email"); v != "" {
		card += "EMAIL:" + v + "\n"
	}
	if v := set("phone"); v != "" {
		card += "TEL:" + v + "\n"
	}
	if v := set("org"); v != "" {
		card += "ORG:" + v + "\n"
	}
	if v := set("tags"); v != "" {
		card += "CATEGORIES:" + strings.ReplaceAll(v, ", ", ",") + "\n"
	}
	if v := set("birthday"); v != "" {
		p := strings.Split(v, ".")
		switch len(p) {
		case 3:
			card += "BDAY:" + p[2] + "-" + p[1] + "-" + p[0] + "\n"
		case 2:
			card += "BDAY:--" + p[1] + p[0] + "\n"
		default:
			t.Fatalf("a birthday %q: read the frame again", v)
		}
	}
	if v := set("changed"); v != "" {
		day, err := time.Parse("2 Jan 2006", v+" 2026")
		if err != nil {
			t.Fatalf("a day %q: read the frame again", v)
		}
		card += "REV:" + day.Format("20060102") + "T100000Z\n"
	}
	return card + "END:VCARD\n"
}

func TestColumnsAreTheFrames(t *testing.T) {
	order23, rows23 := frameColsRows(t, "s-contacts-23")
	order24, rows24 := frameColsRows(t, "s-contacts-24")
	// One person is one card: what -23 and -24 draw of them together.
	people := map[string]map[string]string{}
	for _, rows := range []map[string]map[string]string{rows23, rows24} {
		for name, cells := range rows {
			if people[name] == nil {
				people[name] = map[string]string{}
			}
			for k, v := range cells {
				people[name][k] = v
			}
		}
	}
	var cards []string
	n := 0
	for _, cells := range people {
		n++
		cards = append(cards, frameColsCard(t, "p"+string(rune('0'+n)), cells))
	}
	f := bookFixture(t, nil, cards...)
	f.h.srv.SetClock(time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	list := f.path("contacts")
	tickRail(t, f.h, f.sess, "contacts", list)
	const all = "/app/contacts"

	// Until the person chooses: the menu of components-21, in its order;
	// the form of components-42 — its heading, its address, Apply and the
	// reset. components-42 is the combined view's form (note 25): one book
	// names its source in its head, and its menu has no Source.
	head, action, chosen, foot := frameColumnsMenu(t)
	withoutSource := func(cols []frameColumn) []frameColumn {
		out := []frameColumn{}
		for _, c := range cols {
			if c.Label != "Source" && c.ID != "source" {
				out = append(out, c)
			}
		}
		return out
	}
	unnamed := func(c *view.Columns) []frameColumn {
		out := shownColumns(c)
		for i := range out {
			out[i].ID = ""
		}
		return out
	}
	cols := combined(t, f.h, f.sess, all, contactsPoll).Bar.Columns
	if cols == nil {
		t.Fatal("the combined contacts have no Columns")
	}
	sameMarks(t, "components-42", "columns", cols.Form)
	sameColumns(t, "the columns by default", unnamed(cols), frameDefaultColumns(t))
	var buttons []string
	for _, a := range cols.Form.Actions {
		b := string(a.Label)
		if a.ID == "columns.default" {
			b += " =default"
		}
		buttons = append(buttons, b)
	}
	// The poll's task rides in the address, so that the answer reads the
	// same poll (S-9.6).
	address, _, _ := strings.Cut(cols.Form.Action, "?")
	if string(cols.Label) != head || cols.Form.Method != http.MethodPost || address != action ||
		strings.Join(buttons, " | ") != strings.Join(foot, " | ") {
		t.Errorf("columns: label %q, form %s %s, buttons %q; want (components-42) %q, POST %s, %q",
			cols.Label, cols.Form.Method, cols.Form.Action, buttons, head, action, foot)
	}
	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	if page.Bar.Columns == nil || page.Bar.Columns.Form.Action != list+"/columns" {
		t.Fatalf("one book's columns = %+v", page.Bar.Columns)
	}
	sameColumns(t, "one book's columns by default", unnamed(page.Bar.Columns), withoutSource(frameDefaultColumns(t)))

	// The rows carry every column the menu offers, in the frames' words.
	for _, g := range page.Groups {
		for _, r := range g.Rows {
			want := people[string(r.Name)]
			if want == nil {
				continue
			}
			var tags []string
			for _, tg := range r.Tags {
				tags = append(tags, string(tg.Label))
			}
			got := map[string]string{"phone": string(r.Phone), "email": string(r.Email), "org": string(r.Org),
				"birthday": string(r.Birthday), "tags": strings.Join(tags, ", "), "changed": string(r.Changed)}
			for k, v := range got {
				w, drawn := want[k]
				if !drawn {
					continue
				}
				if w == "—" {
					w = ""
				}
				if v != w {
					t.Errorf("%s: %s = %q, want %q", r.Name, k, v, w)
				}
			}
		}
	}

	// A choice: what components-42 ticks, in its order — Name stays off
	// the form, it is always first. The list comes back with it, and the
	// menu is components-42's; the columns on are s-contacts-23's. One
	// book shows the same without Source.
	post := func(from string, cols []string, cookies []*http.Cookie) *http.Response {
		t.Helper()
		form := url.Values{"col": cols}
		req := postForm(from+"/columns?sort=az", f.sess, form)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := f.h.do(req)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != from+"?sort=az" {
			t.Fatalf("post %q: %d %q %s", cols, rec.Code, rec.Header().Get("Location"), rec.Body.String())
		}
		return rec.Result()
	}
	with := func(path string, cookies []*http.Cookie) view.ContactsListPage {
		t.Helper()
		req := get(path, f.sess)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		page := decode[view.ContactsListPage](t, f.h.do(req))
		if page.Poll != nil && page.Poll.TaskID != "" {
			// A combined view polls; the poll is let finish, so that the
			// next one is not refused for too many at once.
			waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
		}
		return page
	}
	var ticked []string
	for _, it := range chosen {
		if it.On && !it.Fixed {
			ticked = append(ticked, it.ID)
		}
	}
	res := post(all, ticked, nil)
	kept := res.Cookies()
	if len(kept) != 1 || kept[0].Name != "carrel_cols_contacts" || kept[0].MaxAge <= 0 || !kept[0].HttpOnly || kept[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("the choice is kept as %+v", kept)
	}
	after := with(all+"?sort=az", kept)
	sameColumns(t, "the columns chosen", shownColumns(after.Bar.Columns), chosen)
	if got := columnsOn(after.Bar.Columns); strings.Join(got, " ") != strings.Join(order23, " ") {
		t.Errorf("columns on = %q, want (s-contacts-23) %q", got, order23)
	}
	// The form keeps the list's query, and names the poll the list read.
	if address, query, _ := strings.Cut(after.Bar.Columns.Form.Action, "?"); address != all+"/columns" ||
		!strings.Contains("&"+query+"&", "&sort=az&") || !strings.Contains(query, "task=") {
		t.Errorf("the form loses the list's address: %q", after.Bar.Columns.Form.Action)
	}
	sameColumns(t, "one book's columns chosen", shownColumns(with(list, kept).Bar.Columns), withoutSource(chosen))

	// Another: s-contacts-24's. Unknown columns and a second Name are not
	// columns.
	res = post(all, append([]string{"name", "bogus"}, append(order24[1:], order24[1])...), kept)
	kept = res.Cookies()
	if got := columnsOn(with(all, kept).Bar.Columns); strings.Join(got, " ") != strings.Join(order24, " ") {
		t.Errorf("columns on = %q, want (s-contacts-24) %q", got, order24)
	}

	// A choice made in one book, whose menu has no Source, leaves Source
	// where the combined view had it.
	res = post(list, []string{"email", "phone"}, kept)
	if got := columnsOn(with(all, res.Cookies()).Bar.Columns); strings.Join(got, " ") != "name email phone source" {
		t.Errorf("after a choice in one book, the combined view shows %q, want name email phone source", got)
	}

	// Reset: the cookie goes, and the menu is components-21's again. The
	// browser sends the ticked boxes too, Reset's value among them (the
	// owner's phone, 28.11.2026: Reset kept the choice).
	res = post(all, []string{"email", "org", "birthday", "source", "default"}, kept)
	gone := res.Cookies()
	if len(gone) != 1 || gone[0].MaxAge >= 0 {
		t.Fatalf("reset keeps %+v", gone)
	}
	sameColumns(t, "the columns after reset", unnamed(with(all, nil).Bar.Columns), frameDefaultColumns(t))

	// What another browser sent, or an old one, is no choice: the default.
	sameColumns(t, "a cookie of nothing known", unnamed(with(all, []*http.Cookie{{Name: kept[0].Name, Value: "x.y.z"}}).Bar.Columns), frameDefaultColumns(t))
	if got := columnsOn(with(all, []*http.Cookie{{Name: kept[0].Name, Value: "email.name.email"}}).Bar.Columns); strings.Join(got, " ") != "name email" {
		t.Errorf("name first, once: %q", got)
	}

	// Sorting follows the columns: «Recently changed» shows Changed, which
	// the default does not.
	if got := columnsOn(with(list+"?sort=changed", nil).Bar.Columns); !slices.Contains(got, "changed") {
		t.Errorf("sorted by Recently changed, the columns on are %q: Changed is not among them", got)
	}
}
