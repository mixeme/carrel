package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"carrel.dev/carrel/server/internal/session"
)

// tempDirOf points the process's temporary directory at a fresh one and
// returns it: a multipart form Go parses with too little memory writes its
// files there.
func tempDirOf(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, dir)
	}
	return dir
}

func spilled(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// multipartFile is a POST of fields in front of one file of size bytes.
func multipartFile(path string, sess *session.Session, fields map[string]string, field string, size int) *http.Request {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	part, _ := w.CreateFormFile(field, "big.zip")
	_, _ = io.CopyN(part, zeroes{}, int64(size))
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	}
	return req
}

type zeroes struct{}

func (zeroes) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// Audit №4, А4-1: the live layer sends a form as FormData with its token in
// X-CSRF-Token, and a handler reads such a form itself (readForm, 24.11.2026).
// On a route whose body has no ceiling — the restore's step, where an
// archive from this computer comes — Browse read the whole form with Go's
// ParseMultipartForm: everything past a MiB of the file went into a
// temporary file on Carrel's disk, as large as the sender cared to send. A
// person's archive on the server's disk, and a full disk for anyone signed
// in. The form's fields are read in front of the file, as an upload's are,
// and nothing of the file is kept.
func TestALiveFormOfAnUnboundedRouteLeavesNothingOnDisk(t *testing.T) {
	dir := tempDirOf(t)
	h := newHarness(t)
	h.admin("alice", "password123")
	sess := h.login("alice", "password123")
	req := multipartFile("/app/settings/backup/restore?op=browse", sess,
		map[string]string{"store": "", "path": "/backups/"}, "file", 3<<20)
	req.Header.Set("X-CSRF-Token", sess.CSRFToken())
	rec := h.do(req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("browse: %d %s", rec.Code, rec.Body.String())
	}
	if names := spilled(t, dir); len(names) != 0 {
		t.Errorf("the form left %v in the temporary directory", names)
	}
}
