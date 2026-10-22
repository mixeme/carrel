package web_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// livePost is a form of the restore as the library's live layer sends it
// (Typecase's notes 98, 109): its fields, the archive's name and size, and
// the parts of it named — none for an asking of ?op=ranges —, to the page's
// column when zone is set.
func livePost(t *testing.T, f davFixture, href string, fields url.Values, archive []byte, parts []backup.Span, zone bool) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("_csrf", f.sess.CSRFToken())
	for k, vs := range fields {
		for _, v := range vs {
			_ = mw.WriteField(k, v)
		}
	}
	_ = mw.WriteField("file.name", "carrel-backup-2026-08-14.zip")
	_ = mw.WriteField("file.size", strconv.Itoa(len(archive)))
	for _, p := range parts {
		fw, _ := mw.CreateFormFile("file", fmt.Sprintf("%d-%d", p.At, p.Len))
		_, _ = fw.Write(archive[p.At:p.End()])
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, href, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if zone {
		req.Header.Set(render.HeaderView, render.ViewFragment)
		req.Header.Set(render.HeaderTarget, render.TargetMain)
	}
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	return f.h.do(req)
}

// askRanges is the live layer's asking which parts to send: the lines of
// the answer.
func askRanges(t *testing.T, f davFixture, fields url.Values, archive []byte) []backup.Span {
	t.Helper()
	rec := livePost(t, f, "/app/settings/backup/restore?op=ranges", fields, archive, nil, false)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("ranges: %d %s", rec.Code, rec.Body.String())
	}
	var out []backup.Span
	for _, l := range strings.Split(strings.TrimSpace(rec.Body.String()), "\n") {
		if l == "" {
			continue
		}
		var s backup.Span
		if _, err := fmt.Sscanf(l, "%d %d", &s.At, &s.Len); err != nil {
			t.Fatalf("ranges line %q: %v", l, err)
		}
		out = append(out, s)
	}
	return out
}

// S-24.3, the owner's decision of 22.10.2026 (Р-71), Typecase's note 109:
// with the live layer an archive from this computer is opened by its start
// and end alone, step 2 comes in the page's column and says the records are
// compared when restored; Restore asks for the members of the collections
// chosen and restores from them. The page gone — no parts with Restore —,
// step 1 asks for the archive again.
func TestRestoreFromThisComputerGoesInParts(t *testing.T) {
	f := backupFixture(t)
	rnd := rand.New(rand.NewPCG(3, 4))
	noise := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(rnd.Uint32())
		}
		return b
	}
	file := func(name string, body []byte) func(func(string) (io.Writer, error)) (int, error) {
		return func(add func(string) (io.Writer, error)) (int, error) {
			w, err := add("files/" + name)
			if err != nil {
				return 0, err
			}
			_, err = w.Write(body)
			return 1, err
		}
	}
	const password = "correct horse battery staple"
	var buf bytes.Buffer
	// The book lies past the archive's start and before its end: nothing of
	// it comes on step 1.
	_, err := backup.Write(&buf, backup.Manifest{Job: "Личное", Carrel: "test", Taken: taken}, []backup.Source{
		{Entry: backup.Entry{Name: "Docs", Kind: view.KindFile, AccountID: f.id, Path: filesDocs}, Write: file("small.bin", noise(backup.OpenHead+(16<<10)))},
		{Entry: backup.Entry{Name: "People", Kind: view.KindContact, AccountID: f.id, Path: searchBook},
			Write: func(add func(string) (io.Writer, error)) (int, error) {
				w, err := add("contacts.vcf")
				if err != nil {
					return 0, err
				}
				_, err = io.WriteString(w, "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:zed\r\nFN:Zed\r\nEND:VCARD\r\n")
				return 1, err
			}},
		{Entry: backup.Entry{Name: "Big", Kind: view.KindFile, AccountID: f.id, Path: filesDocs}, Write: file("big.bin", noise(backup.OpenTail+(512<<10)))},
	}, password)
	if err != nil {
		t.Fatal(err)
	}
	archive := buf.Bytes()
	step1 := url.Values{"source": {"upload"}, "store": {""}, "folder": {"/carrel/restore"}, "password": {password}}

	open := func() view.RestoreBlock {
		t.Helper()
		spans := askRanges(t, f, step1, archive)
		if fmt.Sprint(spans) != fmt.Sprint(backup.OpenSpans(int64(len(archive)))) {
			t.Fatalf("step 1 asks %+v, want the start and the end", spans)
		}
		rec := livePost(t, f, "/app/settings/backup/restore?from=file", step1, archive, spans, true)
		if rec.Code != http.StatusOK {
			t.Fatalf("open in parts: %d %s", rec.Code, rec.Body.String())
		}
		// The page's column, in place: the page keeps the file.
		part := decode[struct {
			Target string          `json:"target"`
			Body   view.BackupPage `json:"body"`
		}](t, rec)
		if part.Target != render.TargetMain {
			t.Fatalf("step 2 goes to %q", part.Target)
		}
		return part.Body.Restore
	}
	b := open()
	m := b.Match
	if m == nil || len(m.Rows) != 3 {
		t.Fatalf("step 2 in place = %+v", b)
	}
	if m.Ranges != "/app/settings/backup/restore?op=ranges" || m.RangesOf != "file" {
		t.Errorf("step 2 asks %q of %q", m.Ranges, m.RangesOf)
	}
	if sub := plainRuns(b.Subtitle); !strings.HasPrefix(sub, "From this computer · manifest read · 3 collections · 1 record") {
		t.Errorf("subtitle = %q", sub)
	}
	if will := string(m.Rows[1].Will); will != "matched by path — compared when you restore" || m.Rows[1].Danger ||
		!strings.Contains(mockupFrame(t, "s-backup-20"), ">"+will+"<") {
		t.Errorf("the book's row = %q, not the words of s-backup-20", will)
	}

	// Restore with the book alone: its members are asked for, past the
	// start and before the end.
	form := url.Values{"op": {"restore"}, "into.0": {"skip"}, "into.1": {sel(f, searchBook)}, "pick.1": {"on"}, "into.2": {"skip"}}
	members := askRanges(t, f, form, archive)
	size := int64(len(archive))
	if len(members) != 1 || members[0].At < backup.OpenHead || members[0].End() > size-backup.OpenTail {
		t.Fatalf("the book's parts = %+v of %d", members, size)
	}
	rec := livePost(t, f, "/app/settings/backup/restore/match?op=restore", form, archive, members, true)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/settings/backup/restore/report" {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	rep := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore/report", f.sess))).Restore.Report
	if rep == nil || len(rep.Rows) != 1 || rep.Rows[0].Created != 1 {
		t.Fatalf("report = %+v", rep)
	}

	// The page that kept the file is gone: Restore comes without parts.
	open()
	rec = f.h.do(postForm("/app/settings/backup/restore/match?op=restore", f.sess, form))
	if rec.Code != http.StatusOK {
		t.Fatalf("restore without parts: %d %s", rec.Code, rec.Body.String())
	}
	b = decode[view.BackupPage](t, rec).Restore
	if plainRuns(b.Refusal) != "Choose the archive again: Carrel keeps no copy of it." || b.Match != nil || !b.From[2].On {
		t.Errorf("without parts = refusal %q, match %v, from %+v", plainRuns(b.Refusal), b.Match != nil, b.From)
	}
}
