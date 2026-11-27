//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

// The candidate «Projects» (docs/plan/server.md, the owner's word of
// 27.11.2026 «Проведи тесты, которые влияют на выводы»): a project is a task
// whose links name its material — records by UID, a card by urn:uuid, a
// letter by mid:, a folder by its address (RFC 9253 LINK and RELATED-TO with
// VALUE=URI). Whether the idea stands on a server is whether the server
// keeps those lines as written, and whether Carrel's own save keeps what the
// server kept. A line the server drops is its answer, logged with the marker
// «project-links»; a line Carrel drops after the server kept it is Carrel's
// fault, S-7.

// TestYandexProjectLinks runs the check in the task list the account already
// has: a calendar Yandex makes on MKCALENDAR holds events only, so the forms
// run leaves tasks out there.
func TestYandexProjectLinks(t *testing.T) {
	endpoint, user, pass := yandexConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)
	col := ""
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindTask) && !c.ReadOnly {
			col = c.Path
			break
		}
	}
	if col == "" {
		t.Skip("project-links: no task list this account can write in the catalog")
	}
	suffix := fmt.Sprintf("carrel-yandex-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	liveProjectLinks(ctx, t, app, client, connID, col, suffix)
}

// projectLines are the lines of the project task under test, in the order
// written; the UID goes first.
func projectLines(uid, suffix string) []string {
	return []string{
		"UID:" + uid,
		"DTSTAMP:20261127T120000Z",
		"SUMMARY:Ремонт офиса",
		"DUE;VALUE=DATE:20270115",
		"STATUS:IN-PROCESS",
		"RELATED-TO;RELTYPE=CHILD:event-" + suffix,
		"RELATED-TO;RELTYPE=CHILD:task-" + suffix,
		"RELATED-TO;VALUE=URI;RELTYPE=CHILD:urn:uuid:4f2b7c9e-1d3a-4e8b-9a6f-2c1d0e9f8a7b",
		`LINK;LINKREL="related";VALUE=URI:urn:uuid:9c3d0a1e-5b7f-4c2d-8e9a-0b1c2d3e4f50`,
		`LINK;LINKREL="related";VALUE=URI:mid:20261120.4411@stroy.example`,
		// Long enough to be folded by a server: a folder whose name is Cyrillic.
		`LINK;LINKREL="related";VALUE=URI:https://cloud.example.org/remote.php/dav/files/me/%D0%A0%D0%B5%D0%BC%D0%BE%D0%BD%D1%82%20%D0%BE%D1%84%D0%B8%D1%81%D0%B0/`,
		`LINK;LINKREL="describedby";LABEL=Смета;VALUE=URI:https://cloud.example.org/s/estimate.pdf`,
	}
}

// unfolded are the lines of the task itself, folding undone (RFC 5545
// §3.1), CRLF taken as LF.
func unfolded(doc string) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n") {
		if (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) && len(out) > 0 {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return matrixLines(strings.Join(out, "\n"))
}

// propName is the name of a content line, upper case.
func propName(l string) string {
	if i := strings.IndexAny(l, ";:"); i >= 0 {
		return strings.ToUpper(l[:i])
	}
	return strings.ToUpper(l)
}

// sameLine is a line of got that says what want says: the same name, the
// same value and the same parameters in any order and any quoting.
func sameLine(want string, got []string) (string, bool) {
	norm := func(l string) (string, string, map[string]bool) {
		head, value, _ := strings.Cut(l, ":")
		// A value may hold a colon (urn:uuid:…): the head ends at the first
		// colon outside quotes.
		if i := headEnd(l); i >= 0 {
			head, value = l[:i], l[i+1:]
		}
		parts := strings.Split(head, ";")
		params := map[string]bool{}
		for _, p := range parts[1:] {
			k, v, _ := strings.Cut(p, "=")
			params[strings.ToUpper(k)+"="+strings.Trim(v, `"`)] = true
		}
		return strings.ToUpper(parts[0]), value, params
	}
	wn, wv, wp := norm(want)
	var near string
	for _, g := range got {
		gn, gv, gp := norm(g)
		if gn != wn || gv != wv {
			if gn == wn && near == "" && strings.Contains(g, wv[:min(len(wv), 12)]) {
				near = g
			}
			continue
		}
		equal := len(gp) == len(wp)
		for p := range wp {
			equal = equal && gp[p]
		}
		if equal {
			return g, true
		}
		near = g
	}
	return near, false
}

func headEnd(l string) int {
	quoted := false
	for i, r := range l {
		switch {
		case r == '"':
			quoted = !quoted
		case r == ':' && !quoted:
			return i
		}
	}
	return -1
}

// liveProjectLinks seeds the project task in col, reads it back, saves it
// once through Carrel's form with the summary changed and reads it again.
func liveProjectLinks(ctx context.Context, t *testing.T, app *liveApp, client *dav.Client, connID, col, suffix string) {
	uid := "project-" + suffix
	file := col + uid + ".ics"
	lines := projectLines(uid, suffix)
	doc := matrixCal("BEGIN:VTODO\n" + strings.Join(lines, "\n") + "\nEND:VTODO")
	if !trySeed(ctx, t, client, file, "text/calendar; charset=utf-8", doc) {
		t.Logf("project-links: the server refused the project task")
		t.Skip("the server refused the record")
	}
	removeLater(t, client, file)
	seeded := bodyIfThere(ctx, t, client, file)
	if seeded == "" {
		t.Skip("the server kept nothing at the address")
	}
	if kept := uidOf(seeded); kept != "" && kept != uid {
		t.Logf("project-links: the server named the record %q, not %q", kept, uid)
		uid = kept
		removeLater(t, client, col+kept)
	}
	got := unfolded(seeded)
	kept := map[string]bool{}
	for _, want := range lines[1:] {
		line, ok := sameLine(want, got)
		switch {
		case ok && line == want:
			t.Logf("project-links: server kept %s", want)
			kept[want] = true
		case ok:
			t.Logf("project-links: server kept %s, written as %s", want, line)
			kept[want] = true
		case line != "":
			t.Logf("project-links: server CHANGED %s into %s", want, line)
		default:
			t.Logf("project-links: server DROPPED %s", want)
		}
	}
	for _, g := range got {
		if n := propName(g); n == "LINK" || n == "RELATED-TO" {
			if _, ok := sameLine(g, lines); !ok {
				t.Logf("project-links: server holds a line not written: %s", g)
			}
		}
	}

	// Carrel's save with another field changed: every line the server kept
	// stays.
	rec := matrixList(view.KindTask, connID, col) + "/" + url.PathEscape(uid)
	lost := matrixSave(t, app, view.KindTask, rec, map[string]string{"summary": "Ремонт офиса, этап 2"})
	after, _ := rawBody(ctx, t, client, file)
	afterLines := unfolded(after)
	if _, ok := sameLine(`SUMMARY:Ремонт офиса\, этап 2`, afterLines); !ok {
		t.Errorf("the summary was not saved:\n%s", after)
	}
	for _, want := range lines[1:] {
		if !kept[want] || propName(want) == "SUMMARY" || propName(want) == "DTSTAMP" {
			continue
		}
		line, ok := sameLine(want, afterLines)
		switch {
		case ok:
			t.Logf("project-links: Carrel's save kept %s", want)
		case contains(lost, propName(want)):
			t.Logf("project-links: Carrel's save reported the server did not keep %s", want)
		case line != "":
			t.Errorf("project-links: Carrel's save CHANGED %s into %s", want, line)
		default:
			t.Errorf("project-links: Carrel's save DROPPED %s\nafter:\n%s", want, after)
		}
	}
}
