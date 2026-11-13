package web

import (
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// A folder Carrel keeps something in goes on being that folder when it is
// renamed or moved in Files, and is not deleted unseen (Б42 of the audit of
// 11.10.2026): the folder of attachments set in settings (S-20) — renamed
// or moved, the setting names its new place; the sheet of a deletion says
// that the folder holds the attachments, and which published links open it.
// A published file or folder goes on being opened by its link (server
// v0.2.410): renamed or moved, the link opens it at its new place, as a
// record's link follows its move (Б41).

// under says whether segs is prefix or inside it.
func under(segs, prefix []string) bool {
	return len(segs) >= len(prefix) && slices.Equal(segs[:len(prefix)], prefix)
}

// afterPlaced follows a file or a folder moved or renamed by a row of the
// queue: the published links to it or to anything in it open it at its new
// place, read with the credentials of the connection it is on now; the
// attachment folder inside it names its new place.
func (s *Server) afterPlaced(sess *session.Session, t *transfer) {
	if t.state != view.OpDone || (t.kind != "move" && t.kind != "rename") {
		return
	}
	blob := s.catalogOf(sess)
	to := append(append(slices.Clone(t.folder), t.rel...), t.name)
	if conn, ok := blob.Connection(t.account); ok {
		_, _ = s.users.FollowPublishedFiles(sess.AccountID(),
			user.FilePlace{AccountID: t.srcAccount, Col: t.srcCol, Path: t.src},
			user.FilePlace{AccountID: t.account, Col: t.col, Path: to}, conn)
	}
	att := blob.Attachments
	if !t.dir || att == nil || att.AccountID != t.srcAccount || att.Path != t.srcCol || !under(att.Folder, t.src) {
		return
	}
	next := sealed.AttachmentFolder{AccountID: t.account, Path: t.col, Folder: append(to, att.Folder[len(t.src):]...), Monthly: att.Monthly}
	key, _ := sess.DataKey()
	_ = s.users.SetAttachmentFolder(sess.AccountID(), key, next)
}

// deletionRefs are what points at what a deletion takes, the sheet's «What
// is pointing at it now» as a collection's (s-files-22): the folder of
// attachments inside a folder deleted, and the published links that open
// what is deleted, counted.
func (s *Server) deletionRefs(c *fileCtx, items []file.Entry) []view.Reference {
	blob := s.catalogOf(c.sess)
	var refs []view.Reference
	links := 0
	for _, e := range items {
		item := append(slices.Clone(c.segs), e.Name)
		if att := blob.Attachments; e.Dir && att != nil && att.AccountID == c.conn.ID && att.Path == c.col.Path && under(att.Folder, item) {
			// The owner's words of 03.11.2026, in the form of the collection's
			// row (s-profile-19).
			refs = append(refs, view.Reference{What: "Attachments", Then: "the attachment folder is in it; the next attachment too large to go inside its record creates it again, empty, at the same path"})
		}
		for _, l := range s.users.PublishedLinks(c.sess.AccountID()) {
			if l.Target.Kind == string(view.KindFile) && l.Target.AccountID == c.conn.ID && l.Target.Col == c.col.Path &&
				under(strings.Split(l.Target.Path, "/"), item) {
				links++
			}
		}
	}
	if links > 0 {
		refs = append(refs, view.Reference{What: text(plural(links, "published link")), Then: linkThen})
	}
	return refs
}
