package web

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"strings"
	"time"
)

// A zip of Markdown files is the notes' other file (s-io-8): every .md in
// it, its folders too, is a note, and the row names the file. The archive is
// somebody else's (S-29.4): it is told by its content, not its name. An
// import has no ceiling of its own (Р-73, the owner's decision of
// 22.10.2026), so a member is read as far as the archive says it is long: one
// that says it is small and is not stops there, a row that failed. Nothing
// in it is unpacked anywhere — a name is a label of a row, never a path, and
// one that would climb out of the archive is named and left out.

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
		return nil, errRead{"«" + name + "» could not be opened as a zip archive."}
	}
	var members []*zip.File
	for _, zf := range zr.File {
		if !strings.HasSuffix(zf.Name, "/") && !zf.FileInfo().IsDir() {
			members = append(members, zf)
		}
	}
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: name, Size: int64(len(body)), Format: "zip"}
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
		// The reader stops where the archive says the member ends.
		text, err := io.ReadAll(rc)
		rc.Close()
		if errors.Is(err, zip.ErrFormat) {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: "The archive says it holds " + string(formatSize(int64(zf.UncompressedSize64))) + "; it holds more.", Failed: true})
			continue
		}
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
			continue
		}
		// A file without front matter is dated by its own time in the
		// archive, as a file of a folder is (s-io-4). DOS date 0 is no time:
		// read as one it is 30 November 1979.
		stamp := zf.Modified
		if stamp.Before(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)) {
			stamp = time.Time{}
		}
		d.takeMarkdown(c, label, text, stamp)
	}
	if markdown == 0 {
		// Read, and nothing in it to take (s-io-18).
		return nil, errRead{"The archive holds no Markdown file."}
	}
	return d, nil
}
