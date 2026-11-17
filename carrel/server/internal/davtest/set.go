package davtest

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SetShapes are the sizes and times the frames give the files of the set in
// docs/visual/fixtures (s-files-1, s-files-5, states-34): the files on disk
// are stubs, and SeedSet grows them to the size; a folder's time is set
// after what is in it. Head goes before the stub: the header of an image
// whose width and height a frame draws (s-files-1: «Dimensions ·
// 1920×1080»; s-files-5: «2.8 MB · 4032×3024»). The sizes of «2026-07» add
// up to the frame's «7 items · 89.4 MB», the two of «Public» to «2 items ·
// 5.3 MB». Slug is the storage's folder, Rel the path in it.
var SetShapes = []struct {
	Slug, Rel string
	Size      int64
	At        string // 2006-01-02 15:04, in the zone of the set's day
	Head      string
}{
	{Slug: "Documents", Rel: "carrel/attachments/2026-07/whiteboard.jpg", Size: 2936013, At: "2026-07-14 16:20", Head: JPEGHead4032x3024},
	{Slug: "Documents", Rel: "carrel/attachments/2026-07/схема-1.png", Size: 640 << 10, At: "2026-07-01 12:00"},
	{Slug: "Documents", Rel: "carrel/attachments/2026-07/схема-2.png", Size: 712 << 10, At: "2026-07-02 10:30"},
	{Slug: "Documents", Rel: "carrel/attachments/2026-07/смета.pdf", Size: 1258291, At: "2026-07-09 15:45"},
	{Slug: "Documents", Rel: "carrel/attachments/2026-07/панорама.tif", Size: 88122982, At: "2026-07-20 13:10"},
	{Slug: "Public", Rel: "прайс-2026.pdf", Size: 1153434, At: "2026-07-28 10:15"},
	{Slug: "Public", Rel: "схема-проезда.png", Size: 4404019, At: "2026-06-02 16:40"},
	{Slug: "Documents", Rel: "carrel/attachments/2026-08/2026-08-10-sync-board.png", Size: 412 << 10, At: "2026-08-10 11:07", Head: PNGHead(1920, 1080)},
	// The checklist is the frame's Markdown in the set, 3 KB as it stands
	// (s-publish-10): a public page reads it as its body.
	{Slug: "Documents", Rel: "carrel/attachments/2026-08/release-checklist.md", At: "2026-08-09 18:44"},
	{Slug: "Documents", Rel: "carrel/attachments/2026-08/смета-подрядчик.pdf", Size: 1258291, At: "2026-08-05 14:20"},
	{Slug: "Documents", Rel: "carrel/attachments/2026-08/2026-08-04-whiteboard.jpg", Size: 2936013, At: "2026-08-04 17:03", Head: JPEGHead4032x3024},
	{Slug: "Documents", Rel: "carrel/attachments/2026-07", At: "2026-07-01 09:12"},
	{Slug: "Documents", Rel: "ковалёва-обложка-v2.pdf", Size: 1677722, At: "2026-08-06 16:20"},
	// August's folder, changed when its last file came (s-publish-5:
	// «2026-08 · folder · 4 items · 2026-08-10 11:07»; note 180).
	{Slug: "Documents", Rel: "carrel/attachments/2026-08", At: "2026-08-10 11:07"},
}

// JPEGHead4032x3024 is the start of a JPEG and its baseline frame header:
// 3024 lines of 4032, three components — what the head of a camera's
// picture tells, and all of it the size of an image is read from (S-17).
const JPEGHead4032x3024 = "\xff\xd8\xff\xc0\x00\x11\x08\x0b\xd0\x0f\xc0\x03\x01\x22\x00\x02\x11\x01\x03\x11\x01"

// PNGSignature is the eight bytes every PNG begins with.
const PNGSignature = "\x89PNG\r\n\x1a\n"

// PNGHead is the start of a PNG and its IHDR, w by h of eight-bit RGB: all
// of it the size of an image is read from (S-17).
func PNGHead(w, h uint32) string {
	ihdr := binary.BigEndian.AppendUint32(binary.BigEndian.AppendUint32([]byte("IHDR"), w), h)
	ihdr = append(ihdr, 8, 2, 0, 0, 0)
	head := binary.BigEndian.AppendUint32([]byte(PNGSignature), 13)
	return string(binary.BigEndian.AppendUint32(append(head, ihdr...), crc32.ChecksumIEEE(ihdr)))
}

// SetFile is one entry of a folder of the set as a server is to hold it: a
// folder (Rel ends in "/"), or a file — what is written (an image's head
// before the stub), the size it is grown to with zeros (0: as written) and
// when it changed.
type SetFile struct {
	Rel  string
	Data string
	Size int64
	At   time.Time
}

// SetFiles reads a folder of the set (dir on disk, slug its name in
// SetShapes) in the order a walk finds it: each file changed on the day its
// name begins with, else on day — never the day the set is loaded, for when
// a file changed is part of what the screens show («Oldest» of
// attachments-1) —, and then with the sizes, times and image heads
// SetShapes give. A folder's time is the shape's when it has one, else the
// zero time: a server sets it.
func SetFiles(dir, slug string, day time.Time) ([]SetFile, error) {
	var out []SetFile
	at := map[string]int{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			at[rel] = len(out)
			out = append(out, SetFile{Rel: rel + "/"})
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		when := day
		if len(d.Name()) >= 10 {
			if named, err := time.ParseInLocation("2006-01-02", d.Name()[:10], day.Location()); err == nil {
				when = named.Add(12 * time.Hour)
			}
		}
		at[rel] = len(out)
		out = append(out, SetFile{Rel: rel, Data: string(data), At: when})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, one := range SetShapes {
		if one.Slug != slug {
			continue
		}
		i, ok := at[one.Rel]
		if !ok {
			return nil, fmt.Errorf("the set has no %s in %s", one.Rel, slug)
		}
		when, err := time.ParseInLocation("2006-01-02 15:04", one.At, day.Location())
		if err != nil {
			return nil, err
		}
		out[i].At = when
		if one.Size > 0 {
			out[i].Data = one.Head + out[i].Data
			out[i].Size = one.Size
		}
	}
	return out, nil
}

// SeedSet puts a folder of the set (SetFiles) under at, a folder under
// FileRoot: its folders and files as they are walked, then the times — the
// folders' last, for writing in a folder changes it. The fixtures and the
// acceptance stand load the set the same way through it.
func (s *Server) SeedSet(at, dir, slug string, day time.Time) error {
	set, err := SetFiles(dir, slug, day)
	if err != nil {
		return err
	}
	s.SeedFolder(at)
	for _, f := range set {
		if strings.HasSuffix(f.Rel, "/") {
			s.SeedFolder(at + f.Rel)
			continue
		}
		s.SeedFile(at+f.Rel, f.Data)
		if f.Size > 0 {
			s.Grow(at+f.Rel, f.Size)
		}
		s.SetModified(at+f.Rel, f.At)
	}
	for _, f := range set {
		if strings.HasSuffix(f.Rel, "/") && !f.At.IsZero() {
			s.SetModified(at+strings.TrimSuffix(f.Rel, "/"), f.At)
		}
	}
	return nil
}
