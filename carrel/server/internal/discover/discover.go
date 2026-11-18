package discover

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"slices"
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
	stepScheduling  = "scheduling"
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

	// Asked of the principal (RFC 6638 §2.1, §2.4.1): a server that schedules
	// has an outbox, and may send letters when an attendee is changed; the
	// address set says which addresses are the person's own (S-15.4).
	propScheduleOutbox = xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "schedule-outbox-URL"}
	propScheduleInbox  = xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "schedule-inbox-URL"}
	propUserAddresses  = xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "calendar-user-address-set"}
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
		cols, ferr := fileCollections(ctx, client, endpoint)
		if ferr != nil {
			trace.Steps = steps
			return nil, trace, ferr
		}
		// Rights too: a storage that says it can only be read — a folder
		// another Carrel published (S-25.1) — is read-only here from the
		// start, not after a refused write.
		cols = withHomeProps(ctx, client, "", "files", cols, add)
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
	var boxes []string
	res.Scheduling, res.Addresses, boxes = scheduling(ctx, client, principal, add)
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

	// The files of the account: one storage, «Files» (s-files-6, s-plan-5).
	// Nextcloud and ownCloud keep them at files/<id>/ beside
	// principals/users/<id>/, Davis at home/<id>/ beside principals/<id>/,
	// and no property of the principal names it; a principal of another
	// shape is not asked (docs/servers.md, SV-65).
	if home, sure := filesHomeOf(principal); home != "" {
		if col, ok := filesHome(ctx, client, home); ok {
			add(stepHomeSet, "files: "+home, true)
			all = append(all, withHomeProps(ctx, client, home, "files", []Collection{col}, add)...)
		} else if !sure {
			// The shape of Davis is Baikal's too, and Baikal keeps no
			// files: no folder there is no fault.
			add(stepHomeSet, "files: none at "+home, true)
		} else {
			add(stepHomeSet, "files: "+home+" is not a folder here", false)
		}
	}

	// OK means the run reached collections or a home for them, not merely
	// that it did not crash: a principal whose home-sets both failed is a bad
	// check on the screen. An account with nothing in it yet is not: its
	// server named where its collections go, and the first one is made
	// there (S-12) — a new account on Radicale is empty (acceptance on our
	// test servers, 18.11.2026).
	// The scheduling boxes are not calendars of the person, whatever their
	// resource type says: DAViCal keeps .in/ and .out/ in the calendar home
	// as plain calendars (acceptance on our test servers, 18.11.2026).
	if len(boxes) > 0 {
		kept := all[:0]
		for _, c := range all {
			if !slices.ContainsFunc(boxes, func(b string) bool { return hrefKey(b) == hrefKey(c.Path) }) {
				kept = append(kept, c)
			}
		}
		if n := len(all) - len(kept); n > 0 {
			add(stepCollections, fmt.Sprintf("%d scheduling box%s (inbox, outbox) left out", n, map[bool]string{true: "", false: "es"}[n == 1]), true)
		}
		all = kept
	}
	homed := res.CalendarHome != "" || res.AddressBookHome != ""
	if len(all) == 0 && homed {
		add(stepCollections, "0 collection(s): none yet — new ones go to the home", true)
	} else {
		add(stepCollections, fmt.Sprintf("%d collection(s)", len(all)), len(all) > 0)
	}
	trace.Steps = steps
	trace.OK = len(all) > 0 || homed
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

// scheduling asks the principal, in one depth-0 PROPFIND, whether it has a
// schedule-outbox-URL and which calendar addresses are the person's, and
// where its scheduling boxes are (boxes: the inbox and the outbox). A
// refusal is not a failure of the run: nothing else depends on it, and the
// answer stays unknown — which the rewrite sheet says as it is (S-15.4).
func scheduling(ctx context.Context, client *dav.Client, principal string, add func(string, string, bool)) (string, []string, []string) {
	ms, err := client.PropFind(ctx, principal, dav.DepthZero, []xml.Name{propScheduleOutbox, propScheduleInbox, propUserAddresses})
	if err != nil {
		add(stepScheduling, "not answered: "+formatErr(err), false)
		return SchedulingUnknown, nil, nil
	}
	var r *dav.Response
	for i := range ms.Responses {
		if hrefKey(ms.Responses[i].Href) == hrefKey(principal) || len(ms.Responses) == 1 {
			r = &ms.Responses[i]
		}
	}
	if r == nil {
		add(stepScheduling, "not answered: the principal is not in the reply", false)
		return SchedulingUnknown, nil, nil
	}
	var addresses []string
	for _, h := range hrefs(r.Props[propUserAddresses]) {
		if len(h) > len("mailto:") && strings.EqualFold(h[:len("mailto:")], "mailto:") {
			addresses = append(addresses, strings.ToLower(strings.TrimSpace(h[len("mailto:"):])))
		}
	}
	boxes := append(hrefs(r.Props[propScheduleInbox]), hrefs(r.Props[propScheduleOutbox])...)
	if len(hrefs(r.Props[propScheduleOutbox])) == 0 {
		add(stepScheduling, "no schedule-outbox-URL: the server does not schedule", true)
		return SchedulingNo, addresses, boxes
	}
	add(stepScheduling, fmt.Sprintf("schedule-outbox-URL announced; %d own address(es)", len(addresses)), true)
	return SchedulingYes, addresses, boxes
}

// hrefs reads the DAV:href values out of a property's inner XML, by local
// name: every server picks its own prefixes.
func hrefs(inner []byte) []string {
	if len(strings.TrimSpace(string(inner))) == 0 {
		return nil
	}
	dec := xml.NewDecoder(strings.NewReader("<set>" + string(inner) + "</set>"))
	dec.Strict = false
	var out []string
	in := false
	var cur strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch el := tok.(type) {
		case xml.StartElement:
			if el.Name.Local == "href" {
				in = true
				cur.Reset()
			}
		case xml.CharData:
			if in {
				cur.Write(el)
			}
		case xml.EndElement:
			if el.Name.Local == "href" && in {
				in = false
				if v := strings.TrimSpace(cur.String()); v != "" {
					out = append(out, v)
				}
			}
		}
	}
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
	col.Kinds = CalendarKinds(col.Components)
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

// nextcloudPrincipal is the principal of a user on Nextcloud and ownCloud:
// …/principals/users/<id>/, its files at …/files/<id>/. davisPrincipal is
// Davis's, …/principals/<id>/, its files at …/home/<id>/ (WEBDAV_ENABLED) —
// and Baikal's, which keeps none.
var (
	nextcloudPrincipal = regexp.MustCompile(`^(.*)/principals/users/([^/]+)/?$`)
	davisPrincipal     = regexp.MustCompile(`^(.*)/principals/([^/]+)/?$`)
)

// filesHomeOf is where a principal of those shapes keeps its files; "" for
// any other. sure: the server of that shape always has them (Nextcloud).
func filesHomeOf(principal string) (home string, sure bool) {
	if m := nextcloudPrincipal.FindStringSubmatch(principal); m != nil {
		return m[1] + "/files/" + m[2] + "/", true
	}
	if m := davisPrincipal.FindStringSubmatch(principal); m != nil {
		return m[1] + "/home/" + m[2] + "/", false
	}
	return "", false
}

// filesHome is the storage at home when the server has a folder there —
// a collection with neither the calendar nor the address-book mark —, named
// «Files» as the frames name it (s-files-6), not by the user id Nextcloud
// gives it.
func filesHome(ctx context.Context, client *dav.Client, home string) (Collection, bool) {
	ms, err := client.PropFind(ctx, home, dav.DepthZero, []xml.Name{propResourceType})
	if err != nil || len(ms.Responses) == 0 {
		return Collection{}, false
	}
	inner := string(ms.Responses[0].Props[propResourceType])
	if !strings.Contains(inner, "collection") || resourceKind([]byte(inner)) != "" {
		return Collection{}, false
	}
	return Collection{Path: home, DisplayName: "Files", Kinds: []view.Kind{view.KindFile}}, true
}

// fileCollections are the folders at the entered address. A PROPFIND of
// depth 1 answers the address itself too; that answer is the storage, not a
// folder in it — at the server's root and at an address with a path alike
// (SFTPGo under /dav/, Nextcloud under /remote.php/dav/files/<login>/). With
// no folder in it, the address itself is the one collection.
func fileCollections(ctx context.Context, client *dav.Client, endpoint string) ([]Collection, error) {
	infos, err := client.WebDAV().ReadDir(ctx, "", false)
	if err != nil {
		return nil, err
	}
	self := hrefKey(endpoint)
	var out []Collection
	for _, fi := range infos {
		if !fi.IsDir || fi.Path == "/" || hrefKey(fi.Path) == self {
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
			Path:        self + "/",
			DisplayName: pathBase(self + "/"),
			Kinds:       []view.Kind{view.KindFile},
		}}
	}
	return out, nil
}

func calendarToCollection(cal caldav.Calendar) Collection {
	kinds := CalendarKinds(cal.SupportedComponentSet)
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

// CalendarKinds keeps the order event, task, note so the catalog does not
// shuffle between runs.
func CalendarKinds(components []string) []view.Kind {
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

// Refused says whether discovery ended on the server refusing the login and
// password: a 401. The steps that find the principal and the homes go through
// go-webdav, whose error type is internal to it — what can be read of it is
// its text, which begins with the status line («401 Unauthorized: …»);
// Carrel's own requests carry the status itself. A change of that text turns
// TestRediscoveryFailureChangesNothing red.
func Refused(err error) bool {
	if err == nil {
		return false
	}
	return dav.IsStatus(err, 401) || strings.HasPrefix(err.Error(), "401 ")
}
