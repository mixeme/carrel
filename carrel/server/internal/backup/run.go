package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Destination is where a run's archive goes: a folder of a file collection.
// Create never overwrites; Open reads a file back; List names what the folder
// holds; Delete removes one file.
type Destination interface {
	Create(ctx context.Context, name string, body io.Reader) error
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	List(ctx context.Context) ([]string, error)
	Delete(ctx context.Context, name string) error
}

// Result is what a run did.
type Result struct {
	Manifest Manifest
	File     string
	Size     int64
	SHA256   string
	// Verified is the archive read back from where it was written with the
	// same checksum. Written and not read back is not verified.
	Verified bool
	// Deleted are the older archives rotation removed — only after the new
	// one was verified and went in whole.
	Deleted []string
}

// counter counts and hashes what passes.
type counter struct {
	n int64
	h io.Writer
}

func (c *counter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return c.h.Write(p)
}

// Stream writes the archive to w, counting and hashing it.
func Stream(w io.Writer, m Manifest, sources []Source, password string) (Result, error) {
	h := sha256.New()
	c := &counter{h: h}
	got, err := Write(io.MultiWriter(w, c), m, sources, password)
	return Result{Manifest: got, Size: c.n, SHA256: hex.EncodeToString(h.Sum(nil))}, err
}

// ErrNotVerified is an archive whose read-back does not match what was
// written.
var ErrNotVerified = errors.New("backup: the archive read back does not match what was written")

// Run writes the archive to dest as name, reads it back and compares, and
// only then — and only when every collection went in — deletes the job's
// archives beyond keep (S-24.3: rotation after the check, never before).
func Run(ctx context.Context, dest Destination, name, job string, keep int, m Manifest, sources []Source, password string) (Result, error) {
	pr, pw := io.Pipe()
	done := make(chan Result, 1)
	go func() {
		res, err := Stream(pw, m, sources, password)
		pw.CloseWithError(err)
		done <- res
	}()
	err := dest.Create(ctx, name, pr)
	pr.CloseWithError(err) // unblock the writer if the destination gave up
	res := <-done
	res.File = name
	if err != nil {
		return res, fmt.Errorf("backup: writing %s: %w", name, err)
	}

	rc, err := dest.Open(ctx, name)
	if err != nil {
		return res, fmt.Errorf("backup: reading %s back: %w", name, err)
	}
	h := sha256.New()
	_, err = io.Copy(h, rc)
	rc.Close()
	if err != nil {
		return res, fmt.Errorf("backup: reading %s back: %w", name, err)
	}
	if hex.EncodeToString(h.Sum(nil)) != res.SHA256 {
		return res, ErrNotVerified
	}
	res.Verified = true

	if !res.Manifest.Complete() {
		return res, nil // a partial copy never pushes out a whole one
	}
	names, err := dest.List(ctx)
	if err != nil {
		return res, nil // rotation waits for the next run; the copy stands
	}
	for _, old := range Rotation(job, names, keep) {
		if old == name {
			continue
		}
		if dest.Delete(ctx, old) == nil {
			res.Deleted = append(res.Deleted, old)
		}
	}
	return res, nil
}
