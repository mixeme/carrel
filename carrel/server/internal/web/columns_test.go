package web_test

import (
	"net/http"
	"net/url"
	"regexp"
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
	frameColumnCheck  = regexp.MustCompile(`<label class="tc-menu__item[^"]*" role="menuitem"><input class="tc-check__box" type="checkbox" name="col" value="([^"]+)"( checked)?( disabled)?><svg[^>]*><use href="#i-drag"/></svg><span class="tc-tickrow__tick"></span>([^<]*)(?:<span class="tc-push[^"]*">([^<]*)</span>)?</label>`)
	frameColumnsForm  = regexp.MustCompile(`<form class="tc-menu" action="([^"]+)" method="post">`)
	frameColumnsFoot  = regexp.MustCompile(`<button class="tc-btn[^"]*" type="submit"( name="col" value="default")?>([^<]*)</button>`)
	frameColumnsHead  = regexp.MustCompile(`<div class="tc-rubric tc-menu__item[^"]*" role="presentation">([^<]*)</div>`)
	frameColsRow      = regexp.MustCompile(`(?s)<div class="tc-row tc-row--cols[^"]*">(.*?)</div>`)
	frameColsCell     = regexp.MustCompile(`<span class="[^"]*tc-col--(\w+)">((?:<span class="tc-tag">[^<]*</span>)*|[^<]*)</span>`)
	frameColsTag      = regexp.MustCompile(`<span class="tc-tag">([^<]*)</span>`)
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
	frame, _, _ := strings.Cut(mockupFrame(t, "components-42"), "<figcaption")
	if m := frameColumnsHead.FindStringSubmatch(frame); m != nil {
		head = frameText(m[1])
	}
	if m := frameColumnsForm.FindStringSubmatch(frame); m != nil {
		action = m[1]
	}
	for _, m := range frameColumnCheck.FindAllStringSubmatch(frame, -1) {
		items = append(items, frameColumn{ID: m[1], Label: frameText(m[4]), Aside: frameText(m[5]), On: m[2] != "", Fixed: m[3] != ""})
	}
	for _, m := range frameColumnsFoot.FindAllStringSubmatch(frame, -1) {
		b := frameText(m[2])
		if m[1] != "" {
			b += " =default"
		}
		foot = append(foot, b)
	}
	if head == "" || !strings.HasSuffix(action, "/columns") || len(items) < 2 || len(foot) != 2 {
		t.Fatalf("components-42: %q %q %+v %q: read the frame again", head, action, items, foot)
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
			v := frameText(c[2])
			if c[1] == "tags" {
				var tags []string
				for _, g := range frameColsTag.FindAllStringSubmatch(c[2], -1) {
					tags = append(tags, frameText(g[1]))
				}
				v = strings.Join(tags, ", ")
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

	// Until the person chooses: the menu of components-21, in its order;
	// the form of components-42 — its heading, its address beside the
	// list's, Apply and the reset.
	head, action, chosen, foot := frameColumnsMenu(t)
	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	cols := page.Bar.Columns
	if cols == nil {
		t.Fatal("the contacts list has no Columns")
	}
	def := shownColumns(cols)
	for i := range def {
		def[i].ID = ""
	}
	sameColumns(t, "the columns by default", def, frameDefaultColumns(t))
	var buttons []string
	for _, a := range cols.Form.Actions {
		b := string(a.Label)
		if a.ID == "columns.default" {
			b += " =default"
		}
		buttons = append(buttons, b)
	}
	if string(cols.Label) != head || cols.Form.Method != http.MethodPost || cols.Form.Action != list+"/columns" ||
		strings.Join(buttons, " | ") != strings.Join(foot, " | ") {
		t.Errorf("columns: label %q, form %s %s, buttons %q; want (components-42) %q, POST %s, %q",
			cols.Label, cols.Form.Method, cols.Form.Action, buttons, head, strings.TrimSuffix(action, "/columns")+"…/columns", foot)
	}

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
	// menu is components-42's; the columns on are s-contacts-23's.
	post := func(cols []string, cookies []*http.Cookie) *http.Response {
		t.Helper()
		form := url.Values{"col": cols}
		req := postForm(list+"/columns?sort=changed", f.sess, form)
		for _, c := range cookies {
			req.AddCookie(c)
		}
		rec := f.h.do(req)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != list+"?sort=changed" {
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
		return decode[view.ContactsListPage](t, f.h.do(req))
	}
	var ticked []string
	for _, it := range chosen {
		if it.On && !it.Fixed {
			ticked = append(ticked, it.ID)
		}
	}
	res := post(ticked, nil)
	kept := res.Cookies()
	if len(kept) != 1 || kept[0].MaxAge <= 0 || !kept[0].HttpOnly || kept[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("the choice is kept as %+v", kept)
	}
	after := with(list+"?sort=changed", kept)
	sameColumns(t, "the columns chosen", shownColumns(after.Bar.Columns), chosen)
	if got := columnsOn(after.Bar.Columns); strings.Join(got, " ") != strings.Join(order23, " ") {
		t.Errorf("columns on = %q, want (s-contacts-23) %q", got, order23)
	}
	if after.Bar.Columns.Form.Action != list+"/columns?sort=changed" {
		t.Errorf("the form loses the list's address: %q", after.Bar.Columns.Form.Action)
	}

	// Another: s-contacts-24's. Unknown columns and a second Name are not
	// columns.
	res = post(append([]string{"name", "bogus"}, append(order24[1:], order24[1])...), kept)
	kept = res.Cookies()
	if got := columnsOn(with(list, kept).Bar.Columns); strings.Join(got, " ") != strings.Join(order24, " ") {
		t.Errorf("columns on = %q, want (s-contacts-24) %q", got, order24)
	}

	// Reset: the cookie goes, and the menu is components-21's again.
	res = post([]string{"default"}, kept)
	gone := res.Cookies()
	if len(gone) != 1 || gone[0].MaxAge >= 0 {
		t.Fatalf("reset keeps %+v", gone)
	}
	reset := shownColumns(with(list, nil).Bar.Columns)
	for i := range reset {
		reset[i].ID = ""
	}
	sameColumns(t, "the columns after reset", reset, frameDefaultColumns(t))

	// What another browser sent, or an old one, is no choice: the default.
	junk := shownColumns(with(list, []*http.Cookie{{Name: kept[0].Name, Value: "x.y.z"}}).Bar.Columns)
	for i := range junk {
		junk[i].ID = ""
	}
	sameColumns(t, "a cookie of nothing known", junk, frameDefaultColumns(t))
	if got := columnsOn(with(list, []*http.Cookie{{Name: kept[0].Name, Value: "email.name.email"}}).Bar.Columns); strings.Join(got, " ") != "name email" {
		t.Errorf("name first, once: %q", got)
	}

	// The combined book has the same menu, at its own address.
	tickRail(t, f.h, f.sess, "contacts", list)
	all := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if all.Bar.Columns == nil || !strings.HasPrefix(all.Bar.Columns.Form.Action, "/app/contacts/columns") {
		t.Errorf("combined columns = %+v", all.Bar.Columns)
	}
}
