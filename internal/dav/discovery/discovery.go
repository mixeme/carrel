// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

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
	if err != nil {
		return nil, trace, err
	}

	var collections []Collection
	if calHome, err := findCalendarHome(ctx, client, principal, trace); err == nil && calHome != "" {
		cols, stepErr := listCollections(ctx, client, calHome, KindCalendar, trace, "calendar_collections")
		if stepErr != nil {
			trace.Add("calendar_collections", stepErr.Error(), 0, "")
		} else {
			collections = append(collections, cols...)
		}
	}
	if abHome, err := findAddressBookHome(ctx, client, principal, trace); err == nil && abHome != "" {
		cols, stepErr := listCollections(ctx, client, abHome, KindAddressBook, trace, "addressbook_collections")
		if stepErr != nil {
			trace.Add("addressbook_collections", stepErr.Error(), 0, "")
		} else {
			collections = append(collections, cols...)
		}
	}
	if len(collections) == 0 {
		return nil, trace, fmt.Errorf("discovery: no calendar or address book collections found")
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

func findPrincipal(ctx context.Context, client *dav.Client, trace *Trace) (string, error) {
	ms, err := client.PropFind(ctx, "/", dav.DepthZero, []xml.Name{dav.CurrentUserPrincipalName})
	if err != nil {
		trace.Add("principal", err.Error(), 0, "")
		return "", err
	}
	if len(ms.Responses) == 0 {
		err := fmt.Errorf("discovery: empty principal response")
		trace.Add("principal", err.Error(), 0, "")
		return "", err
	}
	var principal dav.CurrentUserPrincipal
	if err := ms.Responses[0].DecodeProp(&principal); err != nil {
		trace.Add("principal", err.Error(), 0, "")
		return "", err
	}
	path := principal.Href.Path
	if path == "" {
		err := fmt.Errorf("discovery: empty principal href")
		trace.Add("principal", err.Error(), 0, "")
		return "", err
	}
	trace.Add("principal", "ok", http.StatusOK, path)
	return path, nil
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
