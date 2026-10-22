package backup

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"filippo.io/age"
)

// An archive from this computer can come in parts (S-24.3, Typecase's notes
// 98 and 109): the browser keeps the file and sends the stretches the server
// names. To open it the server needs its start — an age file's header — and
// its end — the manifest, written last, and the zip's directory; to restore a
// collection, its members. These are where those lie in the file as it is,
// sealed or not.

// Span is a stretch of a file: Len bytes from At.
type Span struct{ At, Len int64 }

// End is where the span stops.
func (s Span) End() int64 { return s.At + s.Len }

// The start and the end an archive is opened by: the start holds an age
// file's header and its first chunk; the end the manifest (at most
// MaxManifestBytes) and a directory of MaxEntries members.
const (
	OpenHead = 64 << 10
	OpenTail = 8 << 20
)

// OpenSpans are the stretches of an archive of size bytes that open it:
// its start and its end, one stretch when they meet.
func OpenSpans(size int64) []Span {
	if size <= OpenHead+OpenTail {
		return []Span{{0, size}}
	}
	return []Span{{0, OpenHead}, {size - OpenTail, OpenTail}}
}

// The STREAM chunks of an age payload: 64 KiB of plaintext, sealed with a
// 16-byte tag (age's format, «stream»).
const (
	plainChunk  = 64 << 10
	sealedChunk = plainChunk + 16
	nonceSize   = 16
)

// Layout is where the members of an archive file lie: its directory read
// once, decrypted when the file is sealed — opening an age file costs its
// key's work every time.
type Layout struct {
	size    int64
	sealed  bool
	payload int64 // where a sealed file's payload starts
	members []member
	dirAt   int64
}

// ReadLayout reads the directory of the archive file in r (size bytes):
// decrypted with password when it is an age file.
func ReadLayout(r io.ReaderAt, size int64, password string) (*Layout, error) {
	head := make([]byte, min(size, OpenHead))
	if _, err := r.ReadAt(head, 0); err != nil && err != io.EOF {
		return nil, err
	}
	l := &Layout{size: size, sealed: Encrypted(head)}
	plain, plainSize := r, size
	if l.sealed {
		id, err := age.NewScryptIdentity(password)
		if err != nil {
			return nil, err
		}
		if plain, plainSize, err = age.DecryptReaderAt(r, size, id); err != nil {
			return nil, ErrWrongPassword
		}
		if l.payload, err = payloadAt(head); err != nil {
			return nil, err
		}
	}
	var err error
	l.members, l.dirAt, err = members(plain, plainSize)
	return l, err
}

// Spans are the stretches of the file that hold every member under the
// dirs: each from its local header to the next member's; in a sealed file,
// the whole chunks that hold them. Neighbours are joined, in the file's
// order.
func (l *Layout) Spans(dirs []string) []Span {
	var spans []Span
	for i, m := range l.members {
		end := l.dirAt
		if i+1 < len(l.members) {
			end = l.members[i+1].at
		}
		for _, d := range dirs {
			if d != "" && strings.HasPrefix(m.name, d) {
				spans = append(spans, Span{m.at, end - m.at})
				break
			}
		}
	}
	if l.sealed {
		for i, s := range spans {
			first, last := s.At/plainChunk, (s.End()-1)/plainChunk
			start := l.payload + first*sealedChunk
			spans[i] = Span{start, min(l.size, l.payload+(last+1)*sealedChunk) - start}
		}
	}
	return joined(spans)
}

// payloadAt is where an age file's payload starts: after the header's last
// line («--- » and its MAC) and the stream's nonce.
func payloadAt(head []byte) (int64, error) {
	i := bytes.Index(head, []byte("\n--- "))
	if i < 0 {
		return 0, errors.New("backup: the archive's header is longer than its start")
	}
	j := bytes.IndexByte(head[i+1:], '\n')
	if j < 0 {
		return 0, errors.New("backup: the archive's header is longer than its start")
	}
	return int64(i+1+j+1) + nonceSize, nil
}

// joined are the spans in order, those that touch or overlap made one.
func joined(spans []Span) []Span {
	slices.SortFunc(spans, func(a, b Span) int {
		switch {
		case a.At < b.At:
			return -1
		case a.At > b.At:
			return 1
		}
		return 0
	})
	var out []Span
	for _, s := range spans {
		if n := len(out); n > 0 && s.At <= out[n-1].End() {
			out[n-1].Len = max(out[n-1].End(), s.End()) - out[n-1].At
			continue
		}
		out = append(out, s)
	}
	return out
}

// member is an entry of a zip's directory: its name and where its local
// header is.
type member struct {
	name string
	at   int64
}

// The signatures and sizes of a zip's records (APPNOTE 4.3).
const (
	sigEnd         = 0x06054b50
	sigEnd64       = 0x06064b50
	sigEnd64Locate = 0x07064b50
	sigDir         = 0x02014b50
	endLen         = 22
	end64LocateLen = 20
	dirLen         = 46
)

// members are the entries of the zip in r, by where they lie, and where its
// directory starts. archive/zip keeps the local headers' offsets to itself.
func members(r io.ReaderAt, size int64) ([]member, int64, error) {
	// The end record, with no comment after it as Carrel writes it, else
	// up to a comment's longest.
	tail := make([]byte, min(size, endLen+0xffff))
	if _, err := r.ReadAt(tail, size-int64(len(tail))); err != nil && err != io.EOF {
		return nil, 0, err
	}
	e := -1
	for i := len(tail) - endLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:]) == sigEnd {
			e = i
			break
		}
	}
	if e < 0 {
		return nil, 0, errors.New("backup: not a Carrel archive: no zip directory at its end")
	}
	end := tail[e:]
	count := int64(binary.LittleEndian.Uint16(end[10:]))
	dirSize := int64(binary.LittleEndian.Uint32(end[12:]))
	dirAt := int64(binary.LittleEndian.Uint32(end[16:]))
	if count == 0xffff || dirSize == 0xffffffff || dirAt == 0xffffffff {
		// zip64: the locator in front of the end record names the zip64
		// end record.
		endAt := size - int64(len(tail)) + int64(e)
		loc := make([]byte, end64LocateLen)
		if _, err := r.ReadAt(loc, endAt-end64LocateLen); err != nil || binary.LittleEndian.Uint32(loc) != sigEnd64Locate {
			return nil, 0, errors.New("backup: not a Carrel archive: its zip64 end is not where it says")
		}
		rec := make([]byte, 56)
		if _, err := r.ReadAt(rec, int64(binary.LittleEndian.Uint64(loc[8:]))); err != nil || binary.LittleEndian.Uint32(rec) != sigEnd64 {
			return nil, 0, errors.New("backup: not a Carrel archive: its zip64 end is not where it says")
		}
		count = int64(binary.LittleEndian.Uint64(rec[32:]))
		dirSize = int64(binary.LittleEndian.Uint64(rec[40:]))
		dirAt = int64(binary.LittleEndian.Uint64(rec[48:]))
	}
	if count > MaxEntries || dirAt < 0 || dirSize < 0 || dirAt+dirSize > size {
		return nil, 0, fmt.Errorf("backup: not a Carrel archive: its directory of %d members does not fit", count)
	}
	dir := make([]byte, dirSize)
	if _, err := r.ReadAt(dir, dirAt); err != nil && err != io.EOF {
		return nil, 0, err
	}
	var out []member
	for len(dir) >= dirLen && binary.LittleEndian.Uint32(dir) == sigDir {
		csize := binary.LittleEndian.Uint32(dir[20:])
		usize := binary.LittleEndian.Uint32(dir[24:])
		nameLen := int(binary.LittleEndian.Uint16(dir[28:]))
		extraLen := int(binary.LittleEndian.Uint16(dir[30:]))
		commentLen := int(binary.LittleEndian.Uint16(dir[32:]))
		at := int64(binary.LittleEndian.Uint32(dir[42:]))
		if len(dir) < dirLen+nameLen+extraLen+commentLen {
			break
		}
		name := string(dir[dirLen : dirLen+nameLen])
		if at == 0xffffffff {
			// The zip64 extra holds, in order, the sizes that did not fit
			// and then the offset (APPNOTE 4.5.3).
			extra := dir[dirLen+nameLen : dirLen+nameLen+extraLen]
			for len(extra) >= 4 {
				id, n := binary.LittleEndian.Uint16(extra), int(binary.LittleEndian.Uint16(extra[2:]))
				if len(extra) < 4+n {
					break
				}
				if id == 0x0001 {
					v := extra[4 : 4+n]
					for _, big := range []bool{usize == 0xffffffff, csize == 0xffffffff} {
						if big && len(v) >= 8 {
							v = v[8:]
						}
					}
					if len(v) >= 8 {
						at = int64(binary.LittleEndian.Uint64(v))
					}
				}
				extra = extra[4+n:]
			}
		}
		out = append(out, member{name, at})
		dir = dir[dirLen+nameLen+extraLen+commentLen:]
	}
	if int64(len(out)) != count {
		return nil, 0, fmt.Errorf("backup: not a Carrel archive: its directory names %d members and holds %d", count, len(out))
	}
	slices.SortFunc(out, func(a, b member) int {
		switch {
		case a.at < b.at:
			return -1
		case a.at > b.at:
			return 1
		}
		return 0
	})
	return out, dirAt, nil
}
