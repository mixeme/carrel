package davtest

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
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

// SeedSet puts a folder of the set (dir on disk, slug its name in
// SetShapes) under at, a folder under FileRoot: its folders and files, each
// file changed on the day its name begins with, else on day — never the day
// the set is loaded, for when a file changed is part of what the screens
// show («Oldest» of attachments-1) —, and then the sizes, times and image
// heads SetShapes give. The fixtures and the acceptance stand load the set
// the same way through it.
func (s *Server) SeedSet(at, dir, slug string, day time.Time) error {
	s.SeedFolder(at)
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		target := at + filepath.ToSlash(rel)
		if d.IsDir() {
			s.SeedFolder(target + "/")
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s.SeedFile(target, string(data))
		when := day
		if len(d.Name()) >= 10 {
			if named, err := time.ParseInLocation("2006-01-02", d.Name()[:10], day.Location()); err == nil {
				when = named.Add(12 * time.Hour)
			}
		}
		s.SetModified(target, when)
		return nil
	})
	if err != nil {
		return err
	}
	for _, one := range SetShapes {
		if one.Slug != slug {
			continue
		}
		target := at + one.Rel
		if one.Size > 0 {
			data, ok := s.File(target)
			if !ok {
				return fmt.Errorf("the set has no %s", target)
			}
			s.SeedFile(target, one.Head+data)
			s.Grow(target, one.Size)
		}
		when, err := time.ParseInLocation("2006-01-02 15:04", one.At, day.Location())
		if err != nil {
			return err
		}
		s.SetModified(target, when)
	}
	return nil
}
