package web_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/view"
)

var taken = time.Date(2026, 8, 16, 3, 10, 0, 0, time.UTC)

// uploadArchive brings an archive from this computer and opens it.
func uploadArchive(t *testing.T, f davFixture, archive []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("_csrf", f.sess.CSRFToken())
	fw, _ := mw.CreateFormFile("archive", "carrel.zip")
	_, _ = fw.Write(archive)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/app/settings/backup/restore?source=upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	return f.h.do(req)
}

// S-29.4: an archive is somebody else's file, and it is read under the
// ceilings of an import (S-24.3: restoring is an import) — a member is not
// read into memory past CARREL_IMPORT_MAX_BYTES, and a collection holding
// more than CARREL_IMPORT_MAX_RECORDS is refused whole, as the import
// wizard refuses it. Before, every member was read with io.ReadAll: a small
// zip that inflates to gigabytes took the process down (Ф3-2).
func TestRestoreReadsArchiveMembersUnderTheImportCeilings(t *testing.T) {
	f := backupFixture(t)
	f.h.cfg.ImportMaxBytes, f.h.cfg.ImportMaxRecords = 64<<10, 3
	card := func(uid string) string {
		return "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:" + uid + "\r\nFN:" + uid + "\r\nEND:VCARD\r\n"
	}
	var archive bytes.Buffer
	_, err := backup.Write(&archive, backup.Manifest{Job: "j", Carrel: "test", Taken: taken}, []backup.Source{
		{Entry: backup.Entry{Name: "Bomb", Kind: view.KindContact, AccountID: "x", Path: "/x/"},
			Write: func(add func(string) (io.Writer, error)) (int, error) {
				w, err := add("contacts.vcf")
				if err != nil {
					return 0, err
				}
				// One card, padded past the ceiling with a NOTE that deflates to nothing.
				_, err = io.WriteString(w, "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:big\r\nFN:Big\r\nNOTE:"+strings.Repeat("a", 65<<10)+"\r\nEND:VCARD\r\n")
				return 1, err
			}},
		{Entry: backup.Entry{Name: "Many", Kind: view.KindContact, AccountID: "x", Path: "/y/"},
			Write: func(add func(string) (io.Writer, error)) (int, error) {
				w, err := add("contacts.vcf")
				if err != nil {
					return 0, err
				}
				_, err = io.WriteString(w, card("a")+card("b")+card("c")+card("d"))
				return 4, err
			}},
		{Entry: backup.Entry{Name: "Fine", Kind: view.KindContact, AccountID: "x", Path: "/z/"},
			Write: func(add func(string) (io.Writer, error)) (int, error) {
				w, err := add("contacts.vcf")
				if err != nil {
					return 0, err
				}
				_, err = io.WriteString(w, card("ok"))
				return 1, err
			}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if rec := uploadArchive(t, f, archive.Bytes()); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if page.Restore.Match == nil || len(page.Restore.Match.Rows) != 3 {
		t.Fatalf("matching = %+v", page.Restore.Match)
	}
	rows := page.Restore.Match.Rows
	for i, want := range []string{"larger than", "records", ""} {
		will := string(rows[i].Will)
		if want == "" {
			// Its server is not here: another book is chosen, and said so.
			if will != "that server is gone — another collection chosen" || !rows[i].Danger {
				t.Errorf("row %d = %q", i, will)
			}
			continue
		}
		if !strings.Contains(will, "cannot be restored") || !strings.Contains(will, want) || !rows[i].Danger {
			t.Errorf("row %d = %q", i, will)
		}
	}
	made := f.srv.CountRequests("PUT", "")
	rep := restoreRun(t, f, "/app/settings/backup/restore/match?op=restore", matchForm(*page.Restore.Match))
	if !strings.Contains(reportLines(rep), "not restored") {
		t.Fatalf("restore = %+v", rep)
	}
	if n := f.srv.CountRequests("PUT", "") - made; n != 1 {
		t.Errorf("%d records were written, want the one of the collection under the ceilings", n)
	}
}

// S-29.4: nothing is written by a path taken from the archive. The manifest
// names a collection; that name became the folder the files went under, so
// a manifest saying "../../x" wrote outside the chosen storage (Ф3-3).
func TestRestoreNeverWritesWhereTheManifestPoints(t *testing.T) {
	f := backupFixture(t)
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, _ := zw.Create("01-x/files/note.txt")
	_, _ = io.WriteString(w, "hello")
	m, _ := zw.Create(backup.ManifestName)
	_ = json.NewEncoder(m).Encode(backup.Manifest{Format: backup.FormatVersion, Carrel: "test", Job: "j", Taken: taken,
		Collections: []backup.Entry{{Name: "../../evil", Kind: view.KindFile, AccountID: "x", Path: "/x/", Count: 1, Dir: "01-x/"}}})
	_ = zw.Close()
	if rec := uploadArchive(t, f, archive.Bytes()); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	before := len(f.srv.Requests())
	restoreRun(t, f, "/app/settings/backup/restore/match?op=restore", url.Values{
		"into.0": {sel(f, filesDocs)}, "pick.0": {"on"},
	})
	wrote := 0
	for _, r := range f.srv.Requests()[before:] {
		method, p, _ := strings.Cut(r, " ")
		if method != "MKCOL" && method != "PUT" {
			continue
		}
		wrote++
		if strings.Contains(p, "..") || !strings.HasPrefix(p, filesDocs) {
			t.Errorf("%s written outside the storage", r)
		}
	}
	if wrote == 0 {
		t.Error("nothing was written at all")
	}
	if names := f.srv.Names(filesDocs); len(names) == 0 {
		t.Error("no restored folder under the storage")
	}
}

// backup.OpenAt refuses an archive it should not hold in memory: a manifest
// past its ceiling, or more members than any copy writes (S-29.4).
func TestOpenAtRefusesAnArchivePastItsCeilings(t *testing.T) {
	var big bytes.Buffer
	zw := zip.NewWriter(&big)
	m, _ := zw.Create(backup.ManifestName)
	_, _ = m.Write([]byte(`{"format":1,"job":"` + strings.Repeat("j", int(backup.MaxManifestBytes)) + `"}`))
	_ = zw.Close()
	if _, _, err := backup.Open(big.Bytes(), ""); err == nil || !strings.Contains(err.Error(), "manifest") {
		t.Errorf("a manifest past the ceiling opened: %v", err)
	}
	var many bytes.Buffer
	zw = zip.NewWriter(&many)
	for i := 0; i <= backup.MaxEntries; i++ {
		if _, err := zw.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("01-x/%d", i), Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	m, _ = zw.Create(backup.ManifestName)
	_ = json.NewEncoder(m).Encode(backup.Manifest{Format: backup.FormatVersion})
	_ = zw.Close()
	if _, _, err := backup.Open(many.Bytes(), ""); err == nil || !strings.Contains(err.Error(), "members") {
		t.Errorf("an archive with too many members opened: %v", err)
	}
}
