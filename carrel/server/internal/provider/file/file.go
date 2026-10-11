package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
)

var (
	ErrBadSegment = errors.New("file: invalid path segment")
	ErrNotUnder   = errors.New("file: path escapes collection root")
	// ErrTooMany stops a folder copy at CARREL_FILES_MAX_ENTRIES entries.
	ErrTooMany = errors.New("file: too many entries")
)

// Join builds a collection-relative path from segments (S-17.4).
func Join(root string, segments ...string) (string, error) {
	root = strings.TrimSuffix(normalizeRoot(root), "/")
	if root == "" {
		root = "/"
	}
	var parts []string
	if root != "/" {
		parts = append(parts, strings.TrimPrefix(root, "/"))
	}
	for _, seg := range segments {
		if err := checkSegment(seg); err != nil {
			return "", err
		}
		parts = append(parts, seg)
	}
	if len(parts) == 0 {
		return root + "/", nil
	}
	joined := "/" + strings.Join(parts, "/")
	if !strings.HasPrefix(joined, root) && root != "/" {
		return "", ErrNotUnder
	}
	if root != "/" && !strings.HasPrefix(joined+"/", root+"/") {
		return "", ErrNotUnder
	}
	return joined, nil
}

func normalizeRoot(root string) string {
	if root == "" {
		return "/"
	}
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	return root
}

// CheckName reports whether name can be one member of a folder: not empty,
// not "." or "..", without a slash or a backslash.
func CheckName(name string) error { return checkSegment(name) }

// Escapes reports whether a segment tries to leave its folder — "..", or a
// separator smuggled inside one — rather than being merely empty or ".":
// such a request is refused, and the attempt is worth a line in the log
// (S-30.1).
func Escapes(seg string) bool {
	return seg == ".." || strings.ContainsAny(seg, "/\\")
}

func checkSegment(seg string) error {
	if seg == "" {
		return ErrBadSegment
	}
	if seg == "." || seg == ".." {
		return ErrBadSegment
	}
	if strings.Contains(seg, "/") || strings.Contains(seg, "\\") {
		return ErrBadSegment
	}
	return nil
}

// Suffixed is the name "Keep both" offers for the nth copy: a number before
// the extension, the way a taken collection address gets one (S-17.3).
// "report.pdf" becomes "report-2.pdf"; a name without an extension, a folder
// or a dotfile gets it at the end.
func Suffixed(name string, n int) string {
	ext := path.Ext(name)
	if ext == name || strings.Count(name, ".") == 1 && strings.HasPrefix(name, ".") {
		ext = ""
	}
	return strings.TrimSuffix(name, ext) + "-" + strconv.Itoa(n) + ext
}

// Provider streams files through DAV (S-5, S-17).
type Provider struct {
	t dav.Transport
	c dav.Creator
	s dav.Streamer
}

// New wires a file provider. t must also implement dav.Creator for uploads
// and dav.Streamer for downloads.
func New(t dav.Transport) (*Provider, error) {
	c, ok := t.(dav.Creator)
	if !ok {
		return nil, fmt.Errorf("file: transport lacks PutCreate")
	}
	s, ok := t.(dav.Streamer)
	if !ok {
		return nil, fmt.Errorf("file: transport lacks Open")
	}
	return &Provider{t: t, c: c, s: s}, nil
}

// Get opens a byte range stream without buffering the file and without the
// response cap: how large a file is, the far server decides (C-12).
func (p *Provider) Get(ctx context.Context, path string, rng *dav.Range) (io.ReadCloser, string, error) {
	d, err := p.s.Open(ctx, path, rng)
	if err != nil {
		return nil, "", err
	}
	return d.Body, d.ETag, nil
}

// Open is Get with what a browser needs to take the file: status, length,
// type, version and range.
func (p *Provider) Open(ctx context.Context, path string, rng *dav.Range) (*dav.Download, error) {
	return p.s.Open(ctx, path, rng)
}

// UploadCreate writes a new file; occupied names return HTTP 412 (S-8, S-17).
// A server that ignores If-None-Match overwrites instead, so a caller that
// must not overwrite asks Stat first.
func (p *Provider) UploadCreate(ctx context.Context, path string, body io.Reader) (string, error) {
	return p.c.PutCreate(dav.Stream(ctx), path, body)
}

// UploadOverwrite writes a file whether its name is taken or not. It is the
// only PUT without a precondition (S-17.1): the answer "Overwrite this one" to
// a question in the queue, never a setting.
func (p *Provider) UploadOverwrite(ctx context.Context, path string, body io.Reader) (string, error) {
	return p.t.Put(dav.Stream(ctx), path, body, "")
}

// UploadReplace is UploadOverwrite of the version the person was asked
// about: etag, the version that stood there when the question arose, goes
// as If-Match, so a file somebody wrote in between stays and the answer is
// 412 (ErrChanged). Without a version — the server gave none — it is
// UploadOverwrite.
func (p *Provider) UploadReplace(ctx context.Context, path string, body io.Reader, etag string) (string, error) {
	if etag != "" {
		etag = `"` + strings.Trim(etag, `"`) + `"`
	}
	return p.t.Put(dav.Stream(ctx), path, body, etag)
}

// MkCol creates a folder. A name already taken answers 405 (RFC 4918 §9.3.1).
func (p *Provider) MkCol(ctx context.Context, path string) error {
	return p.t.MkCol(ctx, path)
}

// Move moves or renames inside one server, without traffic. Without overwrite
// a taken name answers 412.
func (p *Provider) Move(ctx context.Context, src, dst string, overwrite bool) error {
	return p.t.Move(ctx, src, dst, overwrite)
}

// Delete removes a file, or a folder with everything in it. A version, when
// the listing gave one, goes as If-Match: a file changed since stays.
func (p *Provider) Delete(ctx context.Context, path, etag string) error {
	if etag != "" {
		etag = `"` + strings.Trim(etag, `"`) + `"`
	}
	return p.t.Delete(ctx, path, etag)
}

// ErrOccupied reports whether err is a 412 from conditional create.
func ErrOccupied(err error) bool {
	return dav.IsStatus(err, 412)
}

// Copy streams one file to another place, on this server or another: GET,
// then PUT of the body as it comes, never buffered (S-17.1). Without
// overwrite the PUT is conditional and a taken name answers 412.
func Copy(ctx context.Context, from *Provider, src string, to *Provider, dst string, overwrite bool) error {
	d, err := from.Open(ctx, src, nil)
	if err != nil {
		return err
	}
	defer d.Body.Close()
	var body io.Reader = d.Body
	if d.Length >= 0 {
		body = dav.Sized(body, d.Length)
	}
	if d.Type != "" {
		body = dav.Typed(body, d.Type)
	}
	if overwrite {
		_, err = to.UploadOverwrite(ctx, dst, body)
	} else {
		_, err = to.UploadCreate(ctx, dst, body)
	}
	return err
}

// CopyTree copies a folder with everything in it, a level at a time: MKCOL for
// the folder and each folder under it, Copy for each file. The destination
// must not exist: MKCOL of a taken name answers 405 and nothing is copied. It
// stops after max entries with ErrTooMany. It returns how many entries it
// copied; what arrived before a failure stays where it arrived.
func CopyTree(ctx context.Context, from *Provider, src string, to *Provider, dst string, max int) (int, error) {
	src, dst = strings.TrimSuffix(src, "/")+"/", strings.TrimSuffix(dst, "/")+"/"
	if err := to.MkCol(ctx, dst); err != nil {
		return 0, err
	}
	type level struct{ src, dst string }
	queue := []level{{src, dst}}
	n := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		entries, err := from.List(ctx, cur.src)
		if err != nil {
			return n, err
		}
		for _, e := range entries {
			if n >= max {
				return n, fmt.Errorf("%w: stopped after %d entries", ErrTooMany, max)
			}
			if err := checkSegment(e.Name); err != nil {
				return n, err
			}
			target := cur.dst + e.Name
			if e.Dir {
				if err := to.MkCol(ctx, target+"/"); err != nil {
					return n, err
				}
				queue = append(queue, level{e.Path, target + "/"})
			} else if err := Copy(ctx, from, e.Path, to, target, false); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// TreeFile is one member of a folder's tree: where it stands under the
// folder, whether it is a folder itself, and — of a file — its size and the
// version its server gives it, when it gives one.
type TreeFile struct {
	Rel  string
	Dir  bool
	Size int64
	ETag string
}

// Tree lists everything under a folder, a level at a time, in the order of
// its paths. More than max entries is ErrTooMany: a tree that cannot be
// listed whole is not one to compare.
func (p *Provider) Tree(ctx context.Context, root string, max int) ([]TreeFile, error) {
	root = strings.TrimSuffix(root, "/") + "/"
	type level struct{ path, rel string }
	queue := []level{{root, ""}}
	var out []TreeFile
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		entries, err := p.List(ctx, cur.path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if len(out) >= max {
				return nil, fmt.Errorf("%w: stopped after %d entries", ErrTooMany, max)
			}
			f := TreeFile{Rel: cur.rel + e.Name, Dir: e.Dir}
			if e.Dir {
				queue = append(queue, level{e.Path, f.Rel + "/"})
			} else {
				f.Size, f.ETag = e.Size, strings.Trim(e.ETag, `"`)
			}
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

// TreeDiffers names the first member in which two listings of a tree
// differ — one has it and the other does not, a folder against a file,
// another size; with versions, another version where both give one. Empty:
// the same tree. A copy is compared with its original without versions: a
// version is its server's own.
func TreeDiffers(a, b []TreeFile, versions bool) string {
	for i := 0; i < len(a) || i < len(b); i++ {
		switch {
		case i >= len(a):
			return b[i].Rel
		case i >= len(b):
			return a[i].Rel
		case a[i].Rel != b[i].Rel:
			return min(a[i].Rel, b[i].Rel)
		case a[i].Dir != b[i].Dir, !a[i].Dir && a[i].Size != b[i].Size:
			return a[i].Rel
		case versions && a[i].ETag != "" && b[i].ETag != "" && a[i].ETag != b[i].ETag:
			return a[i].Rel
		}
	}
	return ""
}
