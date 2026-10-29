package web

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
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
// names one; everything else is the collection whole. ?uid= names one record
// of any kind: the item «Export .vcf» of a record's menu hands over that
// record alone, named after it.

// exportFormat is what one kind of record is handed over as.
type exportFormat struct {
	ext   string
	media string
}

// notebookFormat is a notebook handed over whole: a .zip with a .md of each
// note (s-agenda-30, «Download .zip»); one note is its .md.
var notebookFormat = exportFormat{".zip", "application/zip"}

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

// recordExportAction is «Export .vcf», «.ics» or «.md» in the menu of one
// record: the format is named in the item, and it hands the file over at
// once (the mockup's rule of the ellipsis, §6).
func (s *Server) recordExportAction(o origin, spec kindSpec, uid string) view.Action {
	return view.Action{ID: "record.export", Label: text("Export " + exportFormats[spec.kind].ext), Icon: "i-export", Method: "GET", Enabled: true,
		Href: s.postQuery(o.listPath(spec.segment)+"/export", url.Values{"uid": {uid}})}
}

func (s *Server) exportAction(c *collectionCtx, q url.Values) view.Action {
	return view.Action{ID: "collection.export", Label: "Export", Icon: "i-export", Href: s.exportHref(c, q), Method: "GET", Enabled: true}
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
// the component of the kind, with the window when there is one. What the
// server handed over and Carrel could not read comes back beside the records
// (exportUnread).
func (c *collectionCtx) exportRecords(ctx context.Context, from, to *time.Time) ([]*model.Object, []cal.Unreadable, error) {
	if c.card != nil {
		objs, err := c.contactsAll(ctx)
		bad, err := unreadableCards(err)
		return objs, bad, err
	}
	return c.calQuery(ctx, c.spec.component, from, to)
}

// allRecords reads every record of the collection: one REPORT for an address
// book, one a component for a calendar — all three, so a stray task in a
// calendar of events is read too. A resource two REPORTs name is read once.
func (c *collectionCtx) allRecords(ctx context.Context) ([]*model.Object, error) {
	if c.card != nil {
		return c.contactsAll(ctx)
	} // no omission flow: refuse partial reads
	out, _, err := c.allRecordsAndUnread(ctx)
	return out, err
}

// allRecordsAndUnread is allRecords with what the server handed over and
// Carrel could not read, each resource once.
func (c *collectionCtx) allRecordsAndUnread(ctx context.Context) ([]*model.Object, []cal.Unreadable, error) {
	if c.card != nil {
		objs, err := c.contactsAll(ctx)
		bad, err := unreadableCards(err)
		return objs, bad, err
	}
	var out []*model.Object
	var unread []cal.Unreadable
	seen := map[string]bool{}
	for _, p := range calendarParts {
		objs, bad, err := c.calQuery(ctx, p.component, nil, nil)
		if err != nil {
			return nil, nil, err
		}
		for _, obj := range objs {
			if !seen[obj.Path] {
				seen[obj.Path] = true
				out = append(out, obj)
			}
		}
		for _, b := range bad {
			if !seen[b.Path] {
				seen[b.Path] = true
				unread = append(unread, b)
			}
		}
	}
	return out, unread, nil
}

// exportUnread is what an export does with the records Carrel could not read
// (Б3 of the audit of 11.10.2026: they were left out of the file without a
// word, the file before a calendar is deleted among them). A calendar file
// takes such a record as the server holds it — its components are copied,
// nothing is decoded —, so it is asked for again, raw. lost are the ones that
// cannot go into the file: the server would not give the record, nothing in
// it is a component, or the file is Markdown, which has no place for a
// record nobody read.
func (c *collectionCtx) exportUnread(ctx context.Context, bad []cal.Unreadable, calendar bool) (raw [][]byte, lost []cal.Unreadable) {
	for _, b := range bad {
		if !calendar || c.cal == nil {
			lost = append(lost, b)
			continue
		}
		doc, err := c.cal.Raw(ctx, b.Path)
		if err == nil && len(cal.Components(doc)) == 0 {
			err = errors.New("no component of a calendar in it")
		}
		if err != nil {
			b.Err = fmt.Errorf("%v; asked for as it is: %v", b.Err, err)
			lost = append(lost, b)
			continue
		}
		raw = append(raw, doc)
	}
	return raw, lost
}

// unreadKept is exportUnread for a calendar file nobody is asked about —
// an archive of a backup, the attachment of a letter: the records that go in
// as the server holds them, and an error naming the ones that cannot, so
// the file is not called whole without them.
func (c *collectionCtx) unreadKept(ctx context.Context, bad []cal.Unreadable) ([][]byte, error) {
	raw, lost := c.exportUnread(ctx, bad, true)
	if len(lost) == 0 {
		return raw, nil
	}
	names := make([]string, len(lost))
	for i, b := range lost {
		names[i] = path.Base(b.Path)
	}
	return raw, fmt.Errorf("%s the server returned could not be read and would be left out: %s", plural(len(lost), "record"), strings.Join(names, ", "))
}

// exportLacks is the answer of an export that would leave records out: no
// file, the records named, and the file without them one explicit step away
// (?without=1).
func (s *Server) exportLacks(c *collectionCtx, r *http.Request, lost []cal.Unreadable) view.Notice {
	q := r.URL.Query()
	q.Set("without", "1")
	n := view.Notice{
		Role:  view.NoticeAlert,
		Title: text("The file would lack " + plural(len(lost), "record")),
		Body: text("The server returned them, but Carrel could not read them and cannot put them into the file as they are. " +
			"Nothing was handed over. They are left on the server as they are; other clients may still open them."),
		Details: &view.Diagnostic{Summary: text("Unreadable records")},
		Actions: []view.Action{{ID: "collection.export.without", Label: "Export without them", Icon: "i-export", Href: s.exportHref(c, q), Method: "GET", Enabled: true}},
	}
	for _, b := range lost {
		n.Details.Lines = append(n.Details.Lines, text(b.Path+" · "+b.Err.Error()))
	}
	return n
}

// getKindExport hands the collection over as a file.
func (s *Server) getKindExport(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	if uid := r.FormValue("uid"); uid != "" {
		s.exportRecord(w, r, c, uid)
		return
	}
	whole := exportWhole(c, r)
	var objs []*model.Object
	var bad []cal.Unreadable
	var err error
	if whole {
		objs, bad, err = c.allRecordsAndUnread(r.Context())
	} else {
		from, to := exportWindow(c, r)
		objs, bad, err = c.exportRecords(r.Context(), from, to)
	}
	if err != nil {
		// Nothing was read, so there is no file to hand over: the list says
		// what the server answered (S-21), and the export is one click away
		// again.
		s.writePage(w, r, http.StatusOK, s.emptyListPage(c, s.unavailableNotice(c, c.listPath(), err)))
		return
	}
	f := exportFormats[c.spec.kind]
	switch {
	case whole:
		f = exportFormats[view.KindEvent]
	case c.spec.kind == view.KindNote:
		f = notebookFormat
	}
	raw, lost := c.exportUnread(r.Context(), bad, f.ext == ".ics")
	if len(lost) > 0 && r.URL.Query().Get("without") == "" {
		s.writePage(w, r, http.StatusOK, s.emptyListPage(c, s.exportLacks(c, r, lost)))
		return
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
		writeCalendar(w, objs, raw)
		return
	}
	writeExport(w, c, objs, raw)
}

// writeExport streams the records. A write that breaks off — the browser went
// away — ends the answer: there is nothing to say on a body already begun.
// raw are the documents of records Carrel could not read, for a calendar
// file (exportUnread).
func writeExport(w io.Writer, c *collectionCtx, objs []*model.Object, raw [][]byte) {
	switch {
	case c.card != nil:
		for _, obj := range objs {
			if card.Encode(w, obj) != nil {
				return
			}
		}
	case c.spec.kind == view.KindNote:
		writeNotebook(w, c, objs)
	default:
		writeCalendar(w, objs, raw)
	}
}

// writeNotebook is the notebook as a .zip: each note a .md named after it,
// a name taken twice numbered («Idea.md», «Idea (2).md»).
func writeNotebook(w io.Writer, c *collectionCtx, objs []*model.Object) {
	z := zip.NewWriter(w)
	taken := map[string]bool{}
	for _, obj := range objs {
		stem := collectionSlug(recordTitle(c.spec.kind, obj))
		name := stem + ".md"
		for n := 2; taken[strings.ToLower(name)]; n++ {
			name = fmt.Sprintf("%s (%d).md", stem, n)
		}
		taken[strings.ToLower(name)] = true
		f, err := z.Create(name)
		if err != nil || cal.NoteMarkdown(f, obj) != nil {
			return
		}
	}
	_ = z.Close()
}

// writeCalendar puts every record under one envelope, copying the components
// as the server sent them. A time zone several records name is written once:
// a calendar defines each of its zones once (RFC 5545 §3.6.5).
// raw are documents nobody decoded, copied the same way after the records.
func writeCalendar(w io.Writer, objs []*model.Object, raw [][]byte) {
	head := strings.Join([]string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:" + cal.ProdID, "CALSCALE:GREGORIAN", "",
	}, "\r\n")
	if _, err := io.WriteString(w, head); err != nil {
		return
	}
	docs := make([][]byte, 0, len(objs)+len(raw))
	for _, obj := range objs {
		docs = append(docs, obj.CalendarDoc())
	}
	zones := map[string]bool{}
	for _, doc := range append(docs, raw...) {
		for _, part := range cal.Components(doc) {
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
	shell.Rail = s.kindRail(c.sess, s.catalogOf(c.sess), c.spec, nil, c.listPath())
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

// exportRecord hands over one record in the format of its kind's export
// (S-19), named after its title (review Р-10, Export Markdown): a card as
// vCard, an event or a task as a calendar of one, a note as Markdown.
func (s *Server) exportRecord(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string) {
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.cardAnswer(r, c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	}
	f := exportFormats[c.spec.kind]
	// as=ics: a record of a calendar as the server holds it, whatever its
	// kind — a note's journal beside its Markdown (Р20).
	whole := c.card == nil && r.URL.Query().Get("as") == "ics"
	if whole {
		f = exportFormats[view.KindEvent]
	}
	h := w.Header()
	h.Set("Content-Type", f.media)
	name := collectionSlug(recordTitle(c.spec.kind, obj)) + f.ext
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name}); v != "" {
		h.Set("Content-Disposition", v)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	if whole {
		writeCalendar(w, []*model.Object{obj}, nil)
		return
	}
	writeExport(w, c, []*model.Object{obj}, nil)
}
