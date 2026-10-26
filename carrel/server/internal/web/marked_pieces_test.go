package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
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

// s-profile-9: «Last archived ⟨2026-08-14⟩; a restore makes a ⟨new⟩
// collection at a new address.»
func TestCollectionDeleteMarksItsPieces(t *testing.T) {
	msgs := frameMsgs(t, "s-profile-9")
	shape := regexp.MustCompile(`^Last archived ⟨mono:(\d{4}-\d\d-\d\d)⟩; .*⟨strong:new⟩`)
	var archive string
	var day []string
	for _, m := range msgs {
		if d := shape.FindStringSubmatch(m); d != nil {
			archive, day = m, d
		}
	}
	if archive == "" {
		t.Fatalf("s-profile-9 draws no archive line with its day monospaced and «new» in bold: %q", msgs)
	}
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T100000Z\nSUMMARY:One\nEND:VEVENT"))
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.BackupJobs = append(b.BackupJobs, sealed.BackupJob{ID: "j-1", Name: "Личное",
			Sources: []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: f.col}}})
	})
	lastArchived(t, f, "j-1", day[1])
	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get("/app/collections/delete?"+url.Values{"a": {f.id}, "c": {f.col}}.Encode(), f.sess)))
	if got := saidRuns(sheet.Archive); got != archive {
		t.Errorf("archive = %q,\nthe frame %q", got, archive)
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
	// Nothing connected yet: who is signed in, and nothing more (s-profile-25).
	if got := saidRuns(page.Subtitle); got != "Signed in as ⟨strong:mix⟩" {
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
	p := openMerge(t, f.h, f.sess, onlyGroup(t, f.page(t)), nil)
	// The plan is said without a bold sentence (s-dup-4, note 135).
	if b := bold("s-dup-4"); len(b) != 0 || p.Warning.Title != "" || !msgDrawn(frameMsgs(t, "s-dup-4"), string(p.Warning.Body)) {
		t.Errorf("merge: title %q, body %q; the frame %q", p.Warning.Title, p.Warning.Body, frameMsgs(t, "s-dup-4"))
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
