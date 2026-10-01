package web_test

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"carrel.dev/carrel/server/view"
)

// The card of a contact as the frames draw it (the client's scan of the
// contacts section, 30.09.2026, group В): the head's buttons with icons,
// phones and the birthday in mono, the tags a row of the fields between
// Birthday and Note, «in 7 months» by the birthday, «Show 3» by the kept
// properties, the other books with their colours and why they differ, the
// timeline inset with the count in its rubric and colours by its rows, and
// the source with the path and whether it changed on the server.

var (
	frameCardHead  = regexp.MustCompile(`(?s)<div class="tc-main">\s*<div class="tc-head">.*?<div class="tc-acts">(.*?)</div>`)
	frameCardField = regexp.MustCompile(`(?s)<dt>([^<]*)</dt>\s*<dd( class="tc-mono")?>(.*?)</dd>`)
	frameFieldLink = regexp.MustCompile(`(?s)<a class="tc-fields__link" href="([^"]*)">(.*?)</a>`)
	frameDim       = regexp.MustCompile(`<span class="tc-dim">([^<]*)</span>`)
	frameShowKept  = regexp.MustCompile(`Kept as the server sent them <button class="tc-btn is-sm" type="button">([^<]*)</button>`)
	frameInset     = regexp.MustCompile(`<div class="tc-rubric">Timeline <span class="tc-mono[^"]*">(\d+)</span>`)
	frameInsetRow  = regexp.MustCompile(`<div class="tc-row tc-row--find"><span class="tc-mark" data-tc-colour="(#[0-9A-Fa-f]{6})"></span>`)
	frameOpenTL    = regexp.MustCompile(`<button class="tc-btn" type="button"><svg[^>]*><use href="#(i-[\w-]+)"/></svg>Open the timeline</button>`)
	frameCardDiag  = regexp.MustCompile(`<div class="tc-diag">Path\s+(\S+)\s+Read\s+[^·<]*·\s*([^<]*)</div>`)
	frameBookRow   = regexp.MustCompile(`(?s)<li><span class="tc-mark[^"]*" data-tc-colour="(#[0-9A-Fa-f]{6})"></span>\s*<a[^>]*>([^<]*)</a>\s*<span class="tc-dim">· ([^<]*)</span></li>`)
)

func TestContactCardIsTheFrames(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	// The inset shows the timeline this session gathered (S-15.4).
	tl := decode[view.ContactTimelinePage](t, app.h.do(get(card+"/timeline", app.sess)))
	waitPoll(t, app.h, app.sess, tl.Poll.TaskID, func(b view.PollBar) bool { return !b.Running })
	tl = decode[view.ContactTimelinePage](t, app.h.do(get(card+"/timeline", app.sess)))
	page := decode[view.ContactPage](t, app.h.do(get(card, app.sess)))
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-6"), "<figcaption")

	head := frameCardHead.FindStringSubmatch(frame)
	if head == nil {
		t.Fatal("s-contacts-6 draws no head: read the frame again")
	}
	// The «⋯» is a button with its menu, drawn closed (from-typecase note
	// 28, task 49): the buttons of the head are those before its menu, and
	// the menu is the card's own, item by item.
	buttons, menu, closed := strings.Cut(head[1], `<div class="tc-menu`)
	if !closed || !strings.Contains(buttons, `<span class="tc-drop">`) || !strings.Contains(menu, " hidden>") {
		t.Fatal("s-contacts-6 draws no closed «⋯» with its menu in the head: read the frame again")
	}
	sameMenu(t, "card", page.Menu, "s-contacts-6")
	var want, got []string
	for _, b := range frameHeadButton.FindAllStringSubmatch(buttons, -1) {
		if label := frameText(b[2]); label != "" {
			icon := frameIcon.FindStringSubmatch(b[2])[1]
			want = append(want, label+" ["+icon+"]")
		}
	}
	for _, a := range page.Actions {
		if !a.Danger {
			got = append(got, string(a.Label)+" ["+a.Icon+"]")
		}
	}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("head = %q, the frame %q", got, want)
	}
	if p := actionByID(page.Actions, "record.print"); p == nil || p.Href != card+"?print=1" {
		t.Errorf("Print = %+v, want the card with print=1", p)
	}

	// The field list: mono where the frame draws it, the tags as a row of
	// it between Birthday and Note, and what the frame writes dim by the
	// birthday.
	byLabel := map[string]view.Field{}
	var order []string
	for _, f := range page.Fields {
		byLabel[string(f.Label)] = f
		order = append(order, string(f.Label))
	}
	var frameOrder []string
	for _, m := range frameCardField.FindAllStringSubmatch(frame, -1) {
		label := frameText(m[1])
		frameOrder = append(frameOrder, label)
		f, ok := byLabel[label]
		if !ok {
			continue // the set's Анна has fewer rows than the frame's
		}
		if f.Mono != (m[2] != "") {
			t.Errorf("%q: mono %v, the frame %v", label, f.Mono, m[2] != "")
		}
		// A value that goes somewhere is a link (task 52): a phone to
		// call, an address to write to — where the frame draws one, the
		// server's is the same.
		if l := frameFieldLink.FindStringSubmatch(m[3]); l != nil && (f.Href != html.UnescapeString(l[1]) || string(f.Value) != frameText(l[2])) {
			t.Errorf("%q: %q → %q, the frame %q → %q", label, f.Value, f.Href, frameText(l[2]), l[1])
		}
		if label == "Tags" && (len(f.Tags) == 0 || f.Value != "") {
			t.Errorf("Tags is a row of tags: %+v", f)
		}
		if d := frameDim.FindStringSubmatch(m[3]); d != nil {
			shape := "^" + frameDigits.ReplaceAllString(regexp.QuoteMeta(strings.TrimPrefix(frameText(d[1]), "· ")), `\d+`) + "$"
			if !regexp.MustCompile(shape).MatchString(string(f.Aside)) {
				t.Errorf("%q: aside %q, the frame %q", label, f.Aside, d[1])
			}
		}
	}
	for _, f := range page.Fields {
		phone, email := strings.HasPrefix(string(f.Label), "Phone"), strings.HasPrefix(string(f.Label), "Email")
		if phone && !strings.HasPrefix(f.Href, "tel:") || email && f.Href != "mailto:"+string(f.Value) {
			t.Errorf("%q = %q goes to %q: every phone is a tel: link, every address a mailto:", f.Label, f.Value, f.Href)
		}
	}
	at := func(list []string, s string) int {
		for i, x := range list {
			if x == s {
				return i
			}
		}
		return -1
	}
	if b, tg, n := at(order, "Birthday"), at(order, "Tags"), at(order, "Note"); !(b >= 0 && b+1 == tg && tg+1 == n) || at(frameOrder, "Tags") != at(frameOrder, "Birthday")+1 {
		t.Errorf("fields %q, the frame %q: Tags between Birthday and Note", order, frameOrder)
	}

	// «Show 3» by the kept properties.
	if m := frameShowKept.FindStringSubmatch(frame); m == nil {
		t.Fatal("s-contacts-6 draws no «Show N»: read the frame again")
	} else if shape := regexp.MustCompile("^" + frameDigits.ReplaceAllString(m[1], `\d+`) + "$"); !shape.MatchString(string(page.ForeignShow)) || !strings.HasSuffix(string(page.ForeignShow), " "+strconv.Itoa(len(page.Foreign))) {
		t.Errorf("foreign show = %q for %d, the frame %q", page.ForeignShow, len(page.Foreign), m[1])
	}

	// The inset: its count, a colour by each row, the icon of the way to it.
	if m := frameInset.FindStringSubmatch(frame); m == nil || len(frameInsetRow.FindAllString(frame, -1)) == 0 {
		t.Fatal("s-contacts-6 draws no inset with its count: read the frame again")
	}
	total := 0
	for _, g := range tl.Groups {
		total += len(g.Rows)
	}
	if page.TimelineCount == nil || *page.TimelineCount != total || len(page.Timeline) == 0 {
		t.Errorf("inset count %v, rows %d, the timeline %d", page.TimelineCount, len(page.Timeline), total)
	}
	for _, r := range page.Timeline {
		if r.Colour == "" {
			t.Errorf("an inset row without its colour: %+v", r)
		}
	}
	if m := frameOpenTL.FindStringSubmatch(frame); m == nil || page.More.Icon != m[1] {
		t.Errorf("More = %+v, the frame %v", page.More, m)
	}

	// The source: the path, and whether it changed on the server.
	m := frameCardDiag.FindStringSubmatch(frame)
	if m == nil {
		t.Fatal("s-contacts-6 draws no path in its source: read the frame again")
	}
	if !strings.HasSuffix(string(page.Source.Path), ".vcf") || string(page.Source.Compared) != frameText(m[2]) {
		t.Errorf("source = %+v, the frame %q", page.Source, m[2])
	}

	// The same person in other books: a colour, a link, and why, capital
	// first as the frame writes it.
	books := frameBookRow.FindAllStringSubmatch(frame, -1)
	if len(books) == 0 {
		t.Fatal("s-contacts-6 draws no other books: read the frame again")
	}
	artem := decode[view.ContactPage](t, app.h.do(get(app.listPath(t, "contacts", "komanda")+"/d41a-6f30-92be", app.sess)))
	if len(artem.Books) == 0 {
		t.Fatalf("Артём is linked in the set, his card has no other books: %+v", artem)
	}
	for _, b := range artem.Books {
		if b.Colour == "" || b.Href == "" || b.Value == "" || !unicode.IsUpper([]rune(string(b.Value))[0]) {
			t.Errorf("book row = %+v, the frame %q", b, books[0][2]+" · "+books[0][3])
		}
	}
}
