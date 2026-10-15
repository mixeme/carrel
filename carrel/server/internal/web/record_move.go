package web

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
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
	// The sheet's form names the collection in one value (s-contacts-2:
	// «connection/collection», TargetOption.Value); its links in two.
	if account, col, ok := strings.Cut(to, "/"); ok {
		unescaped, err := url.PathUnescape(col)
		if err != nil {
			return nil
		}
		to, into = account, unescaped
	}
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

// moveRank is where a collection stands among its server's in the move
// sheet: the one the record is in, the writable ones, the read-only ones.
func moveRank(o view.TargetOption) int {
	switch {
	case o.Here:
		return 0
	case o.ReadOnly:
		return 2
	}
	return 1
}

// moveValue is how the sheet's form names a collection (TargetOption.Value).
func moveValue(account, col string) string {
	return account + "/" + url.PathEscape(col)
}

// moveWarned names what the sheet's Warning says will happen: a move inside
// one server, one to another account, a copy, a copy with a UID of its own
// (moveSheet). A form whose collection needs another word than the one it
// was shown is answered with the sheet again, that collection chosen and its
// word said, before anything is written (S-18: Carrel says it before it
// starts).
func moveWarned(c, dest *collectionCtx, copying bool) string {
	switch {
	case copying && dest != nil && knownOneUIDPerAccount(c, dest):
		return "copy-uid"
	case copying:
		return "copy"
	case dest != nil && dest.conn.ID != c.conn.ID:
		return "account"
	}
	return "server"
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
func (s *Server) moveSheet(c *collectionCtx, uid, title string, copying bool, dest *collectionCtx, invited bool) view.MoveSheet {
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
		from := len(sheet.Target.Options)
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(c.spec.kind) {
				continue
			}
			o := origin{conn: conn, col: col}
			opt := view.TargetOption{
				ID:    s.postQuery(address, moveQuery(copying, conn.ID, col.Path)),
				Label: text(collectionLabel(col)), Stamp: o.stamp(), Colour: catalogColour(col),
				Value: moveValue(conn.ID, col.Path),
			}
			switch {
			case conn.ID == c.conn.ID && col.Path == c.col.Path:
				opt.ReadOnly, opt.Reason, opt.Here = true, "already there", true
			case col.ReadOnly:
				opt.ReadOnly, opt.Reason = true, "read-only"
			default:
				opt.Href = opt.ID // the sheet with it chosen (U-8)
			}
			sheet.Target.Options = append(sheet.Target.Options, opt)
		}
		// Within a server, the one it is in first, then where it can go,
		// then the read-only ones (s-contacts-2).
		slices.SortStableFunc(sheet.Target.Options[from:], func(a, b view.TargetOption) int {
			return moveRank(a) - moveRank(b)
		})
	}
	// The sheet is a form (s-contacts-2): the collection is chosen in it and
	// goes under Field; Move is its button. A collection chosen by its link
	// is chosen in the form as well.
	sheet.Target.Field = "to"
	confirm := view.Action{ID: "record.move.confirm", Label: "Move", Method: "POST", Primary: true, Enabled: true, Submits: true}
	if copying {
		confirm.ID, confirm.Label = "record.copy.confirm", "Copy"
	}
	// Nothing chosen yet, the first collection it can go to is (s-contacts-2
	// draws «Архив» chosen): the form always names one, and Move is never a
	// press refused after the fact.
	if dest == nil {
		if i := slices.IndexFunc(sheet.Target.Options, func(o view.TargetOption) bool { return !o.ReadOnly }); i >= 0 {
			dest = s.moveTarget(c, sheet.Target.Options[i].Value, "")
		}
	}
	if reason := recordMoveBlock(c, dest, copying); reason != "" {
		// Nothing it can go to, or a closed one chosen by its address: the
		// button says why before it is pressed.
		confirm.Enabled, confirm.Reason = false, text(reason)
	} else {
		chosen := moveQuery(copying, dest.conn.ID, dest.col.Path)
		sheet.Target.Chosen = s.postQuery(address, chosen)
		confirm.Href = s.postQuery(address, chosen)
	}
	hidden := []view.FormRow{csrfField(c.sess), {Name: "warned", Kind: view.InputHidden, Value: text(moveWarned(c, dest, copying))}}
	if copying {
		hidden = append(hidden, view.FormRow{Name: "copy", Kind: view.InputHidden, Value: "1"})
	}
	sheet.Form = view.Form{Action: s.route(address), Method: "POST", Sets: []view.FieldSet{{Rows: hidden}}}
	record, where := recordNouns[c.spec.kind], moveCollectionNouns[c.spec.kind]
	switch {
	case copying && dest != nil && knownOneUIDPerAccount(c, dest):
		sheet.Warning = line("A copy is written into the chosen " + where + " whole, with a UID of its own: this server holds one record per UID across the account, " +
			"so for other apps the copy is another " + record + ". The " + record + " here stays as it is.")
	case copying:
		sheet.Warning = line("A copy is written into the chosen " + where + " whole, with the same UID; the " + record + " here stays as it is.")
	case dest != nil && dest.conn.ID != c.conn.ID:
		sheet.Warning = line("This goes to another account, so nothing moves on a server: Carrel reads the " + c.spec.noun +
			" whole, writes it there, reads it back, and deletes the original only if the copy kept everything.")
	default:
		// MOVE monospaced (s-contacts-2).
		sheet.Warning = join("Inside one server this is ", monoRun("MOVE"), ": instant, no traffic. To another "+where+" the "+record+
			" is copied and then deleted — Carrel says so before it starts, not after.")
	}
	// What the person may not have thought of (the owner's decision of
	// 12.10.2026, А12 of the audit of 11.10.2026): the record leaves a
	// server that schedules, and that server may tell its attendees the
	// event is cancelled. «May»: whether a letter goes out is the server's
	// setting, which Carrel does not see. Whether the place it goes to
	// invites them is the person's own choice of a collection, and is not
	// said.
	if invited && !copying && dest != nil && dest.conn.ID != c.conn.ID && c.conn.Scheduling == discover.SchedulingYes {
		sheet.Cancels = text(connLabel(c.conn) + " may tell its attendees that the " + c.spec.noun + " is cancelled.")
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
	title, invited := "", false
	if obj, err := c.find(r.Context(), uid); err == nil {
		title = recordTitle(c.spec.kind, obj)
		invited = c.cal != nil && len(cal.Attendees(obj)) > 0
	}
	sheet := s.moveSheet(c, uid, title, copying, s.moveTarget(c, q.Get("to"), q.Get("into")), invited)
	// Opened by its address, the sheet stands over the screen it came from
	// (s-contacts-2), else over the record's page.
	sheet.Under = s.underOf(r, q.Get("back"), c.recordPath(uid))
	if back := q.Get("back"); back != "" {
		sheet.Form.Sets[0].Rows = append(sheet.Form.Sets[0].Rows, view.FormRow{Name: "back", Kind: view.InputHidden, Value: text(back)})
	}
	s.writeSheetOf(w, r, sheet)
}

// writeSheetOf answers the move sheet: the zone of the sheet to a fragment
// request for it, else the page.
func (s *Server) writeSheetOf(w http.ResponseWriter, r *http.Request, sheet view.MoveSheet) {
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
	// The sheet's form said another word than this collection needs: the
	// sheet again, with it chosen and said (S-18).
	if warned := r.FormValue("warned"); warned != "" && dest != nil && warned != moveWarned(c, dest, copying) {
		title, invited := "", false
		if obj, err := c.find(r.Context(), uid); err == nil {
			title = recordTitle(c.spec.kind, obj)
			invited = c.cal != nil && len(cal.Attendees(obj)) > 0
		}
		sheet := s.moveSheet(c, uid, title, copying, dest, invited)
		sheet.Under = s.underOf(r, r.FormValue("back"), c.recordPath(uid))
		s.writeSheetOf(w, r, sheet)
		return
	}
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
	// A copy inside an account that holds one record per UID across all its
	// collections (SV-55: iCloud) cannot carry the UID the original holds, so
	// it gets one of its own. Such an account is known once the session has
	// seen it refuse; until then the copy goes with its UID, as everywhere.
	fresh := ""
	names := []string{path.Base(obj.Path), fileStem(uid) + "-2" + c.spec.ext}
	if copying && knownOneUIDPerAccount(c, dest) {
		if sent, fresh, err = withOwnUID(dest, sent); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		names = []string{fileStem(fresh) + c.spec.ext}
	}
	var dst string
	for i := 0; i < len(names); i++ {
		dst = strings.TrimSuffix(dest.col.Path, "/") + "/" + names[i]
		sent.Path = dst
		err = dest.place(ctx, dst, sent)
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
		// 412 on a name nothing holds is not about the name: the account
		// holds this UID in another collection (SV-55). A server that says
		// 412 for a taken name goes on to «-2» as before.
		if copying && fresh == "" && i == 0 && nameIsFree(ctx, dest, dst) {
			c.sess.NoteRefusal(dest.conn.ID, "", refusedSameUID)
			if sent, fresh, err = withOwnUID(dest, sent); err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			names = []string{names[0], fileStem(fresh) + c.spec.ext}
		}
	}
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, moveFailure(dest, nothing, "PUT", dst, err)))
		return
	}
	if fresh != "" {
		s.copiedWithOwnUID(w, r, c, dest, uid, obj, fresh, sent)
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
	if loss := writeLoss(dest.conn.ID, sent, got, c.sess.Losses(), landed != uid); loss != nil {
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
		// drop out of it without a word. Its links go with it (Б41).
		c.noteWrite(obj.Path, nil)
		s.followLinks(c.sess, placeOf(c, uid), dest, landed)
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
		s.followLinks(c.sess, placeOf(c, uid), dest, uid)
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
				s.followLinks(c.sess, placeOf(c, uid), dest, renamed)
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
		// Where it went under which UID is not known: its links cannot
		// follow it (Б41).
		s.linksGone(c.sess, placeOf(c, uid))
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
	s.followLinks(c.sess, placeOf(c, uid), dest, uid)
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

// refusedSameUID is what the session keeps of an account that holds one
// record per UID across all its collections (docs/servers.md, SV-55), with
// the whole account for its collection.
const refusedSameUID = "uid-account-wide"

// knownOneUIDPerAccount reports a copy within one connection whose account
// the session has seen refuse a UID it holds elsewhere: the original's UID is
// held there by the original itself.
func knownOneUIDPerAccount(c, dest *collectionCtx) bool {
	return c.sess != nil && dest.conn.ID == c.conn.ID && c.sess.Refused(dest.conn.ID, "", refusedSameUID)
}

// nameIsFree reports that nothing answers at p.
func nameIsFree(ctx context.Context, dest *collectionCtx, p string) bool {
	_, err := dest.fetch(ctx, p)
	return notFoundErr(err) || dav.IsStatus(err, http.StatusNotFound)
}

// withOwnUID is the copy about to be written, under a fresh UID.
func withOwnUID(dest *collectionCtx, sent *model.Object) (*model.Object, string, error) {
	fresh, err := model.NewUID()
	if err != nil {
		return nil, "", err
	}
	got, err := setUID(dest, sent, fresh)
	return got, fresh, err
}

// copiedWithOwnUID answers a copy written under a UID of its own (SV-55):
// the original's page, saying what was done, with the copy one link away —
// the way a move that came out under another identity is told (SV-18). What
// pointed at the original points at it still: the original stays.
func (s *Server) copiedWithOwnUID(w http.ResponseWriter, r *http.Request, c, dest *collectionCtx, uid string, obj *model.Object, fresh string, sent *model.Object) {
	body := "The server holds one record per UID across the whole account, so the copy in " + collectionLabel(dest.col) +
		" has a UID of its own. For other apps it is another " + c.spec.noun + ", not this one twice; duplicates are matched by that identifier."
	got, err := dest.find(r.Context(), fresh)
	if err != nil {
		dest.noteWrite(sent.Path, sent)
	} else {
		dest.noteWrite(got.Path, got)
		if loss := writeLoss(dest.conn.ID, sent, got, c.sess.Losses(), false); loss != nil {
			body += " That server did not keep " + strings.Join(loss.Properties, ", ") + " in the copy; the original still holds it."
		}
	}
	s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, obj, view.Notice{
		Role: view.NoticeAlert, Title: "Copied, with a UID of its own", Body: text(body),
		Actions: []view.Action{s.openThere(dest, fresh, "Open it there")},
	}))
}

// noMoveHere reports a server that has no MOVE rather than one that refused
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
