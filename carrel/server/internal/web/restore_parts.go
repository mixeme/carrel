package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// An archive from this computer with the library's live layer comes in parts
// (S-24.3, Typecase's notes 98 and 109; the owner's decision of 22.10.2026,
// Р-71): the page keeps the file; on step 1 the server names the archive's
// start and end and opens it by them; on Restore it names the members of the
// collections chosen, and the browser sends those. What came is held in the
// session's memory until Restore or Cancel, as an archive sent whole is, and
// never on Carrel's disk.

// restoreFileField is the name of step 1's file field: the parts come under
// it, «file.name» and «file.size» say which file they are of.
const restoreFileField = "file"

// errPartMissing is a read of the archive where no part came.
var errPartMissing = errors.New("a part of the archive Carrel needs did not come with the form")

// partsArchive is an archive the browser sent in parts.
type partsArchive struct {
	name string
	size int64

	mu    sync.Mutex
	parts []archivePart // by where they start
	held  int64
}

type archivePart struct {
	at   int64
	data []byte
}

func (a *partsArchive) Size() int64 { return a.size }

func (a *partsArchive) ReadAt(p []byte, off int64) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for n < len(p) {
		at := off + int64(n)
		if at >= a.size {
			return n, io.EOF
		}
		i := slices.IndexFunc(a.parts, func(pt archivePart) bool { return at >= pt.at && at < pt.at+int64(len(pt.data)) })
		if i < 0 {
			return n, errPartMissing
		}
		pt := a.parts[i]
		n += copy(p[n:], pt.data[at-pt.at:])
	}
	return n, nil
}

// has says whether every byte of s came.
func (a *partsArchive) has(s backup.Span) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for at := s.At; at < s.End(); {
		i := slices.IndexFunc(a.parts, func(pt archivePart) bool { return at >= pt.at && at < pt.at+int64(len(pt.data)) })
		if i < 0 {
			return false
		}
		at = a.parts[i].at + int64(len(a.parts[i].data))
	}
	return true
}

func (a *partsArchive) add(pt archivePart) {
	a.mu.Lock()
	defer a.mu.Unlock()
	i, _ := slices.BinarySearchFunc(a.parts, pt.at, func(p archivePart, at int64) int {
		switch {
		case p.at < at:
			return -1
		case p.at > at:
			return 1
		}
		return 0
	})
	a.parts = slices.Insert(a.parts, i, pt)
	a.held += int64(len(pt.data))
}

// sentFile is which file the parts of a form are of: «file.name» and
// «file.size»; false when the form says nothing of parts.
func sentFile(fields url.Values) (string, int64, bool) {
	size, err := strconv.ParseInt(fields.Get(restoreFileField+".size"), 10, 64)
	if err != nil || size <= 0 {
		return "", 0, false
	}
	return fields.Get(restoreFileField + ".name"), size, true
}

// readParts reads the parts of the archive a form brings after its fields
// into a: each a part of the file field named «offset-length», within the
// file and within limit bytes for all that a holds.
func readParts(up *uploadBody, a *partsArchive, limit int64) error {
	for part := up.next; ; part = nil {
		if part == nil {
			var err error
			if part, err = up.mr.NextPart(); err == io.EOF {
				return nil
			} else if err != nil {
				return errors.New("The archive's parts did not arrive whole: " + err.Error())
			}
		}
		if part.FormName() != restoreFileField || part.FileName() == "" {
			if err := up.keep(part); err != nil {
				return err
			}
			continue
		}
		atS, lenS, _ := strings.Cut(part.FileName(), "-")
		at, err1 := strconv.ParseInt(atS, 10, 64)
		n, err2 := strconv.ParseInt(lenS, 10, 64)
		if err1 != nil || err2 != nil || at < 0 || n <= 0 || at+n > a.size {
			return fmt.Errorf("A part of the archive is not one of it: %q.", part.FileName())
		}
		if a.held+n > limit {
			return errPartsTooLarge{limit}
		}
		data, err := io.ReadAll(io.LimitReader(part, n+1))
		if err != nil {
			return errors.New("The archive's parts did not arrive whole: " + err.Error())
		}
		if int64(len(data)) != n {
			return fmt.Errorf("A part of the archive is not as long as it says: %q.", part.FileName())
		}
		a.add(archivePart{at, data})
	}
}

// errPartsTooLarge is a restore that would hold more of the archive in
// memory than Carrel takes from this computer.
type errPartsTooLarge struct{ limit int64 }

func (e errPartsTooLarge) Error() string {
	return fmt.Sprintf("The chosen collections are larger than Carrel takes from this computer (%s). Put the archive into a folder of your storage and choose From a server.", formatSize(e.limit))
}

// partsLimit is how much of an archive from this computer a session holds:
// its start and end, and the members restored up to the ceiling of an
// import.
func (s *Server) partsLimit() int64 {
	return backup.OpenHead + backup.OpenTail + s.cfg.ImportMaxBytes
}

// restoreRanges answers the live layer's asking (?op=ranges) which parts of
// the archive to send, a line «offset length» each: on step 1 — the form
// names its source — the archive's start and end; at Restore of an archive
// opened by parts, the members of the collections chosen that have not come
// yet. Else nothing: the form goes without parts.
func (s *Server) restoreRanges(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	up, err := uploadOf(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	fields := up.fields
	var spans []backup.Span
	name, size, ok := sentFile(fields)
	switch t := restoreOf(sess); {
	case !ok:
	case fields.Get("source") != "":
		spans = backup.OpenSpans(size)
	case t != nil && fields.Get("op") == "restore":
		if a := t.parts(); a != nil && a.name == name && a.size == size {
			spans, _ = s.restoreSpans(sess, t, a, fields)
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	var out strings.Builder
	for _, sp := range spans {
		fmt.Fprintf(&out, "%d %d\n", sp.At, sp.Len)
	}
	_, _ = io.WriteString(w, out.String())
}

// restoreSpans are the parts of the archive the collections the form
// chooses need and that have not come, and whether all of them fit what a
// session holds of it.
func (s *Server) restoreSpans(sess *session.Session, t *restoreTask, a *partsArchive, fields url.Values) ([]backup.Span, bool) {
	blob := s.catalogOf(sess)
	choice := postedChoice(fields, blob, t, t.choiceOf(blob))
	var dirs []string
	for i, e := range t.manifest.Collections {
		if choice.picked[i] && choice.into[i] != restoreSkip {
			dirs = append(dirs, e.Dir)
		}
	}
	var out []backup.Span
	need := a.held
	for _, sp := range t.layout.Spans(dirs) {
		if !a.has(sp) {
			out = append(out, sp)
			need += sp.Len
		}
	}
	return out, need <= s.partsLimit()
}

// holds says whether the members of a collection of the archive can be read
// now: always, but for an archive from this computer whose parts for it
// have not come.
func (t *restoreTask) holds(e backup.Entry) bool {
	a := t.parts()
	if a == nil {
		return true
	}
	for _, sp := range t.layout.Spans([]string{e.Dir}) {
		if !a.has(sp) {
			return false
		}
	}
	return true
}

// parts is the open archive when it came from this computer in parts; nil
// for another.
func (t *restoreTask) parts() *partsArchive {
	t.mu.Lock()
	defer t.mu.Unlock()
	a, _ := t.src.(*partsArchive)
	return a
}

// restoreFields are what a form of the restore sent: the fields in front of
// an archive's parts, or the form.
func restoreFields(r *http.Request) url.Values {
	if isFileUpload(r) {
		if up, err := uploadOf(r); err == nil {
			return up.fields
		}
		return url.Values{}
	}
	_ = r.ParseForm()
	return r.PostForm
}

// chooseAgain is the refusal of step 1 when the archive from this computer
// is no longer in the page: Carrel keeps no copy of it (Typecase's note 109).
const chooseAgain = "Choose the archive again: Carrel keeps no copy of it."

// partsMatch is what step 2 of an archive opened by parts asks the live
// layer: the address of its ranges, and the field step 1 kept the file
// under. Nothing for another archive.
func (s *Server) partsMatch(t *restoreTask, m *view.RestoreMatch) {
	if t.parts() != nil {
		m.Ranges, m.RangesOf = s.route("/app/settings/backup/restore?op=ranges"), restoreFileField
	}
}
