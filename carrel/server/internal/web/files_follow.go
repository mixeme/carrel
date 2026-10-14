package web

import (
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// A folder Carrel keeps something in goes on being that folder when it is
// renamed or moved in Files, and is not deleted unseen (Б42 of the audit of
// 11.10.2026): the folder of attachments set in settings (S-20) — renamed
// or moved, the setting names its new place; the sheet of a deletion says
// that the folder holds the attachments, and which published links open it.

// under says whether segs is prefix or inside it.
func under(segs, prefix []string) bool {
	return len(segs) >= len(prefix) && slices.Equal(segs[:len(prefix)], prefix)
}

// afterPlaced follows a folder moved or renamed by a row of the queue: the
// attachment folder inside it names its new place.
func (s *Server) afterPlaced(sess *session.Session, t *transfer) {
	if t.state != view.OpDone || !t.dir || (t.kind != "move" && t.kind != "rename") {
		return
	}
	blob := s.catalogOf(sess)
	att := blob.Attachments
	if att == nil || att.AccountID != t.srcAccount || att.Path != t.srcCol || !under(att.Folder, t.src) {
		return
	}
	to := append(append(slices.Clone(t.folder), t.rel...), t.name)
	next := sealed.AttachmentFolder{AccountID: t.account, Path: t.col, Folder: append(to, att.Folder[len(t.src):]...), Monthly: att.Monthly}
	key, _ := sess.DataKey()
	_ = s.users.SetAttachmentFolder(sess.AccountID(), key, next)
}

// deletionNotes are what the sheet of a deletion adds to its warning: a
// folder that holds the attachment folder, and the published links that
// open what is deleted.
func (s *Server) deletionNotes(c *fileCtx, items []file.Entry) []string {
	blob := s.catalogOf(c.sess)
	var notes []string
	links := 0
	for _, e := range items {
		item := append(slices.Clone(c.segs), e.Name)
		if att := blob.Attachments; e.Dir && att != nil && att.AccountID == c.conn.ID && att.Path == c.col.Path && under(att.Folder, item) {
			notes = append(notes, "«"+e.Name+"» holds the folder of attachments: the next large attachment makes it again, empty.")
		}
		for _, l := range s.users.PublishedLinks(c.sess.AccountID()) {
			if l.Target.Kind == string(view.KindFile) && l.Target.AccountID == c.conn.ID && l.Target.Col == c.col.Path &&
				under(strings.Split(l.Target.Path, "/"), item) {
				links++
			}
		}
	}
	switch {
	case links == 1:
		notes = append(notes, "A published link opens what is deleted: it stops opening.")
	case links > 1:
		notes = append(notes, plural(links, "published link")+" open what is deleted: they stop opening.")
	}
	return notes
}
