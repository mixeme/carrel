package web

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Which copy of a linked record (from-typecase note 57, Typecase's Р-58; the
// owner's decisions of 11 and 13.10.2026). A record linked by the duplicates
// screen stands in several collections, and a change goes to one copy, or
// to the copies chosen: Edit of such a record opens «Which copy to edit?»
// first (s-contacts-32, one sheet for every kind), and the tick of a linked
// task in the combined list «Which copy to mark done?» (s-tasks-16). The
// form of the copy chosen says, before its fields, that it saves that copy
// only (s-contacts-31). A record in no group has no sheet: its Edit opens
// the form, its tick marks it.

// copyField is the name the sheets post a copy under.
const copyField = "copy"

// copyValue is how a sheet names a copy: its connection and its collection,
// «a-…/%2F…%2F», the segments of its address.
func copyValue(ref sealed.RecordRef) string {
	return ref.AccountID + "/" + url.PathEscape(ref.Col)
}

// copyHolders are what holds a record, by kind, for the line under a
// sheet's head: «This person stands in three books.»
var copyHolders = map[view.Kind][2]string{
	view.KindContact: {"person", "books"}, view.KindEvent: {"event", "calendars"},
	view.KindTask: {"task", "lists"}, view.KindNote: {"note", "notebooks"},
}

// copiesSheet is the sheet of the copies of the record opened in c: the
// form of Edit (done false) or of Mark done. False when the record is in no
// linked group, or none of its other copies is there any more.
func (s *Server) copiesSheet(ctx context.Context, c *collectionCtx, uid string, obj *model.Object, done bool, from string) (view.CopiesSheet, bool) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if !ok {
		return view.CopiesSheet{}, false
	}
	n := len(lc.others) + 1
	holders := copyHolders[c.spec.kind]
	sheet := view.CopiesSheet{Kind: c.spec.kind, Subject: text(recordTitle(c.spec.kind, obj) + " · " + strconv.Itoa(n) + " copies")}
	stands := "This " + holders[0] + " stands in " + countWord(n) + " " + holders[1] + ". "
	// Each copy says what it holds otherwise and, after «·», what it holds
	// of the first such thing (s-contacts-32: «Email differ ·
	// v.lebedeva@example.org»; s-tasks-16: «Due differ · due 13 Aug»); the
	// copy opened, what it holds of the first thing another holds otherwise —
	// on Edit after «the copy you opened».
	differ := differs(differTable(c.spec.kind))
	opened := ""
	for _, m := range lc.others {
		if d := differ(obj, m.obj); m.obj != nil && len(d) > 0 {
			opened = copyHolds(c, obj, d[0])
			break
		}
	}
	self := view.Option{Value: copyValue(lc.self), Label: text(string(c.stamp())), Colour: catalogColour(c.col), Selected: true, Meta: "the copy you opened"}
	row := view.FormRow{Name: copyField, Label: "Copy", Kind: view.InputRadio}
	why := "read-only: it cannot be edited"
	switch {
	case done:
		row.Kind, why = view.InputCheck, "read-only: it cannot be marked done"
		if opened != "" {
			self.Meta = text(opened)
		}
	case opened != "":
		self.Meta = text("the copy you opened · " + opened)
	}
	row.Options = []view.Option{self}
	for _, m := range lc.others {
		opt := view.Option{Value: copyValue(m.ref), Label: text(string(m.c.stamp())), Colour: catalogColour(m.c.col)}
		if m.obj != nil && (m.c.col.ReadOnly || m.c.feed != nil) {
			opt.Disabled, opt.Reason = true, text(why)
		}
		d := differ(obj, m.obj)
		switch {
		case m.obj == nil:
			opt.Meta, opt.Disabled = "The server did not answer; the record stays linked", true
		case len(d) == 0:
			opt.Meta = "Holds the same"
		default:
			// A copy that cannot be chosen says why after «·» instead
			// (s-contacts-32: «Phone differ · read-only: …»).
			opt.Meta = text(strings.Join(d, ", ") + " differ")
			if v := copyHolds(m.c, m.obj, d[0]); v != "" && !opt.Disabled {
				opt.Meta += text(" · " + v)
			}
		}
		row.Options = append(row.Options, opt)
	}
	if done {
		sheet.Title = "Which copy to mark done?"
		sheet.Note = text(stands + "Tick the copy that is done; a copy that is not ticked stays as it is.")
		sheet.Form = view.Form{Action: s.route(c.recordPath(uid) + "/done"), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
			csrfField(c.sess), {Name: "from", Kind: view.InputHidden, Value: text(from)}, row,
		}}}}
		sheet.Actions = []view.Action{
			{ID: "copies.done", Label: "Mark done", Method: "POST", Primary: true, Enabled: true, Submits: true},
			{ID: "copies.cancel", Label: "Cancel", Href: s.route(doneBack(c, uid, from)), Method: "GET", Enabled: true},
		}
		return sheet, true
	}
	sheet.Title = "Which copy to edit?"
	sheet.Note = text(stands + "Saving changes one copy only; the others stay as they are.")
	sheet.Form = view.Form{Action: s.route(c.recordPath(uid) + "/edit"), Method: "GET", Sets: []view.FieldSet{{Rows: []view.FormRow{row}}}}
	sheet.Actions = []view.Action{
		{ID: "copies.edit", Label: "Edit this copy", Method: "GET", Primary: true, Enabled: true, Submits: true},
		{ID: "copies.cancel", Label: "Cancel", Href: s.route(c.recordPath(uid)), Method: "GET", Enabled: true},
	}
	return sheet, true
}

// differTable is what the copies of a kind are compared by on a sheet: a
// task's time is its due date (s-tasks-16: «Due differ»; the card's rows
// keep «Time», as s-tasks-2 draws them).
func differTable(k view.Kind) []differProp {
	switch k {
	case view.KindContact:
		return contactDiffers
	case view.KindTask:
		return taskDiffers
	}
	return calendarDiffers
}

// taskDiffers are calendarDiffers with the time called by a task's name.
var taskDiffers = func() []differProp {
	out := slices.Clone(calendarDiffers)
	for i := range out {
		if out[i].label == "time" {
			out[i].label = "due"
		}
	}
	return out
}()

// copyHolds is what a copy holds of a thing it is compared by, as a sheet
// names it after «·»: the address, the number, the place, the due date
// («due 13 Aug»); "" for what has no short words — a photo, a text.
func copyHolds(c *collectionCtx, obj *model.Object, label string) string {
	first := func(prop string) string { return strings.TrimSpace(firstValue(obj, prop)) }
	switch strings.ToLower(label) {
	case "name":
		return first("FN")
	case "phone":
		return first("TEL")
	case "email":
		return first("EMAIL")
	case "organisation":
		return structured(first("ORG"))
	case "title":
		return first("SUMMARY")
	case "where":
		return first("LOCATION")
	case "priority":
		if p := priorityOf(obj); p <= 9 {
			return "priority " + strconv.Itoa(p)
		}
	case "tags":
		return strings.Join(categoriesOf(c.spec.kind, obj), ", ")
	case "due":
		if due, err := cal.DueOf(obj, c.tf.tz); err == nil && !due.Start.IsZero() {
			return "due " + c.tf.shortDue(due)
		}
	case "time":
		if tm, err := cal.TimingOf(obj, c.tf.tz); err == nil && !tm.Start.IsZero() {
			return c.tf.when(tm)
		}
	}
	return ""
}

// chosenCopy is the copy of the record in c a sheet named: the record
// itself, or another member of its group; false when the name is no copy.
func (s *Server) chosenCopy(ctx context.Context, c *collectionCtx, uid, value string, lc linkedCard) (*collectionCtx, string, bool) {
	if value == copyValue(lc.self) {
		return c, uid, true
	}
	for _, m := range lc.others {
		if value == copyValue(m.ref) {
			return m.c, m.ref.UID, true
		}
	}
	return nil, "", false
}

// copyEditTo is where Edit this copy goes: the form of another copy the
// sheet chose, naming that copy as the sheet did — the form says it then
// (copyNotice); "" — the form of the record opened (s-contacts-32).
func (s *Server) copyEditTo(r *http.Request, c *collectionCtx, uid string) string {
	value := r.URL.Query().Get(copyField)
	if value == "" {
		return ""
	}
	lc, ok := s.linkedOf(r.Context(), c, uid)
	if !ok {
		return ""
	}
	mc, muid, ok := s.chosenCopy(r.Context(), c, uid, value, lc)
	if !ok || mc == c {
		return ""
	}
	return mc.recordPath(muid) + "/edit?" + url.Values{copyField: {value}}.Encode()
}

// copyNotice is the line of the form of a linked record's copy opened from
// «Which copy to edit?», before its fields (s-contacts-31: «after the
// sheet»): the save goes to this copy only. nil — the form was not opened
// from the sheet, or the record is in no group (s-notes-20 draws the form
// of a linked note without it).
func (s *Server) copyNotice(r *http.Request, c *collectionCtx, uid string) *view.Notice {
	value := r.URL.Query().Get(copyField)
	if value == "" {
		return nil
	}
	lc, ok := s.linkedOf(r.Context(), c, uid)
	if !ok || value != copyValue(lc.self) {
		return nil
	}
	return &view.Notice{Role: view.NoticeInfo, Body: text("Saving changes this copy only — " + string(c.stamp()) + ". The others stay as they are.")}
}

// getKindCopies is «Which copy to edit?» (s-contacts-32). A record in no
// group goes to its form.
func (s *Server) getKindCopies(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid)+"/copies", err)))
		return
	}
	sheet, ok := s.copiesSheet(r.Context(), c, uid, obj, false, "")
	if !ok {
		s.redirect(w, r, c.recordPath(uid)+"/edit")
		return
	}
	s.writeCopies(w, r, sheet)
}

// writeCopies answers a sheet of copies: the zone of the sheet to a fragment request
// for it, else the page (C-7.2).
func (s *Server) writeCopies(w http.ResponseWriter, r *http.Request, sheet view.CopiesSheet) {
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// getTaskDone is «Which copy to mark done?» by its address (s-tasks-16). A
// task in no group goes to its card.
func (s *Server) getTaskDone(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openCollection(w, r, "tasks", true)
	if !ok {
		return
	}
	uid := r.PathValue("uid")
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid)+"/done", err)))
		return
	}
	sheet, ok := s.copiesSheet(r.Context(), c, uid, obj, true, r.URL.Query().Get("from"))
	if !ok {
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	s.writeCopies(w, r, sheet)
}

// postTaskDone is the tick of a linked task and the sheet it opens
// (s-tasks-16): without a copy named, the sheet, the copy the row opens
// ticked; with copies, each marked done as Mark done marks it — a copy not
// ticked stays as it is —, and the list again. A task in no group is marked
// at once.
func (s *Server) postTaskDone(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openCollection(w, r, "tasks", true)
	if !ok {
		return
	}
	uid := r.PathValue("uid")
	from := r.FormValue("from")
	if from == "" {
		from = r.URL.Query().Get("from")
	}
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.notFound(w, r)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.recordPage(c, uid, nil, s.unavailableNotice(c, c.recordPath(uid)+"/done", err)))
		return
	}
	r.FormValue(copyField) // parses the form
	chosen := r.Form[copyField]
	lc, linked := s.linkedOf(r.Context(), c, uid)
	if len(chosen) == 0 {
		if linked {
			sheet, _ := s.copiesSheet(r.Context(), c, uid, obj, true, from)
			s.writeCopies(w, r, sheet)
			return
		}
		chosen = []string{copyValue(sealed.RecordRef{AccountID: c.conn.ID, Col: c.col.Path, UID: uid})}
		lc.self = sealed.RecordRef{AccountID: c.conn.ID, Col: c.col.Path, UID: uid}
	}
	for _, value := range chosen {
		mc, muid, ok := s.chosenCopy(r.Context(), c, uid, value, lc)
		if !ok || mc.col.ReadOnly || mc.feed != nil {
			continue
		}
		got := obj
		if mc != c {
			if got, err = mc.find(r.Context(), muid); err != nil {
				s.writePage(w, r, http.StatusOK, s.recordPage(mc, muid, nil, s.unavailableNotice(mc, mc.recordPath(muid), err)))
				return
			}
		}
		if _, err := s.markTaskDone(r.Context(), mc, got); err != nil {
			s.writePage(w, r, http.StatusOK, s.recordPage(mc, muid, got, s.writeFailure(mc, "PUT", got.Path, err)))
			return
		}
		if left := s.closeSubtasks(r.Context(), mc, muid); left != nil {
			s.writePage(w, r, http.StatusOK, s.recordPage(mc, muid, nil, *left))
			return
		}
	}
	s.redirect(w, r, doneBack(c, uid, from))
}

// markTaskDone marks a task done as Mark done does — the Done box alone, the
// rest as the server holds it, with the version read — and says whether it
// is done now: one that repeats goes on to its next time and stays open
// (А2).
func (s *Server) markTaskDone(ctx context.Context, c *collectionCtx, obj *model.Object) (bool, error) {
	if taskDone(obj) {
		return true, nil
	}
	have, alarms, err := s.currentValues(c, obj)
	if err != nil {
		return false, err
	}
	want := formValues{}
	for k, v := range have {
		want[k] = append([]string(nil), v...)
	}
	want["done"] = []string{"on"}
	ch := s.buildChange(c, obj, alarms, have, want, time.Now())
	if len(ch.errors) > 0 {
		return false, errMarkDone
	}
	if len(ch.ops) == 0 && !ch.alarmsChanged {
		return taskDone(obj), nil
	}
	etag := obj.ETag
	sent, fault, err := s.applyChecked(c, obj, ch, have, want)
	switch {
	case err != nil:
		return false, err
	case fault != nil:
		return false, errMarkDone
	}
	sent.ETag = etag
	if err := c.update(ctx, sent); err != nil {
		return false, err
	}
	sent.ETag = ""
	c.noteWrite(sent.Path, sent)
	return taskDone(sent), nil
}

// errMarkDone is a task the Done box could not be written to as it is.
var errMarkDone = markDoneError("the task could not be marked done as it is")

type markDoneError string

func (e markDoneError) Error() string { return string(e) }

// linkedTick is the tick of a linked task's row in the combined list: the
// sheet of its copies (s-tasks-16), by the POST the tick already makes.
func (s *Server) linkedTick(from origin, segment, uid string) view.Action {
	return view.Action{ID: "task.done", Label: "Mark done", Method: "POST", Enabled: true,
		Href: s.postQuery(from.recordPath(segment, uid)+"/done", url.Values{"from": {"all"}})}
}

// editViaCopies sends Edit of a linked record's page to «Which copy to
// edit?» (s-contacts-32) instead of its form.
func (s *Server) editViaCopies(c *collectionCtx, uid string, actions []view.Action) {
	for i := range actions {
		if actions[i].ID == "record.edit" {
			actions[i].Href = s.route(c.recordPath(uid) + "/copies")
		}
	}
}
