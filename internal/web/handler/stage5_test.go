// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/fanout"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const testCalendarPath = "/calendars/user/default/"

// calBox is a calendar collection holding all three component kinds, spoken to
// over HTTP so the tests below exercise the client, the provider and the
// handler together (§21).
type calBox struct {
	*httptest.Server

	mu      sync.Mutex
	objects map[string]*calObject
	ctag    int
	// puts records what was written, so a test can check the body that went
	// to the server rather than only what the screen says afterwards.
	puts        []calPut
	deleted     []string
	failNextPut bool
}

type calObject struct {
	etag string
	body string
}

type calPut struct {
	Path, Body, IfMatch, IfNoneMatch string
}

// calQuery is the part of a CalDAV report the box needs to answer: which
// component is asked for, and any text match on one of its properties.
type calQuery struct {
	XMLName xml.Name
	Hrefs   []string `xml:"href"`
	Filter  struct {
		Comp struct {
			Name string `xml:"name,attr"`
			Comp struct {
				Name        string `xml:"name,attr"`
				PropFilters []struct {
					Name      string `xml:"name,attr"`
					TextMatch string `xml:"text-match"`
				} `xml:"prop-filter"`
			} `xml:"comp-filter"`
		} `xml:"comp-filter"`
	} `xml:"filter"`
}

func startCalBox(t *testing.T) *calBox {
	t.Helper()
	box := &calBox{objects: map[string]*calObject{}, ctag: 1}
	box.seed("meeting.ics", `"e1"`, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VEVENT\r\n"+
		"UID:meeting\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Budget meeting\r\n"+
		"DTSTART:20260812T100000Z\r\nDTEND:20260812T110000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n")
	box.seed("chase.ics", `"t1"`, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VTODO\r\n"+
		"UID:chase\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Chase the invoice\r\n"+
		"DESCRIPTION:about the budget\r\nDUE:20260812T170000Z\r\nSTATUS:NEEDS-ACTION\r\n"+
		"CATEGORIES:money\r\nX-CARREL-KEEP:keep-me\r\nEND:VTODO\r\nEND:VCALENDAR\r\n")
	box.seed("filed.ics", `"t2"`, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VTODO\r\n"+
		"UID:filed\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Filed the receipts\r\n"+
		"STATUS:COMPLETED\r\nPERCENT-COMPLETE:100\r\nCOMPLETED:20260805T090000Z\r\n"+
		"END:VTODO\r\nEND:VCALENDAR\r\n")
	box.seed("thought.ics", `"j1"`, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VJOURNAL\r\n"+
		"UID:thought\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:A thought\r\n"+
		"DESCRIPTION:Ideas about the budget.\r\nDTSTART;VALUE=DATE:20260812\r\n"+
		"CATEGORIES:ideas\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n")
	box.Server = httptest.NewServer(http.HandlerFunc(box.serve))
	t.Cleanup(box.Close)
	return box
}

func (b *calBox) seed(name, etag, body string) string {
	path := testCalendarPath + name
	b.objects[path] = &calObject{etag: etag, body: body}
	return path
}

func (b *calBox) body(t *testing.T, name string) string {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	obj := b.objects[testCalendarPath+name]
	if obj == nil {
		t.Fatalf("no object %q on the server", name)
	}
	return obj.body
}

// lastPut is the most recent write, which is what a save is judged by.
func (b *calBox) lastPut(t *testing.T) calPut {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.puts) == 0 {
		t.Fatal("nothing was written to the server")
	}
	return b.puts[len(b.puts)-1]
}

func (b *calBox) serve(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" && b.objects[path] != nil:
		b.multistatus(w, fmt.Sprintf(
			`<d:response><d:href>%s</d:href><d:propstat><d:prop><d:getetag>%s</d:getetag></d:prop>`+
				`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, path, b.objects[path].etag))
	case r.Method == "PROPFIND" && r.Header.Get("Depth") == "0":
		b.multistatus(w, fmt.Sprintf(
			`<d:response><d:href>%s</d:href><d:propstat><d:prop><cs:getctag>ctag-%d</cs:getctag></d:prop>`+
				`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, path, b.ctag))
	case r.Method == "PROPFIND":
		var body strings.Builder
		fmt.Fprintf(&body, `<d:response><d:href>%s</d:href><d:propstat><d:prop>`+
			`<d:resourcetype><d:collection/></d:resourcetype></d:prop>`+
			`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, testCalendarPath)
		for objectPath, obj := range b.objects {
			fmt.Fprintf(&body, `<d:response><d:href>%s</d:href><d:propstat><d:prop><d:resourcetype/>`+
				`<d:getetag>%s</d:getetag><d:getcontenttype>text/calendar</d:getcontenttype></d:prop>`+
				`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, objectPath, obj.etag)
		}
		b.multistatus(w, body.String())
	case r.Method == "REPORT":
		b.report(w, r)
	case r.Method == http.MethodGet:
		obj := b.objects[path]
		if obj == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("ETag", obj.etag)
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = io.WriteString(w, obj.body)
	case r.Method == http.MethodPut:
		raw, _ := io.ReadAll(r.Body)
		b.puts = append(b.puts, calPut{
			Path: path, Body: string(raw),
			IfMatch: r.Header.Get("If-Match"), IfNoneMatch: r.Header.Get("If-None-Match"),
		})
		if b.failNextPut {
			b.failNextPut = false
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		existing, exists := b.objects[path]
		if r.Header.Get("If-None-Match") == "*" && exists {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		if match := r.Header.Get("If-Match"); match != "" && (!exists || existing.etag != match) {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		b.ctag++
		etag := fmt.Sprintf("%q", fmt.Sprintf("v%d", b.ctag))
		b.objects[path] = &calObject{etag: etag, body: string(raw)}
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodDelete:
		if b.objects[path] == nil {
			http.NotFound(w, r)
			return
		}
		delete(b.objects, path)
		b.deleted = append(b.deleted, path)
		b.ctag++
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func (b *calBox) report(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var query calQuery
	if err := xml.Unmarshal(raw, &query); err != nil {
		http.Error(w, "bad report", http.StatusBadRequest)
		return
	}
	var paths []string
	if query.XMLName.Local == "calendar-multiget" {
		paths = query.Hrefs
	} else {
		for path, obj := range b.objects {
			if b.matches(obj.body, query) {
				paths = append(paths, path)
			}
		}
	}
	var body strings.Builder
	for _, path := range paths {
		obj := b.objects[path]
		if obj == nil {
			fmt.Fprintf(&body, `<d:response><d:href>%s</d:href><d:status>HTTP/1.1 404 Not Found</d:status></d:response>`, path)
			continue
		}
		fmt.Fprintf(&body, `<d:response><d:href>%s</d:href><d:propstat><d:prop><d:getetag>%s</d:getetag>`+
			`<cal:calendar-data>%s</cal:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status>`+
			`</d:propstat></d:response>`, path, obj.etag, xmlEscapeText(obj.body))
	}
	b.multistatus(w, body.String())
}

// matches applies the component and text filters. The time range is not applied:
// a server that answers a wider set than asked is the case worth testing, and
// the provider narrows what it gets.
func (b *calBox) matches(body string, query calQuery) bool {
	inner := query.Filter.Comp.Comp
	if inner.Name != "" && !strings.Contains(body, "BEGIN:"+inner.Name+"\r\n") {
		return false
	}
	for _, filter := range inner.PropFilters {
		if strings.TrimSpace(filter.TextMatch) == "" {
			continue
		}
		found := false
		for _, line := range strings.Split(body, "\r\n") {
			if !strings.HasPrefix(line, filter.Name+":") && !strings.HasPrefix(line, filter.Name+";") {
				continue
			}
			if strings.Contains(strings.ToLower(line), strings.ToLower(strings.TrimSpace(filter.TextMatch))) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (b *calBox) multistatus(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, xml.Header+
		`<d:multistatus xmlns:d="DAV:" xmlns:cs="http://calendarserver.org/ns/" `+
		`xmlns:cal="urn:ietf:params:xml:ns:caldav">`+body+`</d:multistatus>`)
}

// calendarApp is an instance signed in as an administrator with one calendar
// connected and the fan-out registry wired, which is what every stage-5 screen
// needs.
func calendarApp(t *testing.T, box *calBox) (*app, string, string) {
	t.Helper()
	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.Import = config.Import{MaxBytes: 1 << 20, MaxCards: 100}
	a.Fanout = fanout.NewRegistry(nil)
	t.Cleanup(a.Fanout.Close)
	a.setupAdmin("root", "", testPassword)

	sess := a.session()
	acc := account.Account{
		ID: "cal-acc", Label: "Work", BaseURL: box.URL + "/",
		Username: "mix", Password: "secret", Enabled: true,
		Collections: []discovery.Collection{{
			Path: testCalendarPath, DisplayName: "Personal", Kind: discovery.KindCalendar,
		}},
	}
	if err := a.Store.PutDAVAccount(store.Actor{ID: sess.UserID, Login: sess.Login}, sess.UserID, sess.DEK(), acc); err != nil {
		t.Fatal(err)
	}
	return a, acc.ID, EncodeCollectionPath(testCalendarPath)
}

func TestTasksListSeparatesOpenFromDone(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	open := a.get("/app/tasks/" + accID + "/" + colEnc)
	if open.Code != http.StatusOK {
		t.Fatalf("task list = %d, body = %s", open.Code, open.Body.String())
	}
	body := open.Body.String()
	if !strings.Contains(body, "Chase the invoice") {
		t.Errorf("the open task is missing:\n%s", body)
	}
	if strings.Contains(body, "Filed the receipts") {
		t.Error("a completed task is on the open list")
	}
	// The event and the note in the same collection are not tasks.
	if strings.Contains(body, "Budget meeting") || strings.Contains(body, "A thought") {
		t.Error("the task list shows objects of other kinds")
	}
	if !strings.Contains(body, "Open (1)") || !strings.Contains(body, "Done (1)") {
		t.Errorf("the counts are wrong:\n%s", body)
	}

	done := a.get("/app/tasks/" + accID + "/" + colEnc + "?filter=done")
	if !strings.Contains(done.Body.String(), "Filed the receipts") {
		t.Errorf("the done filter does not show the completed task:\n%s", done.Body.String())
	}
}

// Ticking a task off a list is a three-property edit. Everything else on the
// object, including a property Carrel knows nothing about, has to survive (§8).
func TestTaskToggleCompletesAndKeepsTheRest(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.post("/app/tasks/"+accID+"/"+colEnc+"/chase", url.Values{
		fieldAction: {"toggle"},
		"etag":      {`"t1"`},
		"filter":    {"open"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("toggle = %d, body = %s", rec.Code, rec.Body.String())
	}
	written := box.lastPut(t)
	if written.IfMatch != `"t1"` {
		t.Errorf("If-Match = %q, want the version the screen was showing", written.IfMatch)
	}
	for _, want := range []string{"STATUS:COMPLETED", "PERCENT-COMPLETE:100", "COMPLETED:", "X-CARREL-KEEP:keep-me", "SUMMARY:Chase the invoice"} {
		if !strings.Contains(written.Body, want) {
			t.Errorf("the written task is missing %q:\n%s", want, written.Body)
		}
	}
}

func TestTaskCreateWritesAVTodo(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.post("/app/tasks/"+accID+"/"+colEnc+"/new", url.Values{
		"summary":          {"Renew the domain"},
		"status":           {"NEEDS-ACTION"},
		"due_date":         {"2027-01-05"},
		"categories":       {"admin, money"},
		"priority":         {"0"},
		"percent_complete": {"0"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create = %d, body = %s", rec.Code, rec.Body.String())
	}
	written := box.lastPut(t)
	if written.IfNoneMatch != "*" {
		t.Errorf("If-None-Match = %q, want * so a create cannot overwrite", written.IfNoneMatch)
	}
	for _, want := range []string{"BEGIN:VTODO", "SUMMARY:Renew the domain", "DUE;VALUE=DATE:20270105", "CATEGORIES:admin,money"} {
		if !strings.Contains(written.Body, want) {
			t.Errorf("the new task is missing %q:\n%s", want, written.Body)
		}
	}
}

func TestTaskFormRejectsATaskWithNoSummary(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.post("/app/tasks/"+accID+"/"+colEnc+"/new", url.Values{
		"summary": {"  "}, "status": {"NEEDS-ACTION"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "needs a summary") {
		t.Errorf("no explanation on the form:\n%s", rec.Body.String())
	}
	box.mu.Lock()
	writes := len(box.puts)
	box.mu.Unlock()
	if writes != 0 {
		t.Errorf("%d writes left the process for a form that did not validate", writes)
	}
}

func TestNoteFullScreenReadAndEdit(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	read := a.get("/app/notes/" + accID + "/" + colEnc + "/thought")
	if read.Code != http.StatusOK {
		t.Fatalf("note read = %d, body = %s", read.Code, read.Body.String())
	}
	body := read.Body.String()
	for _, want := range []string{
		`data-note-doc`,
		`note-read-title`,
		`Neighbours`,
		`Ideas about the budget`,
		`?edit=1`,
		`data-copy-url`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("read view missing %q", want)
		}
	}
	if strings.Contains(body, `data-note-form`) {
		t.Error("read view should not include the edit form")
	}

	edit := a.get("/app/notes/" + accID + "/" + colEnc + "/thought?edit=1")
	if edit.Code != http.StatusOK {
		t.Fatalf("note edit = %d", edit.Code)
	}
	editBody := edit.Body.String()
	if !strings.Contains(editBody, `data-note-form`) {
		t.Error("edit view should include the edit form")
	}
	if !strings.Contains(editBody, `data-note-markup`) {
		t.Error("edit view should include markup buttons")
	}
	if !strings.Contains(editBody, `data-related-picker`) {
		t.Error("edit view should include the related-to picker")
	}
	if strings.Contains(editBody, `UIDs, comma-separated`) {
		t.Error("edit view still shows the old related placeholder")
	}
}

func TestNoteRelatedSearchFindsObjects(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.get("/app/notes/" + accID + "/" + colEnc + "/related-search?q=budget")
	if rec.Code != http.StatusOK {
		t.Fatalf("related search = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want JSON", ct)
	}
	var hits []struct {
		UID, Title, Kind string
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &hits); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("expected several matches, got %d: %+v", len(hits), hits)
	}
	seen := map[string]bool{}
	for _, hit := range hits {
		seen[hit.UID] = true
	}
	for _, uid := range []string{"meeting", "chase", "thought"} {
		if !seen[uid] {
			t.Errorf("related search missed %q in %+v", uid, hits)
		}
	}
}

func TestNoteSaveKeepsRelatedUIDs(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.post("/app/notes/"+accID+"/"+colEnc+"/thought", url.Values{
		"etag":        {`"j1"`},
		"summary":     {"A thought"},
		"description": {"Ideas about the budget."},
		"date":        {"2026-08-12"},
		"categories":  {"ideas"},
		"related":     {"meeting, chase"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save = %d, body = %s", rec.Code, rec.Body.String())
	}
	written := box.lastPut(t)
	for _, want := range []string{"RELATED-TO:meeting", "RELATED-TO:chase"} {
		if !strings.Contains(written.Body, want) {
			t.Errorf("saved note is missing %q:\n%s", want, written.Body)
		}
	}
}

func TestNoteMarkdownReadMode(t *testing.T) {
	box := startCalBox(t)
	box.seed("md-note.ics", `"m1"`, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VJOURNAL\r\n"+
		"UID:md-note\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Markdown note\r\n"+
		"DESCRIPTION:**Ideas** and <script>alert(1)</script>.\r\nDTSTART;VALUE=DATE:20260812\r\n"+
		"END:VJOURNAL\r\nEND:VCALENDAR\r\n")
	a, accID, colEnc := calendarApp(t, box)

	read := a.get("/app/notes/" + accID + "/" + colEnc + "/md-note")
	if read.Code != http.StatusOK {
		t.Fatalf("note read = %d, body = %s", read.Code, read.Body.String())
	}
	body := read.Body.String()
	if !strings.Contains(body, "<strong>Ideas</strong>") {
		t.Errorf("markdown was not rendered:\n%s", body)
	}
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("raw HTML was not escaped in read mode")
	}

	edit := a.get("/app/notes/" + accID + "/" + colEnc + "/md-note?edit=1")
	if edit.Code != http.StatusOK {
		t.Fatalf("note edit = %d", edit.Code)
	}
	if !strings.Contains(edit.Body.String(), "**Ideas** and &lt;script&gt;alert(1)&lt;/script&gt;.") &&
		!strings.Contains(edit.Body.String(), "**Ideas** and <script>alert(1)</script>.") {
		t.Errorf("edit mode should show the stored text, not HTML:\n%s", edit.Body.String())
	}
}

func TestNoteSaveWithoutChangingDescription(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	before := box.body(t, "thought.ics")
	descLine := icalDescriptionLine(before)
	if descLine == "" {
		t.Fatal("seed note has no DESCRIPTION line")
	}

	edit := a.get("/app/notes/" + accID + "/" + colEnc + "/thought?edit=1")
	if edit.Code != http.StatusOK {
		t.Fatalf("note edit = %d", edit.Code)
	}

	rec := a.post("/app/notes/"+accID+"/"+colEnc+"/thought", url.Values{
		"etag":        {`"j1"`},
		"summary":     {"A thought"},
		"description": {"Ideas about the budget."},
		"date":        {"2026-08-12"},
		"categories":  {"ideas"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save = %d, body = %s", rec.Code, rec.Body.String())
	}
	written := box.lastPut(t)
	if !strings.Contains(written.Body, descLine) {
		t.Errorf("DESCRIPTION changed after a save with no edits:\nwant %q\nin %q", descLine, written.Body)
	}
}

func icalDescriptionLine(body string) string {
	for _, line := range strings.Split(body, "\r\n") {
		if strings.HasPrefix(line, "DESCRIPTION:") {
			return line
		}
	}
	return ""
}

func TestNotesListShowsJournalsAndFiltersByTag(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.get("/app/notes/" + accID + "/" + colEnc)
	if rec.Code != http.StatusOK {
		t.Fatalf("notes list = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "A thought") {
		t.Errorf("the note is missing:\n%s", body)
	}
	if strings.Contains(body, "Chase the invoice") {
		t.Error("a task is on the notes list")
	}
	if !strings.Contains(body, "ideas") {
		t.Errorf("the tag is not offered as a filter:\n%s", body)
	}

	empty := a.get("/app/notes/" + accID + "/" + colEnc + "?tag=absent")
	if strings.Contains(empty.Body.String(), "A thought") {
		t.Error("a tag filter that matches nothing still listed the note")
	}
}

// §23.9: one field, from anywhere, filed without asking where.
func TestQuickNoteWritesAJournal(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	form := a.get("/app/notes/quick")
	if form.Code != http.StatusOK {
		t.Fatalf("quick note form = %d, body = %s", form.Code, form.Body.String())
	}
	rec := a.post("/app/notes/quick", url.Values{"body": {"Ring the bank back"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("quick note = %d, body = %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/app/notes/"+accID+"/"+colEnc+"/") {
		t.Errorf("Location = %q, want the new note in the only collection", loc)
	}
	written := box.lastPut(t)
	for _, want := range []string{"BEGIN:VJOURNAL", "DESCRIPTION:Ring the bank back", "DTSTART;VALUE=DATE:"} {
		if !strings.Contains(written.Body, want) {
			t.Errorf("the quick note is missing %q:\n%s", want, written.Body)
		}
	}

	// The collection just written to becomes the default for the next one.
	sess := a.session()
	views, err := a.Store.Views(sess.UserID, sess.DEK())
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := views.Default(account.ViewNotes)
	if !ok || ref.AccountID != accID {
		t.Errorf("default notes collection = %+v, want the one just used", ref)
	}
}

func TestQuickNoteNeedsSomeText(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	rec := a.post("/app/notes/quick", url.Values{"body": {"   "}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "needs some text") {
		t.Errorf("no explanation:\n%s", rec.Body.String())
	}
}

func TestNoteExportIsMarkdownWithFrontMatter(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.get("/app/notes/" + accID + "/" + colEnc + "/thought/export")
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "a-thought.md") {
		t.Errorf("Content-Disposition = %q, want the note's own name", cd)
	}
	body := rec.Body.String()
	for _, want := range []string{"---", "title: A thought", "uid: thought", "tags:", "  - ideas", "Ideas about the budget."} {
		if !strings.Contains(body, want) {
			t.Errorf("the exported note is missing %q:\n%s", want, body)
		}
	}
}

func TestNotesExportIsAZipOfMarkdown(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	rec := a.get("/app/notes/" + accID + "/" + colEnc + "/export")
	if rec.Code != http.StatusOK {
		t.Fatalf("export = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type = %q", ct)
	}
	archive := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("the archive does not open: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("%d entries, want the one note", len(zr.File))
	}
	if !strings.HasSuffix(zr.File[0].Name, ".md") {
		t.Errorf("entry name = %q", zr.File[0].Name)
	}
	entry, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer entry.Close()
	content, err := io.ReadAll(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "title: A thought") {
		t.Errorf("the entry is not the note:\n%s", content)
	}
}

// The import previews first and writes only on confirmation, and a UID that is
// already in the collection is given a new one rather than overwriting (§23.9).
func TestNotesImportPreviewsThenCreates(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	markdown := "---\ntitle: Minutes\ndate: 2026-08-12\ntags:\n  - meetings\nuid: thought\n---\n\nWe agreed on the budget.\n"
	var payload bytes.Buffer
	mw := multipart.NewWriter(&payload)
	_ = mw.WriteField(CSRFField, a.token())
	part, err := mw.CreateFormFile("file", "minutes.md")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, markdown)
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	preview := a.postRaw("/app/notes/"+accID+"/"+colEnc+"/import", mw.FormDataContentType(), &payload)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview = %d, body = %s", preview.Code, preview.Body.String())
	}
	body := preview.Body.String()
	if !strings.Contains(body, "Minutes") {
		t.Errorf("the preview does not name the note:\n%s", body)
	}
	if !strings.Contains(body, "New UID will be assigned") {
		t.Errorf("the collision with the existing note is not shown:\n%s", body)
	}
	box.mu.Lock()
	writes := len(box.puts)
	box.mu.Unlock()
	if writes != 0 {
		t.Fatalf("%d notes were written before the import was confirmed", writes)
	}

	report := a.post("/app/notes/"+accID+"/"+colEnc+"/import", url.Values{"action": {"confirm_import"}})
	if report.Code != http.StatusOK {
		t.Fatalf("confirm = %d, body = %s", report.Code, report.Body.String())
	}
	if !strings.Contains(report.Body.String(), "Imported 1 note") {
		t.Errorf("the report does not say what happened:\n%s", report.Body.String())
	}
	written := box.lastPut(t)
	for _, want := range []string{"BEGIN:VJOURNAL", "SUMMARY:Minutes", "We agreed on the budget.", "CATEGORIES:meetings"} {
		if !strings.Contains(written.Body, want) {
			t.Errorf("the imported note is missing %q:\n%s", want, written.Body)
		}
	}
	if strings.Contains(written.Body, "UID:thought\r\n") {
		t.Errorf("the import overwrote the identity of an existing note:\n%s", written.Body)
	}
	// The note that was already there is untouched.
	if !strings.Contains(box.body(t, "thought.ics"), "SUMMARY:A thought") {
		t.Error("the existing note was overwritten by the import")
	}
}

func TestUnifiedPollsEveryKindAndReportsProgress(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	page := a.get("/app/calendar?from=2026-08-01&to=2026-08-31")
	if page.Code != http.StatusOK {
		t.Fatalf("calendar = %d, body = %s", page.Code, page.Body.String())
	}
	// The screen goes out before the poll finishes: that is the point of §16.
	// The source panel is on it from the start, the records arrive after.
	if !strings.Contains(page.Body.String(), "Personal") {
		t.Errorf("the source is not named in the panel:\n%s", page.Body.String())
	}
	body := a.findResults(t, page.Body.String(), "mode=time&from=2026-08-01&to=2026-08-31")
	for _, want := range []string{"Budget meeting", "Chase the invoice", "A thought"} {
		if !strings.Contains(body, want) {
			t.Errorf("the calendar view is missing %q:\n%s", want, body)
		}
	}
	// §16 wants a line per source, not one spinner for everything.
	if !strings.Contains(body, "Queried 1 of 1 source") {
		t.Errorf("no per-source progress:\n%s", body)
	}
}

func TestUnifiedHonoursTheKindBoxes(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	query := "mode=time&from=2026-08-01&to=2026-08-31&kind=notes"
	page := a.get("/app/calendar?" + strings.TrimPrefix(query, "mode=time&"))
	body := a.findResults(t, page.Body.String(), query)
	if !strings.Contains(body, "A thought") {
		t.Errorf("the note is missing:\n%s", body)
	}
	if strings.Contains(body, "Budget meeting") || strings.Contains(body, "Chase the invoice") {
		t.Errorf("kinds that were not ticked were polled anyway:\n%s", body)
	}
}

func TestSearchFindsAcrossKinds(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	empty := a.get("/app/search")
	if empty.Code != http.StatusOK {
		t.Fatalf("the search form = %d", empty.Code)
	}
	if strings.Contains(empty.Body.String(), "Queried") {
		t.Error("an empty search started a poll")
	}

	page := a.get("/app/search?q=budget")
	if page.Code != http.StatusOK {
		t.Fatalf("search = %d, body = %s", page.Code, page.Body.String())
	}
	body := a.findResults(t, page.Body.String(), "mode=search&q=budget")
	// The event matches on SUMMARY, the task and the note on DESCRIPTION.
	for _, want := range []string{"Budget meeting", "Chase the invoice", "A thought"} {
		if !strings.Contains(body, want) {
			t.Errorf("the search is missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Filed the receipts") {
		t.Errorf("a record that does not match was returned:\n%s", body)
	}
}

func TestFindResultsAndStreamFollowTheSameTask(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	page := a.get("/app/search?q=budget")
	taskID := findTaskID(t, page.Body.String())

	results := a.findResults(t, page.Body.String(), "mode=search&q=budget")
	if !strings.Contains(results, "Budget meeting") {
		t.Errorf("the fragment does not carry the results:\n%s", results)
	}

	stream := a.get("/app/find/" + taskID + "/stream?mode=search&q=budget")
	if stream.Code != http.StatusOK {
		t.Fatalf("stream = %d", stream.Code)
	}
	if ct := stream.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q", ct)
	}
	sent := stream.Body.String()
	if !strings.Contains(sent, "event: results") || !strings.Contains(sent, "Budget meeting") {
		t.Errorf("the stream carried no results:\n%s", sent)
	}
	// A finished poll closes its own connection rather than being left open.
	if !strings.Contains(sent, "event: done") {
		t.Errorf("the stream did not end itself:\n%s", sent)
	}
}

func TestFindResultsOfAnUnknownTaskAreGone(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	rec := a.get("/app/find/not-a-task/results?mode=search&q=budget")
	if rec.Code != http.StatusGone {
		t.Errorf("status = %d, want 410 so the poller stops", rec.Code)
	}
}

// §14: an unticked collection is not polled at all, and the choice outlives the
// screen it was made on.
func TestSourceSelectionIsRememberedAndHonoured(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	rec := a.post("/app/calendar/sources", url.Values{"mode": {"time"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("saving the selection = %d, body = %s", rec.Code, rec.Body.String())
	}
	sess := a.session()
	views, err := a.Store.Views(sess.UserID, sess.DEK())
	if err != nil {
		t.Fatal(err)
	}
	chosen, hasChoice := views.Selection(account.ViewAgenda)
	if !hasChoice || len(chosen) != 0 {
		t.Fatalf("selection = %+v, hasChoice = %v, want an empty choice honoured as one", chosen, hasChoice)
	}

	page := a.get("/app/calendar?from=2026-08-01&to=2026-08-31")
	if page.Code != http.StatusOK {
		t.Fatalf("calendar = %d", page.Code)
	}
	body := page.Body.String()
	if !strings.Contains(body, "No sources are ticked") {
		t.Errorf("the empty selection was not honoured:\n%s", body)
	}
	if strings.Contains(body, "Budget meeting") {
		t.Error("an unticked collection was polled")
	}
}

func TestTaskConflictOffersTheChoice(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)
	box.mu.Lock()
	box.failNextPut = true
	box.mu.Unlock()

	rec := a.post("/app/tasks/"+accID+"/"+colEnc+"/chase", url.Values{
		"summary":          {"Chase the invoice again"},
		"status":           {"NEEDS-ACTION"},
		"etag":             {`"t1"`},
		"priority":         {"0"},
		"percent_complete": {"0"},
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Conflict", "Keep server version", "Apply my edit"} {
		if !strings.Contains(body, want) {
			t.Errorf("the conflict screen is missing %q:\n%s", want, body)
		}
	}
}

// findResults follows a fan-out the way the browser's fallback poller does, and
// returns the fragment once every source has answered. A screen is rendered
// before its poll finishes, so asserting on the first response would be a race.
func (a *app) findResults(t *testing.T, page, query string) string {
	t.Helper()
	taskID := findTaskID(t, page)
	url := "/app/find/" + taskID + "/results?" + query + "&poll=1"
	deadline := time.Now().Add(5 * time.Second)
	body := ""
	for time.Now().Before(deadline) {
		rec := a.get(url)
		if rec.Code != http.StatusOK {
			t.Fatalf("results = %d, body = %s", rec.Code, rec.Body.String())
		}
		body = rec.Body.String()
		// "Stop polling" is only offered while something is still running.
		if !strings.Contains(body, "Stop polling") {
			return body
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the poll never finished:\n%s", body)
	return body
}

// findTaskID pulls the poll identifier out of a rendered fan-out screen, which
// is how the browser's own poller and stream find it.
func findTaskID(t *testing.T, body string) string {
	t.Helper()
	const marker = "/app/find/"
	at := strings.Index(body, marker)
	if at < 0 {
		t.Fatalf("no fan-out task on the page:\n%s", body)
	}
	rest := body[at+len(marker):]
	end := strings.IndexAny(rest, "/\"")
	if end <= 0 {
		t.Fatalf("could not read the task identifier from %q", rest)
	}
	return rest[:end]
}
