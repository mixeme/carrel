package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// The attachments of a published note (Пб3, s-publish-4, -8). The sheet
// says how many there are and asks whether they go too; ticked, each gets
// its own link, made with the note's, expiring and revoked with it, its
// secret derived from the note's (user.ChildToken) — so the note's page can
// point at each without any secret being kept. Unticked, the page says how
// many files there are and not their names: a name gives away more than it
// looks. A file written into the note is opened by its place among the
// note's attachments, and only while the file there is the one published —
// the same name and size —: a place a removal shifted is no door to another
// file. A file the note links to in a file collection is published as that
// file is.

// noteFile is one attachment of a note as a link can carry it.
type noteFile struct {
	a    cal.Attachment
	name string
	// conn, col and segs are where a linked file lies; inline is a file
	// written into the note. ok is false for a link that is not into a file
	// collection of the catalog: it is not published.
	conn   sealed.Connection
	col    sealed.Collection
	segs   []string
	inline bool
	ok     bool
}

// noteFiles are the attachments of obj in their order.
func (s *Server) noteFiles(sess *session.Session, obj *model.Object) []noteFile {
	var out []noteFile
	var blob sealed.Blob
	loaded := false
	for _, a := range cal.Attachments(obj) {
		f := noteFile{a: a, inline: a.Inline, ok: a.Inline}
		if a.Inline {
			f.name = a.FileName
			if f.name == "" {
				f.name = fmt.Sprintf("Attachment %d", a.Index+1)
			}
		} else {
			f.name = linkName(a)
			if sess != nil {
				if !loaded {
					blob, loaded = s.catalogOf(sess), true
				}
				if account, col, segs, ok := attachLinkAt(blob, a.URI); ok {
					f.conn, _ = blob.Connection(account)
					f.col, f.segs, f.ok = col, segs, true
				}
			}
		}
		out = append(out, f)
	}
	return out
}

// attachmentsSet is the sheet's question of them (s-publish-4): how many
// and how large, and whether they go too.
func attachmentsSet(files []noteFile) view.FieldSet {
	aside := fmt.Sprintf("%d file", len(files))
	if len(files) != 1 {
		aside += "s"
	}
	var total int64
	known := true
	for _, f := range files {
		if f.a.Size < 0 {
			known = false
			break
		}
		total += f.a.Size
	}
	if known {
		aside += " · " + string(formatSize(total))
	}
	return view.FieldSet{Legend: "Attachments", LegendAside: text(aside), Rows: []view.FormRow{{
		Name: "attachments", Label: "Publish them too — each gets its own link", Kind: view.InputCheck,
		Hint: "The links are created together with this one, expire with it and are revoked with it. " +
			"Without this the page says how many files there are, but not their names: a file name gives away more than it looks.",
	}}}
}

// withAttachments puts the question into the sheet of a note that has
// attachments, after what it publishes and before its terms.
func (s *Server) withAttachments(ctx context.Context, sess *session.Session, c *collectionCtx, uid string, sheet *view.PublishSheet) {
	if c.spec.kind != view.KindNote || uid == "" {
		return
	}
	obj, err := c.find(ctx, uid)
	if err != nil {
		return
	}
	files := s.noteFiles(sess, obj)
	if len(files) == 0 || len(sheet.Form.Sets) == 0 {
		return
	}
	// The CSRF and nothing else first, the question, then the terms.
	first := sheet.Form.Sets[0]
	var head, terms []view.FormRow
	for _, row := range first.Rows {
		if row.Name == csrfFormField {
			head = append(head, row)
		} else {
			terms = append(terms, row)
		}
	}
	sets := []view.FieldSet{{Rows: head}, attachmentsSet(files), {Rows: terms}}
	sheet.Form.Sets = append(sets, sheet.Form.Sets[1:]...)
}

// attachmentChildren are the links that go with a note's when its sheet
// ticked them: each file that can be published.
func (s *Server) attachmentChildren(ctx context.Context, sess *session.Session, c *collectionCtx, uid string) []user.Child {
	obj, err := c.find(ctx, uid)
	if err != nil {
		return nil
	}
	var out []user.Child
	for _, f := range s.noteFiles(sess, obj) {
		switch {
		case !f.ok:
		case f.inline:
			out = append(out, user.Child{Conn: c.conn, Target: user.PublishTarget{
				AccountID: c.conn.ID, Kind: string(view.KindNote), Col: c.col.Path, UID: uid, Attach: f.a.Index + 1,
				AttachSize: f.a.Size, Title: f.name,
			}})
		default:
			out = append(out, user.Child{Conn: f.conn, Target: user.PublishTarget{
				AccountID: f.conn.ID, Kind: string(view.KindFile), Col: f.col.Path, Path: strings.Join(f.segs, "/"), Title: f.name,
			}})
		}
	}
	return out
}

// guestAttachments is what the note's page says of its attachments: with
// the links made with it, each file by its name and size, a door to its own
// link (s-publish-8); without, how many there are.
func (s *Server) guestAttachments(r *http.Request, link user.OpenLink, token string, obj *model.Object, page *view.PublishedPage) {
	files := s.noteFiles(nil, obj)
	if len(files) == 0 {
		return
	}
	if link.Target.Children == 0 {
		// How many, not which (s-publish-23): a file's name gives away more
		// than it looks.
		n := "1 attached file is"
		if len(files) != 1 {
			n = fmt.Sprintf("%d attached files are", len(files))
		}
		page.AttachNote = text(n + " not published.")
		return
	}
	for n := 0; n < link.Target.Children; n++ {
		child := user.ChildToken(token, n)
		t, ok := s.users.LookPublished(child)
		if !ok {
			continue
		}
		for _, f := range files {
			if !publishedAs(t.Target, f, link.Target.UID) {
				continue
			}
			row := view.PublishedAttachment{Name: text(f.name), Href: s.route("/p/" + child)}
			if f.a.Size >= 0 {
				row.Size = formatSize(f.a.Size)
			}
			page.Attachments = append(page.Attachments, row)
			break
		}
	}
}

// publishedAs is whether the child link t opens the attachment f of the note
// uid: a file written into the note at the same place, with the name and
// the size it was published with; a linked file by its path.
func publishedAs(t user.PublishTarget, f noteFile, uid string) bool {
	if f.inline {
		return t.Kind == string(view.KindNote) && t.UID == uid && t.Attach == f.a.Index+1 && t.Title == f.name && t.AttachSize == f.a.Size
	}
	u, err := url.Parse(f.a.URI)
	return err == nil && t.Kind == string(view.KindFile) && t.Title == f.name && strings.HasSuffix(u.Path, "/"+t.Path)
}

// publishedAttachment is the page of a file written into a published note,
// or with download the file (s-publish-9 for a file of Files): read from
// the note on every visit, the one it was published as or nothing.
func (s *Server) publishedAttachment(w http.ResponseWriter, r *http.Request, link user.OpenLink, download bool) {
	c, ok := s.openPublished(w, r, link)
	if !ok {
		return
	}
	page := view.PublishedPage{Plain: view.Plain{Title: text(link.Target.Title)}, Kind: view.KindFile, ReadOnly: true, Foot: publicFoot(c.tf, link)}
	obj, err := c.find(r.Context(), link.Target.UID)
	var data []byte
	var a cal.Attachment
	if err == nil {
		a, data, err = cal.AttachmentData(obj, link.Target.Attach-1)
		files := s.noteFiles(nil, obj)
		if err == nil && (link.Target.Attach > len(files) || !publishedAs(link.Target, files[link.Target.Attach-1], link.Target.UID)) {
			err = cal.ErrNoAttachment
		}
	}
	switch {
	case err != nil && (notFoundErr(err) || err == cal.ErrNoAttachment):
		page.Notices = []view.Notice{publicGone()}
		_ = s.users.MarkLinkGone(link.ID)
		s.writePage(w, r, http.StatusOK, page)
		return
	case err != nil:
		page.Notices = []view.Notice{publicTrouble()}
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	name := link.Target.Title
	if download {
		downloadHeaders(w.Header(), name, a.Type, r.URL.Query().Get("inline") != "")
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}
	href := s.route("/p/" + r.PathValue("token") + "/download")
	mt := fileType(name, a.Type)
	page.File = &view.PublishedFile{Name: text(name), Size: formatSize(int64(len(data))), Type: text(mt), Preview: view.PreviewNone, Href: href}
	page.Lede = runs(monoRun(mt + " · " + string(page.File.Size)))
	if inlineTypes[mt] {
		page.File.Preview, page.File.Href = view.PreviewImage, href+"?inline=1"
	}
	label := view.Text("Download")
	if page.File.Preview == view.PreviewNone {
		label += " " + view.Text(page.File.Size)
	}
	page.Actions = []view.Action{{ID: "published.download", Label: label, Icon: "i-download", Href: href, Method: "GET", Primary: true, Enabled: true}}
	s.writePage(w, r, http.StatusOK, page)
}

// noteVisitorAlways are what of a note its guest has: its title, its text
// and its days — a new note built of them, never one cut down from the
// record (S-25.1). Its tags, its attachments, a property another client
// wrote or one Carrel will learn of later are not in it.
var noteVisitorAlways = map[string]bool{
	"UID": true, "DTSTAMP": true, "DTSTART": true, "CREATED": true, "LAST-MODIFIED": true, "SUMMARY": true, "DESCRIPTION": true,
}

// visitorNote is a note as the guest of a link may have it; nil — it could
// not be built, and the guest gets nothing of it.
func visitorNote(o *model.Object) *model.Object {
	if o == nil {
		return nil
	}
	out, err := cal.Visitor(o, func(p string) bool { return noteVisitorAlways[strings.ToUpper(p)] }, "")
	if err != nil {
		return nil
	}
	return out
}

// visitorNotes are the notes the guest may have; one that could not be
// built is left out.
func visitorNotes(objs []*model.Object) []*model.Object {
	var out []*model.Object
	for _, o := range objs {
		if v := visitorNote(o); v != nil {
			out = append(out, v)
		}
	}
	return out
}
