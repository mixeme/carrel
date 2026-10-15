package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The pieces of a server's sentence a frame marks (C-6.2 Runs; plan
// «Выделенные куски в других фразах сервера»): a word to type or a command
// monospaced, a count or the one word that matters in bold. A frame's
// sentence is written the way saidRuns writes the server's.

var (
	pieceMono   = regexp.MustCompile(`(?s)<(?:code|span) class="tc-mono[^"]*">(.*?)</(?:code|span)>`)
	pieceStrong = regexp.MustCompile(`(?s)<b>(.*?)</b>`)
	pieceDanger = regexp.MustCompile(`(?s)<span class="tc-dim--danger[^"]*">(.*?)</span>`)
	pieceMsg    = regexp.MustCompile(`(?s)<div class="tc-msg[^"]*">(.*?)</div>`)
)

// framePieces is a frame's sentence with its marked pieces kept.
func framePieces(s string) string {
	s = pieceMono.ReplaceAllString(s, "⟨mono:$1⟩")
	s = pieceStrong.ReplaceAllString(s, "⟨strong:$1⟩")
	s = pieceDanger.ReplaceAllString(s, "⟨danger:$1⟩")
	return authText(s)
}

// frameMsgs are the messages of a frame, marked, its caption left out.
func frameMsgs(t *testing.T, id string) []string {
	t.Helper()
	frame, _, _ := strings.Cut(mockupFrame(t, id), "<figcaption")
	var out []string
	for _, m := range pieceMsg.FindAllStringSubmatch(frame, -1) {
		out = append(out, framePieces(m[1]))
	}
	if len(out) == 0 {
		t.Fatalf("%s draws no message: read the frame again", id)
	}
	return out
}

func msgDrawn(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// s-contacts-2, s-files-4: «Inside one server this is ⟨MOVE⟩: …».
func TestMoveSheetsMarkMOVE(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "x")
	f.srv.SeedFolder(filesDocs + "dest/")
	op := f.at("op", "") + "?" + url.Values{"op": {"move"}, "item": {"a.txt"}}.Encode()
	sheet := opSheet(t, f, op, url.Values{"to": {f.at("", "dest", "")}})
	if got := saidRuns(sheet.Warning); !msgDrawn(frameMsgs(t, "s-files-4"), got) {
		t.Errorf("files: %q, the frame %q", got, frameMsgs(t, "s-files-4"))
	}

	app := newMockupApp(t)
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	page := decode[view.ContactPage](t, app.h.do(get(card, app.sess)))
	var move string
	for _, m := range page.Menu {
		if m.ID == "record.move" {
			move = m.Href
		}
	}
	rs := decode[view.MoveSheet](t, app.h.do(get(move, app.sess)))
	if got := saidRuns(rs.Warning); !msgDrawn(frameMsgs(t, "s-contacts-2"), got) {
		t.Errorf("contact: %q, the frame %q", got, frameMsgs(t, "s-contacts-2"))
	}
}

// s-profile-9: «… all ⟨51⟩ events in it are deleted …», «Last time it was
// in an archive: ⟨2026-08-14⟩. Restoring brings the records back into a
// ⟨new⟩ collection …».
func TestCollectionDeleteMarksItsPieces(t *testing.T) {
	msgs := frameMsgs(t, "s-profile-9")
	shape := regexp.MustCompile(`⟨strong:\d+⟩`)
	var count string
	for _, m := range msgs {
		if shape.MatchString(m) && strings.Contains(m, "are deleted on") {
			count = m
		}
	}
	if count == "" {
		t.Fatalf("s-profile-9 draws no count in bold: %q", msgs)
	}
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T100000Z\nSUMMARY:One\nEND:VEVENT"),
		"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260101T000000Z\nDTSTART:20260811T100000Z\nSUMMARY:Two\nEND:VEVENT"))
	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get("/app/collections/delete?"+url.Values{"a": {f.id}, "c": {f.col}}.Encode(), f.sess)))
	got := saidRuns(sheet.Warning)
	want := shape.ReplaceAllString(count, "⟨strong:2⟩")
	host := regexp.MustCompile(`deleted on .*?\. Carrel`)
	want = host.ReplaceAllString(want, "deleted on X. Carrel")
	got = host.ReplaceAllString(got, "deleted on X. Carrel")
	// What the calendar holds names its unit: «events», or «records» for
	// one that takes tasks too.
	unit := regexp.MustCompile(`⟩ \w+ in it`)
	want, got = unit.ReplaceAllString(want, "⟩ X in it"), unit.ReplaceAllString(got, "⟩ X in it")
	if want != got {
		t.Errorf("warning = %q,\nthe frame %q", got, want)
	}
}

// s-profile-10: «Signed in as ⟨mix⟩ · …»; s-davloom-1: «… does ⟨not⟩
// disconnect devices …».
func TestSettingsSentencesMarkTheirPieces(t *testing.T) {
	sub := regexp.MustCompile(`(?s)<div class="tc-head__sub">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, "s-profile-10"))
	if sub == nil || !strings.HasPrefix(framePieces(sub[1]), "Signed in as ⟨strong:") {
		t.Fatalf("s-profile-10 draws no login in bold: %v", sub)
	}
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("mix", "password123")
	mix := h.login("mix", "password123")
	page := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", mix)))
	if got := saidRuns(page.Subtitle); !strings.HasPrefix(got, "Signed in as ⟨strong:mix⟩ · ") {
		t.Errorf("subtitle = %q", got)
	}
	dl := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", mix)))
	if got := saidRuns(dl.Note); !msgDrawn(frameMsgs(t, "s-davloom-1"), got) {
		t.Errorf("davloom note = %q, the frame %q", got, frameMsgs(t, "s-davloom-1"))
	}
}

// The first sentence of a message a frame draws in bold is the notice's
// Title, its stop with it: s-dup-5, s-dup-6.
func TestNoticeTitlesAreTheFramesBoldSentence(t *testing.T) {
	bold := func(id string) map[string]bool {
		out := map[string]bool{}
		for _, m := range frameMsgs(t, id) {
			if p := regexp.MustCompile(`^⟨strong:([^⟩]*)⟩`).FindStringSubmatch(m); p != nil {
				out[p[1]] = true
			}
		}
		return out
	}
	f := newDupFixture(t)
	empty := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	var titles []string
	for _, n := range empty.Notices {
		titles = append(titles, string(n.Title))
	}
	if b := bold("s-dup-6"); !msgDrawn(titles, firstKey(b)) {
		t.Errorf("nothing loaded: titles %q, the frame %v", titles, b)
	}
	f.load(t)
	p := openMerge(t, f.h, f.sess, actionOf(t, onlyGroup(t, f.page(t)), "duplicates.merge"), nil)
	if b := bold("s-dup-4"); !b[string(p.Warning.Title)] {
		t.Errorf("merge: title %q, the frame %v", p.Warning.Title, b)
	}
	f.srv.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	f.other.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	got := decode[view.DuplicatesMergePage](t, f.h.do(postForm(mergeAction(t, p, "duplicates.merge.confirm").Href, f.sess, url.Values{})))
	if b := bold("s-dup-5"); !b[string(got.Warning.Title)] {
		t.Errorf("stopped: title %q, the frame %v", got.Warning.Title, b)
	}
}

func firstKey(m map[string]bool) string {
	for k := range m {
		return k
	}
	return ""
}

// s-notes-1: the note's day in the panel's line, monospaced — apart from
// the line, as the note page keeps it (NoteMeta.Date).
func TestNotePanelDayIsApart(t *testing.T) {
	sub := regexp.MustCompile(`(?s)<div class="tc-side"[^>]*>.*?<div class="tc-head__sub">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, "s-notes-1"))
	if sub == nil || !strings.Contains(framePieces(sub[1]), "⟨mono:") {
		t.Fatalf("s-notes-1 draws no day in the panel's line: %v", sub)
	}
	f := calendarFixture(t, nil,
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Reading\nDTSTART:20260810T080500Z\nEND:VJOURNAL"))
	d := decode[detailsFragment](t, f.h.do(fragment(get(f.path("notes")+"/n1", f.sess), "details"))).Body
	if d.Date == "" || strings.Contains(string(d.Subtitle), string(d.Date)) {
		t.Errorf("panel = %+v: the day apart from the line", d)
	}
}
