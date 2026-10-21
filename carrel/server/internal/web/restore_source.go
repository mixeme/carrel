package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// An open restore reads its archive where it lies, a piece at a time, and
// keeps none of it on Carrel's disk (S-24.3, the owner's decision of
// 19.10.2026): the manifest and the zip's directory at the end, the cipher's
// header at the start, the records of the collections restored. age reads
// from any chunk and zip by its directory, so the middle of an archive is
// fetched only when it is restored.

// archive is where an open restore reads from.
type archive interface {
	io.ReaderAt
	Size() int64
}

// memArchive is an archive a browser sent without a script, up to the
// ceiling of an import: held in the session's memory, as the parsed file of
// an import is, until Restore or Cancel.
type memArchive struct{ *bytes.Reader }

// errNoRange is a server that answers a range with the whole file: the
// archive cannot be read on it without fetching all of it.
var errNoRange = errors.New("this server sends the whole file where a part is asked (it ignores HTTP Range), so an archive on it cannot be opened without downloading all of it")

// errArchiveChanged is an archive that is not the one opened: its version
// changed between the steps.
var errArchiveChanged = errors.New("the archive changed on its server since it was opened; open it again")

// The pieces a far archive is fetched in, and how many are kept in memory:
// the zip reader and the cipher read a few kilobytes at a time, a request
// each would be thousands of requests.
const (
	rangeBlock = 1 << 20
	rangeKept  = 8
)

// rangeArchive is an archive on a WebDAV storage, read by HTTP Range: a
// block at a time, the last few blocks kept; every answer must be a part
// (206) of the same version (ETag) the archive was opened at.
type rangeArchive struct {
	files *file.Provider
	path  string
	size  int64
	etag  string

	mu     sync.Mutex
	blocks map[int64][]byte
	order  []int64
}

// openRange opens the archive at path on files: its size and its version
// from the answer to its first byte.
func openRange(ctx context.Context, files *file.Provider, path string) (*rangeArchive, error) {
	d, err := files.Open(ctx, path, &dav.Range{Start: 0, End: 0})
	if err != nil {
		return nil, err
	}
	d.Body.Close()
	if d.Status != http.StatusPartialContent {
		return nil, errNoRange
	}
	size, ok := rangeTotal(d.Range)
	if !ok {
		return nil, errNoRange
	}
	return &rangeArchive{files: files, path: path, size: size, etag: d.ETag, blocks: map[int64][]byte{}}, nil
}

// rangeTotal is the whole length of a Content-Range: «bytes 0-0/1234».
func rangeTotal(cr string) (int64, bool) {
	_, total, ok := strings.Cut(cr, "/")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(total), 10, 64)
	return n, err == nil && n >= 0
}

func (a *rangeArchive) Size() int64 { return a.size }

func (a *rangeArchive) ReadAt(p []byte, off int64) (int, error) {
	if off >= a.size {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) && off < a.size {
		b, err := a.block(off / rangeBlock)
		if err != nil {
			return n, err
		}
		c := copy(p[n:], b[off%rangeBlock:])
		n += c
		off += int64(c)
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// block is the i-th block of the archive, fetched once while it is among the
// last few read.
func (a *rangeArchive) block(i int64) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if b, ok := a.blocks[i]; ok {
		return b, nil
	}
	start := i * rangeBlock
	end := min(start+rangeBlock, a.size) - 1
	d, err := a.files.Open(context.Background(), a.path, &dav.Range{Start: start, End: end})
	if err != nil {
		return nil, err
	}
	defer d.Body.Close()
	switch {
	case d.Status != http.StatusPartialContent:
		return nil, errNoRange
	case a.etag != "" && d.ETag != "" && d.ETag != a.etag:
		return nil, errArchiveChanged
	}
	b, err := io.ReadAll(io.LimitReader(d.Body, end-start+2))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) != end-start+1 {
		return nil, errArchiveChanged
	}
	a.blocks[i] = b
	a.order = append(a.order, i)
	if len(a.order) > rangeKept {
		delete(a.blocks, a.order[0])
		a.order = a.order[1:]
	}
	return b, nil
}
