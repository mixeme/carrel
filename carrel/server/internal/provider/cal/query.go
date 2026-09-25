package cal

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
)

var (
	// ErrNoReporter: the connection cannot send a CalDAV REPORT. The source is
	// an error on the screen (S-21), never a silent PROPFIND of everything.
	ErrNoReporter = errors.New("cal: the connection offers no CalDAV REPORT")
	// ErrNotFound: no record with this UID in the collection.
	ErrNotFound = errors.New("cal: no record with this UID")
	// ErrLatinCaseOnly: the server refused to compare in every script, and its
	// default ignores letter case in Latin only (i;ascii-casemap). The records
	// it found come with this error.
	ErrLatinCaseOnly = errors.New("cal: the server ignores letter case in Latin script only")
	// ErrServerDoesNotFilter: the server's text-match cannot be taken at its
	// word, and Carrel did the searching. Two ways to get here. The server
	// answered with records that do not hold the text — it returned the
	// collection and left the searching to whoever asked (docs/servers.md,
	// SV-16: SoGo). Or it answered with nothing while the collection holds
	// records that do — its filter matches almost nothing (SV-47: Xandikos).
	// Either way the records came from Carrel's own sifting, and the screen
	// says so beside them.
	ErrServerDoesNotFilter = errors.New("cal: the server's text-match cannot be trusted, so Carrel searched")
)

// Unreadable is a resource the REPORT returned that could not become a record.
// A collection with one broken object still lists the rest and names the one
// it skipped; it does not go blank.
type Unreadable struct {
	Path string
	UID  string
	Err  error
}

type calReporting interface {
	CalReporter() dav.CalReporter
}

// Query reads the records of one component with a single REPORT. The bodies
// come back in that reply and are not fetched again (S-11). A nil window reads
// the whole collection; the agenda always passes one.
func (p *Provider) Query(ctx context.Context, collection, component string, from, to *time.Time) ([]*model.Object, []Unreadable, error) {
	filter := caldav.CompFilter{Name: component}
	if from != nil {
		filter.Start = from.UTC()
	}
	if to != nil {
		filter.End = to.UTC()
	}
	return p.query(ctx, collection, component, filter)
}

// searchProps are the properties a search looks in, per main component: what
// the lists and the records show.
var searchProps = map[string][]string{
	ical.CompEvent:   {ical.PropSummary, ical.PropLocation, ical.PropDescription},
	ical.CompToDo:    {ical.PropSummary, ical.PropDescription},
	ical.CompJournal: {ical.PropSummary, ical.PropDescription},
}

// Search reads the records of one component that hold text in a shown
// property (S-15.2). Prop-filters inside one comp-filter must all match (RFC
// 4791 §9.7.1), so "any of them" is one REPORT per property; a record found
// by two of them is kept once. The match is the server's, in i;unicode-casemap:
// letter case does not count in any script. A server that refuses that
// collation is asked again with its default, and what it found comes with
// ErrLatinCaseOnly (docs/servers.md, SV-2).
func (p *Provider) Search(ctx context.Context, collection, component, text string) ([]*model.Object, []Unreadable, error) {
	props, ok := searchProps[component]
	if !ok {
		props = []string{ical.PropSummary}
	}
	searcher := p.searcher()
	latinOnly := false
	seen := map[string]bool{}
	var objs []*model.Object
	var bad []Unreadable
	for _, prop := range props {
		var found []*model.Object
		var unreadable []Unreadable
		var err error
		if searcher != nil && !latinOnly {
			var cos []caldav.CalendarObject
			cos, err = searcher.SearchCalendar(ctx, collection, dav.CalSearch{
				Component: component, Prop: prop, Text: text, Collation: dav.UnicodeCaseMap,
			})
			found, unreadable = fromReports(cos, component)
			latinOnly = refusedCollation(err)
		}
		if searcher == nil || latinOnly {
			filter := caldav.CompFilter{
				Name:  component,
				Props: []caldav.PropFilter{{Name: prop, TextMatch: &caldav.TextMatch{Text: text}}},
			}
			found, unreadable, err = p.query(ctx, collection, component, filter)
		}
		if err != nil {
			return nil, nil, err
		}
		for _, o := range found {
			if !seen[o.Path] {
				seen[o.Path] = true
				objs = append(objs, o)
			}
		}
		for _, u := range unreadable {
			if !seen[u.Path] {
				seen[u.Path] = true
				bad = append(bad, u)
			}
		}
	}
	// Nothing at all is the other way a text-match can fail, and the worse
	// one, because it looks like an answer. Xandikos matches only a whole
	// value, and only one without a space in it — «Two words» does not find
	// «Two words» — and says 207 with nothing, the same as an honest miss
	// (docs/servers.md, SV-47). So an empty answer is checked against the
	// collection itself, sifted here. A server that searched well loses
	// nothing by it but one read, and says nothing more: the note goes on the
	// screen only when the sifting found what the server did not.
	// The cost is a full read of the component for every search that finds
	// nothing. ReadWhole is how a caller pays it once: the web layer keeps the
	// read under the collection's change tag (S-4.2), and the next search of
	// an unchanged collection is sifted from it without asking at all.
	if len(objs) == 0 {
		if all, err := p.readWhole(ctx, collection, component); err == nil {
			if kept, _ := sift(all, props, text); len(kept) > 0 {
				return kept, bad, ErrServerDoesNotFilter
			}
		}
	}
	// A server that answers a text-match with the whole collection has not
	// searched at all (docs/servers.md, SV-16: SoGo). Sifting what came back
	// through the same properties and the same term keeps the promise of S-15;
	// on a server that filtered, nothing is taken away by this.
	if kept, sifted := sift(objs, props, text); sifted {
		if latinOnly {
			return kept, bad, ErrLatinCaseOnly
		}
		return kept, bad, ErrServerDoesNotFilter
	}
	if latinOnly {
		return objs, bad, ErrLatinCaseOnly
	}
	return objs, bad, nil
}

// readWhole reads a component whole: through ReadWhole when the caller gave
// one, from the server otherwise.
func (p *Provider) readWhole(ctx context.Context, collection, component string) ([]*model.Object, error) {
	if p.ReadWhole != nil {
		return p.ReadWhole(ctx, collection, component)
	}
	all, _, err := p.query(ctx, collection, component, caldav.CompFilter{Name: component})
	return all, err
}

// Sift keeps the records of a component that a search for text finds: the
// properties Search asks the server about, compared the same way. It is the
// search of records already read whole (S-4.2).
func Sift(objs []*model.Object, component, text string) []*model.Object {
	props, ok := searchProps[component]
	if !ok {
		props = []string{ical.PropSummary}
	}
	kept, _ := sift(objs, props, text)
	return kept
}

// sift keeps the records that hold text in one of props, and says whether it
// had to take anything away. The comparison is the one asked of the server:
// letter case does not count, in any script.
func sift(objs []*model.Object, props []string, text string) ([]*model.Object, bool) {
	needle := strings.ToLower(text)
	kept := make([]*model.Object, 0, len(objs))
	for _, o := range objs {
		for _, prop := range props {
			if strings.Contains(strings.ToLower(Text(o, prop)), needle) {
				kept = append(kept, o)
				break
			}
		}
	}
	return kept, len(kept) != len(objs)
}

// Count is how many records of a component the calendar holds, read without
// their bodies (dav.CalCounter): the rail's number of a calendar (C-6.3).
func (p *Provider) Count(ctx context.Context, collection, component string) (int, error) {
	rs, ok := p.t.(calReporting)
	if !ok {
		return 0, ErrNoReporter
	}
	c, ok := rs.CalReporter().(dav.CalCounter)
	if !ok {
		return 0, ErrNoReporter
	}
	return c.CountCalendar(ctx, collection, component)
}

func (p *Provider) searcher() dav.CalSearcher {
	rs, ok := p.t.(calReporting)
	if !ok {
		return nil
	}
	s, _ := rs.CalReporter().(dav.CalSearcher)
	return s
}

// refusedCollation reports a refusal a server gives a collation it does not
// know: sabre/dav answers 400, RFC 4791 names the precondition
// supported-collation.
func refusedCollation(err error) bool {
	for _, code := range []int{400, 403, 412, 415, 422, 501} {
		if dav.IsStatus(err, code) {
			return true
		}
	}
	return false
}

// FindUID reads the record with this UID. CalDAV text-match is a substring
// test, so the exact UID is checked here.
func (p *Provider) FindUID(ctx context.Context, collection, component, uid string) (*model.Object, error) {
	filter := caldav.CompFilter{
		Name:  component,
		Props: []caldav.PropFilter{{Name: ical.PropUID, TextMatch: &caldav.TextMatch{Text: uid}}},
	}
	objs, bad, err := p.query(ctx, collection, component, filter)
	if err != nil {
		return nil, err
	}
	for _, o := range objs {
		if UID(o) == uid {
			return o, nil
		}
	}
	for _, u := range bad {
		if u.UID == uid {
			return nil, u.Err
		}
	}
	return nil, ErrNotFound
}

func (p *Provider) query(ctx context.Context, collection, component string, filter caldav.CompFilter) ([]*model.Object, []Unreadable, error) {
	rs, ok := p.t.(calReporting)
	if !ok {
		return nil, nil, ErrNoReporter
	}
	q := &caldav.CalendarQuery{
		CompFilter: caldav.CompFilter{Name: ical.CompCalendar, Comps: []caldav.CompFilter{filter}},
	}
	cos, err := rs.CalReporter().QueryCalendar(ctx, collection, q)
	// A collection read whole whose answer is larger than one answer may be
	// (CARREL_DAV_MAX_RESPONSE_BYTES) is read in parts: its listing, then
	// multigets. Only a whole read goes this way — a window or a text-match
	// is the server's to apply, and its answer is small anyway.
	if errors.Is(err, dav.ErrResponseTooLarge) && filter.Start.IsZero() && filter.End.IsZero() && len(filter.Props) == 0 {
		if f, ok := rs.CalReporter().(dav.CalFetcher); ok {
			cos, err = f.FetchCalendar(ctx, collection)
		}
	}
	if err != nil {
		return nil, nil, err
	}
	objs, bad := fromReports(cos, component)
	return objs, bad, nil
}

// fromReports turns REPORT entries into records of one component.
func fromReports(cos []caldav.CalendarObject, component string) ([]*model.Object, []Unreadable) {
	var objs []*model.Object
	var bad []Unreadable
	for _, co := range cos {
		obj, err := fromReport(co)
		if err != nil {
			bad = append(bad, Unreadable{Path: co.Path, UID: reportUID(co), Err: err})
			continue
		}
		if obj.MainComponent() != component {
			continue
		}
		objs = append(objs, obj)
	}
	return objs, bad
}

// fromReport turns a REPORT entry into a record. go-webdav hands the body over
// decoded, so it is encoded once to become the document the record keeps; the
// encoder refuses what RFC 5545 forbids (a VEVENT without DTSTAMP), and that
// refusal is what makes the entry unreadable.
func fromReport(co caldav.CalendarObject) (*model.Object, error) {
	if co.Data == nil {
		return nil, errors.New("the reply carried no calendar data")
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(co.Data); err != nil {
		return nil, err
	}
	obj, err := Parse(co.Path, strings.Trim(co.ETag, `"`), &buf)
	if err != nil {
		return nil, err
	}
	obj.Modified = co.ModTime
	return obj, nil
}

func reportUID(co caldav.CalendarObject) string {
	if co.Data == nil {
		return ""
	}
	for _, child := range co.Data.Children {
		if p := child.Props.Get(ical.PropUID); p != nil {
			return p.Value
		}
	}
	return ""
}

// NewObject builds the skeleton of a new record: VCALENDAR with VERSION and
// PRODID, one main component with UID and DTSTAMP. Everything else arrives
// through a patch, the same way an edit does — a form never builds a record.
func NewObject(component, uid string, now time.Time) (*model.Object, error) {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//Carrel//Carrel//EN")
	main := ical.NewComponent(component)
	main.Props.SetText(ical.PropUID, uid)
	main.Props.SetDateTime(ical.PropDateTimeStamp, now.UTC())
	cal.Children = append(cal.Children, main)
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, err
	}
	return Parse("", "", &buf)
}

// Delete removes a record with If-Match (S-8). A delete without a version is
// refused, like an update without one.
func (p *Provider) Delete(ctx context.Context, path, etag string) error {
	if etag == "" {
		return ErrEmptyETag
	}
	return p.t.Delete(ctx, path, `"`+strings.Trim(etag, `"`)+`"`)
}
