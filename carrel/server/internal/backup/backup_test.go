package backup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

func sources(fail bool) []Source {
	book := Source{Entry: Entry{Name: "Контакты", Kind: view.KindContact, AccountID: "a1", Path: "/book/"},
		Write: func(add func(string) (io.Writer, error)) (int, error) {
			w, err := add("contacts.vcf")
			if err != nil {
				return 0, err
			}
			_, err = io.WriteString(w, "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:a\r\nFN:Ann\r\nEND:VCARD\r\n")
			return 1, err
		}}
	cal := Source{Entry: Entry{Name: "Work", Kind: view.KindEvent, AccountID: "a1", Path: "/cal/"},
		Write: func(add func(string) (io.Writer, error)) (int, error) {
			if fail {
				return 0, errors.New("PROPFIND /cal/ → 401 Unauthorized")
			}
			w, _ := add("calendar.ics")
			_, err := io.WriteString(w, "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n")
			return 3, err
		}}
	return []Source{book, cal}
}

var taken = time.Date(2026, 8, 16, 3, 10, 0, 0, time.UTC)

// S-24.2: one archive, the collections in their folders, the manifest last
// with every count; sealed with the job's password, and the wrong password
// opens nothing. A failed collection is in the manifest with its error.
func TestArchiveRoundTrip(t *testing.T) {
	for _, password := range []string{"", "correct horse battery staple"} {
		var buf bytes.Buffer
		m, err := Write(&buf, Manifest{Job: "Рабочее", Carrel: "test", Taken: taken}, sources(true), password)
		if err != nil {
			t.Fatal(err)
		}
		if Encrypted(buf.Bytes()) != (password != "") {
			t.Fatalf("sealed = %v with password %q", Encrypted(buf.Bytes()), password)
		}
		if m.Complete() || m.Collections[1].Error == "" || m.Collections[0].Count != 1 {
			t.Errorf("manifest = %+v", m)
		}
		zr, got, err := Open(buf.Bytes(), password)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Collections) != 2 || got.Collections[0].Dir != "01-контакты/" || got.Job != "Рабочее" {
			t.Errorf("read manifest = %+v", got)
		}
		f, err := zr.Open("01-контакты/contacts.vcf")
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(f)
		if !strings.Contains(string(b), "FN:Ann") {
			t.Errorf("contacts = %q", b)
		}
		if password != "" {
			if _, _, err := Open(buf.Bytes(), "wrong"); !errors.Is(err, ErrWrongPassword) {
				t.Errorf("wrong password: %v", err)
			}
			if bytes.Contains(buf.Bytes(), []byte("FN:Ann")) || bytes.Contains(buf.Bytes(), []byte("manifest")) {
				t.Error("the sealed archive shows its contents")
			}
		}
	}
}

// memDest is a folder in memory that can be told to corrupt what it reads
// back or to refuse the write.
type memDest struct {
	mu      sync.Mutex
	files   map[string][]byte
	corrupt bool
	refuse  bool
	order   []string
}

func (d *memDest) Create(_ context.Context, name string, body io.Reader) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if d.refuse {
		return errors.New("507 Insufficient Storage")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.files[name]; ok {
		return errors.New("412: taken")
	}
	d.files[name] = b
	d.order = append(d.order, "create "+name)
	return nil
}

func (d *memDest) Open(_ context.Context, name string) (io.ReadCloser, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	b := append([]byte(nil), d.files[name]...)
	if d.corrupt && len(b) > 0 {
		b[len(b)/2] ^= 0xff
	}
	d.order = append(d.order, "read "+name)
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (d *memDest) List(context.Context) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []string
	for n := range d.files {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

func (d *memDest) Delete(_ context.Context, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.files, name)
	d.order = append(d.order, "delete "+name)
	return nil
}

func seeded(names ...string) *memDest {
	d := &memDest{files: map[string][]byte{}}
	for _, n := range names {
		d.files[n] = []byte("old")
	}
	return d
}

// S-24.3: written, read back, compared — and only then the oldest beyond
// keep go. Another job's archives and other files are never touched.
func TestRunRotatesAfterTheCheck(t *testing.T) {
	d := seeded("carrel-work-2026-08-13T031000Z.zip.age", "carrel-work-2026-08-14T031000Z.zip.age",
		"carrel-work-2026-08-15T031000Z.zip.age", "carrel-personal-2026-08-01T000000Z.zip", "notes.txt")
	name := FileName("work", taken, true)
	res, err := Run(context.Background(), d, name, "work", 3, Manifest{Taken: taken}, sources(false), "pw")
	if err != nil || !res.Verified || res.Size == 0 {
		t.Fatalf("run = %+v %v", res, err)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != "carrel-work-2026-08-13T031000Z.zip.age" {
		t.Errorf("deleted = %v", res.Deleted)
	}
	want := []string{"create " + name, "read " + name, "delete carrel-work-2026-08-13T031000Z.zip.age"}
	if strings.Join(d.order, "|") != strings.Join(want, "|") {
		t.Errorf("order = %v", d.order)
	}
	if _, ok := d.files["carrel-personal-2026-08-01T000000Z.zip"]; !ok {
		t.Error("another job's archive was deleted")
	}
}

// A copy whose read-back differs is not verified and deletes nothing; a
// partial copy is verified but deletes nothing either; a refused write
// deletes nothing.
func TestRunKeepsOldCopiesWhenTheNewOneIsNotWhole(t *testing.T) {
	old := "carrel-work-2026-08-13T031000Z.zip"
	for name, d := range map[string]*memDest{"corrupt": seeded(old), "refused": seeded(old), "partial": seeded(old)} {
		d.corrupt, d.refuse = name == "corrupt", name == "refused"
		res, err := Run(context.Background(), d, FileName("work", taken, false), "work", 1, Manifest{Taken: taken}, sources(name == "partial"), "")
		if _, ok := d.files[old]; !ok || len(res.Deleted) != 0 {
			t.Errorf("%s: the old copy went (%v %v)", name, res, err)
		}
		switch name {
		case "corrupt":
			if !errors.Is(err, ErrNotVerified) || res.Verified {
				t.Errorf("corrupt: %v %+v", err, res)
			}
		case "refused":
			if err == nil {
				t.Error("a refused write is not an error")
			}
		case "partial":
			if err != nil || !res.Verified || res.Manifest.Complete() {
				t.Errorf("partial: %v %+v", err, res)
			}
		}
	}
}

func TestFileNameAndSlug(t *testing.T) {
	if got := FileName("Личное на домашний диск!", taken, true); got != "carrel-личное-на-домашний-диск-2026-08-16T031000Z.zip.age" {
		t.Errorf("name = %q", got)
	}
	if got := FileName("***", taken, false); got != "carrel-backup-2026-08-16T031000Z.zip" {
		t.Errorf("empty name = %q", got)
	}
}
