// Package backup writes and reads Carrel's backup archive (S-24): one zip per
// run, the records of every collection in their own interchange formats and
// the files of every file collection as files, laid out by connection and
// collection, with a manifest that says where each came from. Encrypted by
// default with age under the job's own password, so the copy opens when the
// account here is gone.
//
// The package reads nothing by itself: the run hands it what to write, read
// through the same providers the screens use (S-24.3). It does not import
// internal/web and fills no view page.
package backup

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"

	"filippo.io/age"

	"carrel.dev/carrel/server/view"
)

// ManifestName is the manifest's place in the archive.
const ManifestName = "manifest.json"

// FormatVersion is the manifest's own version.
const FormatVersion = 1

// Manifest is what makes an archive restorable: without it a set of files
// does not say where to put them back.
type Manifest struct {
	Format      int       `json:"format"`
	Carrel      string    `json:"carrel"` // the version that wrote it
	Job         string    `json:"job"`
	Taken       time.Time `json:"taken"`
	Collections []Entry   `json:"collections"`
}

// Entry is one collection of the archive.
type Entry struct {
	Connection string      `json:"connection"` // the server's host, a label
	AccountID  string      `json:"accountID"`
	Path       string      `json:"path"` // where it lay on its server
	Name       string      `json:"name"`
	Kind       view.Kind   `json:"kind"` // file for a file collection
	Kinds      []view.Kind `json:"kinds,omitempty"`
	Count      int         `json:"count"` // records, or files
	Dir        string      `json:"dir"`   // its folder in the archive
	// Error is why the collection is not in the archive whole: the run
	// says so here and on the screen, never silently.
	Error string `json:"error,omitempty"`
}

// Complete reports whether every collection went in whole.
func (m Manifest) Complete() bool {
	for _, e := range m.Collections {
		if e.Error != "" {
			return false
		}
	}
	return true
}

// Source is one collection to copy. Write puts its contents under dir in
// the archive through add, and returns how many records or files went in.
type Source struct {
	Entry Entry
	Write func(add func(name string) (io.Writer, error)) (int, error)
}

// Write writes the archive of sources to w: encrypted under password, or a
// plain zip when password is empty. A source that fails is recorded in the
// manifest with its error and the rest go on. The manifest is written last,
// when every count is known.
func Write(w io.Writer, m Manifest, sources []Source, password string) (Manifest, error) {
	out := w
	var sealer io.WriteCloser
	if password != "" {
		r, err := age.NewScryptRecipient(password)
		if err != nil {
			return m, err
		}
		if sealer, err = age.Encrypt(w, r); err != nil {
			return m, err
		}
		out = sealer
	}
	zw := zip.NewWriter(out)
	m.Format = FormatVersion
	m.Collections = nil
	for i, src := range sources {
		e := src.Entry
		e.Dir = fmt.Sprintf("%02d-%s/", i+1, slug(e.Name))
		add := func(name string) (io.Writer, error) {
			return zw.CreateHeader(&zip.FileHeader{Name: e.Dir + name, Method: zip.Deflate, Modified: m.Taken})
		}
		n, err := src.Write(add)
		e.Count = n
		if err != nil {
			e.Error = err.Error()
		}
		m.Collections = append(m.Collections, e)
	}
	f, err := zw.Create(ManifestName)
	if err != nil {
		return m, err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return m, err
	}
	if err := zw.Close(); err != nil {
		return m, err
	}
	if sealer != nil {
		return m, sealer.Close()
	}
	return m, nil
}

// ErrWrongPassword is an encrypted archive the password does not open.
var ErrWrongPassword = errors.New("backup: the password does not open this archive")

// Encrypted reports whether data is an age file.
func Encrypted(data []byte) bool {
	return bytes.HasPrefix(data, []byte("age-encryption.org/v1\n"))
}

// Open reads an archive held in memory.
func Open(data []byte, password string) (*zip.Reader, Manifest, error) {
	return OpenAt(bytes.NewReader(data), int64(len(data)), password)
}

// OpenAt reads an archive of size bytes from r — a file on disk, never all
// of it in memory: decrypted with password when it is an age file, then the
// zip and its manifest.
func OpenAt(r io.ReaderAt, size int64, password string) (*zip.Reader, Manifest, error) {
	var m Manifest
	head := make([]byte, 32)
	n, _ := r.ReadAt(head, 0)
	if Encrypted(head[:n]) {
		id, err := age.NewScryptIdentity(password)
		if err != nil {
			return nil, m, err
		}
		if r, size, err = age.DecryptReaderAt(r, size, id); err != nil {
			return nil, m, ErrWrongPassword
		}
	}
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, m, fmt.Errorf("backup: not a Carrel archive: %w", err)
	}
	if len(zr.File) > MaxEntries {
		return nil, m, fmt.Errorf("backup: the archive has %d members; one this Carrel reads has at most %d", len(zr.File), MaxEntries)
	}
	f, err := zr.Open(ManifestName)
	if err != nil {
		return nil, m, errors.New("backup: the archive has no manifest")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, MaxManifestBytes+1))
	if err != nil {
		return nil, m, fmt.Errorf("backup: the manifest cannot be read: %w", err)
	}
	if len(raw) > MaxManifestBytes {
		return nil, m, fmt.Errorf("backup: the manifest is larger than %d bytes", MaxManifestBytes)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, m, fmt.Errorf("backup: the manifest cannot be read: %w", err)
	}
	if m.Format != FormatVersion {
		return nil, m, fmt.Errorf("backup: manifest format %d is not one this Carrel reads", m.Format)
	}
	return zr, m, nil
}

// FileName is the name a run's archive gets: the job, the time, and whether
// it is sealed. Names sort by time, and a name is never reused.
func FileName(job string, at time.Time, encrypted bool) string {
	name := "carrel-" + slug(job) + "-" + at.UTC().Format("2006-01-02T150405Z") + ".zip"
	if encrypted {
		name += ".age"
	}
	return name
}

// Rotation is what to delete once a new archive is read back: the job's
// archives beyond keep, oldest first. names are the folder's file names.
func Rotation(job string, names []string, keep int) []string {
	prefix := "carrel-" + slug(job) + "-"
	var mine []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) && (strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".zip.age")) {
			mine = append(mine, n)
		}
	}
	if keep <= 0 || len(mine) <= keep {
		return nil
	}
	sort.Strings(mine)
	return mine[:len(mine)-keep]
}

// Slug is a name as one safe file-name segment: letters and digits of any
// script kept, everything else a dash. A name from a manifest goes through
// it before it becomes a path (S-29.4).
func Slug(s string) string { return slug(s) }

// slug is a name as a file-name segment: letters and digits of any script
// kept, everything else a dash.
func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if out == "" {
		return "backup"
	}
	return out
}

// The ceilings of an archive read (S-29.4): what is held in memory of a
// file somebody else may have made. MaxManifestBytes bounds the manifest;
// MaxEntries the members, far above what a copy of CARREL_FILES_MAX_ENTRIES
// files per collection writes. The records of a member are read under the
// import ceilings by the caller: restoring is an import (S-24.3).
const (
	MaxManifestBytes = 1 << 20
	MaxEntries       = 50_000
)
