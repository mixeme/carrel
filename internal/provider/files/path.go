// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package files

import (
	"errors"
	"path"
	"strings"
)

// ErrOutsideCollection is returned for a path that would leave the collection
// it was resolved against. §24.4 asks for `../` to be refused rather than
// quietly normalised away, because a request that tried to escape is worth
// refusing on its own terms.
var ErrOutsideCollection = errors.New("files: path leaves the collection")

// ErrBadName is returned for a name a DAV path cannot carry.
var ErrBadName = errors.New("files: invalid file name")

// CleanRelative normalises a path taken from a URL into a slash-separated
// relative path with no empty, dot or dot-dot segments.
//
// The result never starts or ends with a slash: whether a path names a
// collection is the server's answer, not something a trailing slash decides
// here.
func CleanRelative(rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", nil
	}
	if err := checkChars(rel); err != nil {
		return "", err
	}
	var out []string
	for _, segment := range strings.Split(rel, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			return "", ErrOutsideCollection
		}
		out = append(out, segment)
	}
	return strings.Join(out, "/"), nil
}

// Resolve returns the absolute DAV path of rel inside root.
//
// The check is belt and braces: the segments are refused one by one, and the
// joined result is then required to still sit under the root. One of those alone
// would be enough; both together mean a mistake in either has to be made twice
// to matter.
func Resolve(root, rel string) (string, error) {
	root = NormalizeDir(root)
	if root == "" {
		return "", errors.New("files: collection path is required")
	}
	clean, err := CleanRelative(rel)
	if err != nil {
		return "", err
	}
	if clean == "" {
		return root, nil
	}
	joined := path.Clean(root + clean)
	if joined != root && !strings.HasPrefix(joined, root) {
		return "", ErrOutsideCollection
	}
	return joined, nil
}

// Relative is the inverse of Resolve: the path of abs inside root, or an error
// when abs is not inside it at all. It is how a path that arrived from an
// `ATTACH` URI is checked against the collection it claims to be in.
func Relative(root, abs string) (string, error) {
	root = NormalizeDir(root)
	trimmed := strings.TrimSuffix(abs, "/")
	if trimmed+"/" == root {
		return "", nil
	}
	if !strings.HasPrefix(abs, root) {
		return "", ErrOutsideCollection
	}
	return CleanRelative(strings.TrimPrefix(trimmed, root))
}

// CleanName checks one path segment — a file being uploaded, a folder being
// created. A name is a name: anything with a separator in it is a path, and a
// form that asked for a name gets to refuse one.
func CleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", ErrBadName
	}
	if strings.ContainsAny(name, "/\\") {
		return "", ErrBadName
	}
	if err := checkChars(name); err != nil {
		return "", err
	}
	return name, nil
}

// NormalizeDir returns a directory path that starts and ends with a slash.
func NormalizeDir(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// Parent is the relative path of the directory holding rel, and whether rel had
// one at all — the root of a collection does not.
func Parent(rel string) (string, bool) {
	clean, err := CleanRelative(rel)
	if err != nil || clean == "" {
		return "", false
	}
	i := strings.LastIndex(clean, "/")
	if i < 0 {
		return "", true
	}
	return clean[:i], true
}

// Base is the last segment of a relative path.
func Base(rel string) string {
	clean, err := CleanRelative(rel)
	if err != nil || clean == "" {
		return ""
	}
	if i := strings.LastIndex(clean, "/"); i >= 0 {
		return clean[i+1:]
	}
	return clean
}

// Join appends a name to a relative directory path.
func Join(dir, name string) string {
	dir = strings.Trim(dir, "/")
	name = strings.Trim(name, "/")
	switch {
	case dir == "":
		return name
	case name == "":
		return dir
	}
	return dir + "/" + name
}

// checkChars refuses control characters and the backslash. A backslash is legal
// in a DAV name and means a separator on one of the platforms this runs on,
// which is exactly the ambiguity a path check should not have to reason about.
func checkChars(s string) error {
	if strings.ContainsRune(s, '\\') {
		return ErrBadName
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return ErrBadName
		}
	}
	return nil
}
