package web

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"path"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// S-18: a record moves or is copied to another collection of the same kind
// that can be written, on any connection. The sheet and its POST share one
// address; copy=1 is the copy, to= names the connection and into= the
// collection path of the target.

// moveTarget is the collection a move or a copy goes to, or nil when the
// request names none the catalog holds for this kind.
func (s *Server) moveTarget(c *collectionCtx, to, into string) *collectionCtx {
	if to == "" || into == "" {
		return nil
	}
	dest, ok := s.collectionOf(c.sess, c.spec, to, into)
	if !ok || dest.col.Gone {
		return nil
	}
	return dest
}

// recordMoveBlock is why a move or a copy from c to dest cannot start, or "".
func recordMoveBlock(c, dest *collectionCtx, copying bool) string {
	switch {
	case dest == nil:
		return "Choose a collection."
	case dest.conn.ID == c.conn.ID && dest.col.Path == c.col.Path:
		return "The " + c.spec.noun + " is already here."
	case dest.col.ReadOnly:
		return "The chosen collection is read-only."
	case !copying && c.col.ReadOnly:
		return "This collection is read-only, so the " + c.spec.noun + " cannot leave it. It can be copied."
	}
	return ""
}

func moveQuery(copying bool, account, col string) url.Values {
	q := url.Values{}
	if copying {
		q.Set("copy", "1")
	}
	if account != "" {
		q.Set("to", account)
		q.Set("into", col)
	}
	return q
}

// moveCollectionNouns name what a record of a kind lives in, as the message
// of the move sheet says it (s-contacts-2).
var moveCollectionNouns = map[view.Kind]string{
	view.KindContact: "address book",
	view.KindEvent:   "calendar",
	view.KindTask:    "task list",
	view.KindNote:    "notebook",
}

// moveSheet says where the record can go before anything happens: every live
// collection of the kind, the current and read-only ones shown and closed
// with the reason, and what the move will do — on one server a MOVE, to
// another connection a copy and a delete (S-18). An option's ID is the sheet
// with that option chosen. title is the record's, "" when it could not be
// read.
func (s *Server) moveSheet(c *collectionCtx, uid, title string, copying bool, dest *collectionCtx) view.MoveSheet {
	address := c.recordPath(uid) + "/move"
	blob := s.catalogOf(c.sess)
	verb := "Move "
	if copying {
		verb = "Copy "
	}
	from := "from " + string(origin{conn: c.conn, col: c.col}.stamp())
	if title != "" {
		from = title + " · " + from
	}
	sheet := view.MoveSheet{Kind: c.spec.kind, Copy: copying, Title: text(verb + c.spec.noun), Subject: text(from)}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(c.spec.kind) {
				continue
			}
			o := origin{conn: conn, col: col}
			opt := view.TargetOption{
				ID:    s.postQuery(address, moveQuery(copying, conn.ID, col.Path)),
				Label: text(collectionLabel(col)), Stamp: o.stamp(), Colour: catalogColour(col),
			}
			switch {
			case conn.ID == c.conn.ID && col.Path == c.col.Path:
				opt.ReadOnly, opt.Reason = true, "already there"
			case col.ReadOnly:
				opt.ReadOnly, opt.Reason = true, "read-only"
			}
			sheet.Target.Options = append(sheet.Target.Options, opt)
		}
	}
	confirm := view.Action{ID: "record.move.confirm", Label: "Move here", Method: "POST", Primary: true, Enabled: true}
	if copying {
		confirm.ID, confirm.Label = "record.copy.confirm", "Copy here"
	}
	if reason := recordMoveBlock(c, dest, copying); reason != "" {
		confirm.Enabled, confirm.Reason = false, text(reason)
	} else {
		chosen := moveQuery(copying, dest.conn.ID, dest.col.Path)
		sheet.Target.Chosen = s.postQuery(address, chosen)
		confirm.Href = s.postQuery(address, chosen)
	}
	record, where := recordNouns[c.spec.kind], moveCollectionNouns[c.spec.kind]
	switch {
	case copying:
		sheet.Warning = text("A copy is written into the chosen " + where + " whole, with the same UID; the " + record + " here stays as it is.")
	case dest != nil && dest.conn.ID != c.conn.ID:
		sheet.Warning = text("This goes to another account, so nothing moves on a server: Carrel reads the " + c.spec.noun +
			" whole, writes it there, reads it back, and deletes the original only if the copy kept everything.")
	default:
		sheet.Warning = text("Inside one server this is MOVE: instant, no traffic. To another " + where + " the " + record +
			" is copied and then deleted — Carrel says so before it starts, not after.")
	}
	cancel := view.Action{ID: "record.move.cancel", Label: "Cancel", Href: s.route(c.recordPath(uid)), Method: "GET", Enabled: true}
	sheet.Actions = []view.Action{confirm, cancel}
	return sheet
}

// getKindMove is the sheet (C-5.4); without the fragment header it is the
// page, and choosing a target is following its link (U-8).
func (s *Server) getKindMove(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	q := r.URL.Query()
	copying := q.Get("copy") != ""
	if !copying && c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	// The head names the record (s-contacts-2); a record that cannot be read
	// now is still offered the sheet, named by where it lives.
	title := ""
	if obj, err := c.find(r.Context(), uid); err == nil {
		title = recordTitle(c.spec.kind, obj)
	}
	sheet := s.moveSheet(c, uid, title, copying, s.moveTarget(c, q.Get("to"), q.Get("into")))
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// postKindMove runs a confirmed move or copy. On one connection a move is a
// MOVE: the server keeps every byte. Otherwise the object read whole (S-7) is
// created there, read back and compared, and only a move without loss deletes
// the original, with If-Match of the version read. The UID stays: it is the
// key duplicates are found by (S-16).
func (s *Server) postKindMove(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	copying := r.FormValue("copy") != ""
	if !copying && c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	dest := s.moveTarget(c, r.FormValue("to"), r.FormValue("into"))
	nothing := "Nothing was moved"
	if copying {
		nothing = "Nothing was copied"
	}
	ctx := r.Context()
	obj, err := c.find(ctx, uid)
	switch {
	case notFoundErr(err):
		s.redirect(w, r, c.listPath())
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid), err)))
		return
	}
	s.moveRecord(w, r, c, dest, uid, obj, copying, nothing)
}

// moveRecord moves or copies obj, the record read at uid, to dest. nothing
// is the title of every answer where the record stayed: «Nothing was moved»
// from the sheet, «Saved, but not moved» after a note's edit.
func (s *Server) moveRecord(w http.ResponseWriter, r *http.Request, c, dest *collectionCtx, uid string, obj *model.Object, copying bool, nothing string) {
	ctx := r.Context()
	if reason := recordMoveBlock(c, dest, copying); reason != "" {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{Role: view.NoticeAlert, Title: text(nothing), Body: text(reason)}))
		return
	}
	// A collection holds one resource per UID (RFC 4791 §5.3.2, RFC 6352
	// §6.3.2); asking first says so instead of a bare refusal.
	switch _, err := dest.find(ctx, uid); {
	case err == nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
			Role: view.NoticeAlert, Title: text(nothing),
			Body:    text(collectionLabel(dest.col) + " already holds a " + c.spec.noun + " with this UID: the same record, or a copy made earlier."),
			Actions: []view.Action{s.openThere(dest, uid, "Open it there")},
		}))
		return
	case !notFoundErr(err):
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.unavailableNotice(dest, c.recordPath(uid), err)))
		return
	}
	if !copying && dest.conn.ID == c.conn.ID {
		if s.moveOnServer(w, r, c, dest, uid, obj, nothing) {
			return
		}
		// The server has no MOVE (SV-44). The record goes over the same way
		// it would to another server: the copy is written and read back
		// first, and only a copy that is really there costs the original.
	}

	// The copy is its own object: writing it stamps it and gives it a path,
	// and the original keeps the version the delete is conditional on.
	doc, err := c.body(obj)
	var sent *model.Object
	if err == nil {
		sent, err = c.parse(obj.Path, "", doc)
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	var dst string
	for _, name := range []string{path.Base(obj.Path), fileStem(uid) + "-2" + c.spec.ext} {
		dst = strings.TrimSuffix(dest.col.Path, "/") + "/" + name
		sent.Path = dst
		err = dest.create(ctx, dst, sent)
		// A taken name is tried once more under another; but a refusal on a
		// write the guard had already sent is this copy, arriving twice.
		// Writing it again under «-2» would leave the person two records
		// where they asked for one.
		if landedOnTheFirstTry(err) {
			err = nil
			break
		}
		if !dav.IsStatus(err, http.StatusPreconditionFailed) {
			break
		}
	}
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, moveFailure(dest, nothing, "PUT", dst, err)))
		return
	}
	if copying {
		// A copy is a record made in another collection, and read back the
		// way a new one is: a server that rewrites the UID of what is written
		// to it (SV-25: Google) keeps it under an identity of its own, and it
		// is found at the address it was written to.
		s.finishCreate(w, r, dest, uid, sent)
		return
	}
	// The copy is read back by its identity; where the server gave it another
	// (SV-25: Google), by the address it was written to, as finishCreate
	// does. Read back only by UID, a move into Google never finished: the
	// copy was not found, the original was kept, and the person had the
	// card twice.
	landed := uid
	got, err := dest.find(ctx, uid)
	if notFoundErr(err) {
		if at, ferr := dest.fetch(ctx, dst); ferr == nil {
			if made := uidOf(dest, at); made != "" && made != uid {
				got, err, landed = at, nil, made
				if kept, kerr := dest.find(ctx, made); kerr == nil {
					got = kept
				}
			}
		}
	}
	if err != nil {
		dest.noteWrite(dst, sent)
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.keptNotice(dest, uid, "Copied, and the original kept",
			"The copy could not be read back to check it, so the "+c.spec.noun+" was not deleted here. Both are there now.")))
		return
	}
	dest.noteWrite(got.Path, got)
	// An identity the server chose is not a loss: the UID was Carrel's to
	// give, never the person's (S-7.1).
	compare := model.CompareAfterPut
	if landed != uid {
		compare = model.CompareAfterCreate
	}
	if loss := compare(dest.conn.ID, sent, got, c.sess.Losses()); loss != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.keptNotice(dest, landed, "Copied, and the original kept",
			"The server of "+collectionLabel(dest.col)+" did not keep "+strings.Join(loss.Properties, ", ")+
				". The original stays here, so nothing is lost; delete it yourself if the copy is enough.")))
		return
	}
	err = c.remove(ctx, obj.Path, obj.ETag)
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		cur, _ := c.find(ctx, uid)
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, cur, s.keptNotice(dest, landed, "Copied, but the original changed, so it stays",
			"The "+c.spec.noun+" changed here after Carrel read it. The copy holds the version from before; both are there now.")))
	case err != nil:
		n := s.keptNotice(dest, landed, "Copied, but the original could not be deleted",
			"Both are there now. Delete the original yourself once the server lets you.")
		n.Details = moveFailure(c, "", "DELETE", obj.Path, err).Details
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, n))
	case landed != uid:
		// Moved, under an identity the other server gave it. What pointed at
		// the old one is pointed at the new, the way a SoGo move is mended
		// (relink.go): otherwise a task made from this person's timeline would
		// drop out of it without a word.
		c.noteWrite(obj.Path, nil)
		rep := s.relink(ctx, c.sess, s.catalogOf(c.sess), []string{uid}, landed)
		n := view.Notice{
			Role:  view.NoticeAlert,
			Title: "Moved, but the server gave it another identity",
			Body: text("The " + c.spec.noun + " is in " + collectionLabel(dest.col) + " with everything it held, but that server " +
				"replaced its UID. Duplicates are matched by that identifier." + rep.sentence()),
			Actions: []view.Action{s.openThere(dest, landed, "Open it there")},
		}
		if lines := rep.lines(); len(lines) > 0 {
			n.Details = &view.Diagnostic{Summary: "Linked records", Lines: lines}
		}
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, n))
	default:
		c.noteWrite(obj.Path, nil)
		s.redirect(w, r, dest.recordPath(uid))
	}
}

// moveOnServer is a MOVE within one connection. The name stays; a name taken
// there (412 with Overwrite: F) becomes {uid}-2 once.
// It reports whether it answered. A server with no MOVE at all — Xandikos
// answers 405 to every one of them, across collections and for a rename
// alike, and lists neither MOVE nor COPY in Allow (docs/servers.md, SV-44) —
// is left to the caller, which carries the record over as a copy and deletes
// the original once the copy has been read back. Nothing is written here
// before that is decided, so the two paths cannot both answer.
func (s *Server) moveOnServer(w http.ResponseWriter, r *http.Request, c, dest *collectionCtx, uid string, obj *model.Object, nothing string) bool {
	ctx := r.Context()
	// A card is the one record a server has been seen to rename on MOVE
	// (SV-18: SoGo). What the destination held before is kept, so that a card
	// under a new identity can be found as the one resource that was not
	// there — and whatever pointed at its old UID pointed at it (relink.go).
	var before map[string]bool
	if c.card != nil {
		before = resourcePaths(ctx, dest)
	}
	var dst string
	var err error
	for _, name := range []string{path.Base(obj.Path), fileStem(uid) + "-2" + c.spec.ext} {
		dst = strings.TrimSuffix(dest.col.Path, "/") + "/" + name
		if err = c.t.Move(ctx, obj.Path, dst, false); !dav.IsStatus(err, http.StatusPreconditionFailed) {
			break
		}
	}
	if err != nil {
		if noMoveHere(err) {
			return false
		}
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, moveFailure(c, nothing, "MOVE", obj.Path+" → "+dst, err)))
		return true
	}
	c.noteWrite(obj.Path, nil)
	got, err := dest.find(ctx, uid)
	if err != nil {
		// The record is there, but not under the identity it had: SoGo gives a
		// card moved into another book a UID of its own (docs/servers.md,
		// SV-18). That identifier is what S-16 counts duplicates by and what
		// S-18 promises not to change, so the person is told — sending them to
		// an address that no longer opens would be the worse answer.
		obj.Path, obj.ETag = dst, ""
		dest.noteWrite(dst, obj)
		if moved := arrived(ctx, dest, before); moved != nil {
			if renamed := card.UID(moved); renamed != "" {
				rep := s.relink(ctx, c.sess, s.catalogOf(c.sess), []string{uid}, renamed)
				n := view.Notice{
					Role:  view.NoticeAlert,
					Title: "Moved, but the server gave it another identity",
					Body: text("The " + c.spec.noun + " is in " + collectionLabel(dest.col) + " with everything it held, but that server " +
						"replaced its UID. Duplicates are matched by that identifier." + rep.sentence()),
					Actions: []view.Action{s.openThere(dest, renamed, "Open it there")},
				}
				if lines := rep.lines(); len(lines) > 0 {
					n.Details = &view.Diagnostic{Summary: "Linked records", Lines: lines}
				}
				s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, n))
				return true
			}
		}
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, view.Notice{
			Role:  view.NoticeAlert,
			Title: "Moved, but the server gave it another identity",
			Body: text("The " + c.spec.noun + " is in " + collectionLabel(dest.col) + " with everything it held, but that server " +
				"replaced its UID, so the address it had no longer opens it. Duplicates are matched by that identifier."),
			Actions: []view.Action{{
				ID: "collection.open", Label: text("Open " + collectionLabel(dest.col)),
				Href: s.route(dest.listPath()), Method: "GET", Enabled: true,
			}},
		}))
		return true
	}
	dest.noteWrite(got.Path, got)
	s.redirect(w, r, dest.recordPath(uid))
	return true
}

// resourcePaths are the paths of what a collection holds, or nil when it could
// not be read — then nothing can be told apart afterwards, and nothing is
// guessed.
func resourcePaths(ctx context.Context, c *collectionCtx) map[string]bool {
	ms, err := c.t.PropFind(ctx, c.col.Path, dav.DepthOne, []xml.Name{propResourceType})
	if err != nil {
		return nil
	}
	self := strings.TrimSuffix(hrefPath(c.col.Path), "/")
	out := map[string]bool{}
	for _, resp := range ms.Responses {
		if p := strings.TrimSuffix(hrefPath(resp.Href), "/"); p != "" && p != self {
			out[p] = true
		}
	}
	return out
}

// arrived is the one resource the collection holds now and did not hold
// before, read — or nil when there is none, more than one, or it cannot be
// read. More than one means someone else wrote there at the same moment, and
// picking one would be a guess.
func arrived(ctx context.Context, c *collectionCtx, before map[string]bool) *model.Object {
	if before == nil {
		return nil
	}
	now := resourcePaths(ctx, c)
	var fresh []string
	for p := range now {
		if !before[p] {
			fresh = append(fresh, p)
		}
	}
	if len(fresh) != 1 {
		return nil
	}
	obj, err := c.fetch(ctx, fresh[0])
	if err != nil {
		return nil
	}
	return obj
}

// noMoveHere reports a server that has no MOVE// noMoveHere reports a server that has no MOVE rather than one that refused
// this one: 405 with the method not allowed at all, or 501. A refusal about
// this particular record — a precondition, a right, a name — is not this, and
// goes to the person as it did before.
func noMoveHere(err error) bool {
	return dav.IsStatus(err, http.StatusMethodNotAllowed) || dav.IsStatus(err, http.StatusNotImplemented)
}

func (s *Server) openThere(dest *collectionCtx, uid string, label view.Text) view.Action {
	return view.Action{ID: "record.open", Label: label, Href: s.route(dest.recordPath(uid)), Method: "GET", Enabled: true}
}

// keptNotice is a move that ended as a copy: both records exist, and the copy
// is one link away.
func (s *Server) keptNotice(dest *collectionCtx, uid, title, body string) view.Notice {
	return view.Notice{
		Role: view.NoticeAlert, Title: text(title), Body: text(body),
		Actions: []view.Action{s.openThere(dest, uid, "Open the copy")},
	}
}

// moveFailure is a refused step, with the request and the server's answer. A
// 403 says what it means (C-9.1).
func moveFailure(c *collectionCtx, title, method, target string, err error) view.Notice {
	n := view.Notice{
		Role:  view.NoticeAlert,
		Title: text(title),
		Body:  text("The " + c.spec.noun + " is where it was."),
		Details: &view.Diagnostic{
			Summary: text(method + " " + target),
			Lines:   []view.Text{text(connLabel(c.conn)), text(err.Error())},
		},
	}
	if dav.IsStatus(err, http.StatusForbidden) {
		n.Body = text("This account can read " + collectionLabel(c.col) + " but not write there. The " + c.spec.noun + " is where it was.")
	}
	return n
}

// fileStem is a UID as a file name when it is safe as one, else a fresh
// Carrel name: a foreign UID may hold "/" or be a whole URL.
func fileStem(uid string) string {
	ok := uid != "" && len(uid) <= 64 && strings.Trim(uid, ".") != ""
	for _, r := range uid {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' || r == '@') {
			ok = false
			break
		}
	}
	if ok {
		return uid
	}
	if fresh, err := model.NewUID(); err == nil {
		return fresh
	}
	return "record"
}
