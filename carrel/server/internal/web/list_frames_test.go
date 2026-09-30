package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The list of contacts as the frames draw it (the client's scan of the
// contacts section, 30.09.2026, group Б): the line under the title, the
// head's buttons with their icons and New contact's arrow «Write into»
// (s-contacts-9), the count of each group, the row open in the panel and
// the panel itself, the filter and Compact rows that work without scripts,
// the icon of Columns; and what a list with nothing to show says (U-6.0).

var (
	frameListHead   = regexp.MustCompile(`(?s)<div class="tc-main">\s*<div class="tc-head[^"]*">.*?<div class="tc-head__sub">(.*?)</div>.*?<div class="tc-acts">(.*?)</div>`)
	frameHeadButton = regexp.MustCompile(`(?s)<button class="([^"]*)"[^>]*>(.*?)</button>`)
	frameWhereItem  = regexp.MustCompile(`(?s)<button class="([^"]*tc-menu__item[^"]*)"[^>]*role="menuitem"><span class="tc-mark" data-tc-colour="(#[0-9A-Fa-f]{6})"></span>([^<]*)(.*?)</button>`)
	frameGroupHead  = regexp.MustCompile(`<div class="tc-list__group"><span class="tc-rubric">([^<]*)</span>(<span class="tc-num">\d+</span>)?</div>`)
	frameOpenRow    = regexp.MustCompile(`(?s)<div class="tc-row [^"]*is-on[^"]*">.*?<span class="tc-row__title[^"]*">([^<]*)</span>`)
	framePanelUID   = regexp.MustCompile(`<div class="tc-diag">UID\s+(\S+)`)
	frameColumnsBtn = regexp.MustCompile(`<button class="tc-btn[^"]*" type="button"><svg[^>]*><use href="#(i-[\w-]+)"/></svg>Columns</button>`)
	frameDigits     = regexp.MustCompile(`\d+`)
)

func TestContactsListIsTheFrames(t *testing.T) {
	app := newMockupApp(t)
	key, _ := app.sess.DataKey()
	// «last used» in the arrow is the book a contact was last made in.
	if err := app.h.users.SetLastUsed(app.sess.AccountID(), key, sealed.Selection{Kind: view.KindContact,
		AccountID: app.realAccount(t, "kontakty"), Path: app.realPath(t, "kontakty")}); err != nil {
		t.Fatal(err)
	}
	page := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-1"), "<figcaption")

	head := frameListHead.FindStringSubmatch(frame)
	if head == nil {
		t.Fatal("s-contacts-1 draws no head: read the frame again")
	}
	shape := regexp.MustCompile("^" + frameDigits.ReplaceAllString(regexp.QuoteMeta(frameText(head[1])), `\d+`) + "$")
	if !shape.MatchString(string(page.Subtitle)) {
		t.Errorf("subtitle = %q, the frame %q", page.Subtitle, frameText(head[1]))
	}
	var want, got []string
	for _, b := range frameHeadButton.FindAllStringSubmatch(head[2], -1) {
		icons := frameIcon.FindAllStringSubmatch(b[2], -1)
		one := frameText(b[2]) + " [" + icons[0][1] + "]"
		if len(icons) > 1 {
			one += " arrow"
		}
		want = append(want, one)
	}
	for _, a := range page.Bar.Actions {
		one := string(a.Label) + " [" + a.Icon + "]"
		if len(a.Arrow) > 0 {
			one += " arrow"
		}
		got = append(got, one)
	}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("head = %q, the frame %q", got, want)
	}
	if p := actionByID(page.Bar.Actions, "list.print"); p == nil || p.Href != "/app/contacts?print=1" {
		t.Errorf("Print = %+v, want the list with print=1", p)
	}

	// New contact's arrow: the books of the view, «last used», «ro».
	var items []string
	for _, m := range frameWhereItem.FindAllStringSubmatch(mockupFrame(t, "s-contacts-9"), -1) {
		one := frameText(m[3])
		if strings.Contains(m[4], "tc-push") && !strings.Contains(m[4], "tc-badge") {
			one += " (" + frameText(m[4]) + ")"
		}
		if strings.Contains(m[4], "tc-badge") {
			one += " ro"
		}
		if strings.Contains(" "+m[1]+" ", " is-on ") {
			one += " on"
		}
		items = append(items, one)
	}
	if len(items) < 3 {
		t.Fatalf("s-contacts-9 = %q: read the frame again", items)
	}
	newContact := actionByID(page.Bar.Actions, "record.new")
	var arrow []string
	for _, it := range newContact.Arrow {
		one := string(it.Label)
		if it.Aside != "" {
			one += " (" + string(it.Aside) + ")"
		}
		if it.ReadOnly {
			one += " ro"
			if it.Enabled || it.Reason == "" {
				t.Errorf("a read-only book is offered: %+v", it)
			}
		}
		if it.On {
			one += " on"
		}
		if it.Colour == "" {
			t.Errorf("no colour: %+v", it)
		}
		arrow = append(arrow, one)
	}
	// The books stand in the catalog's order, which the stand gives; the
	// frame's order is its server's (C-6.3).
	slices.Sort(arrow)
	slices.Sort(items)
	if strings.Join(arrow, ", ") != strings.Join(items, ", ") {
		t.Errorf("New contact's arrow = %q, the frame s-contacts-9 %q", arrow, items)
	}
	for _, it := range newContact.Arrow {
		if it.On && newContact.Href != it.Href {
			t.Errorf("the button writes into «last used» %q, it leads to %q", it.Href, newContact.Href)
		}
	}

	// Every group says how many cards it holds.
	if g := frameGroupHead.FindAllStringSubmatch(frame, -1); len(g) == 0 || g[0][2] == "" {
		t.Fatalf("s-contacts-1 draws no count by its groups: read the frame again")
	}
	for _, g := range page.Groups {
		if g.Count != len(g.Rows) || g.Count == 0 {
			t.Errorf("group %q: count %d, rows %d", g.Rubric, g.Count, len(g.Rows))
		}
	}

	// Columns has its icon.
	if m := frameColumnsBtn.FindStringSubmatch(frame); m == nil || page.Bar.Columns == nil || page.Bar.Columns.Icon != m[1] {
		t.Errorf("Columns = %+v, the frame %v", page.Bar.Columns, m)
	}

	// The row open in the panel, the panel, and the way to close it.
	open := frameOpenRow.FindStringSubmatch(frame)
	uid := framePanelUID.FindStringSubmatch(framePanel(t, "s-contacts-1"))
	if open == nil || uid == nil {
		t.Fatal("s-contacts-1 draws no open row with its panel: read the frame again")
	}
	var href string
	for _, g := range page.Groups {
		for _, r := range g.Rows {
			if string(r.Name) == frameText(open[1]) {
				href = r.Href
			}
		}
	}
	withPanel := decode[view.ContactsListPage](t, app.h.do(get("/app/contacts?task="+page.Poll.TaskID+"&open="+url.QueryEscape(href), app.sess)))
	d := withPanel.Details
	if d == nil || string(d.Title) != frameText(open[1]) || string(d.Source.UID) != uid[1] || d.Close != "/app/contacts?task="+page.Poll.TaskID {
		t.Fatalf("panel = %+v", d)
	}
	var on []string
	for _, g := range withPanel.Groups {
		for _, r := range g.Rows {
			if r.On {
				on = append(on, string(r.Name))
			}
		}
	}
	if strings.Join(on, ",") != frameText(open[1]) {
		t.Errorf("rows on = %q, the frame %q", on, open[1])
	}

	// The filter is a form without scripts, and it filters.
	ff := page.Bar.FilterForm
	if ff == nil || ff.Method != "GET" || ff.Action != "/app/contacts" {
		t.Fatalf("filter form = %+v", ff)
	}
	sent := url.Values{}
	for _, s := range ff.Sets {
		for _, r := range s.Rows {
			sent.Set(r.Name, string(r.Value))
		}
	}
	if !sent.Has("filter") || sent.Get("task") != page.Poll.TaskID {
		t.Errorf("the filter's fields = %v", sent)
	}
	sent.Set("filter", "ковал")
	filtered := decode[view.ContactsListPage](t, app.h.do(get("/app/contacts?"+sent.Encode(), app.sess)))
	var names []string
	for _, g := range filtered.Groups {
		for _, r := range g.Rows {
			names = append(names, string(r.Name))
		}
	}
	if strings.Join(names, ",") != "Анна Ковалёва" || filtered.Bar.Filter != "ковал" {
		t.Errorf("filtered = %q (filter %q)", names, filtered.Bar.Filter)
	}

	// Compact rows: a link, remembered by this browser.
	c := page.Bar.Compact
	if c == nil || c.On || !strings.Contains(c.Href, "compact=1") {
		t.Fatalf("compact = %+v", c)
	}
	rec := app.h.do(get(c.Href, app.sess))
	if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "compact") {
		t.Fatalf("compact: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	var cookie *http.Cookie
	for _, k := range rec.Result().Cookies() {
		if k.Name == "carrel_compact_contacts" {
			cookie = k
		}
	}
	if cookie == nil || cookie.Value != "1" || !cookie.HttpOnly {
		t.Fatalf("cookie = %+v", cookie)
	}
	req := get("/app/contacts?task="+page.Poll.TaskID, app.sess)
	req.AddCookie(cookie)
	if again := decode[view.ContactsListPage](t, app.h.do(req)); again.Bar.Compact == nil || !again.Bar.Compact.On || !strings.Contains(again.Bar.Compact.Href, "compact=0") {
		t.Errorf("compact after the choice = %+v", again.Bar.Compact)
	}
}

// states-4: a book the server granted no write to says so under its title
// and by it; one book's line is the connection and how many cards.
func TestOneBookSaysWhatItIs(t *testing.T) {
	f := contactsFixture(t, true, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	sub := regexp.MustCompile(`<div class="tc-head__sub">([^<]*)</div>`).FindStringSubmatch(mockupFrame(t, "states-4"))
	if sub == nil {
		t.Fatal("states-4 draws no line under its title: read the frame again")
	}
	_, tail, _ := strings.Cut(frameText(sub[1]), "vps · ")
	if !page.ReadOnly || !strings.HasSuffix(string(page.Subtitle), " · "+tail) {
		t.Errorf("read-only book: %v %q, the frame «… · %s»", page.ReadOnly, page.Subtitle, tail)
	}
	w := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	page = decode[view.ContactsListPage](t, w.h.do(get(w.path("contacts"), w.sess)))
	if page.ReadOnly || !strings.HasSuffix(string(page.Subtitle), " · 1 card") {
		t.Errorf("one book: %v %q, want «<connection> · 1 card» (s-io-1)", page.ReadOnly, page.Subtitle)
	}
}

// U-6.0: a list with nothing to show says what to do and gives the button.
func TestAnEmptyContactsListSaysWhatToDo(t *testing.T) {
	f := contactsFixture(t, false)
	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if page.Empty == nil || page.Empty.Title == "" || actionByID(page.Empty.Actions, "record.new") == nil {
		t.Errorf("an empty book: %+v", page.Empty)
	}
	tickRail(t, f.h, f.sess, "contacts")
	all := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	if all.Empty == nil || len(all.Empty.Actions) == 0 || all.Poll != nil {
		t.Fatalf("nothing ticked: empty %+v, poll %+v", all.Empty, all.Poll)
	}
	tick := all.Empty.Actions[0]
	if rec := f.h.do(postForm(tick.Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/contacts" {
		t.Fatalf("%s: %d %q", tick.Label, rec.Code, rec.Header().Get("Location"))
	}
	if got := selected(t, f.h, f.sess, view.KindContact); len(got) != 1 {
		t.Errorf("after %q: %v", tick.Label, got)
	}
	if one := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess))); one.Empty == nil {
		t.Errorf("still empty: %+v", one)
	}
}
