package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The shell as the frames draw it (the client's scan of the contacts
// section, 30.09.2026, group А): the top bar's «New note» with its arrow,
// the icons of Refresh and of the sections, the account button's way
// without scripts, the rail's open collection and its feeds, the name and
// the build apart at the foot, the davloom mark, and ticks that send
// themselves. Everything is read from the frames.

var (
	frameShellNew       = regexp.MustCompile(`(?s)<span class="tc-drop__anchor">\s*<button class="tc-btn is-primary[^"]*" type="button">(.*?)</button>\s*<span class="tc-drop__panel">(.*?)</span>\s*</span>`)
	frameShellMenuPart  = regexp.MustCompile(`(?s)<span class="tc-menu__sep"></span>|<button class="[^"]*tc-menu__item[^"]*"[^>]*role="menuitem">(.*?)</button>`)
	frameShellKbd       = regexp.MustCompile(`(?s)<kbd[^>]*>.*?</kbd>`)
	frameShellRefresh   = regexp.MustCompile(`<button class="tc-btn" type="button"><svg[^>]*><use href="#(i-[\w-]+)"/></svg>\d+ min ago</button>`)
	frameNavIcon        = regexp.MustCompile(`(?s)<a class="[^"]*tc-nav__item[^"]*" href="#"><svg[^>]*><use href="#(i-[\w-]+)"/></svg>([^<]+)`)
	frameShellTick      = regexp.MustCompile(`(?s)<div class="((?:[^" ]+ )*tc-tickrow(?: [^"]*)?)">(.*?)</div>`)
	frameShellTickLabel = regexp.MustCompile(`<span class="tc-tickrow__label">([^<]*)</span>`)
	frameShellAbout     = regexp.MustCompile(`<a class="tc-railfoot__about" href="#rest">([^<]*)<span class="tc-mono">([^<]*)</span></a>`)
	frameShellEmblem    = regexp.MustCompile(`<svg class="tc-emblem"[^>]*><title>([^<]*)</title>`)
)

// frameShellItem is a button or an item as a frame draws it: the words, the
// icon in brackets, «— » before an item a line stands above.
func frameShellItem(inner string, line bool) string {
	icon := ""
	if i := frameIcon.FindStringSubmatch(inner); i != nil {
		icon = i[1]
	}
	item := frameText(frameShellKbd.ReplaceAllString(inner, "")) + " [" + icon + "]"
	if line {
		item = "— " + item
	}
	return item
}

// frameNewMenu is s-notes-7: the top bar's «New note» and its open arrow.
func frameNewMenu(t *testing.T) (button string, items []string) {
	t.Helper()
	m := frameShellNew.FindStringSubmatch(mockupFrame(t, "s-notes-7"))
	if m == nil {
		t.Fatal("s-notes-7 draws no open «New note»: read the frame again")
	}
	button = frameShellItem(m[1], false)
	line := false
	for _, p := range frameShellMenuPart.FindAllStringSubmatch(m[2], -1) {
		if strings.HasPrefix(p[0], "<span") {
			line = true
			continue
		}
		items = append(items, frameShellItem(p[1], line))
		line = false
	}
	if len(items) < 4 {
		t.Fatalf("s-notes-7: menu %v: read the frame again", items)
	}
	return button, items
}

func TestTopBarIsTheFrames(t *testing.T) {
	app := newMockupApp(t)
	page := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	top := page.Top

	button, items := frameNewMenu(t)
	// The button's first icon is its own; the chevron after it is the arrow's.
	if got := string(top.New.Label) + " [" + top.New.Icon + "]"; got != button {
		t.Errorf("New = %q, the frame %q", got, button)
	}
	if !top.New.Primary || top.New.Href == "" || !top.New.Enabled {
		t.Errorf("New = %+v: the primary button that makes a note", top.New)
	}
	if got := serverMenu(top.New.Arrow); strings.Join(got, " | ") != strings.Join(items, " | ") {
		t.Errorf("New's arrow = %q,\nthe frame s-notes-7 %q", got, items)
	}
	if len(top.New.Arrow) == 0 || top.New.Arrow[0].ID != top.New.ID || top.New.Arrow[0].Href != top.New.Href {
		t.Errorf("the arrow's first item repeats the button (C-6.2): %+v", top.New.Arrow)
	}

	m := frameShellRefresh.FindStringSubmatch(mockupFrame(t, "s-contacts-1"))
	if m == nil {
		t.Fatal("s-contacts-1 draws no refresh: read the frame again")
	}
	if top.Refresh.Action.Icon != m[1] {
		t.Errorf("Refresh icon = %q, the frame %q", top.Refresh.Action.Icon, m[1])
	}
	if top.UserHref != "/app/settings/account" {
		t.Errorf("the account button leads to %q without scripts, want its page", top.UserHref)
	}
}

func TestRailIsTheFrames(t *testing.T) {
	app := newMockupApp(t)
	page := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-1"), "<figcaption")

	// The sections, each with its icon.
	var want, got []string
	for _, m := range frameNavIcon.FindAllStringSubmatch(frame, -1) {
		want = append(want, frameText(m[2])+" ["+m[1]+"]")
	}
	for _, n := range page.Rail.Nav {
		got = append(got, string(n.Label)+" ["+n.Icon+"]")
	}
	if strings.Join(got, " ") != strings.Join(want, " ") || len(want) < 6 {
		t.Errorf("nav = %q, the frame %q", got, want)
	}

	// The open collection: the combined row on the combined list.
	openRows := func(frame string) (open, feeds []string) {
		for _, m := range frameShellTick.FindAllStringSubmatch(frame, -1) {
			l := frameShellTickLabel.FindStringSubmatch(m[2])
			if l == nil {
				continue
			}
			classes := " " + m[1] + " "
			if strings.Contains(classes, " is-on ") {
				open = append(open, l[1])
			}
			if strings.Contains(classes, " is-ext ") {
				feeds = append(feeds, l[1])
			}
		}
		return open, feeds
	}
	rows := func(r view.Rail) (open, feeds []string) {
		for _, s := range r.Sections {
			for _, row := range s.Rows {
				if row.On {
					open = append(open, string(row.Label))
				}
				if row.Feed {
					feeds = append(feeds, string(row.Label))
				}
			}
		}
		return open, feeds
	}
	wantOpen, _ := openRows(frame)
	if gotOpen, _ := rows(page.Rail); strings.Join(gotOpen, ",") != strings.Join(wantOpen, ",") || len(wantOpen) != 1 {
		t.Errorf("s-contacts-1: open rows %q, the frame %q", gotOpen, wantOpen)
	}
	// The card's rail: the book the record is in (s-contacts-6).
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	cp := decode[view.ContactPage](t, app.h.do(get(card, app.sess)))
	wantOpen, _ = openRows(mockupFrame(t, "s-contacts-6"))
	if gotOpen, _ := rows(cp.Rail); strings.Join(gotOpen, ",") != strings.Join(wantOpen, ",") || len(wantOpen) != 1 {
		t.Errorf("s-contacts-6: open rows %q, the frame %q", gotOpen, wantOpen)
	}
	// A timeline's rail: the combined calendar open, the feeds drawn as
	// feeds (s-contacts-7).
	tl := decode[view.ContactTimelinePage](t, app.h.do(get(card+"/timeline", app.sess)))
	wantOpen, wantFeeds := openRows(mockupFrame(t, "s-contacts-7"))
	gotOpen, gotFeeds := rows(tl.Rail)
	if strings.Join(gotOpen, ",") != strings.Join(wantOpen, ",") || strings.Join(gotFeeds, ",") != strings.Join(wantFeeds, ",") || len(wantFeeds) == 0 {
		t.Errorf("s-contacts-7: open %q feeds %q, the frame %q %q", gotOpen, gotFeeds, wantOpen, wantFeeds)
	}

	// The foot: the name and the build apart, the davloom mark, and no
	// Apply — the frame draws none, the ticks send themselves.
	m := frameShellAbout.FindStringSubmatch(frame)
	if m == nil {
		t.Fatal("s-contacts-1 draws no about line: read the frame again")
	}
	about := page.Rail.About
	if frameText(m[1]) != string(about.Label) || frameText(m[2]) != string(about.Build) || about.Href != "/about" {
		t.Errorf("about = %q %q, the frame %q %q", about.Label, about.Build, frameText(m[1]), frameText(m[2]))
	}
	e := frameShellEmblem.FindStringSubmatch(frame)
	if e == nil {
		t.Fatal("s-contacts-1 draws no davloom mark: read the frame again")
	}
	shape := regexp.MustCompile(`^davloom · running · \d+ devices?$`)
	if !shape.MatchString(e[1]) || !shape.MatchString(string(page.Rail.Davloom)) {
		t.Errorf("davloom = %q, the frame %q", page.Rail.Davloom, e[1])
	}
	for _, a := range page.Rail.Foot {
		if a.ID == "rail.apply" {
			t.Errorf("the foot carries Apply: %+v", page.Rail.Foot)
		}
	}
	if page.Rail.Ticks == nil || page.Rail.Ticks.Method != "POST" || page.Rail.Ticks.Action != "/app/sources/contacts" {
		t.Errorf("ticks = %+v, want the rail's form", page.Rail.Ticks)
	}
}

// Without davloom the mark is not there (components-36, the first strip).
func TestRailWithoutDavloomHasNoMark(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if page.Rail.Davloom != "" {
		t.Errorf("davloom off, mark %q", page.Rail.Davloom)
	}
}

// U-8: a tick of the rail is a submit button of the rail's form. It sends
// toggle with its row's href; the server flips that one source — the
// combined row all of them — and goes back where the form says.
func TestRailTickSendsItself(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[struct {
		Rail view.Rail `json:"rail"`
	}](t, f.h.do(get("/app/calendar", f.sess)))
	ticks := page.Rail.Ticks
	if ticks == nil {
		t.Fatal("the rail has no form for its ticks")
	}
	form := url.Values{}
	for _, s := range ticks.Sets {
		for _, r := range s.Rows {
			form.Set(r.Name, string(r.Value))
		}
	}
	if form.Get("back") != "/app/calendar" {
		t.Errorf("back = %q, want the list", form.Get("back"))
	}
	var one view.SourceRow
	for _, s := range page.Rail.Sections {
		for _, r := range s.Rows {
			if r.Href != "/app/calendar" {
				one = r
			}
		}
	}
	if one.Href == "" {
		t.Fatalf("no calendar in the rail: %+v", page.Rail.Sections)
	}
	send := func(v string) {
		t.Helper()
		form.Set("toggle", v)
		rec := f.h.do(postForm(ticks.Action, f.sess, form))
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/calendar" {
			t.Fatalf("toggle %s: %d %q", v, rec.Code, rec.Header().Get("Location"))
		}
	}
	before := len(selected(t, f.h, f.sess, view.KindEvent))
	send(one.Href)
	if got := selected(t, f.h, f.sess, view.KindEvent); len(got) != 1-before {
		t.Errorf("after the toggle: %v, before %d ticked", got, before)
	}
	send(one.Href)
	if got := selected(t, f.h, f.sess, view.KindEvent); len(got) != before {
		t.Errorf("after the toggle back: %v, before %d ticked", got, before)
	}
	if before == 1 {
		send(one.Href)
	}
	send("/app/calendar") // the combined row ticks every one
	if got := selected(t, f.h, f.sess, view.KindEvent); len(got) != 1 {
		t.Errorf("after the combined row: %v, want the calendar ticked", got)
	}
	send("/app/calendar") // and, all ticked, takes them all off
	if got := selected(t, f.h, f.sess, view.KindEvent); len(got) != 0 {
		t.Errorf("after the combined row again: %v, want nothing", got)
	}
	// A way back that is not a page of this Carrel is not taken.
	form.Set("back", "https://elsewhere.example/app/")
	form.Set("toggle", one.Href)
	if rec := f.h.do(postForm(ticks.Action, f.sess, form)); rec.Header().Get("Location") != "/app/calendar" {
		t.Errorf("back elsewhere led to %q", rec.Header().Get("Location"))
	}
}
