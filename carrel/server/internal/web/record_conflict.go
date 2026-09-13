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
}

func (s *Server) saveDraft(c *collectionCtx, uid, etag string, readAt time.Time, mine formValues) {
	data, err := json.Marshal(editDraft{
		Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, UID: uid, ETag: etag, ReadAt: readAt, Mine: mine,
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
		if changed && !sameRows(mine, theirs[f.name]) {
			out = append(out, f)
		}
	}
	return out
}

func (s *Server) showValues(c *collectionCtx, f editField, vals []string, alarms []cal.Alarm) string {
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
	default:
		for _, v := range vals {
			if strings.TrimSpace(v) != "" {
				parts = append(parts, v)
			}
		}
	}
	if len(parts) == 0 {
		return "Empty"
	}
	return strings.Join(parts, ", ")
}

func (s *Server) conflictPage(c *collectionCtx, d editDraft, obj *model.Object, theirs formValues, alarms []cal.Alarm, notices []view.Notice) view.ConflictPage {
	now := time.Now()
	sh := s.buildShell(c.sess, "Changed elsewhere")
	sh.Notices = append(sh.Notices, notices...)
	page := view.ConflictPage{Shell: sh, Kind: c.spec.kind, UID: d.UID, ReadAt: c.tf.formatWhen(d.ReadAt, false, now)}
	path := c.recordPath(d.UID) + "/conflict"
	cancel := view.Action{ID: "conflict.cancel", Label: "Cancel", Href: s.postQuery(path, url.Values{"op": {"cancel"}}), Method: "POST", Enabled: true}
	if obj == nil {
		page.Actions = []view.Action{cancel}
		return page
	}
	page.ServerAt = s.serverStamp(c, obj)
	for _, f := range conflictFields(c.spec.kind, d, theirs) {
		page.Rows = append(page.Rows, view.ConflictRow{
			Property: text(f.label),
			Mine:     text(s.showValues(c, f, d.Mine[f.name], alarms)),
			Theirs:   text(s.showValues(c, f, theirs[f.name], alarms)),
			Choice:   view.ChoiceUnset,
		})
	}
	op := func(name string) string {
		return s.postQuery(path, url.Values{"op": {name}, "etag": {obj.ETag}})
	}
	writable := len(page.Rows) > 0
	page.Actions = []view.Action{
		{ID: "conflict.combine", Label: "Write this combination", Href: op("combine"), Method: "POST", Enabled: writable},
		{ID: "conflict.mine", Label: "Use only my edit", Href: op("mine"), Method: "POST", Enabled: writable},
		{ID: "conflict.theirs", Label: "Use only the server version", Href: op("theirs"), Method: "POST", Enabled: true},
		cancel,
	}
	if !writable {
		page.Actions[0].Reason, page.Actions[1].Reason = "The server version already has your change.", "The server version already has your change."
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: "Nothing differs", Body: "The server version already holds everything you changed."})
	}
	return page
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
		if op == "mine" || r.FormValue(fmt.Sprintf("choice.%d", i)) != string(view.ChoiceTheirs) {
			want[f.name] = d.Mine[f.name]
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
	if len(ch.ops) == 0 && !ch.alarmsChanged {
		clearDraft(c)
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	sent, err := c.apply(obj, ch)
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
	s.finishWrite(w, r, c, uid, sent)
}
