package web

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/provider/cal"
)

// A zip of Markdown files is the notes' other file (s-io-8): every .md in
// it, its folders too, is a note, and the row names the file. The archive is
// somebody else's (S-29.4): it is told by its content, not its name; its
// members are counted against CARREL_IMPORT_MAX_RECORDS before one is read;
// what is unpacked is counted against CARREL_IMPORT_MAX_BYTES as it is read,
// so a member that says it is small and is not, stops there; nothing in it
// is unpacked anywhere — a name is a label of a row, never a path, and one
// that would climb out of the archive is named and left out.

// zipMagic is how a zip archive begins.
var zipMagic = []byte("PK\x03\x04")

// isZip says whether body is a zip archive, by its content.
func isZip(body []byte) bool { return bytes.HasPrefix(body, zipMagic) }

// insideArchive says whether a member's name stays inside the archive: no
// absolute path, no step up, no backslash, nothing empty.
func insideArchive(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, ":") {
		return false
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." || seg == "." {
			return false
		}
	}
	return true
}

// readZipNotes reads a zip of Markdown files as notes.
func (s *Server) readZipNotes(c *collectionCtx, name string, body []byte) (*importDraft, error) {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("not a zip archive: %w", err)
	}
	var members []*zip.File
	for _, zf := range zr.File {
		if !strings.HasSuffix(zf.Name, "/") && !zf.FileInfo().IsDir() {
			members = append(members, zf)
		}
	}
	if len(members) > s.cfg.ImportMaxRecords {
		return nil, errTooManyRecords{len(members), s.cfg.ImportMaxRecords}
	}
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: name, Size: int64(len(body)), Format: "zip"}
	left := s.cfg.ImportMaxBytes
	markdown := 0
	for _, zf := range members {
		d.Files++
		label := zf.Name
		switch {
		case !insideArchive(label):
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: "A name that leaves the archive."})
			continue
		case !isMarkdown(label):
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: "Not Markdown."})
			continue
		case zf.Flags&0x1 != 0:
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: "Encrypted inside the archive: Carrel does not open it.", Failed: true})
			continue
		}
		markdown++
		rc, err := zf.Open()
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
			continue
		}
		text, err := io.ReadAll(io.LimitReader(rc, left+1))
		rc.Close()
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
			continue
		}
		if int64(len(text)) > left {
			return nil, &http.MaxBytesError{Limit: s.cfg.ImportMaxBytes}
		}
		left -= int64(len(text))
		if len(bytes.TrimSpace(text)) == 0 {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: "Empty file: nothing to store and nothing to name it by", Failed: true})
			continue
		}
		// A file without front matter is dated by its own time in the
		// archive, as a file of a folder is (s-io-4).
		stamp := zf.Modified
		if stamp.IsZero() {
			stamp = time.Now()
		}
		obj, err := cal.ParseNote(path.Base(label), text, stamp)
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
			continue
		}
		if err := d.addFrom(c, obj, label); err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
		}
	}
	if markdown == 0 {
		d.Note = "The archive holds no Markdown file."
	}
	return d, nil
}
