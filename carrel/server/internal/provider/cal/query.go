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
	if latinOnly {
		return objs, bad, ErrLatinCaseOnly
	}
	return objs, bad, nil
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
