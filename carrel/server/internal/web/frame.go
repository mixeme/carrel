package web

import (
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/view"
)

// The frame around a list and a record (C-6.3): the back rail of a record
// screen, the category filter of a list, and the print header (C-10, U-10).

// backAction is the one row of a back rail. An empty label is the way back to
// the list of the kind: «Back to list» from every card, an event's too
// (owner's decision 24.09.2026, s-agenda-2).
func (s *Server) backAction(spec kindSpec, label, href string) view.Action {
	if label == "" {
		label = "Back to list"
	}
	return view.Action{ID: "nav.back", Label: text(label), Icon: "i-back", Href: s.route(href), Method: "GET", Enabled: true}
}

func (s *Server) listBack(c *collectionCtx) view.Action {
	return s.backAction(c.spec, "", c.listPath())
}

func (s *Server) cardBack(c *collectionCtx, uid string) view.Action {
	return s.backAction(c.spec, "Back to the card", c.recordPath(uid))
}

// recordRail is built after the screen's own reads and writes: they are what
// fills its counts.
func (s *Server) recordRail(c *collectionCtx, back view.Action) view.Rail {
	rail := s.backRail(c.sess, s.catalogOf(c.sess), c.spec, c.conn.ID, back)
	markOpen(rail.Sections, s.route(c.listPath()))
	return rail
}

// neighbourSpan is how many notes the rail of a note's page shows on each
// side of the open one: the whole notebook (the owner's decision of
// 24.11.2026 — one on each side showed «то 3 заметки, то 2»), a window only
// for a very large one. The frames draw one on a side (Typecase's task 289).
const neighbourSpan = 20

// noteNeighbours are the notes around uid in its notebook, in the order the
// notebook's list shows by default — newest first, then by title — with the
// open one on. They come from the list this session loaded: a notebook never
// listed, or a note not in its list, has none, and nothing is asked for them.
func (s *Server) noteNeighbours(c *collectionCtx, uid string) []view.Action {
	var notes []indexEntry
	for _, e := range c.index() {
		if e.Kind == view.KindNote {
			notes = append(notes, e)
		}
	}
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].At != notes[j].At {
			return notes[i].At > notes[j].At
		}
		return notes[i].Title < notes[j].Title
	})
	at := slices.IndexFunc(notes, func(e indexEntry) bool { return e.UID == uid })
	if at < 0 {
		return nil
	}
	var out []view.Action
	for _, e := range notes[max(0, at-neighbourSpan):min(len(notes), at+neighbourSpan+1)] {
		out = append(out, view.Action{ID: "nav.neighbour", Label: text(e.Title), Icon: "i-note",
			Href: s.route(c.recordPath(e.UID)), Method: "GET", Enabled: true, On: e.UID == uid})
	}
	return out
}

// categoriesOf are the categories of a record: CATEGORIES split on its commas.
func categoriesOf(k view.Kind, obj *model.Object) []string {
	if k != view.KindContact {
		return cal.TextList(obj, "CATEGORIES")
	}
	var out []string
	for _, v := range obj.PropText("CATEGORIES") {
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				out = append(out, t)
			}
		}
	}
	return out
}

// tagFilter is the category filter of one list address: ?tag= names the
// category shown, compared without case. It cuts what the list already read
// and asks no server. Every link it makes keeps the other parameters of the
// address: the week, the task segment, the task of a combined view.
type tagFilter struct {
	tag   string
	base  string
	query url.Values
	route func(string) string
}

func (s *Server) tagFilter(base string, query url.Values) tagFilter {
	f := tagFilter{tag: strings.TrimSpace(query.Get("tag")), base: base, query: url.Values{}, route: s.route}
	for k, v := range query {
		if k != "tag" {
			f.query[k] = v
		}
	}
	return f
}

// href is the list filtered to tag; an empty tag is the whole list.
func (f tagFilter) href(tag string) string {
	q := url.Values{}
	for k, v := range f.query {
		q[k] = v
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	if len(q) == 0 {
		return f.route(f.base)
	}
	return f.route(f.base + "?" + q.Encode())
}

func (f tagFilter) has(k view.Kind, obj *model.Object) bool {
	for _, c := range categoriesOf(k, obj) {
		if strings.EqualFold(c, f.tag) {
			return true
		}
	}
	return false
}

// keep is the records the filter shows.
func (f tagFilter) keep(k view.Kind, items []listed) []listed {
	if f.tag == "" {
		return items
	}
	out := make([]listed, 0, len(items))
	for _, it := range items {
		if f.has(k, it.obj) {
			out = append(out, it)
		}
	}
	return out
}

// tags are the categories of one record, each a link to the list filtered to
// it. The one the list is filtered by stands first: a row's cell shows the
// first tag and counts the rest (Typecase's note 44), and the row then says
// why it is in the list. The others keep the record's order.
func (f tagFilter) tags(values []string) []view.Tag {
	var out []view.Tag
	for _, v := range values {
		tag := view.Tag{Label: text(v), Href: f.href(v), On: f.tag != "" && strings.EqualFold(v, f.tag)}
		if tag.On {
			out = slices.Insert(out, 0, tag)
			continue
		}
		out = append(out, tag)
	}
	return out
}

// rail is Rail.Tags: "all", then every category of the records read, the
// most used first. The set comes from everything read, not from what the
// filter shows, so choosing a category does not take the others away. A
// category named in the address that no record has stays, marked, so the
// filter can be cleared. No categories and no filter: no tags.
func (f tagFilter) rail(k view.Kind, items []listed) []view.Tag {
	type category struct {
		label string
		n     int
	}
	byKey := map[string]*category{}
	var order []*category
	for _, it := range items {
		counted := map[string]bool{}
		for _, v := range categoriesOf(k, it.obj) {
			key := strings.ToLower(v)
			if counted[key] {
				continue
			}
			counted[key] = true
			c := byKey[key]
			if c == nil {
				c = &category{label: v}
				byKey[key] = c
				order = append(order, c)
			}
			c.n++
		}
	}
	if len(order) == 0 && f.tag == "" {
		return nil
	}
	sort.SliceStable(order, func(i, j int) bool {
		if order[i].n != order[j].n {
			return order[i].n > order[j].n
		}
		return strings.ToLower(order[i].label) < strings.ToLower(order[j].label)
	})
	out := []view.Tag{{Label: "all", Href: f.href(""), On: f.tag == ""}}
	found := false
	for _, c := range order {
		on := f.tag != "" && strings.EqualFold(c.label, f.tag)
		found = found || on
		out = append(out, view.Tag{Label: text(c.label), Href: f.href(c.label), On: on})
	}
	if f.tag != "" && !found {
		out = append(out, view.Tag{Label: text(f.tag), Href: f.href(f.tag), On: true})
	}
	return out
}

// recordTags are a record's categories as links to its collection's list
// filtered to each.
func (s *Server) recordTags(c *collectionCtx, obj *model.Object) []view.Tag {
	return s.tagFilter(c.listPath(), nil).tags(categoriesOf(c.spec.kind, obj))
}

// taken is the time of the print header: absolute and with the year (C-10),
// since a printout is read long after it was taken.
func (f timeFmt) taken(t time.Time) view.When {
	if t.IsZero() {
		return ""
	}
	return view.When(f.date(t) + ", " + f.clock(t))
}

// collectionNouns name the collections a combined view shows in its print
// header.
var collectionNouns = map[view.Kind]string{
	view.KindContact: "book",
	view.KindEvent:   "calendar",
	view.KindTask:    "list",
	view.KindNote:    "notebook",
}

// recordNouns name the records themselves, as the frames count them: a
// contact is a card, an event an event. A poll over a list says «Polled 5 of
// 6 sources - 14 events», and «record» in place of the noun would be the
// server talking about its own types.
var recordNouns = map[view.Kind]string{
	view.KindContact: "card",
	view.KindEvent:   "event",
	view.KindTask:    "task",
	view.KindNote:    "note",
	view.KindFile:    "file",
}

// printRange joins the parts of Print.Range that are there.
func printRange(parts ...string) view.Text {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return text(strings.Join(kept, " · "))
}

// recordPrint is the print header of a record read: its title, where it
// lives, and when it was read; on paper its title the heading, where it
// lives under it, and the foot — the kind and when it was read (print-3,
// -5; Typecase's note 132).
func (s *Server) recordPrint(c *collectionCtx, title view.Text) view.Print {
	return view.Print{Title: title, Range: text(string(c.stamp())), Taken: c.tf.taken(c.tf.now),
		Heading: title, Sources: text(string(c.stamp())),
		Foot: text("Carrel · " + c.spec.noun + " · captured " + string(c.tf.numStamp(c.tf.now)))}
}

var showLabels = map[string]string{"open": "Open", "done": "Done", "all": "All"}
