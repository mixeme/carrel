package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// editDraft is a rejected edit (S-8): what the person changed, against which
// version, when they read it. It lives in DraftRejectedEdit and dies with the
// session.
type editDraft struct {
	Kind    view.Kind  `json:"kind"`
	Account string     `json:"account"`
	Col     string     `json:"col"`
	UID     string     `json:"uid"`
	ETag    string     `json:"etag"`
	ReadAt  time.Time  `json:"readAt"`
	Mine    formValues `json:"mine"`
	// Base is what the version the form was opened with held of the fields
	// changed (Mine's keys): with it the conflict screen tells a field both
	// changed from one only the person did (s-conflict-1, Рв2). Empty when
	// that version was no longer held (Unsure).
	Base formValues `json:"base,omitempty"`
	// Notebook is the notebook a note's edit chose other than its own
	// (s-notes-20): part of the edit, so the move waits with it and happens
	// once the person's edit is written (S-18).
	Notebook string `json:"notebook,omitempty"`
	// Unsure: the version the form was opened with was no longer held when
	// the edit was refused, so Mine was measured against the server's
	// version — and holds also what only the server changed (Б43 of the
	// audit of 11.10.2026). The conflict screen says so.
	Unsure bool `json:"unsure,omitempty"`
}

func (s *Server) saveDraft(c *collectionCtx, uid, etag string, readAt time.Time, mine, base formValues, notebook string, unsure bool) {
	held := formValues{}
	if !unsure {
		for k := range mine {
			held[k] = base[k]
			if v, ok := base[k+".type"]; ok {
				held[k+".type"] = v
			}
		}
	}
	data, err := json.Marshal(editDraft{
		Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, UID: uid, ETag: etag, ReadAt: readAt, Mine: mine, Base: held,
		Notebook: notebook, Unsure: unsure,
	})
	if err == nil {
		c.sess.SetDraft(session.DraftRejectedEdit, data)
	}
}

func (s *Server) loadDraft(c *collectionCtx, uid string) (editDraft, bool) {
	var d editDraft
	data := c.sess.Draft(session.DraftRejectedEdit)
	if len(data) == 0 || json.Unmarshal(data, &d) != nil {
		return d, false
	}
	return d, d.Kind == c.spec.kind && d.Account == c.conn.ID && d.Col == c.col.Path && d.UID == uid
}

func clearDraft(c *collectionCtx) {
	c.sess.SetDraft(session.DraftRejectedEdit, nil)
}

// conflictFields are the fields the person changed that the server version
// does not already hold, in form order. Row N of the conflict page is field N
// here; its choice comes back as choice.N.
func conflictFields(kind view.Kind, d editDraft, theirs formValues) []editField {
	var out []editField
	for _, f := range editFields[kind] {
		mine, changed := d.Mine[f.name]
		relabelled := f.typed && !sameLabels(d.Mine[f.name+".type"], theirs[f.name+".type"])
		typed := strings.TrimSpace(first(d.Mine[f.name+".add"])) != ""
		if changed && (!sameRows(mine, theirs[f.name]) || relabelled || typed) {
			out = append(out, f)
		}
	}
	return out
}

// conflictDiff is how a field differs (Рв2): both sides changed it, or only
// the person did — the server holds what the form was opened with;
// unknown without that version.
func conflictDiff(f editField, d editDraft, theirs formValues) view.ConflictDiff {
	if d.Unsure || d.Base == nil {
		return view.DiffUnknown
	}
	if _, held := d.Base[f.name]; !held {
		return view.DiffUnknown
	}
	if sameRows(d.Base[f.name], theirs[f.name]) && (!f.typed || sameLabels(d.Base[f.name+".type"], theirs[f.name+".type"])) {
		return view.DiffMine
	}
	return view.DiffBoth
}

// agreeing are the properties the person changed to what the server version
// already holds (s-conflict-3, Рв5): said above the cards, kept as they are.
func agreeing(kind view.Kind, d editDraft, theirs formValues) []view.Text {
	var out []view.Text
	for _, f := range editFields[kind] {
		mine, changed := d.Mine[f.name]
		if changed && sameRows(mine, theirs[f.name]) && (!f.typed || sameLabels(d.Mine[f.name+".type"], theirs[f.name+".type"])) {
			out = append(out, text(f.prop))
		}
	}
	return out
}

func (s *Server) showValues(c *collectionCtx, f editField, vals, labels []string, alarms []cal.Alarm) string {
	var parts []string
	switch f.typ {
	case fieldTime:
		row := triple(vals)
		switch {
		case row[0] == "":
		case row[2] == "on" || row[1] == "":
			parts = append(parts, row[0])
		default:
			parts = append(parts, row[0]+" "+row[1])
		}
	case fieldDone:
		parts = append(parts, map[bool]string{true: "Done", false: "Open"}[first(vals) == "on"])
	case fieldReminders:
		for i, v := range vals {
			if v == "" {
				continue
			}
			a := cal.NewAlarm(v)
			if i < len(alarms) && alarms[i].Trigger == v {
				a = alarms[i]
			}
			parts = append(parts, c.tf.reminder(a))
		}
	case fieldRepeat:
		v := append(append([]string(nil), vals...), make([]string, 6)...)[:6]
		for _, fl := range repeatFields(v) {
			parts = append(parts, string(fl.Label)+" "+string(fl.Value))
		}
	case fieldAddress:
		for _, v := range vals {
			var shown []string
			for _, p := range addressRowParts(v) {
				if p != "" {
					shown = append(shown, p)
				}
			}
			if len(shown) > 0 {
				parts = append(parts, strings.Join(shown, " "))
			}
		}
	default:
		for i, v := range vals {
			if strings.TrimSpace(v) == "" {
				continue
			}
			if i < len(labels) && labels[i] != "" {
				v += " · " + labels[i]
			}
			parts = append(parts, v)
		}
	}
	if len(parts) == 0 {
		// Nothing: the screen draws its dash (s-conflict-1, Рв3).
		return ""
	}
	return strings.Join(parts, ", ")
}

func (s *Server) conflictPage(c *collectionCtx, d editDraft, obj *model.Object, theirs formValues, alarms []cal.Alarm, notices []view.Notice) view.ConflictPage {
	now := time.Now()
	sh := s.buildShell(c.sess, "Changed elsewhere")
	sh.Rail = s.recordRail(c, s.listBack(c))
	sh.Notices = append(sh.Notices, notices...)
	page := view.ConflictPage{Shell: sh, Kind: c.spec.kind, UID: d.UID, ReadAt: c.tf.formatWhen(d.ReadAt, false, now)}
	path := c.recordPath(d.UID) + "/conflict"
	// Cancel leads back to the record (s-conflict-1, Рв8): the refused edit
	// kept in the session is overwritten by the next one, and goes with it.
	cancel := view.Action{ID: "conflict.cancel", Label: "Cancel", Href: s.route(c.recordPath(d.UID)), Method: "GET", Enabled: true}
	sh.Top.Refresh = s.recordRefresh(c, path)
	page.Shell = sh
	if obj == nil {
		page.Actions = []view.Action{cancel}
		return page
	}
	page.ServerAt = s.serverStamp(c, obj)
	// The screen says what happened before it asks anything, under its
	// title (s-conflict-1, Рв1): two columns of values do not by themselves
	// tell a person that their save was refused and that nothing was
	// written.
	page.Subtitle = text("Another client changed this " + string(c.spec.kind) +
		" while you were editing it. Nothing has been overwritten.")
	page.Agree = agreeing(c.spec.kind, d, theirs)
	for i, f := range conflictFields(c.spec.kind, d, theirs) {
		page.Rows = append(page.Rows, view.ConflictRow{
			// The record's own name for the property, not a label of ours
			// (s-conflict-1): a conflict is read against the file, and a
			// property Carrel does not model has no label to invent.
			Property: text(f.prop),
			Mine:     text(s.showValues(c, f, d.Mine[f.name], d.Mine[f.name+".type"], alarms)),
			Theirs:   text(s.showValues(c, f, theirs[f.name], theirs[f.name+".type"], alarms)),
			Choice:   view.ChoiceMine,
			Name:     choiceField(i),
			Diff:     conflictDiff(f, d, theirs),
		})
	}
	if d.Unsure && len(page.Rows) > 0 {
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: "Some of these may not be your changes",
			Body: "Carrel no longer holds the version you opened, so it cannot tell what you changed from what only the other client changed: " +
				"every field that differs is listed. Where you changed nothing, choose the server's value — «Use only my edit» would put the old one back."})
	}
	op := func(name string) string {
		return s.postQuery(path, url.Values{"op": {name}, "etag": {obj.ETag}})
	}
	// A note's edit that also chose another notebook: the move is part of
	// the edit and waits with it (S-18 says where a record goes before it
	// goes). Writing the person's edit moves it; the server version and
	// Cancel leave it where it is.
	moving := d.Notebook != "" && d.Notebook != c.listPath()
	if moving {
		where := d.Notebook
		if dest := s.notebookTarget(c, d.Notebook); dest != nil {
			where = string(dest.stamp())
		}
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: "Not moved yet",
			Body: text("Your edit also moves this " + c.spec.noun + " to " + where + ". Writing your edit or this combination moves it; " +
				"«Use only the server version» and Cancel leave it where it is.")})
	}
	writable := len(page.Rows) > 0 || moving
	// Without scripts the choice of every row is a form (U-8): its radios
	// carry their rows' names, and «Write this combination» sends it. The
	// other three need no choice and go as forms of their own.
	combine := view.Action{ID: "conflict.combine", Label: "Write this combination", Method: "POST", Primary: true, Enabled: writable}
	page.Form = &view.Form{Action: s.route(path), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		csrfField(c.sess),
		{Name: "etag", Kind: view.InputHidden, Value: text(obj.ETag)},
		{Name: "op", Kind: view.InputHidden, Value: "combine"},
	}}}}
	page.Actions = []view.Action{
		{ID: "conflict.mine", Label: "Use only my edit", Href: op("mine"), Method: "POST", Enabled: writable},
		{ID: "conflict.theirs", Label: "Use only the server version", Href: op("theirs"), Method: "POST", Enabled: true},
		cancel,
	}
	if !writable && !moving {
		combine.Reason, page.Actions[0].Reason = "The server version already has your change.", "The server version already has your change."
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: "Nothing differs", Body: "The server version already holds everything you changed."})
	}
	page.Form.Actions = []view.Action{combine}
	return page
}

// choiceField is the field of row n of the conflict's form.
func choiceField(n int) string { return fmt.Sprintf("choice.%d", n) }

// chosenSide is what the form says of a row: the side it names, else the
// one the page showed chosen (ConflictRow.Choice). A browser sends every
// radio that is checked; a row it did not send is not the person's edit by
// default — that was the page's guess, and the page said it.
func chosenSide(r *http.Request, n int, shown view.ConflictChoice) view.ConflictChoice {
	switch v := view.ConflictChoice(r.FormValue(choiceField(n))); v {
	case view.ChoiceMine, view.ChoiceTheirs:
		return v
	}
	return shown
}

func (s *Server) getKindConflict(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	uid := r.PathValue("uid")
	d, ok := s.loadDraft(c, uid)
	if !ok {
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	obj, err := c.find(r.Context(), uid)
	switch {
	case notFoundErr(err):
		s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, nil, nil, nil, []view.Notice{{
			Role: view.NoticeAlert, Title: "Deleted elsewhere", Body: "The record was deleted on the server while you edited it. Your edit cannot be written to it.",
		}}))
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, nil, nil, nil, []view.Notice{s.unavailableNotice(c, c.recordPath(uid)+"/conflict", err)}))
		return
	}
	theirs, alarms, err := s.currentValues(c, obj)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, nil, nil, nil, []view.Notice{readNotice(obj.Path, err)}))
		return
	}
	s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, obj, theirs, alarms, nil))
}

// postKindConflict writes the chosen combination onto the server version the
// page was built from, with If-Match of that version. If the server moved on
// again, the page is built anew; nothing is guessed.
func (s *Server) postKindConflict(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, true)
	if !ok {
		return
	}
	if c.col.ReadOnly {
		s.notFound(w, r)
		return
	}
	uid := r.PathValue("uid")
	here := c.recordPath(uid) + "/conflict"
	d, ok := s.loadDraft(c, uid)
	if !ok {
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	op := r.FormValue("op")
	switch op {
	case "cancel", "theirs":
		clearDraft(c)
		s.redirect(w, r, c.recordPath(uid))
		return
	case "combine", "mine":
	default:
		s.redirect(w, r, here)
		return
	}
	etag := r.FormValue("etag")
	obj, err := c.find(r.Context(), uid)
	if err != nil || obj.ETag != etag {
		s.redirect(w, r, here)
		return
	}
	theirs, alarms, err := s.currentValues(c, obj)
	if err != nil {
		s.redirect(w, r, here)
		return
	}
	want := formValues{}
	for k, v := range theirs {
		want[k] = v
	}
	for i, f := range conflictFields(c.spec.kind, d, theirs) {
		if op == "mine" || chosenSide(r, i, view.ChoiceMine) == view.ChoiceMine {
			want[f.name] = d.Mine[f.name]
			if f.typed {
				want[f.name+".type"] = d.Mine[f.name+".type"]
			}
			// What the form sent beside the field goes with it
			// (changedValues).
			if line, ok := d.Mine[f.name+".add"]; ok {
				want[f.name+".add"] = line
			}
			if box, ok := d.Mine[taskDayBox]; ok && f.taskDay {
				want[taskDayBox] = box
			}
		}
	}
	ch := s.buildChange(c, obj, alarms, theirs, want, time.Now())
	if len(ch.errors) > 0 {
		var lines []view.Text
		for name, msg := range ch.errors {
			lines = append(lines, text(name+": "+msg))
		}
		s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{{
			Role: view.NoticeAlert, Title: "This combination cannot be written", Body: "Choose differently or cancel.",
			Details: &view.Diagnostic{Summary: "Fields", Lines: lines},
		}}))
		return
	}
	moving := d.Notebook != "" && d.Notebook != c.listPath()
	if len(ch.ops) == 0 && !ch.alarmsChanged {
		clearDraft(c)
		if moving {
			s.moveRecord(w, r, c, s.notebookTarget(c, d.Notebook), uid, obj, false, "Nothing was moved")
			return
		}
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	// The patch is applied to the record read: what it held is taken first.
	wasEmails := cardEmails(obj)
	sent, fault, err := s.applyChecked(c, obj, ch, theirs, want)
	if err == nil && fault != nil {
		// Not what was chosen (S-7.3): nothing is sent, and the choice is
		// shown again over the record as the server holds it.
		if obj, err = c.find(r.Context(), uid); err != nil {
			s.redirect(w, r, here)
			return
		}
		s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{faultNotice(fault)}))
		return
	}
	if err == nil {
		sent.ETag = etag
		err = c.update(r.Context(), sent)
	}
	switch {
	case dav.IsStatus(err, http.StatusPreconditionFailed):
		s.redirect(w, r, here)
		return
	case err != nil:
		s.writePage(w, r, http.StatusOK, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{s.writeFailure(c, "PUT", obj.Path, err)}))
		return
	}
	clearDraft(c)
	if moving {
		s.saveAndMove(w, r, c, s.notebookTarget(c, d.Notebook), uid, sent, c.recordPath(uid))
		return
	}
	// The conflict's form has no lines of addresses: a replaced one is
	// offered nothing but «Do not rewrite» unless it is the one pair.
	s.finishCardWrite(w, r, c, uid, wasEmails, nil, sent, c.recordPath(uid))
}
