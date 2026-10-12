package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Attach takes every file its form carries (Б6 of the audit of 11.10.2026,
// docs/plan/scans/silent-choices.md). The page's own field sends one file; a
// form that carries several — another client of the contract, a field that
// lets many be chosen — had the first written and the rest dropped without a
// word. Now all of them go into the record by one write, each by the rule of
// S-20, or none does and the notice says which file stood in the way.

// attachForm is the form of Attach with several files: the fields in front
// of them, as a page sends its sizes.
func attachForm(path string, sess *session.Session, fields url.Values, files ...upload) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", sess.CSRFToken())
	for k, vs := range fields {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	for _, f := range files {
		part, _ := w.CreateFormFile("file", f.name)
		_, _ = part.Write([]byte(f.body))
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

func attachNames(rows []view.AttachRow) string {
	var names []string
	for _, r := range rows {
		names = append(names, string(r.Name))
	}
	return strings.Join(names, ", ")
}

func TestAttachTakesEveryFileOfTheForm(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Sync-up", "X-OTHER-CLIENT;X-P=a:kept as it is\n"))
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	puts := f.srv.CountRequests("PUT", f.col)

	resp := f.h.do(attachForm(href, f.sess, nil, upload{"agenda.md", "# Agenda"}, upload{"notes.txt", "notes"}, upload{"plan.txt", "the plan"}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("attach: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "e1.ics"), "FILENAME=agenda.md", "FILENAME=notes.txt", "FILENAME=plan.txt", "X-OTHER-CLIENT;X-P=a:kept as it is")
	if got := attachNames(eventAt(t, f, "e1").Attach); got != "agenda.md, notes.txt, plan.txt" {
		t.Errorf("attachments = %s, want the three in the order sent", got)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 1 {
		t.Errorf("%d writes of the record, want one", n)
	}
}

// Each file goes by the rule of S-20 on the record the files before it made:
// a small one inside, a large one to the folder, and two of one name do not
// take each other's place there.
func TestAttachOfSeveralFilesFollowsTheRuleForEach(t *testing.T) {
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	big, other := strings.Repeat("x", 2048), strings.Repeat("y", 3000)

	resp := f.h.do(attachForm(href, f.sess, nil, upload{"scan.pdf", big}, upload{"note.txt", "small"}, upload{"scan.pdf", other}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("attach: %d %s", resp.Code, resp.Body.String())
	}
	if got, _ := f.srv.File(attachFiles + "carrel/2026-08-10 scan.pdf"); got != big {
		t.Errorf("the first large file is not in the folder as it was sent: %+v", folderRows(t, f, "carrel"))
	}
	if got, _ := f.srv.File(attachFiles + "carrel/2026-08-10 scan-2.pdf"); got != other {
		t.Errorf("the second large file is not beside the first: %+v", folderRows(t, f, "carrel"))
	}
	rows := eventAt(t, f, "e1").Attach
	if len(rows) != 3 || rows[0].Inline || !rows[1].Inline || rows[2].Inline || rows[1].Name != "note.txt" {
		t.Fatalf("attachments = %+v", rows)
	}
}

// A file that cannot be attached stops the whole form: nothing is written,
// and what went to the folder for the files before it is taken back — no
// file is left that no record points at.
func TestAttachOfSeveralFilesIsAllOrNothing(t *testing.T) {
	t.Run("no folder", func(t *testing.T) {
		f := attachFixture(t, 1024, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, nil, upload{"note.txt", "small"}, upload{"scan.pdf", strings.Repeat("x", 2048)})))
		if !hasNotice(page.Notices, "Nothing was attached: no folder for attachments is chosen") {
			t.Fatalf("notices = %+v", page.Notices)
		}
		if !strings.Contains(noticeBodies(page.Notices), "scan.pdf") {
			t.Errorf("the notice does not name the file that stood in the way: %+v", page.Notices)
		}
		if strings.Contains(body(t, f, "e1.ics"), "ATTACH") {
			t.Errorf("the record carries an attachment")
		}
	})
	t.Run("a later file refused", func(t *testing.T) {
		srv := davtest.New(t)
		srv.RefuseUnsizedPUT = true
		f := attachFixtureOn(t, srv, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		big := strings.Repeat("x", 2048)
		// The first has its size in front of it and is written; the second has
		// none, and this server refuses it.
		fields := url.Values{"size.first.pdf": {strconv.Itoa(len(big))}}
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, fields, upload{"first.pdf", big}, upload{"second.pdf", big})))
		if !hasNotice(page.Notices, "Nothing was attached: this server needs the size of a file before it writes it") {
			t.Fatalf("notices = %+v", page.Notices)
		}
		if !strings.Contains(noticeBodies(page.Notices), "second.pdf") {
			t.Errorf("the notice does not name the file that stood in the way: %+v", page.Notices)
		}
		if rows := folderRows(t, f, "carrel"); len(rows) != 0 {
			t.Errorf("the folder holds %+v, want the first file taken back", rows)
		}
		if strings.Contains(body(t, f, "e1.ics"), "ATTACH") {
			t.Errorf("the record carries an attachment")
		}
	})
}

func noticeBodies(ns []view.Notice) string {
	var b strings.Builder
	for _, n := range ns {
		b.WriteString(string(n.Body))
		b.WriteString("\n")
	}
	return b.String()
}

// A new form's Attach keeps every file too: they wait for Save together, or
// none is kept when together they would not fit inside the record.
func TestNewEventAttachTakesEveryFileOfTheForm(t *testing.T) {
	f := attachFixture(t, 1024, []string{"carrel"})
	list := f.path("calendar")
	page := decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess)))
	attach := actionByID(page.Form.Actions, "attach.add")
	form := formFields(page.Form)
	form.Set("summary", "Поездка")

	resp := f.h.do(attachForm(attach.Href, f.sess, form, upload{"agenda.md", "# Agenda"}, upload{"notes.txt", "notes"}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("attach: %d %s", resp.Code, resp.Body.String())
	}
	page = decode[view.EventEditPage](t, f.h.do(get(resp.Header().Get("Location"), f.sess)))
	if got := attachNames(page.Attach); got != "agenda.md, notes.txt" {
		t.Fatalf("the files waiting = %s, want both", got)
	}

	// Two that fit one by one and not together: neither is kept.
	attach = actionByID(page.Form.Actions, "attach.add")
	half := strings.Repeat("x", 600)
	page = decode[view.EventEditPage](t, f.h.do(attachForm(attach.Href, f.sess, formFields(page.Form), upload{"a.bin", half}, upload{"b.bin", half})))
	if !hasNotice(page.Notices, "Nothing was attached: save the record first") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if got := attachNames(page.Attach); got != "agenda.md, notes.txt" {
		t.Errorf("the files waiting = %s, want the two kept before", got)
	}
}
