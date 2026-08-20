// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

// Homes are the calendar and address-book home-set paths of one account.
type Homes struct {
	Calendar    string
	AddressBook string
}

// ResolveHomes reads home-set hrefs from the stored principal (§10.1).
func ResolveHomes(ctx context.Context, client *dav.Client, principal string) (Homes, error) {
	if strings.TrimSpace(principal) == "" {
		return Homes{}, fmt.Errorf("discovery: account has no CalDAV principal")
	}
	trace := &Trace{}
	cal, _ := findCalendarHome(ctx, client, principal, trace)
	ab, _ := findAddressBookHome(ctx, client, principal, trace)
	if cal == "" && ab == "" {
		return Homes{}, fmt.Errorf("discovery: no calendar or address book home-set found")
	}
	return Homes{Calendar: cal, AddressBook: ab}, nil
}

// CreateParams are the fields of the new-collection sheet (§10.1).
type CreateParams struct {
	Kind        Kind
	DisplayName string
	Address     string
	Color       string
	Components  []string
}

// CreateCollection makes a calendar or address book on the server and returns
// its metadata read back from a Depth:0 PROPFIND (§10.1).
func CreateCollection(ctx context.Context, client *dav.Client, homes Homes, existing []Collection, p CreateParams) (Collection, error) {
	if err := ValidateAddress(p.Address); err != nil {
		return Collection{}, err
	}
	name := strings.TrimSpace(p.DisplayName)
	if name == "" {
		return Collection{}, fmt.Errorf("enter a name")
	}
	switch p.Kind {
	case KindCalendar:
		if homes.Calendar == "" {
			return Collection{}, fmt.Errorf("this account has no calendar home-set")
		}
		return createCalendar(ctx, client, homes.Calendar, existing, name, p)
	case KindAddressBook:
		if homes.AddressBook == "" {
			return Collection{}, fmt.Errorf("this account has no address book home-set")
		}
		return createAddressBook(ctx, client, homes.AddressBook, existing, name, p)
	default:
		return Collection{}, fmt.Errorf("discovery: cannot create %s collections here", p.Kind)
	}
}

func createCalendar(ctx context.Context, client *dav.Client, home string, existing []Collection, name string, p CreateParams) (Collection, error) {
	href := CollectionHref(home, p.Address)
	comps := p.Components
	if len(comps) == 0 {
		comps = []string{"VEVENT"}
	}
	color := strings.TrimSpace(p.Color)
	if color == "" {
		color = ColorFromAddress(p.Address)
	}
	compXML := supportedComponentSetInner(comps)
	props := []dav.ColProp{
		{Name: dav.DisplayNameName, Value: name},
		{Name: dav.CalendarColorName, Value: color},
		{Name: dav.SupportedCalendarComponentSetName, Value: compXML},
	}
	if err := client.MkColProps(ctx, "MKCALENDAR", href, props); err != nil {
		return Collection{}, dav.WrapRequestError("MKCALENDAR", href, err)
	}
	return refreshCollection(ctx, client, href, KindCalendar)
}

func createAddressBook(ctx context.Context, client *dav.Client, home string, existing []Collection, name string, p CreateParams) (Collection, error) {
	href := CollectionHref(home, p.Address)
	rt := `<D:collection/><C:addressbook/>`
	props := []dav.ColProp{
		{Name: dav.DisplayNameName, Value: name},
		{Name: dav.ResourceTypeName, Value: rt},
	}
	if err := client.MkColProps(ctx, "MKCOL", href, props); err != nil {
		return Collection{}, dav.WrapRequestError("MKCOL", href, err)
	}
	col, err := refreshCollection(ctx, client, href, KindAddressBook)
	if err != nil {
		return col, err
	}
	col.Color = ColorFromAddress(p.Address)
	return col, nil
}

func supportedComponentSetInner(comps []string) string {
	var b strings.Builder
	for _, c := range comps {
		c = strings.TrimSpace(strings.ToUpper(c))
		if c == "" {
			continue
		}
		fmt.Fprintf(&b, `<C:comp xmlns:C="`+dav.NamespaceCalDAV+`" name=%q/>`, c)
	}
	return b.String()
}

// RenameParams change the display name and optional calendar colour (§10.1).
type RenameParams struct {
	DisplayName string
	Color       string // calendars only; empty leaves unchanged
}

// RenameCollection patches displayname and calendar-color on the server.
func RenameCollection(ctx context.Context, client *dav.Client, col Collection, p RenameParams) (Collection, error) {
	name := strings.TrimSpace(p.DisplayName)
	if name == "" {
		return Collection{}, fmt.Errorf("enter a name")
	}
	var set []dav.ColProp
	set = append(set, dav.ColProp{Name: dav.DisplayNameName, Value: name})
	if col.Kind == KindCalendar && strings.TrimSpace(p.Color) != "" {
		set = append(set, dav.ColProp{Name: dav.CalendarColorName, Value: strings.TrimSpace(p.Color)})
	}
	if err := client.PropPatch(ctx, col.Path, set, nil); err != nil {
		return Collection{}, dav.WrapRequestError("PROPPATCH", col.Path, err)
	}
	out, err := refreshCollection(ctx, client, col.Path, col.Kind)
	if err != nil {
		return out, err
	}
	if col.Kind == KindAddressBook {
		out.Color = ColorFromAddress(pathLeaf(col.Path))
	}
	return out, nil
}

// DeleteCollection removes a collection on the server (§10.1).
func DeleteCollection(ctx context.Context, client *dav.Client, colPath string) error {
	if err := client.Delete(ctx, colPath, ""); err != nil {
		return dav.WrapRequestError("DELETE", colPath, err)
	}
	return nil
}

func refreshCollection(ctx context.Context, client *dav.Client, href string, kind Kind) (Collection, error) {
	props := []xml.Name{
		dav.ResourceTypeName,
		dav.DisplayNameName,
		dav.CurrentUserPrivilegeSetName,
		dav.GetCTagName,
	}
	if kind == KindCalendar {
		props = append(props, dav.CalendarColorName, dav.SupportedCalendarComponentSetName)
	}
	ms, err := client.PropFind(ctx, href, dav.DepthZero, props)
	if err != nil {
		return Collection{}, err
	}
	if len(ms.Responses) == 0 {
		return Collection{Path: href, Kind: kind}, nil
	}
	col, ok, err := decodeCollection(ms.Responses[0], kind)
	if err != nil {
		return Collection{}, err
	}
	if !ok {
		return Collection{Path: href, Kind: kind}, nil
	}
	if kind == KindAddressBook && col.Color == "" {
		col.Color = ColorFromAddress(pathLeaf(href))
	}
	return col, nil
}

func pathLeaf(href string) string {
	href = strings.TrimSuffix(normalizePath(href), "/")
	if i := strings.LastIndex(href, "/"); i >= 0 {
		return href[i+1:]
	}
	return href
}

// FindCollection returns one stored collection by path.
func FindCollection(collections []Collection, path string) (Collection, bool) {
	want := normalizePath(path)
	for _, col := range collections {
		if normalizePath(col.Path) == want {
			return col, true
		}
	}
	return Collection{}, false
}

// FormatRequestDiag renders a failed create/rename/delete for the UI (§10.1).
func FormatRequestDiag(err error) string {
	var reqErr *dav.RequestError
	if errors.As(err, &reqErr) {
		var b strings.Builder
		fmt.Fprintf(&b, "[%s]  %s %s → %d\n", strings.ToLower(reqErr.Method), reqErr.Method, reqErr.Path, reqErr.Code)
		if reqErr.Body != "" {
			b.WriteString(reqErr.Body)
		}
		return strings.TrimSpace(b.String())
	}
	if err != nil {
		return err.Error()
	}
	return ""
}
