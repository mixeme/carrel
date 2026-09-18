package discover

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
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
	stepProperties  = "properties"
)

// Properties go-webdav does not read: colour, the change tag, and — for address
// books, whose FindAddressBooks never asks — the current user's rights (S-6).
var (
	propCalendarColor = xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	propGetCTag       = xml.Name{Space: "http://calendarserver.org/ns/", Local: "getctag"}
	propSyncToken     = xml.Name{Space: "DAV:", Local: "sync-token"}
	propPrivileges    = xml.Name{Space: "DAV:", Local: "current-user-privilege-set"}

	// A server with a trash bin keeps a deleted calendar at its address, among
	// the children of the home, and calls it a calendar to any request that
	// does not ask about the deletion: Nextcloud answers its own deleted-at
	// only when asked (docs/servers.md, SV-30). Asking costs nothing — it
	// rides in the PROPFIND that already reads colour and the change tag — and
	// without it a collection the person deleted comes back at the next
	// discovery (S-6).
	propDeletedAt = xml.Name{Space: "http://nextcloud.com/ns", Local: "deleted-at"}

	// Asked of the entered address itself when there is no principal: the
	// handed-over link may be one collection rather than a home (S-6).
	propResourceType = xml.Name{Space: "DAV:", Local: "resourcetype"}
	propDisplayName  = xml.Name{Space: "DAV:", Local: "displayname"}
	propCompSet      = xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "supported-calendar-component-set"}
)

// Run discovers collections at endpoint (S-6). Trace records every step.
// certPin is the accepted leaf fingerprint of the connection (F-13), empty for
// standard verification only; it applies to every request of the run.
func Run(ctx context.Context, g *dav.Guard, endpoint, username, password, certPin string) (*Result, *view.DiagnosticTrace, error) {
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

	httpClient := g.HTTPClientWithCertPin(certPin)
	entered := strings.TrimRight(endpoint, "/") + "/"
	var wellKnownTarget string

	for _, wk := range []string{"/.well-known/caldav", "/.well-known/carddav"} {
		candidate, detail, err := dav.ResolveWellKnown(ctx, httpClient, entered, wk)
		if err != nil {
			add(stepWellKnown, detail, false)
			continue
		}
		ok, note := acceptWellKnownTarget(ctx, g, entered, candidate, username, password, certPin)
		if ok {
			add(stepWellKnown, detail, true)
			wellKnownTarget = candidate
			break
		}
		add(stepWellKnown, detail+"; "+note, false)
	}
	if wellKnownTarget == "" {
		add(stepDirect, "using entered URL as-is", true)
	} else {
		endpoint = wellKnownTarget
	}

	client, err := dav.NewClient(g, endpoint, username, password, certPin)
	if err != nil {
		trace.Steps = steps
		return nil, trace, err
	}

	principal, perr := client.WebDAV().FindCurrentUserPrincipal(ctx)
	if perr != nil {
		add(stepPrincipal, formatErr(perr), false)
		// The link may be one collection rather than a home: Mail.ru hands out
		// a calendar's own address, and such an address names no principal
		// (S-6). A file collection is one whose resource type carries neither
		// the calendar nor the address-book mark, so the mark is what decides.
		if col, ok := addressIsCollection(ctx, client, endpoint); ok {
			add(stepCollections, "the entered address is itself "+collectionNoun(col), true)
			trace.Steps = steps
			trace.OK = true
			return &Result{BaseURL: endpoint, Collections: []Collection{col}}, trace, nil
		}
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

	res := &Result{BaseURL: endpoint, Principal: principal}
	var all []Collection
	calHome, err := client.CalDAV().FindCalendarHomeSet(ctx, principal)
	if err != nil {
		add(stepHomeSet, "calendar: "+formatErr(err), false)
	} else {
		add(stepHomeSet, "calendar: "+calHome, true)
		res.CalendarHome = calHome
		cals, err := client.CalDAV().FindCalendars(ctx, calHome)
		if err != nil {
			add(stepCollections, "calendar: "+formatErr(err), false)
		} else {
			var found []Collection
			for _, cal := range cals {
				found = append(found, calendarToCollection(cal))
			}
			all = append(all, named(withHomeProps(ctx, client, calHome, "calendar", found, add))...)
		}
	}

	cardHome, err := client.CardDAV().FindAddressBookHomeSet(ctx, principal)
	if err != nil {
		add(stepHomeSet, "address book: "+formatErr(err), false)
	} else {
		add(stepHomeSet, "address book: "+cardHome, true)
		res.AddressBookHome = cardHome
		books, err := client.CardDAV().FindAddressBooks(ctx, cardHome)
		if err != nil {
			add(stepCollections, "address book: "+formatErr(err), false)
		} else {
			var found []Collection
			for _, ab := range books {
				found = append(found, addressBookToCollection(ab))
			}
			all = append(all, named(withHomeProps(ctx, client, cardHome, "address book", found, add))...)
		}
	}

	add(stepCollections, fmt.Sprintf("%d collection(s)", len(all)), len(all) > 0)
	trace.Steps = steps
	// OK means the run reached collections, not merely that it did not crash:
	// a principal whose home-sets both failed is a bad check on the screen.
	trace.OK = len(all) > 0
	res.Collections = all
	return res, trace, nil
}

// withHomeProps reads colour and change tag for every collection of one home
// in a single depth-1 PROPFIND. A refusal leaves the collections as found:
// C-10 derives a colour from the path, and a missing tag only means the next
// read cannot be skipped. The refusal is still written on the trace.
func withHomeProps(ctx context.Context, client *dav.Client, home, label string, cols []Collection, add func(string, string, bool)) []Collection {
	if len(cols) == 0 {
		return cols
	}
	ms, err := client.PropFind(ctx, home, dav.DepthOne,
		[]xml.Name{propCalendarColor, propGetCTag, propSyncToken, propPrivileges, propDeletedAt})
	if err != nil {
		add(stepProperties, label+": "+formatErr(err), false)
		return cols
	}
	byPath := make(map[string]dav.Response, len(ms.Responses))
	for _, r := range ms.Responses {
		byPath[hrefKey(r.Href)] = r
	}
	var coloured, deleted int
	kept := cols[:0]
	for i := range cols {
		r, ok := byPath[hrefKey(cols[i].Path)]
		if !ok {
			kept = append(kept, cols[i])
			continue
		}
		if strings.TrimSpace(string(r.Props[propDeletedAt])) != "" {
			// In the server's own trash bin: not a collection of this account
			// any more, whatever its resource type still says (SV-30).
			deleted++
			continue
		}
		if c := strings.TrimSpace(string(r.Props[propCalendarColor])); c != "" {
			cols[i].Colour = c
			coloured++
		}
		tag := strings.TrimSpace(string(r.Props[propGetCTag]))
		if tag == "" {
			tag = strings.TrimSpace(string(r.Props[propSyncToken]))
		}
		cols[i].ChangeTag = tag
		if canWrite, known := writable(r.Props[propPrivileges]); known {
			cols[i].ReadOnly = !canWrite
		}
		kept = append(kept, cols[i])
	}
	note := fmt.Sprintf("%s: %d of %d with colour", label, coloured, len(cols))
	if deleted > 0 {
		note += fmt.Sprintf("; %d in the server's trash bin, left out", deleted)
	}
	add(stepProperties, note, true)
	return kept
}

// writable reads a current-user-privilege-set. Known is false when the server
// did not send one: rights are then left as go-webdav found them, not guessed.
func writable(inner []byte) (canWrite, known bool) {
	if len(strings.TrimSpace(string(inner))) == 0 {
		return false, false
	}
	// The inner XML keeps prefixes declared on the root; the decoder is
	// lenient about them, and only local names matter here.
	dec := xml.NewDecoder(strings.NewReader("<set>" + string(inner) + "</set>"))
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			return canWrite, true
		}
		if el, ok := tok.(xml.StartElement); ok {
			switch el.Name.Local {
			case "write", "write-content", "bind", "all":
				canWrite = true
			}
		}
	}
}

// hrefKey reduces an href to a comparable path: servers answer with absolute
// URLs or paths, percent-encoded or not, with or without the trailing slash.
// named gives a name to a collection whose server sent none. iCloud sends no
// displayname at all for the only address book it has (docs/servers.md,
// SV-29), and the last segment of that address reads "card" to the person
// whose contacts are in it. What the collection is, is an honest name for it:
// it is an address book. Several nameless ones of the same kind in one home
// are told apart by that last segment — two rows reading the same would be
// worse than a technical word (owner's decision, 18.09.2026).
func named(cols []Collection) []Collection {
	nameless := map[string]int{}
	for _, c := range cols {
		if strings.TrimSpace(c.DisplayName) == "" {
			nameless[kindNoun(c)]++
		}
	}
	for i := range cols {
		if strings.TrimSpace(cols[i].DisplayName) != "" {
			continue
		}
		noun := kindNoun(cols[i])
		cols[i].DisplayName = noun
		if nameless[noun] > 1 {
			cols[i].DisplayName = noun + " (" + lastSegment(cols[i].Path) + ")"
		}
	}
	return cols
}

// kindNoun is what a collection is, for a server that never said what it is
// called: a collection of contacts is an address book, anything else here is a
// calendar, whatever components it holds.
func kindNoun(c Collection) string {
	for _, k := range c.Kinds {
		if k == view.KindContact {
			return "Address book"
		}
	}
	return "Calendar"
}

// lastSegment is the final name in an address, which is what tells two
// otherwise identical collections apart.
func lastSegment(path string) string {
	p := strings.TrimSuffix(path, "/")
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func hrefKey(href string) string {
	p := href
	if u, err := url.Parse(href); err == nil {
		p = u.Path
	}
	return strings.TrimSuffix(p, "/")
}

// addressIsCollection asks the entered address what it is. A server that names
// no principal may still have handed over one collection — Mail.ru gives out a
// calendar's own address — and S-6 tells a file collection apart by exactly
// this: a resource type carrying neither the calendar nor the address-book
// mark. A refusal here is not a failure of the run: the caller falls back to
// file collections, which is what happened before this existed.
func addressIsCollection(ctx context.Context, client *dav.Client, endpoint string) (Collection, bool) {
	ms, err := client.PropFind(ctx, "", dav.DepthZero, []xml.Name{propResourceType, propDisplayName, propCompSet})
	if err != nil || len(ms.Responses) == 0 {
		return Collection{}, false
	}
	r := ms.Responses[0]
	kind := resourceKind(r.Props[propResourceType])
	if kind == "" {
		return Collection{}, false
	}
	path := hrefKey(r.Href) + "/"
	if p := hrefKey(endpoint); path == "/" && p != "" {
		path = p + "/"
	}
	col := Collection{
		Path:        path,
		DisplayName: strings.TrimSpace(string(r.Props[propDisplayName])),
		ChangeTag:   "",
	}
	if col.DisplayName == "" {
		col.DisplayName = pathBase(path)
	}
	if kind == "addressbook" {
		col.Kinds = []view.Kind{view.KindContact}
		return col, true
	}
	col.Components = componentNames(r.Props[propCompSet])
	col.Kinds = calendarKinds(col.Components)
	if len(col.Kinds) == 0 {
		// A calendar that named no components holds events until it says
		// otherwise: RFC 4791 §5.2.3 makes the set optional, not empty.
		col.Kinds = []view.Kind{view.KindEvent}
	}
	return col, true
}

// resourceKind is "calendar", "addressbook" or "" — read from the inner XML by
// local name, because the prefixes are declared on the root of the answer and
// every server picks its own.
func resourceKind(inner []byte) string {
	if len(strings.TrimSpace(string(inner))) == 0 {
		return ""
	}
	dec := xml.NewDecoder(strings.NewReader("<set>" + string(inner) + "</set>"))
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		if el, ok := tok.(xml.StartElement); ok {
			switch el.Name.Local {
			case "calendar":
				return "calendar"
			case "addressbook":
				return "addressbook"
			}
		}
	}
}

// componentNames reads the comp names out of supported-calendar-component-set.
func componentNames(inner []byte) []string {
	if len(strings.TrimSpace(string(inner))) == 0 {
		return nil
	}
	dec := xml.NewDecoder(strings.NewReader("<set>" + string(inner) + "</set>"))
	dec.Strict = false
	var out []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		el, ok := tok.(xml.StartElement)
		if !ok || el.Name.Local != "comp" {
			continue
		}
		for _, a := range el.Attr {
			if strings.EqualFold(a.Name.Local, "name") && strings.TrimSpace(a.Value) != "" {
				out = append(out, strings.TrimSpace(a.Value))
			}
		}
	}
}

func collectionNoun(col Collection) string {
	for _, k := range col.Kinds {
		if k == view.KindContact {
			return "an address book"
		}
	}
	return "a calendar"
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
		Path:            cal.Path,
		DisplayName:     cal.Name,
		Kinds:           kinds,
		ReadOnly:        cal.ReadOnly,
		Components:      append([]string(nil), cal.SupportedComponentSet...),
		MaxResourceSize: cal.MaxResourceSize,
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

// calendarKinds keeps the order event, task, note so the catalog does not
// shuffle between runs.
func calendarKinds(components []string) []view.Kind {
	seen := make(map[view.Kind]bool)
	for _, c := range components {
		switch strings.ToUpper(c) {
		case "VEVENT":
			seen[view.KindEvent] = true
		case "VTODO":
			seen[view.KindTask] = true
		case "VJOURNAL":
			seen[view.KindNote] = true
		}
	}
	if len(seen) == 0 {
		return []view.Kind{view.KindEvent, view.KindTask, view.KindNote}
	}
	var out []view.Kind
	for _, k := range []view.Kind{view.KindEvent, view.KindTask, view.KindNote} {
		if seen[k] {
			out = append(out, k)
		}
	}
	return out
}

func addressBookToCollection(ab carddav.AddressBook) Collection {
	return Collection{
		Path:            ab.Path,
		DisplayName:     ab.Name,
		Kinds:           []view.Kind{view.KindContact},
		ReadOnly:        ab.ReadOnly,
		MaxResourceSize: ab.MaxResourceSize,
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

// acceptWellKnownTarget gates a well-known redirect before discovery switches
// to the candidate. HTTPS→HTTP is refused here and again in Guard.CheckRedirect
// so credentials are not sent to a downgraded scheme (S-6).
func acceptWellKnownTarget(ctx context.Context, g *dav.Guard, entered, candidate, username, password, certPin string) (bool, string) {
	enteredURL, err := url.Parse(entered)
	if err != nil {
		return false, formatErr(err)
	}
	candidateURL, err := url.Parse(candidate)
	if err != nil {
		return false, formatErr(err)
	}
	if strings.EqualFold(enteredURL.Scheme, "https") && strings.EqualFold(candidateURL.Scheme, "http") {
		return false, "redirect downgrades HTTPS to HTTP"
	}
	client, err := dav.NewClient(g, candidate, username, password, certPin)
	if err != nil {
		return false, formatErr(err)
	}
	principal, err := client.WebDAV().FindCurrentUserPrincipal(ctx)
	if err != nil {
		return false, formatErr(err)
	}
	if principal == "" {
		return false, "no current-user-principal"
	}
	return true, principal
}
