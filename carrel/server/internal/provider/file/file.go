package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
)

var (
	ErrBadSegment = errors.New("file: invalid path segment")
	ErrNotUnder   = errors.New("file: path escapes collection root")
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
	if strings.HasPrefix(seg, ".") {
		return ErrBadSegment
	}
	return nil
}

// Provider streams files through DAV (S-5).
type Provider struct {
	t dav.Transport
	c dav.Creator
}

// New wires a file provider. t must also implement dav.Creator for uploads.
func New(t dav.Transport) (*Provider, error) {
	c, ok := t.(dav.Creator)
	if !ok {
		return nil, fmt.Errorf("file: transport lacks PutCreate")
	}
	return &Provider{t: t, c: c}, nil
}

// Get opens a byte range stream without buffering the file (S-5).
func (p *Provider) Get(ctx context.Context, path string, rng *dav.Range) (io.ReadCloser, string, error) {
	return p.t.Get(ctx, path, rng)
}

// UploadCreate writes a new file; occupied names return HTTP 412 (S-8, S-17).
func (p *Provider) UploadCreate(ctx context.Context, path string, body io.Reader) (string, error) {
	return p.c.PutCreate(ctx, path, body)
}

// ErrOccupied reports whether err is a 412 from conditional create.
func ErrOccupied(err error) bool {
	return dav.IsStatus(err, 412)
}
