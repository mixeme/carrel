package discover

import (
	"context"
	"fmt"
	"strings"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/view"
)

const (
	stepWellKnown   = ".well-known"
	stepDirect      = "direct URL"
	stepPrincipal   = "principal"
	stepHomeSet     = "home-set"
	stepCollections = "collections"
)

// Run discovers collections at endpoint (S-6). Trace records every step.
func Run(ctx context.Context, g *dav.Guard, endpoint, username, password string) (*Result, *view.DiagnosticTrace, error) {
	trace := &view.DiagnosticTrace{}
	var steps []view.TraceStep

	add := func(name, detail string, ok bool) {
		st := view.CheckBad
		if ok {
			st = view.CheckOK
		}
		steps = append(steps, view.TraceStep{
			Name:   view.Text(name),
			State:  st,
			Detail: view.Text(detail),
		})
	}

	httpClient := g.HTTPClient()
	baseURL := strings.TrimRight(endpoint, "/") + "/"
	var resolved string
	var detail string
	var err error

	for _, wk := range []string{"/.well-known/caldav", "/.well-known/carddav"} {
		resolved, detail, err = dav.ResolveWellKnown(ctx, httpClient, baseURL, wk)
		if err == nil {
			add(stepWellKnown, detail, true)
			baseURL = resolved
			break
		}
		add(stepWellKnown, detail, false)
	}
	if resolved == "" {
		add(stepDirect, "using entered URL as-is", true)
	} else {
		endpoint = baseURL
	}

	client, err := dav.NewClient(g, endpoint, username, password, "")
	if err != nil {
		trace.Steps = steps
		return nil, trace, err
	}

	principal, perr := client.WebDAV().FindCurrentUserPrincipal(ctx)
	if perr != nil {
		add(stepPrincipal, formatErr(perr), false)
		cols, ferr := fileCollections(ctx, client)
		if ferr != nil {
			trace.Steps = steps
			return nil, trace, ferr
		}
		add(stepCollections, fmt.Sprintf("%d file collection(s)", len(cols)), true)
		trace.Steps = steps
		trace.OK = true
		return &Result{
			BaseURL:     endpoint,
			Collections: cols,
			FileOnly:    true,
		}, trace, nil
	}
	add(stepPrincipal, principal, true)

	var all []Collection
	calHome, err := client.CalDAV().FindCalendarHomeSet(ctx, principal)
	if err != nil {
		add(stepHomeSet, "calendar: "+formatErr(err), false)
	} else {
		add(stepHomeSet, "calendar: "+calHome, true)
		cals, err := client.CalDAV().FindCalendars(ctx, calHome)
		if err != nil {
			add(stepCollections, "calendar: "+formatErr(err), false)
		} else {
			for _, cal := range cals {
				all = append(all, calendarToCollection(cal))
			}
		}
	}

	cardHome, err := client.CardDAV().FindAddressBookHomeSet(ctx, principal)
	if err != nil {
		add(stepHomeSet, "address book: "+formatErr(err), false)
	} else {
		add(stepHomeSet, "address book: "+cardHome, true)
		books, err := client.CardDAV().FindAddressBooks(ctx, cardHome)
		if err != nil {
			add(stepCollections, "address book: "+formatErr(err), false)
		} else {
			for _, ab := range books {
				all = append(all, addressBookToCollection(ab))
			}
		}
	}

	add(stepCollections, fmt.Sprintf("%d collection(s)", len(all)), len(all) > 0)
	trace.Steps = steps
	// OK means the run reached collections, not merely that it did not crash:
	// a principal whose home-sets both failed is a bad check on the screen.
	trace.OK = len(all) > 0
	return &Result{
		BaseURL:     endpoint,
		Principal:   principal,
		Collections: all,
	}, trace, nil
}

func fileCollections(ctx context.Context, client *dav.Client) ([]Collection, error) {
	infos, err := client.WebDAV().ReadDir(ctx, "", false)
	if err != nil {
		return nil, err
	}
	var out []Collection
	for _, fi := range infos {
		if !fi.IsDir || fi.Path == "/" {
			continue
		}
		out = append(out, Collection{
			Path:        fi.Path,
			DisplayName: pathBase(fi.Path),
			Kinds:       []view.Kind{view.KindFile},
		})
	}
	if len(out) == 0 {
		out = []Collection{{
			Path:        "/",
			DisplayName: pathBase("/"),
			Kinds:       []view.Kind{view.KindFile},
		}}
	}
	return out, nil
}

func calendarToCollection(cal caldav.Calendar) Collection {
	kinds := calendarKinds(cal.SupportedComponentSet)
	return Collection{
		Path:        cal.Path,
		DisplayName: cal.Name,
		Kinds:       kinds,
		ReadOnly:    cal.ReadOnly,
		Components:  append([]string(nil), cal.SupportedComponentSet...),
	}
}

// CompareCollections returns the proposed change set for re-discovery (S-6.1).
func CompareCollections(found, stored []Collection) Diff {
	byPath := make(map[string]Collection, len(found))
	for _, c := range found {
		byPath[c.Path] = c
	}
	storedPaths := make(map[string]struct{}, len(stored))
	var d Diff
	for _, s := range stored {
		storedPaths[s.Path] = struct{}{}
		if _, ok := byPath[s.Path]; !ok {
			d.Removed = append(d.Removed, s.Path)
		}
	}
	for _, f := range found {
		if _, ok := storedPaths[f.Path]; !ok {
			d.Added = append(d.Added, f)
		}
	}
	return d
}

func calendarKinds(components []string) []view.Kind {
	seen := make(map[view.Kind]struct{})
	for _, c := range components {
		switch strings.ToUpper(c) {
		case "VEVENT":
			seen[view.KindEvent] = struct{}{}
		case "VTODO":
			seen[view.KindTask] = struct{}{}
		case "VJOURNAL":
			seen[view.KindNote] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return []view.Kind{view.KindEvent, view.KindTask, view.KindNote}
	}
	out := make([]view.Kind, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func addressBookToCollection(ab carddav.AddressBook) Collection {
	return Collection{
		Path:        ab.Path,
		DisplayName: ab.Name,
		Kinds:       []view.Kind{view.KindContact},
		ReadOnly:    ab.ReadOnly,
	}
}

func pathBase(p string) string {
	p = strings.TrimSuffix(p, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func formatErr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
