// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

// errNoPrincipal is a successful Multi-Status that did not name a
// current-user-principal. Plain WebDAV servers do not advertise one: it is a
// CalDAV/CardDAV property, and its absence is how a files-only account is
// recognised rather than refused.
var errNoPrincipal = errors.New("discovery: no current-user-principal")

// Kind classifies a discovered collection.
type Kind string

const (
	KindCalendar    Kind = "calendar"
	KindAddressBook Kind = "addressbook"
	KindFiles       Kind = "files"
)

// Collection is one DAV collection from discovery (§6).
type Collection struct {
	Path                string   `json:"path"`
	DisplayName         string   `json:"displayname"`
	Kind                Kind     `json:"kind"`
	Color               string   `json:"color,omitempty"`
	SupportedComponents []string `json:"supported_components,omitempty"`
	Privileges          []string `json:"privileges,omitempty"`
	CTag                string   `json:"ctag,omitempty"`
	ReadOnly            bool     `json:"read_only,omitempty"`
}

// Result is a successful discovery outcome.
type Result struct {
	BaseURL     string       `json:"base_url"`
	Principal   string       `json:"principal"`
	Collections []Collection `json:"collections"`
}

// Credentials are the DAV login details supplied by the user.
type Credentials struct {
	BaseURL  string
	Username string
	Password string
}

// Discover runs the full discovery chain and records each step in trace.
func Discover(ctx context.Context, guard *dav.Guard, creds Credentials) (*Result, *Trace, error) {
	trace := &Trace{}
	if strings.TrimSpace(creds.BaseURL) == "" {
		return nil, trace, fmt.Errorf("discovery: base URL is required")
	}

	client, err := dav.NewClient(guard, creds.BaseURL, creds.Username, creds.Password)
	if err != nil {
		return nil, trace, err
	}

	base, err := resolveBaseURL(ctx, client, creds.BaseURL, trace)
	if err != nil {
		return nil, trace, err
	}
	client, err = dav.NewClient(guard, base, creds.Username, creds.Password)
	if err != nil {
		return nil, trace, err
	}

	principal, err := findPrincipal(ctx, client, trace)
	if err != nil && !errors.Is(err, errNoPrincipal) {
		return nil, trace, err
	}

	var collections []Collection
	if principal == "" {
		// A plain WebDAV server has no CalDAV principal and no home-set. The
		// URL the person entered is the file collection — the same path the
		// files provider already uses when pointed at a live server directly.
		collections = takeFileRoot(ctx, client, base, trace)
	} else {
		calHome, _ := findCalendarHome(ctx, client, principal, trace)
		if calHome != "" {
			cols, stepErr := listCollections(ctx, client, calHome, KindCalendar, trace, "calendar_collections")
			if stepErr != nil {
				trace.Add("calendar_collections", stepErr.Error(), 0, "")
			} else {
				collections = append(collections, cols...)
			}
		}
		abHome, _ := findAddressBookHome(ctx, client, principal, trace)
		if abHome != "" {
			cols, stepErr := listCollections(ctx, client, abHome, KindAddressBook, trace, "addressbook_collections")
			if stepErr != nil {
				trace.Add("addressbook_collections", stepErr.Error(), 0, "")
			} else {
				collections = append(collections, cols...)
			}
		}
		// File collections are not advertised by a home-set: §6 defines them as the
		// plain collections under the root, and asks for no setting of their own —
		// there are some and the files section appears, or there are none and it
		// does not.
		cols, stepErr := listFileCollections(ctx, client, base, principal, calHome, abHome, trace)
		if stepErr != nil {
			trace.Add("file_collections", stepErr.Error(), 0, "")
		} else {
			collections = append(collections, cols...)
		}
	}
	if len(collections) == 0 {
		return nil, trace, fmt.Errorf("discovery: no calendar, address book or file collections found")
	}

	return &Result{
		BaseURL:     base,
		Principal:   principal,
		Collections: collections,
	}, trace, nil
}

func resolveBaseURL(ctx context.Context, client *dav.Client, entered string, trace *Trace) (string, error) {
	u, err := url.Parse(entered)
	if err != nil {
		return "", err
	}
	origin := &url.URL{Scheme: u.Scheme, Host: u.Host}

	for _, name := range []struct {
		step string
		path string
	}{
		{"well_known_caldav", "/.well-known/caldav"},
		{"well_known_carddav", "/.well-known/carddav"},
	} {
		wellKnown := origin.ResolveReference(&url.URL{Path: name.path})
		final, code, err := client.Head(ctx, wellKnown.String())
		trace.Add(name.step, statusText(code, err), code, finalString(final))
		if err == nil && code >= 200 && code < 400 && final != nil {
			return final.String(), nil
		}
	}

	trace.Add("base_url", "using entered URL", http.StatusOK, entered)
	return entered, nil
}

// findPrincipal asks for `current-user-principal`, at the base path first and
// only then at the server root.
//
// The order matters and getting it wrong is invisible without a real server.
// §6 makes the entered URL the main path precisely because Baikal lives at
// something like `/dav.php/`; asking the server root instead reaches the web
// server's home page, which answers 200 with HTML rather than 207 with a
// multistatus, and discovery fails on a URL that is perfectly correct. The root
// is still tried afterwards, because a server that does live there answers it.
//
// A Multi-Status that does not name the property is not a failure of this
// step: it is how a plain WebDAV server answers, and Discover then treats the
// entered URL as a file collection. A transport error — 401, HTML at the path,
// a refused connection — is still a failure.
func findPrincipal(ctx context.Context, client *dav.Client, trace *Trace) (string, error) {
	var targets []string
	if base := normalizePath(client.BaseURL().Path); base != "/" {
		targets = append(targets, base)
	}
	targets = append(targets, "/")

	var lastErr error
	missing := false
	for _, target := range targets {
		path, err := principalAt(ctx, client, target)
		if err != nil {
			lastErr = err
			if errors.Is(err, errNoPrincipal) {
				missing = true
			}
			continue
		}
		trace.Add("principal", "ok", http.StatusOK, path)
		return path, nil
	}
	if missing {
		// A later probe of the site root may have failed for a different
		// reason — HTML at `/` is the Baikal arrangement — and must not
		// hide that the DAV path already answered without a principal.
		trace.Add("principal", "not advertised; treating as files", 0, "")
		return "", errNoPrincipal
	}
	trace.Add("principal", lastErr.Error(), 0, "")
	return "", lastErr
}

// principalAt reads the principal from one path. Every response is looked at
// rather than only the first: a server answering Depth 0 with more than one is
// unusual but not wrong, and the property may not be on the leading entry.
func principalAt(ctx context.Context, client *dav.Client, target string) (string, error) {
	ms, err := client.PropFind(ctx, target, dav.DepthZero, []xml.Name{dav.CurrentUserPrincipalName})
	if err != nil {
		return "", fmt.Errorf("discovery: principal at %s: %w", target, err)
	}
	for _, resp := range ms.Responses {
		var principal dav.CurrentUserPrincipal
		if err := resp.DecodeProp(&principal); err != nil {
			continue
		}
		if path := principal.Href.Path; path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w at %s", errNoPrincipal, target)
}

func findCalendarHome(ctx context.Context, client *dav.Client, principal string, trace *Trace) (string, error) {
	ms, err := client.PropFind(ctx, principal, dav.DepthZero, []xml.Name{dav.CalendarHomeSetName})
	if err != nil {
		trace.Add("calendar_home", err.Error(), 0, "")
		return "", err
	}
	var home dav.CalendarHomeSet
	if err := ms.Responses[0].DecodeProp(&home); err != nil {
		trace.Add("calendar_home", err.Error(), 0, "")
		return "", err
	}
	trace.Add("calendar_home", "ok", http.StatusOK, home.Href.Path)
	return home.Href.Path, nil
}

func findAddressBookHome(ctx context.Context, client *dav.Client, principal string, trace *Trace) (string, error) {
	ms, err := client.PropFind(ctx, principal, dav.DepthZero, []xml.Name{dav.AddressBookHomeSetName})
	if err != nil {
		trace.Add("addressbook_home", err.Error(), 0, "")
		return "", err
	}
	var home dav.AddressBookHomeSet
	if err := ms.Responses[0].DecodeProp(&home); err != nil {
		trace.Add("addressbook_home", err.Error(), 0, "")
		return "", err
	}
	trace.Add("addressbook_home", "ok", http.StatusOK, home.Href.Path)
	return home.Href.Path, nil
}

// takeFileRoot treats the entered URL as one file collection. Plain WebDAV has
// no home-set and no child collections that need classifying: the path is the
// tree, and folders inside it are folders.
func takeFileRoot(ctx context.Context, client *dav.Client, base string, trace *Trace) []Collection {
	root := collectionRoot(base)
	ms, err := client.PropFind(ctx, root, dav.DepthZero, []xml.Name{
		dav.ResourceTypeName,
		dav.DisplayNameName,
		dav.CurrentUserPrivilegeSetName,
	})
	if err != nil {
		trace.Add("file_collections", err.Error(), 0, root)
		return []Collection{{Path: root, Kind: KindFiles}}
	}
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		if normalizePath(path) != root {
			continue
		}
		col, ok, err := decodeCollection(resp, KindFiles)
		if err == nil && ok {
			trace.Add("file_collections", "using the entered URL as a file collection", http.StatusMultiStatus, root)
			return []Collection{col}
		}
	}
	trace.Add("file_collections", "using the entered URL as a file collection", http.StatusMultiStatus, root)
	return []Collection{{Path: root, Kind: KindFiles}}
}

func collectionRoot(base string) string {
	root := base
	if u, err := url.Parse(base); err == nil && u.Path != "" {
		root = u.Path
	}
	return normalizePath(root)
}

// listFileCollections finds the plain collections directly under the root
// (§6). A calendar or address book home lives under the same root on most
// servers — Baikal answers `/dav.php/` with `calendars/`, `addressbooks/` and
// `principals/`, none of them marked as anything at that depth — so a container
// holding one of those homes is skipped rather than offered as a folder of
// files.
func listFileCollections(ctx context.Context, client *dav.Client, base, principal, calHome, abHome string, trace *Trace) ([]Collection, error) {
	root := collectionRoot(base)
	ms, err := client.PropFind(ctx, root, dav.DepthOne, []xml.Name{
		dav.ResourceTypeName,
		dav.DisplayNameName,
		dav.CurrentUserPrivilegeSetName,
	})
	if err != nil {
		return nil, err
	}
	var out []Collection
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		if normalizePath(path) == root {
			continue
		}
		if holdsService(path, principal, calHome, abHome) {
			continue
		}
		col, ok, err := decodeCollection(resp, KindFiles)
		if err != nil || !ok {
			continue
		}
		out = append(out, col)
	}
	trace.Add("file_collections", fmt.Sprintf("found %d collections", len(out)), http.StatusMultiStatus, root)
	return out, nil
}

// holdsService reports whether path is, or contains, one of the DAV homes. Those
// are the protocol's own trees and browsing them as files would offer a person
// their address books as a folder of `.vcf`.
func holdsService(path, principal, calHome, abHome string) bool {
	candidate := normalizePath(path)
	for _, service := range []string{principal, calHome, abHome} {
		if service == "" {
			continue
		}
		if strings.HasPrefix(normalizePath(service), candidate) {
			return true
		}
	}
	return false
}

func listCollections(ctx context.Context, client *dav.Client, home string, kind Kind, trace *Trace, step string) ([]Collection, error) {
	props := []xml.Name{
		dav.ResourceTypeName,
		dav.DisplayNameName,
		dav.CurrentUserPrivilegeSetName,
		dav.GetCTagName,
	}
	if kind == KindCalendar {
		props = append(props, dav.CalendarColorName, dav.SupportedCalendarComponentSetName)
	}
	ms, err := client.PropFind(ctx, home, dav.DepthOne, props)
	if err != nil {
		return nil, err
	}
	var out []Collection
	for _, resp := range ms.Responses {
		path, err := resp.Path()
		if err != nil {
			continue
		}
		if normalizePath(path) == normalizePath(home) {
			continue
		}
		col, ok, err := decodeCollection(resp, kind)
		if err != nil || !ok {
			continue
		}
		out = append(out, col)
	}
	trace.Add(step, fmt.Sprintf("found %d collections", len(out)), http.StatusMultiStatus, home)
	return out, nil
}

func decodeCollection(resp dav.Response, kind Kind) (Collection, bool, error) {
	var resType dav.ResourceType
	if err := resp.DecodeProp(&resType); err != nil {
		return Collection{}, false, err
	}
	switch kind {
	case KindCalendar:
		if !resType.Is(dav.CalendarName) {
			return Collection{}, false, nil
		}
	case KindAddressBook:
		if !resType.Is(dav.AddressBookName) {
			return Collection{}, false, nil
		}
	case KindFiles:
		// A plain collection and nothing more: the calendar and address book
		// markers, and a principal, all disqualify it (§6).
		if !resType.Is(dav.CollectionName) {
			return Collection{}, false, nil
		}
		if resType.Is(dav.CalendarName) || resType.Is(dav.AddressBookName) || resType.Is(dav.PrincipalName) {
			return Collection{}, false, nil
		}
	}

	path, err := resp.Path()
	if err != nil {
		return Collection{}, false, err
	}
	col := Collection{Path: path, Kind: kind}

	var disp dav.DisplayName
	if err := resp.DecodeProp(&disp); err == nil {
		col.DisplayName = disp.Name
	} else if !dav.IsNotFound(err) {
		return Collection{}, false, err
	}

	if kind == KindCalendar {
		var color dav.CalendarColor
		if err := resp.DecodeProp(&color); err == nil {
			col.Color = color.Color
		} else if !dav.IsNotFound(err) {
			return Collection{}, false, err
		}
		var comps dav.SupportedCalendarComponentSet
		if err := resp.DecodeProp(&comps); err == nil {
			for _, c := range comps.Comp {
				col.SupportedComponents = append(col.SupportedComponents, c.Name)
			}
		} else if !dav.IsNotFound(err) {
			return Collection{}, false, err
		}
	}

	var privs dav.CurrentUserPrivilegeSet
	if err := resp.DecodeProp(&privs); err == nil {
		col.Privileges, col.ReadOnly = decodePrivileges(privs)
	} else if !dav.IsNotFound(err) {
		return Collection{}, false, err
	}

	var ctag dav.GetCTag
	if err := resp.DecodeProp(&ctag); err == nil {
		col.CTag = ctag.Tag
	} else if !dav.IsNotFound(err) {
		return Collection{}, false, err
	}

	return col, true, nil
}

func decodePrivileges(privs dav.CurrentUserPrivilegeSet) ([]string, bool) {
	names := make([]string, 0, len(privs.Privileges))
	write := false
	for _, p := range privs.Privileges {
		switch {
		case p.Write != nil || p.WriteProps != nil || p.WriteCont != nil || p.Bind != nil || p.Unbind != nil:
			write = true
			names = append(names, "write")
		case p.Read != nil:
			names = append(names, "read")
		}
	}
	return names, !write
}

func statusText(code int, err error) string {
	if err != nil {
		return err.Error()
	}
	if code == 0 {
		return "no response"
	}
	return http.StatusText(code)
}

func finalString(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}
