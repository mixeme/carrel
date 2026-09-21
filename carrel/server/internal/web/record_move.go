package web

import (
	"net/http"
	"net/url"
	"path"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
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

// moveSheet says where the record can go before anything happens: every live
// collection of the kind, the current and read-only ones shown and closed
// with the reason, and — to another connection — that a move is a copy and a
// delete (S-18). An option's ID is the sheet with that option chosen.
func (s *Server) moveSheet(c *collectionCtx, uid string, copying bool, dest *collectionCtx) view.MoveSheet {
	address := c.recordPath(uid) + "/move"
	blob := s.catalogOf(c.sess)
	sheet := view.MoveSheet{Kind: c.spec.kind, Copy: copying}
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
				opt.ReadOnly, opt.Reason = true, "Already here"
			case col.ReadOnly:
				opt.ReadOnly, opt.Reason = true, "Read-only"
			}
			sheet.Target.Options = append(sheet.Target.Options, opt)
		}
	}
	confirm := view.Action{ID: "record.move.confirm", Label: "Move here", Method: "POST", Enabled: true}
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
	if dest != nil && dest.conn.ID != c.conn.ID && !copying {
		sheet.Warning = text("This goes to another account, so nothing moves on a server: Carrel reads the " + c.spec.noun +
			" whole, writes it there, reads it back, and deletes the original only if the copy kept everything.")
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
	sheet := s.moveSheet(c, r.PathValue("uid"), copying, s.moveTarget(c, q.Get("to"), q.Get("into")))
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
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
		s.moveOnServer(w, r, c, dest, uid, obj)
		return
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
		s.finishWrite(w, r, dest, uid, sent, dest.recordPath(uid))
		return
	}
	got, err := dest.find(ctx, uid)
	if err != nil {
		dest.noteWrite(dst, sent)
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.keptNotice(dest, uid, "Copied, and the original kept",
			"The copy could not be read back to check it, so the "+c.spec.noun+" was not deleted here. Both are there now.")))
		return
	}
	dest.noteWrite(got.Path, got)
	if loss := model.CompareAfterPut(dest.conn.ID, sent, got, c.sess.Losses()); loss != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, s.keptNotice(dest, uid, "Copied, and the original kept",
			"The server of "+collectionLabel(dest.col)+" did not keep "+strings.Join(loss.Properties, ", ")+
				". The original stays here, so nothing is lost; delete it yourself if the copy is enough.")))
		return
	}
	err = c.remove(ctx, obj.Path, obj.ETag)
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		cur, _ := c.find(ctx, uid)
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, cur, s.keptNotice(dest, uid, "Copied, but the original changed, so it stays",
			"The "+c.spec.noun+" changed here after Carrel read it. The copy holds the version from before; both are there now.")))
	case err != nil:
		n := s.keptNotice(dest, uid, "Copied, but the original could not be deleted",
			"Both are there now. Delete the original yourself once the server lets you.")
		n.Details = moveFailure(c, "", "DELETE", obj.Path, err).Details
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, n))
	default:
		c.noteWrite(obj.Path, nil)
		s.redirect(w, r, dest.recordPath(uid))
	}
}

// moveOnServer is a MOVE within one connection. The name stays; a name taken
// there (412 with Overwrite: F) becomes {uid}-2 once.
func (s *Server) moveOnServer(w http.ResponseWriter, r *http.Request, c, dest *collectionCtx, uid string, obj *model.Object) {
	ctx := r.Context()
	var dst string
	var err error
	for _, name := range []string{path.Base(obj.Path), fileStem(uid) + "-2" + c.spec.ext} {
		dst = strings.TrimSuffix(dest.col.Path, "/") + "/" + name
		if err = c.t.Move(ctx, obj.Path, dst, false); !dav.IsStatus(err, http.StatusPreconditionFailed) {
			break
		}
	}
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, moveFailure(c, "Nothing was moved", "MOVE", obj.Path+" → "+dst, err)))
		return
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
		return
	}
	dest.noteWrite(got.Path, got)
	s.redirect(w, r, dest.recordPath(uid))
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
