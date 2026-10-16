package web

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The sheet of deleting a record (note 69 of Typecase, task 166; s-agenda-26,
// s-tasks-18; the owner's decisions А8, А3, А12 of 12.10.2026): Delete posts
// as it always did, and a record that has something to lose answers it with
// the sheet instead of going — what points at it, whether its attendees may
// be told, its attachments, its copies. The sheet posts again with sheet=1
// and what it chose. A record with nothing to lose is deleted at once.

// deleteFile is the tick of a link's file on the sheet: delete it too.
const deleteFile = "attachment"

// deleteNotes are what becomes of what points at a record, as the sheet says
// it (s-agenda-26).
const (
	linkThen    = "answers “not found” from that moment"
	relatedThen = "among the loaded ones; their link is left pointing at nothing"
)

// recordDeleteSheet is the sheet of deleting the record opened in c; false
// when it has nothing to lose — no line of the sheet would name anything.
func (s *Server) recordDeleteSheet(ctx context.Context, c *collectionCtx, uid string, obj *model.Object) (view.RecordDeleteSheet, bool) {
	noun := c.spec.noun
	sheet := view.RecordDeleteSheet{
		Kind: c.spec.kind, Title: text("Delete " + noun), Subject: text(recordTitle(c.spec.kind, obj)),
		Warning: text("The " + noun + " is deleted on " + string(c.stamp()) +
			". Carrel holds no copy — it reads and writes, it does not store — so there is nothing here to undo it with."),
		References: []view.Reference{}, Attachments: []view.Reference{},
	}
	blob := s.catalogOf(c.sess)

	// What is pointing at it now: its published links, and the records
	// related to it among those this session has loaded.
	links := 0
	for _, l := range s.users.PublishedLinks(c.sess.AccountID()) {
		if !l.Gone && l.Target.AccountID == c.conn.ID && l.Target.Col == c.col.Path && l.Target.UID == uid {
			links++
		}
	}
	if links > 0 {
		sheet.References = append(sheet.References, view.Reference{What: text(plural(links, "published link")), Then: linkThen})
	}
	pointing := map[string]bool{}
	eachIndexed(c.sess, blob, func(_ sealed.Collection, e indexEntry) {
		if e.UID != uid && slices.Contains(e.Related, uid) {
			pointing[e.UID] = true
		}
	})
	if n := len(pointing); n > 0 {
		what := plural(n, "record") + " related to it"
		sheet.References = append(sheet.References, view.Reference{What: text(what), Then: relatedThen})
	}

	// Its attendees, on a server that sends invitations itself: it may tell
	// them — «may», as the sheets of a move say it (А12).
	if n := len(obj.Props()["ATTENDEE"]); n > 0 && c.conn.Scheduling == discover.SchedulingYes {
		what := "the meeting"
		if c.spec.kind != view.KindEvent {
			what = "the " + noun
		}
		sheet.Cancels = text("The " + noun + " has " + plural(n, "attendee") + ", and " + connLabel(c.conn) +
			" sends invitations itself: the server may tell them " + what + " is cancelled.")
	}

	// Its attachments: written into it, they go with it; a link stays, and
	// one into the attachment folder may be deleted with it — unticked.
	folder := s.savedFolder(c.sess, blob)
	files := view.FormRow{Name: deleteFile, Kind: view.InputCheck}
	for _, a := range cal.Attachments(obj) {
		if a.Inline {
			name := a.FileName
			if name == "" {
				name = "Attachment " + strconv.Itoa(a.Index+1)
			}
			sheet.Attachments = append(sheet.Attachments, view.Reference{What: text(name), Then: text("written into the " + noun + ", goes with it")})
			continue
		}
		name := linkName(a)
		_, col, segs, ok := attachLinkAt(blob, a.URI)
		if !ok {
			sheet.Attachments = append(sheet.Attachments, view.Reference{What: text(name), Then: "a link outside Carrel, nothing there changes"})
			continue
		}
		place := strings.Join(append([]string{collectionLabel(col)}, segs[:len(segs)-1]...), " / ")
		sheet.Attachments = append(sheet.Attachments, view.Reference{What: text(name), Then: text("a link to " + place + ", the file stays")})
		if inFolder(folder, col, segs) {
			files.Options = append(files.Options, view.Option{Value: strconv.Itoa(a.Index), Label: text("Delete " + name + " too")})
		}
	}

	rows := []view.FormRow{csrfField(c.sess), {Name: "etag", Kind: view.InputHidden, Value: text(obj.ETag)},
		{Name: "sheet", Kind: view.InputHidden, Value: "1"}}
	if len(files.Options) > 0 {
		rows = append(rows, files)
	}
	sets := []view.FieldSet{{Rows: rows}}

	// Its copies (А3): the one opened ticked, the others to tick.
	copies := false
	if lc, ok := s.linkedOf(ctx, c, uid); ok {
		copies = true
		holders := copyHolders[c.spec.kind]
		differ := differs(differTable(c.spec.kind))
		row := view.FormRow{Name: copyField, Label: "Copy", Kind: view.InputCheck, Options: []view.Option{{
			Value: copyValue(lc.self), Label: text(string(c.stamp())), Colour: catalogColour(c.col), Selected: true, Meta: "the copy you opened"}}}
		for _, m := range lc.others {
			opt := view.Option{Value: copyValue(m.ref), Label: text(string(m.c.stamp())), Colour: catalogColour(m.c.col)}
			switch d := differ(obj, m.obj); {
			case m.obj == nil:
				opt.Meta, opt.Disabled = "The server did not answer", true
			case m.c.col.ReadOnly || m.c.feed != nil:
				opt.Disabled, opt.Reason = true, "read-only: it cannot be deleted"
			case len(d) == 0:
				opt.Meta = "Holds the same"
			default:
				opt.Meta = text(strings.Join(d, ", ") + " differ")
			}
			row.Options = append(row.Options, opt)
		}
		sets = append(sets, view.FieldSet{Legend: text("The same " + holders[0] + " in other " + holders[1]), Rows: []view.FormRow{row}})
	}
	sheet.Form = view.Form{Action: s.route(c.recordPath(uid) + "/delete"), Method: "POST", Sets: sets}
	sheet.Actions = []view.Action{
		{ID: "record.delete", Label: "Delete", Method: "POST", Danger: true, Enabled: true, Submits: true},
		{ID: "record.delete.cancel", Label: "Cancel", Href: s.route(c.recordPath(uid)), Method: "GET", Enabled: true},
	}
	return sheet, len(sheet.References) > 0 || sheet.Cancels != "" || len(sheet.Attachments) > 0 || copies
}

// inFolder is whether a link's file lies in the attachment folder.
func inFolder(folder *fileCtx, col sealed.Collection, segs []string) bool {
	return folder != nil && folder.col.Path == col.Path && folder.conn.ID == col.AccountID &&
		len(segs) > len(folder.segs) && slices.Equal(segs[:len(folder.segs)], folder.segs)
}

// writeDeleteSheet answers Delete with the sheet: a fragment over the card,
// or a page with the card under it.
func (s *Server) writeDeleteSheet(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid string, sheet view.RecordDeleteSheet) {
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	sheet.Under = s.underOf(r, "", c.recordPath(uid))
	s.writePage(w, r, http.StatusOK, sheet)
}

// deleteChosen does what the sheet's Delete chose beside the record: the
// files of its links ticked, and its copies ticked (А3). The record itself
// goes after, by the caller, unless its own copy was unticked — then false.
// An error stops before the record is touched.
func (s *Server) deleteChosen(ctx context.Context, r *http.Request, c *collectionCtx, uid string, obj *model.Object) (bool, error) {
	blob := s.catalogOf(c.sess)
	folder := s.savedFolder(c.sess, blob)
	for _, v := range r.Form[deleteFile] {
		i, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		for _, a := range cal.Attachments(obj) {
			if a.Inline || a.Index != i {
				continue
			}
			conn, col, segs, ok := attachLinkAt(blob, a.URI)
			if !ok || !inFolder(folder, col, segs) {
				continue
			}
			fc, ok := s.fileCollection(c.sess, conn, col.Path)
			if !ok {
				continue
			}
			p, err := fc.path(segs, false)
			if err != nil {
				continue
			}
			if err := fc.files.Delete(ctx, p, ""); err != nil && !notFoundErr(err) {
				return false, err
			}
		}
	}
	self := true
	if lc, ok := s.linkedOf(ctx, c, uid); ok {
		ticked := r.Form[copyField]
		self = slices.Contains(ticked, copyValue(lc.self))
		for _, m := range lc.others {
			if m.obj == nil || m.c.col.ReadOnly || m.c.feed != nil || !slices.Contains(ticked, copyValue(m.ref)) {
				continue
			}
			if err := m.c.remove(ctx, m.obj.Path, m.obj.ETag); err != nil && !notFoundErr(err) {
				return false, err
			}
			m.c.noteWrite(m.obj.Path, nil)
			s.linksGone(c.sess, placeOf(m.c, m.ref.UID))
		}
	}
	return self, nil
}
