package web

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The suggestions of Related to (s-tasks-10, s-notes-20; the client's scan
// ЗД7, НД3): as the attendees' field asks /app/contacts/suggest, the field
// of a task and of a note asks its record's address for the records the
// session has loaded whose title holds what was typed. Without scripts the
// same records are the field's options (U-6.3). No server is asked.

// taskTimes are the fields a task's All day switches off while it stands
// (s-tasks-3, C-6.8 Disables): both times, a space between them.
func taskTimes() string { return "start.time due.time" }

// relatedSuggest is where the field of Related to asks for its suggestions:
// the record's own address, «new» for a record not yet made.
func (s *Server) relatedSuggest(c *collectionCtx, st editState) string {
	uid := st.uid
	if st.isNew || uid == "" {
		uid = "new"
	}
	return s.route(c.listPath() + "/" + url.PathEscape(uid) + "/related")
}

// getRelatedSuggest answers the line of Related to: the loaded events, tasks
// and notes of every collection whose title holds ?q= — those it begins
// first, then the newest —, each with its collection's mark and «kind ·
// day», the record itself left out.
func (s *Server) getRelatedSuggest(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	self := r.PathValue("uid")
	out := view.SuggestList{Items: []view.SuggestItem{}}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		s.writePage(w, r, http.StatusOK, out)
		return
	}
	needle := strings.ToLower(q)
	type hit struct {
		e      indexEntry
		colour view.Colour
		at     int
	}
	var hits []hit
	seen := map[string]bool{self: true}
	eachIndexed(c.sess, s.catalogOf(c.sess), func(col sealed.Collection, e indexEntry) {
		if seen[e.UID] || e.Kind == view.KindContact {
			return
		}
		seen[e.UID] = true
		if at := runeIndex(strings.ToLower(e.Title), needle); at >= 0 {
			hits = append(hits, hit{e: e, colour: catalogColour(col), at: at})
		}
	})
	sort.SliceStable(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if (a.at == 0) != (b.at == 0) {
			return a.at == 0
		}
		if a.e.At != b.e.At {
			return a.e.At > b.e.At
		}
		return a.e.Title < b.e.Title
	})
	for _, h := range hits {
		if len(out.Items) >= suggestMost {
			break
		}
		meta := string(h.e.Kind)
		if h.e.At != 0 {
			meta += " · " + c.tf.shortDay(time.Unix(h.e.At, 0))
		}
		out.Items = append(out.Items, view.SuggestItem{Name: foundRuns(h.e.Title, h.at, utf8.RuneCountInString(q)), Colour: h.colour,
			Value: h.e.UID, Meta: text(meta)})
	}
	s.writePage(w, r, http.StatusOK, out)
}
