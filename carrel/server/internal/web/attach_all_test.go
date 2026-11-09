package web_test

import (
	"bytes"
	"io"
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
// S-20, and the notice names each that could not go in.

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
	if got, _ := f.srv.File(attachFiles + "carrel/2026-08-10-scan.pdf"); got != big {
		t.Errorf("the first large file is not in the folder as it was sent: %+v", folderRows(t, f, "carrel"))
	}
	if got, _ := f.srv.File(attachFiles + "carrel/2026-08-10-scan-2.pdf"); got != other {
		t.Errorf("the second large file is not beside the first: %+v", folderRows(t, f, "carrel"))
	}
	rows := eventAt(t, f, "e1").Attach
	if len(rows) != 3 || rows[0].Inline || !rows[1].Inline || rows[2].Inline || rows[1].Name != "note.txt" {
		t.Fatalf("attachments = %+v", rows)
	}
}

// A file that cannot be attached does not stop the others (the owner's
// decision of 03.11.2026: all or nothing was the executor's choice). The
// record is written with every file that can go in, and the notice counts
// them and names each that did not, with its reason.
func TestAttachOfSeveralFilesAttachesWhatItCan(t *testing.T) {
	big := strings.Repeat("x", 2048)
	t.Run("no folder", func(t *testing.T) {
		f := attachFixture(t, 1024, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, nil, upload{"note.txt", "small"}, upload{"scan.pdf", big})))
		n := mustNotice(t, page.Notices, "1 of 2 files attached.")
		if string(n.Body) != "scan.pdf: no folder for attachments is chosen." {
			t.Errorf("body = %q", n.Body)
		}
		if actionByID(n.Actions, "settings.attachments").Href == "" {
			t.Errorf("the notice does not offer to choose the folder: %+v", n.Actions)
		}
		if got := attachNames(page.Attach); got != "note.txt" {
			t.Errorf("attachments = %s, want note.txt", got)
		}
	})
	t.Run("the first file refused, others after it", func(t *testing.T) {
		f := attachFixture(t, 1024, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, nil, upload{"scan.pdf", big}, upload{"note.txt", "small"}, upload{"plan.txt", "plan"})))
		mustNotice(t, page.Notices, "2 of 3 files attached.")
		if got := attachNames(eventAt(t, f, "e1").Attach); got != "note.txt, plan.txt" {
			t.Errorf("attachments = %s, want the two after the refused one", got)
		}
	})
	t.Run("a later file refused", func(t *testing.T) {
		srv := davtest.New(t)
		srv.RefuseUnsizedPUT = true
		f := attachFixtureOn(t, srv, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		// The first has its size in front of it and is written; the second has
		// none, and this server refuses it.
		fields := url.Values{"size.first.pdf": {strconv.Itoa(len(big))}}
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, fields, upload{"first.pdf", big}, upload{"second.pdf", big})))
		n := mustNotice(t, page.Notices, "1 of 2 files attached.")
		if string(n.Body) != "second.pdf: this server needs the size of a file before it writes it." {
			t.Errorf("body = %q", n.Body)
		}
		if got, _ := f.srv.File(attachFiles + "carrel/2026-08-10-first.pdf"); got != big {
			t.Errorf("the first file is not in the folder: %+v", folderRows(t, f, "carrel"))
		}
		if got := attachNames(eventAt(t, f, "e1").Attach); got != "2026-08-10-first.pdf" {
			t.Errorf("attachments = %s, want the link to the first", got)
		}
	})
	t.Run("the server's code", func(t *testing.T) {
		f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
		f.srv.RefuseAt("PUT", attachFiles+"carrel/2026-08-10-scan.pdf", http.StatusInsufficientStorage, "")
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, nil, upload{"notes.txt", "notes"}, upload{"report.txt", "report"}, upload{"scan.pdf", big})))
		n := mustNotice(t, page.Notices, "2 of 3 files attached.")
		if string(n.Body) != "scan.pdf: 507 Insufficient Storage." {
			t.Errorf("body = %q", n.Body)
		}
		// The request stays in the details, the way the other notices keep it.
		if n.Details == nil || !strings.Contains(detailText(n.Details), "scan.pdf · PUT ") {
			t.Errorf("details = %+v, want the PUT of scan.pdf", n.Details)
		}
	})
	t.Run("none of them", func(t *testing.T) {
		f := attachFixture(t, 1024, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, nil, upload{"a.pdf", big}, upload{"b.pdf", big})))
		n := mustNotice(t, page.Notices, "0 of 2 files attached.")
		if string(n.Body) != "a.pdf: no folder for attachments is chosen. b.pdf: no folder for attachments is chosen." {
			t.Errorf("body = %q", n.Body)
		}
		if strings.Contains(body(t, f, "e1.ics"), "ATTACH") {
			t.Errorf("the record carries an attachment")
		}
	})
}

// An upload that broke off keeps the files that came whole before it.
func TestAttachThatBrokeOffKeepsTheFilesBefore(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	req := attachForm(href, f.sess, nil, upload{"notes.txt", "notes"}, upload{"scan.pdf", strings.Repeat("x", 4096)})
	whole, _ := io.ReadAll(req.Body)
	cut := bytes.Index(whole, []byte("xxxx")) + 1000
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(whole[:cut]), brokenReader{}))
	page := decode[view.EventPage](t, f.h.do(req))
	n := mustNotice(t, page.Notices, "1 of 2 files attached.")
	if string(n.Body) != "scan.pdf: the upload broke off; nothing after it was received." {
		t.Errorf("body = %q", n.Body)
	}
	if got := attachNames(eventAt(t, f, "e1").Attach); got != "notes.txt" {
		t.Errorf("attachments = %s, want notes.txt", got)
	}
}

type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// A record the server did not write takes back what went to the folder for
// it: nothing points at those files. One it cannot take back is named, with
// where it stays — before, Carrel said nothing of it.
func TestAttachNamesAFileItCouldNotTakeBack(t *testing.T) {
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.RefuseAt("PUT", f.col+"e1.ics", http.StatusForbidden, "")
	f.srv.RefuseAt("DELETE", attachFiles, http.StatusForbidden, "")
	href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
	page := decode[view.EventPage](t, f.h.do(attachForm(href, f.sess, nil, upload{"scan.pdf", strings.Repeat("x", 2048)})))
	if !strings.Contains(noticeBodies(page.Notices), "Nothing was attached. 2026-08-10-scan.pdf stays in ") ||
		!strings.Contains(noticeBodies(page.Notices), " / carrel: 403 Forbidden: this account has no write access here.") {
		t.Errorf("bodies = %q, want the file that stays and where", noticeBodies(page.Notices))
	}
	var details string
	for _, n := range page.Notices {
		if n.Details != nil {
			details += detailText(n.Details)
		}
	}
	if !strings.Contains(details, "DELETE ") {
		t.Errorf("details = %q, want the DELETE that was refused", details)
	}
	if got, _ := f.srv.File(attachFiles + "carrel/2026-08-10-scan.pdf"); got == "" {
		t.Errorf("the file is not in the folder, yet the notice says it stays")
	}
}

func mustNotice(t *testing.T, ns []view.Notice, title string) view.Notice {
	t.Helper()
	for _, n := range ns {
		if string(n.Title) == title {
			return n
		}
	}
	t.Fatalf("no notice %q: %+v", title, ns)
	return view.Notice{}
}

func detailText(d *view.Diagnostic) string {
	s := string(d.Summary)
	for _, l := range d.Lines {
		s += " | " + string(l)
	}
	return s
}

func noticeBodies(ns []view.Notice) string {
	var b strings.Builder
	for _, n := range ns {
		b.WriteString(string(n.Body))
		b.WriteString("\n")
	}
	return b.String()
}

// A new form's Attach keeps every file too: they wait for Save together, and
// one that would not fit inside the record with those before it is named.
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

	// Two that fit one by one and not together: the first is kept, the
	// second named (the owner's decision of 03.11.2026).
	attach = actionByID(page.Form.Actions, "attach.add")
	half := strings.Repeat("x", 600)
	page = decode[view.EventEditPage](t, f.h.do(attachForm(attach.Href, f.sess, formFields(page.Form), upload{"a.bin", half}, upload{"b.bin", half})))
	n := mustNotice(t, page.Notices, "1 of 2 files attached.")
	if !strings.HasPrefix(string(n.Body), "b.bin: save the record first. Up to ") {
		t.Errorf("body = %q", n.Body)
	}
	if got := attachNames(page.Attach); got != "agenda.md, notes.txt, a.bin" {
		t.Errorf("the files waiting = %s, want a.bin after the two kept before", got)
	}

	// One file alone that does not fit keeps its own notice.
	attach = actionByID(page.Form.Actions, "attach.add")
	page = decode[view.EventEditPage](t, f.h.do(attachForm(attach.Href, f.sess, formFields(page.Form), upload{"c.bin", half})))
	if !hasNotice(page.Notices, "Nothing was attached: save the record first") {
		t.Fatalf("notices = %+v", page.Notices)
	}
}

// Б7 of the audit of 11.10.2026: the files of a new form waited in the one
// slot a session had, and the first Attach of a second new form threw them
// away. Each form has its own.
func TestTwoNewFormsKeepTheirOwnFiles(t *testing.T) {
	f := attachFixture(t, 0, []string{"carrel"})
	list := f.path("calendar")
	begin := func(title, name string) string {
		t.Helper()
		page := decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess)))
		form := formFields(page.Form)
		form.Set("summary", title)
		resp := f.h.do(attachForm(actionByID(page.Form.Actions, "attach.add").Href, f.sess, form, upload{name, "text of " + name}))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("attach to %s: %d %s", title, resp.Code, resp.Body.String())
		}
		return resp.Header().Get("Location")
	}
	first := begin("First", "first.txt")
	second := begin("Second", "second.txt")
	for back, want := range map[string]string{first: "first.txt", second: "second.txt"} {
		page := decode[view.EventEditPage](t, f.h.do(get(back, f.sess)))
		if got := attachNames(page.Attach); got != want {
			t.Errorf("the form at %s holds %q, want %q", back, got, want)
		}
	}
}

// Б7: a new form whose files the session no longer holds — newer forms took
// their place — is not saved without them in silence: nothing is written,
// the form says why, and saving it again saves it without the files.
func TestANewFormThatLostItsFilesSaysSo(t *testing.T) {
	f := attachFixture(t, 0, []string{"carrel"})
	list := f.path("calendar")
	begin := func(title string) string {
		t.Helper()
		page := decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess)))
		form := formFields(page.Form)
		form.Set("summary", title)
		resp := f.h.do(attachForm(actionByID(page.Form.Actions, "attach.add").Href, f.sess, form, upload{title + ".txt", "text"}))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("attach to %s: %d %s", title, resp.Code, resp.Body.String())
		}
		return resp.Header().Get("Location")
	}
	lost := begin("Lost")
	held := decode[view.EventEditPage](t, f.h.do(get(lost, f.sess)))
	for _, title := range []string{"B", "C", "D", "E"} {
		begin(title)
	}
	form := formFields(held.Form)
	form.Set("start.date", "2026-08-12")
	form.Set("start.time", "10:00")
	resp := f.h.do(postForm(list+"/new", f.sess, form))
	if resp.Code != http.StatusOK {
		t.Fatalf("a form that lost its files was saved: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.EventEditPage](t, resp)
	if len(page.Notices) == 0 || page.Notices[0].Title != "Nothing was saved:" || !strings.Contains(string(page.Notices[0].Body), "files") {
		t.Errorf("notices = %+v, want why nothing was saved", page.Notices)
	}
	if resp := f.h.do(postForm(list+"/new", f.sess, formFields(page.Form))); resp.Code != http.StatusSeeOther {
		t.Errorf("saved again: %d, want the event made without the files", resp.Code)
	}
}
