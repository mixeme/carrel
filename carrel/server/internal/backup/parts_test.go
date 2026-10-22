package backup

import (
	"bytes"
	"errors"
	"io"
	"math/rand/v2"
	"testing"

	"carrel.dev/carrel/server/view"
)

// held is a file of which only some stretches came: reading anything else
// fails, as reading an archive whose parts were not sent does.
type held struct {
	data  []byte
	spans []Span
}

var errHole = errors.New("not sent")

func (h held) ReadAt(p []byte, off int64) (int, error) {
	for i := range p {
		at := off + int64(i)
		if at >= int64(len(h.data)) {
			return i, io.EOF
		}
		in := false
		for _, s := range h.spans {
			in = in || (at >= s.At && at < s.End())
		}
		if !in {
			return i, errHole
		}
		p[i] = h.data[at]
	}
	return len(p), nil
}

// S-24.3, Typecase's note 109: an archive from this computer is opened by
// its start and its end, and a collection is read by the stretches its
// members lie in — sealed or not, and a member larger than the end the
// archive was opened by.
func TestArchiveOpensAndRestoresByParts(t *testing.T) {
	big := make([]byte, OpenTail+(1<<20))
	rnd := rand.New(rand.NewPCG(1, 2))
	for i := range big {
		big[i] = byte(rnd.Uint32())
	}
	srcs := []Source{
		sources(false)[0],
		{Entry: Entry{Name: "Files", Kind: view.KindFile, AccountID: "a1", Path: "/files/"},
			Write: func(add func(string) (io.Writer, error)) (int, error) {
				w, err := add("files/big.bin")
				if err != nil {
					return 0, err
				}
				_, err = w.Write(big)
				return 1, err
			}},
		sources(false)[1],
	}
	for _, password := range []string{"", "correct horse battery staple"} {
		var buf bytes.Buffer
		if _, err := Write(&buf, Manifest{Job: "Рабочее", Carrel: "test", Taken: taken}, srcs, password); err != nil {
			t.Fatal(err)
		}
		size := int64(buf.Len())
		open := OpenSpans(size)
		if len(open) != 2 || open[0].At != 0 || open[1].End() != size {
			t.Fatalf("open spans of %d bytes = %+v", size, open)
		}
		h := held{data: buf.Bytes(), spans: open}
		zr, m, err := OpenAt(h, size, password)
		if err != nil {
			t.Fatalf("password %q: opened by its start and end: %v", password, err)
		}
		if len(m.Collections) != 3 {
			t.Fatalf("manifest = %+v", m)
		}
		read := func(name string) error {
			f, err := zr.Open(name)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(io.Discard, f)
			return err
		}
		if err := read(m.Collections[1].Dir + "files/big.bin"); err == nil {
			t.Fatalf("password %q: a member nobody sent was read", password)
		}
		l, err := ReadLayout(h, size, password)
		if err != nil {
			t.Fatal(err)
		}
		spans := l.Spans([]string{m.Collections[1].Dir})
		var sent int64
		for _, s := range spans {
			sent += s.Len
		}
		if sent < int64(len(big)) || sent > int64(len(big))+2*sealedChunk+(1<<10) {
			t.Errorf("password %q: %d bytes asked for a member of %d: %+v", password, sent, len(big), spans)
		}
		h.spans = append(h.spans, spans...)
		if zr, _, err = OpenAt(h, size, password); err != nil {
			t.Fatal(err)
		}
		if err := read(m.Collections[1].Dir + "files/big.bin"); err != nil {
			t.Errorf("password %q: the member its spans were sent for: %v", password, err)
		}
		if _, err := ReadLayout(h, size, "wrong"); password != "" && !errors.Is(err, ErrWrongPassword) {
			t.Errorf("wrong password: %v", err)
		}
	}
}
