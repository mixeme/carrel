package web

import (
	"context"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

// S-19: an export has no screen. It is a menu item that hands over a file,
// written to the response as the records arrive (S-9.3) — no temporary file
// on the way. A calendar takes the week the agenda shows when the address
// names one; everything else is the collection whole.

// exportFormat is what one kind of record is handed over as.
type exportFormat struct {
	ext   string
	media string
}

var exportFormats = map[view.Kind]exportFormat{
	view.KindContact: {".vcf", "text/vcard; charset=utf-8"},
	view.KindEvent:   {".ics", cal.MediaType},
	view.KindTask:    {".ics", cal.MediaType},
	view.KindNote:    {".md", "text/markdown; charset=utf-8"},
}

// exportHref is the address that hands over this collection, with the window
// when the screen has one.
func (s *Server) exportHref(c *collectionCtx, q url.Values) string {
	return s.postQuery(c.listPath()+"/export", q)
}

func (s *Server) exportAction(c *collectionCtx, q url.Values) view.Action {
	return view.Action{ID: "collection.export", Label: "Export", Icon: "i-down", Href: s.exportHref(c, q), Method: "GET", Enabled: true}
}

// exportWindow is the range an address names, both ends or neither. Only a
// calendar has one: a book, a task list and a notebook are handed over whole.
func exportWindow(c *collectionCtx, r *http.Request) (*time.Time, *time.Time) {
	if c.spec.kind != view.KindEvent {
		return nil, nil
	}
	q := r.URL.Query()
	from, okFrom := parseDay(c.tf, q.Get("from"))
	if !okFrom {
		return nil, nil
	}
	to, okTo := parseDay(c.tf, q.Get("to"))
	if !okTo {
		to = from.AddDate(0, 0, 7)
	} else {
		to = to.AddDate(0, 0, 1) // the day named is in the window
	}
	return &from, &to
}

func parseDay(tf timeFmt, v string) (time.Time, bool) {
	d, err := time.ParseInLocation("2006-01-02", v, tf.tz)
	return d, err == nil
}

// exportWhole is a calendar asked for with every component it holds (whole=1):
// what the delete sheet hands over (S-12.4), so a task in a calendar of events
// is not lost with it. It is .ics whatever the kind, and has no window.
func exportWhole(c *collectionCtx, r *http.Request) bool {
	return c.card == nil && r.URL.Query().Get("whole") != ""
}

// exportRecords reads what the file will hold: one REPORT for a book, one for
// the component of the kind, with the window when there is one.
func (c *collectionCtx) exportRecords(ctx context.Context, from, to *time.Time) ([]*model.Object, error) {
	if c.card != nil {
		return c.contactsAll(ctx)
	}
	objs, _, err := c.calQuery(ctx, c.spec.component, from, to)
	return objs, err
}

// allRecords reads every record of the collection: one REPORT for an address
// book, one a component for a calendar — all three, so a stray task in a
// calendar of events is read too. A resource two REPORTs name is read once.
func (c *collectionCtx) allRecords(ctx context.Context) ([]*model.Object, error) {
	if c.card != nil {
		return c.contactsAll(ctx)
	}
	var out []*model.Object
	seen := map[string]bool{}
	for _, p := range calendarParts {
		objs, _, err := c.calQuery(ctx, p.component, nil, nil)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			if !seen[obj.Path] {
				seen[obj.Path] = true
				out = append(out, obj)
			}
		}
	}
	return out, nil
}

// getKindExport hands the collection over as a file.
func (s *Server) getKindExport(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if uid := r.FormValue("uid"); uid != "" && c.spec.kind == view.KindNote {
		s.exportNote(w, r, c, uid)
		return
	}
	whole := exportWhole(c, r)
	var objs []*model.Object
	var err error
	if whole {
		objs, err = c.allRecords(r.Context())
	} else {
		from, to := exportWindow(c, r)
		objs, err = c.exportRecords(r.Context(), from, to)
	}
	if err != nil {
		// Nothing was read, so there is no file to hand over: the list says
		// what the server answered (S-21), and the export is one click away
		// again.
		s.writePage(w, r, http.StatusOK, s.emptyListPage(c, s.unavailableNotice(c, c.listPath(), err)))
		return
	}
	f := exportFormats[c.spec.kind]
	if whole {
		f = exportFormats[view.KindEvent]
	}
	h := w.Header()
	h.Set("Content-Type", f.media)
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": exportFileName(c, f.ext)}); v != "" {
		h.Set("Content-Disposition", v)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	if whole {
		writeCalendar(w, objs)
		return
	}
	writeExport(w, c, objs)
}

// writeExport streams the records. A write that breaks off — the browser went
// away — ends the answer: there is nothing to say on a body already begun.
func writeExport(w io.Writer, c *collectionCtx, objs []*model.Object) {
	switch {
	case c.card != nil:
		for _, obj := range objs {
			if card.Encode(w, obj) != nil {
				return
			}
		}
	case c.spec.kind == view.KindNote:
		for _, obj := range objs {
			if cal.NoteMarkdown(w, obj) != nil {
				return
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return
			}
		}
	default:
		writeCalendar(w, objs)
	}
}

// writeCalendar puts every record under one envelope, copying the components
// as the server sent them. A time zone several records name is written once:
// a calendar defines each of its zones once (RFC 5545 §3.6.5).
func writeCalendar(w io.Writer, objs []*model.Object) {
	head := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:" + cal.ProdID, "CALSCALE:GREGORIAN", "",
	}, "\r\n")
	if _, err := io.WriteString(w, head); err != nil {
		return
	}
	zones := map[string]bool{}
	for _, obj := range objs {
		for _, part := range cal.Components(obj.CalendarDoc()) {
			if part.TZID != "" {
				if zones[part.TZID] {
					continue
				}
				zones[part.TZID] = true
			}
			if _, err := w.Write(part.Body); err != nil {
				return
			}
		}
	}
	_, _ = io.WriteString(w, "END:VCALENDAR\r\n")
}

// exportFileName is the name the file arrives under: the collection's, as an
// address segment, so every system it lands on can keep it.
func exportFileName(c *collectionCtx, ext string) string {
	return collectionSlug(collectionLabel(c.col)) + ext
}

// emptyListPage is the screen of a collection with nothing read: what a route
// that hands over a file answers when it could not read one (S-21).
func (s *Server) emptyListPage(c *collectionCtx, notices ...view.Notice) any {
	shell := s.buildShell(c.sess, collectionLabel(c.col))
	shell.Rail = s.kindRail(c.sess, s.catalogOf(c.sess), c.spec, nil)
	shell.Notices = append(shell.Notices, notices...)
	switch c.spec.kind {
	case view.KindContact:
		return view.ContactsListPage{Shell: shell}
	case view.KindEvent:
		return view.AgendaPage{Shell: shell}
	case view.KindTask:
		return view.TasksPage{Shell: shell}
	default:
		return view.NotesPage{Shell: shell}
	}
}

// exportNote hands over one note as Markdown, the format of the notebook
// export (S-19), named after its title (forms-tail.md, Export Markdown).
func (s *Server) exportNote(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	}
	f := exportFormats[view.KindNote]
	h := w.Header()
	h.Set("Content-Type", f.media)
	name := collectionSlug(noteTitle(cal.Text(obj, "SUMMARY"), cal.Description(obj))) + f.ext
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name}); v != "" {
		h.Set("Content-Disposition", v)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_ = cal.NoteMarkdown(w, obj)
}
