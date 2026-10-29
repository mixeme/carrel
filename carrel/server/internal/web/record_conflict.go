package web

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
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
	// Base is what the version the form was opened with held of every field
	// of the form: with it the conflict screen tells a field both changed
	// from one only the person did, and finds the ones only the server did
	// (s-conflict-1, -4; Рв2). Empty when that version was no longer held
	// (Unsure).
	Base formValues `json:"base,omitempty"`
	// BaseProps are the properties of that version no field of the form
	// shows (outsideForm), as it held them: one the server alone changed is
	// a card of its own (s-conflict-1: X-EVOLUTION-FILE-AS «only on the
	// server»), and «Your edit» puts it back as it was (Рв6). Empty with
	// Base.
	BaseProps map[string][]model.RawValue `json:"baseProps,omitempty"`
	// Notebook is the notebook a note's edit chose other than its own
	// (s-notes-20): part of the edit, so the move waits with it and happens
	// once the person's edit is written (S-18).
	Notebook string `json:"notebook,omitempty"`
	// Unsure: the version the form was opened with was no longer held when
	// the edit was refused, so Mine was measured against the server's
	// version — and holds also what only the server changed (Б43 of the
	// audit of 11.10.2026). The conflict screen says so.
	Unsure        bool `json:"unsure,omitempty"`
	FilesNotSaved bool `json:"filesNotSaved,omitempty"`
}

// saveDraft keeps a refused edit; opened is the version the form was opened
// with, nil when it was no longer held (unsure).
func (s *Server) saveDraft(c *collectionCtx, uid, etag string, readAt time.Time, mine, base formValues, opened *model.Object, notebook string, unsure bool, filesNotSaved ...bool) {
	held := formValues{}
	var props map[string][]model.RawValue
	if !unsure {
		for _, f := range editFields[c.spec.kind] {
			held[f.name] = base[f.name]
			if v, ok := base[f.name+".type"]; ok {
				held[f.name+".type"] = v
			}
		}
		for name, vals := range opened.Props() {
			if outsideForm(c.spec.kind, name) {
				if props == nil {
					props = map[string][]model.RawValue{}
				}
				props[name] = vals
			}
		}
	}
	data, err := json.Marshal(editDraft{
		Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, UID: uid, ETag: etag, ReadAt: readAt, Mine: mine, Base: held,
		BaseProps: props, Notebook: notebook, Unsure: unsure, FilesNotSaved: len(filesNotSaved) > 0 && filesNotSaved[0],
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

// notCompared are the properties outside the form a conflict does not ask
// about: what names the record and its format, what every write stamps,
// and what another page of Carrel writes — the photo, the files.
var notCompared = map[string]bool{
	"UID": true, "VERSION": true, "PRODID": true, "SEQUENCE": true, "CREATED": true,
	"RECURRENCE-ID": true, "PHOTO": true, "ATTACH": true,
}

// outsideForm: no field of the form shows the property, nor writes it
// beside one (editAlso), and the conflict screen compares it (Рв6).
func outsideForm(kind view.Kind, name string) bool {
	if notCompared[name] || isEditStamp(name) {
		return false
	}
	for _, f := range editFields[kind] {
		if f.prop == name {
			return false
		}
	}
	for _, props := range editAlso[kind] {
		if slices.Contains(props, name) {
			return false
		}
	}
	return true
}

// conflictRow is a card of the conflict screen: what it is of and how it
// differs. A field of the form is a card; a field whose rows have labels is
// a card per label (s-conflict-1: TEL;work), and a property the form does
// not show is one of its own (X-EVOLUTION-FILE-AS; Рв6).
type conflictRow struct {
	field editField
	diff  view.ConflictDiff
	// byLabel: the card holds the rows of field under one label, label its
	// key (labelKey); empty is the rows with none.
	byLabel bool
	label   string
	// prop is the property outside the form the card is of.
	prop string
}

// name is the card's property as the record names it (s-conflict-1).
func (cr conflictRow) name() string {
	switch {
	case cr.prop != "":
		return cr.prop
	case cr.byLabel && cr.label != "":
		return cr.field.prop + ";" + cr.label
	}
	return cr.field.prop
}

// choice is what a card has chosen before the person does (Рв4; s-conflict-1,
// -4): the person's edit, but the server's where only the server changed;
// nothing where Carrel cannot tell whose the change is (the owner's decision
// of 16.10.2026, Т1) — a guess there would put back what the other client
// wrote.
func (cr conflictRow) choice() view.ConflictChoice {
	switch cr.diff {
	case view.DiffTheirs:
		return view.ChoiceTheirs
	case view.DiffUnknown:
		return view.ChoiceUnset
	}
	return view.ChoiceMine
}

// chooseEvery is why the combination is not written yet: a card has no side
// chosen (Т1); chooseOne the same of the one card of a record deleted on the
// server (s-conflict-7). Each is also what the screen asks under its title.
const (
	chooseEvery view.Text = "Choose a side for every property."
	chooseOne   view.Text = "Choose a side."
)

// byLabel: the field's rows are told apart by their labels, and a conflict
// is about the rows of one (Рв6: TEL;work and TEL;cell apart).
func byLabel(f editField) bool {
	return f.typed && (f.typ == fieldMulti || f.typ == fieldAddress)
}

// labelKey is the label a row is filed under: the one it shows (shownLabel),
// whatever its case — TYPE=WORK,PREF and the «work» a form posts are one.
func labelKey(prop, raw string) string {
	shown, _ := shownLabel(prop, raw)
	return strings.ToLower(shown)
}

// emptyRow: the row holds nothing — the empty one a form ends with.
func emptyRow(f editField, v string) bool {
	if f.typ == fieldAddress {
		return addressRowParts(v) == [5]string{}
	}
	return strings.TrimSpace(v) == ""
}

// rowsUnder are the rows of f in vals filed under key, in order.
func rowsUnder(f editField, vals formValues, key string) []string {
	var out []string
	labels := vals[f.name+".type"]
	for i, v := range vals[f.name] {
		if !emptyRow(f, v) && labelKey(f.prop, rowAt(labels, i)) == key {
			out = append(out, v)
		}
	}
	return out
}

// conflictCards are the cards of the conflict screen and the properties that
// agree (Рв5), in form order and then the properties outside it by name:
// each field the person changed that the server does not already hold and,
// when the version the form was opened with is known, each the server alone
// changed (s-conflict-4: «DUE · only on the server»; s-conflict-1:
// X-EVOLUTION-FILE-AS). theirs and held are the server version, as the form
// reads it and as it is. Card N's choice comes back as choice.N.
func conflictCards(kind view.Kind, d editDraft, theirs formValues, held map[string][]model.RawValue) ([]conflictRow, []view.Text) {
	known := !d.Unsure && d.Base != nil
	var rows []conflictRow
	var agree []view.Text
	for _, f := range editFields[kind] {
		if byLabel(f) {
			r, a := labelCards(f, d, theirs, known)
			rows, agree = append(rows, r...), append(agree, a...)
			continue
		}
		mine, changed := d.Mine[f.name]
		typed := strings.TrimSpace(first(d.Mine[f.name+".add"])) != ""
		switch {
		case changed && (!sameRows(mine, theirs[f.name]) || typed):
			rows = append(rows, conflictRow{field: f, diff: conflictDiff(f, d, theirs)})
		case changed:
			agree = append(agree, text(f.prop))
		case !known || f.typ == fieldZone:
			// Unknown without the version opened; the zone goes with the
			// time it is of (s-conflict-4: DUE alone).
		default:
			if base, ok := d.Base[f.name]; ok && !sameRows(base, theirs[f.name]) {
				rows = append(rows, conflictRow{field: f, diff: view.DiffTheirs})
			}
		}
	}
	if !known {
		return rows, agree
	}
	var names []string
	for name := range d.BaseProps {
		names = append(names, name)
	}
	for name := range held {
		if _, ok := d.BaseProps[name]; !ok && outsideForm(kind, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		if !sameRaw(d.BaseProps[name], held[name]) {
			rows = append(rows, conflictRow{prop: name, diff: view.DiffTheirs})
		}
	}
	return rows, agree
}

// labelCards are the cards of a field whose rows have labels: one per label
// whose rows differ (Рв6), in the order the labels come in the edit, the
// server version and the version opened; and the labels whose rows the
// person changed to what the server holds.
func labelCards(f editField, d editDraft, theirs formValues, known bool) ([]conflictRow, []view.Text) {
	_, changed := d.Mine[f.name]
	mine := d.Mine
	if !changed {
		mine = d.Base
	}
	var keys []string
	for _, vals := range []formValues{mine, theirs, d.Base} {
		labels := vals[f.name+".type"]
		for i, v := range vals[f.name] {
			if key := labelKey(f.prop, rowAt(labels, i)); !emptyRow(f, v) && !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
	}
	var rows []conflictRow
	var agree []view.Text
	for _, key := range keys {
		m, t := rowsUnder(f, mine, key), rowsUnder(f, theirs, key)
		b := rowsUnder(f, d.Base, key)
		card := conflictRow{field: f, byLabel: true, label: key}
		switch {
		case changed && !known:
			// Without the version opened the person's rows are told from
			// the server's only where they differ.
			if !slices.Equal(m, t) {
				card.diff = view.DiffUnknown
				rows = append(rows, card)
			}
		case changed && !slices.Equal(m, b):
			switch {
			case slices.Equal(m, t):
				agree = append(agree, text(card.name()))
				continue
			case slices.Equal(t, b):
				card.diff = view.DiffMine
			default:
				card.diff = view.DiffBoth
			}
			rows = append(rows, card)
		case known && !slices.Equal(t, b):
			card.diff = view.DiffTheirs
			rows = append(rows, card)
		}
	}
	return rows, agree
}

// sameRaw: two versions hold a property alike — the same values with the
// same parameters, however each is spelt on its line or grouped.
func sameRaw(a, b []model.RawValue) bool {
	return slices.EqualFunc(a, b, func(x, y model.RawValue) bool {
		return x.Text == y.Text && maps.Equal(x.Params, y.Params)
	})
}

// rawShown is a property outside the form as the screen shows it: its
// values as text, a calendar's unescaped as its own texts are.
func (c *collectionCtx) rawShown(name string, vals []model.RawValue) string {
	var parts []string
	for _, v := range vals {
		t := v.Text
		if c.card == nil {
			p := ical.NewProp(name)
			p.Value = t
			if s, err := p.Text(); err == nil {
				t = s
			}
		}
		if strings.TrimSpace(t) != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, ", ")
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
	if sameRows(d.Base[f.name], theirs[f.name]) {
		return view.DiffMine
	}
	return view.DiffBoth
}

// putUnder gives want's rows of f under key the rows src holds under it, where
// the first of them stood, or after the rest; what is under the other labels
// stays as it is. Each row keeps its label as src spells it.
func putUnder(f editField, want, src formValues, key string) {
	var rows, labels []string
	var put, putLabels []string
	srcLabels := src[f.name+".type"]
	for i, v := range src[f.name] {
		if !emptyRow(f, v) && labelKey(f.prop, rowAt(srcLabels, i)) == key {
			put, putLabels = append(put, v), append(putLabels, rowAt(srcLabels, i))
		}
	}
	placed := false
	have := want[f.name+".type"]
	for i, v := range want[f.name] {
		switch l := rowAt(have, i); {
		case emptyRow(f, v):
		case labelKey(f.prop, l) != key:
			rows, labels = append(rows, v), append(labels, l)
		case !placed:
			rows, labels = append(rows, put...), append(labels, putLabels...)
			placed = true
		}
	}
	if !placed {
		rows, labels = append(rows, put...), append(labels, putLabels...)
	}
	want[f.name], want[f.name+".type"] = append(rows, ""), append(labels, "")
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
		// A row's label is in the card's name (TEL;work), not after it.
		for _, v := range vals {
			if strings.TrimSpace(v) != "" {
				parts = append(parts, v)
			}
		}
	}
	if len(parts) == 0 {
		// Nothing: the screen draws its dash (s-conflict-1, Рв3).
		return ""
	}
	return strings.Join(parts, ", ")
}

func (s *Server) conflictPage(c *collectionCtx, d editDraft, obj *model.Object, theirs formValues, alarms []cal.Alarm, notices []view.Notice) view.ConflictPage {
	if d.FilesNotSaved {
		notices = append(notices, chooseFilesAgain)
	}
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
	held := obj.Props()
	rows, agree := conflictCards(c.spec.kind, d, theirs, held)
	page.Agree = agree
	for i, cr := range rows {
		f, mine := cr.field, d.Mine
		if cr.diff == view.DiffTheirs {
			// The person did not touch it: «Your edit» is what was opened.
			mine = d.Base
		}
		var shownMine, shownTheirs string
		switch {
		case cr.prop != "":
			shownMine, shownTheirs = c.rawShown(cr.prop, d.BaseProps[cr.prop]), c.rawShown(cr.prop, held[cr.prop])
		case cr.byLabel:
			shownMine = s.showValues(c, f, rowsUnder(f, mine, cr.label), alarms)
			shownTheirs = s.showValues(c, f, rowsUnder(f, theirs, cr.label), alarms)
		default:
			shownMine, shownTheirs = s.showValues(c, f, mine[f.name], alarms), s.showValues(c, f, theirs[f.name], alarms)
		}
		page.Rows = append(page.Rows, view.ConflictRow{
			// The record's own name for the property, not a label of ours
			// (s-conflict-1): a conflict is read against the file, and a
			// property Carrel does not model has no label to invent.
			Property: text(cr.name()),
			Mine:     text(shownMine),
			Theirs:   text(shownTheirs),
			Choice:   cr.choice(),
			Name:     choiceField(i),
			Diff:     cr.diff,
		})
	}
	// When Carrel no longer holds the version the form was opened with, it
	// cannot tell what the person changed from what only the other client
	// did: every field that differs is a card, «Your form» against the
	// server's, nothing chosen (s-conflict-6, Typecase's 198). The words say
	// it; no notice besides (the owner, 17.10.2026).
	mineLabel := view.Text("Use only my edit")
	if d.Unsure {
		mineLabel = "Use only my form"
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
		// The notebook by its mark and name (s-conflict-11); what the
		// buttons do with the move their words say.
		where := view.Run{Text: text(d.Notebook)}
		if dest := s.notebookTarget(c, d.Notebook); dest != nil {
			where = view.Run{Text: text(string(dest.stamp())), Colour: catalogColour(dest.col)}
		}
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: "Not moved yet", Runs: view.Runs{
			{Text: text("Your edit also moves this " + c.spec.noun + " to ")}, where, {Text: "."},
		}})
	}
	// Nothing to choose (Р-69) is no screen: the GET leads to the card
	// (getKindConflict). What is drawn here has a card or a move.
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
		{ID: "conflict.mine", Label: mineLabel, Href: op("mine"), Method: "POST", Enabled: writable},
		{ID: "conflict.theirs", Label: "Use only the server version", Href: op("theirs"), Method: "POST", Enabled: true},
		cancel,
	}
	// Where nothing is chosen beforehand, the combination waits for a side
	// of every card — on, pale, the browser refusing it until every card has
	// one (Action.Waits, Typecase's 198); the screen asks for it under its
	// title. «Use only my edit» needs none.
	if writable && slices.ContainsFunc(page.Rows, func(r view.ConflictRow) bool { return r.Choice == view.ChoiceUnset }) {
		combine.Waits, combine.Reason = true, chooseEvery
		page.Ask = chooseEvery
	}
	page.Form.Actions = []view.Action{combine}
	return page
}

// deletedPage is the conflict of a record deleted on the server while it
// was edited (Р-67, s-conflict-7): what happened under the title and what
// to do; one card — the record by its name, «Your edit · write it back» its
// name and what the edit changed, «Server · leave it deleted» nothing —, no
// side chosen; «Write this choice» waits for one; Cancel.
func (s *Server) deletedPage(c *collectionCtx, d editDraft, notices []view.Notice) view.ConflictPage {
	page := s.conflictPage(c, d, nil, nil, nil, notices)
	page.Deleted = true
	page.Subtitle = text("Another client deleted this " + string(c.spec.kind) + " while you were editing it. Nothing has been written.")
	page.Ask = chooseOne
	name, shown := s.editSummary(c, d)
	page.Rows = []view.ConflictRow{{Property: text(name), Mine: text(shown), Choice: view.ChoiceUnset, Name: choiceField(0)}}
	page.Form = &view.Form{Action: s.route(c.recordPath(d.UID) + "/conflict"), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		csrfField(c.sess),
		{Name: "etag", Kind: view.InputHidden, Value: text(d.ETag)},
		{Name: "op", Kind: view.InputHidden, Value: "combine"},
	}}}}
	page.Form.Actions = []view.Action{{ID: "conflict.combine", Label: "Write this choice", Method: "POST", Primary: true, Enabled: true, Waits: true, Reason: chooseOne}}
	return page
}

// keepDeletedEdit keeps the edit of a record the server no longer has, as a
// refused one (Р-67): measured against the version the form was opened with
// where it is still held — at the address the record had —, else against an
// empty form (Unsure). The conflict screen asks what to do with it.
func (s *Server) keepDeletedEdit(w http.ResponseWriter, r *http.Request, c *collectionCtx, uid, etag string, readAt time.Time, typed formValues, notebook string) {
	path := strings.TrimSuffix(c.col.Path, "/") + "/" + uid + c.spec.ext
	base, known := s.emptyValues(c), false
	var opened *model.Object
	if read, ok := s.readVersion(c, path, etag); ok {
		if rv, _, err := s.currentValues(c, read); err == nil {
			base, known, opened = rv, true, read
		}
	}
	fillNames(c.spec.kind, base, typed)
	s.saveDraft(c, uid, etag, readAt, changedValues(editFields[c.spec.kind], base, typed), base, opened, notebook, !known)
	s.redirect(w, r, c.recordPath(uid)+"/conflict")
}

// rewritten is what the edit of a record deleted meanwhile writes back: the
// version opened, every field of it, with the edit over it. When that
// version was no longer held (Unsure) the edit is all there is.
func rewritten(d editDraft) formValues {
	want := formValues{}
	for k, v := range d.Base {
		want[k] = v
	}
	for k, v := range d.Mine {
		want[k] = v
	}
	return want
}

// editSummary is the record's name in the edit, and that name with what the
// edit changed after it, « · » between (s-conflict-7): of a field of many
// values only those the version opened did not hold.
func (s *Server) editSummary(c *collectionCtx, d editDraft) (string, string) {
	nameField := "summary"
	if c.spec.kind == view.KindContact {
		nameField = "fn"
	}
	want := rewritten(d)
	name := strings.TrimSpace(strings.Join(want[nameField], " "))
	parts := []string{name}
	for _, f := range editFields[c.spec.kind] {
		vals, ok := d.Mine[f.name]
		if !ok || f.name == nameField {
			continue
		}
		if held := d.Base[f.name]; len(vals) > 1 && len(held) > 0 {
			vals = slices.DeleteFunc(slices.Clone(vals), func(v string) bool { return slices.Contains(held, v) })
		}
		if v := s.showValues(c, f, vals, nil); v != "" {
			parts = append(parts, v)
		}
	}
	return name, strings.Join(parts, " · ")
}

// writeBack writes the edit of a record deleted on the server as the record
// anew, under its UID where it was (Р-67: «Your edit · write it back»): the
// fields as rewritten has them, the properties no field shows as the version
// opened held them. A server that refuses says so over the same choice.
func (s *Server) writeBack(w http.ResponseWriter, r *http.Request, c *collectionCtx, d editDraft) {
	now := time.Now()
	var obj *model.Object
	if c.card != nil {
		obj = card.NewObject(d.UID)
	} else {
		var err error
		if obj, err = cal.NewObject(c.spec.component, d.UID, now); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}
	want, empty := rewritten(d), s.emptyValues(c)
	ch := s.buildChange(c, obj, nil, empty, want, now)
	if len(ch.errors) > 0 {
		s.writePage(w, r, http.StatusOK, s.deletedPage(c, d, []view.Notice{refusedCombination(ch.errors)}))
		return
	}
	for _, name := range slices.Sorted(maps.Keys(d.BaseProps)) {
		ch.ops = append(ch.ops, model.SetRaw{Prop: name, Values: d.BaseProps[name]})
	}
	ch.restored = d.BaseProps
	sent, fault, err := s.applyChecked(c, obj, ch, empty, want)
	if err == nil && fault != nil {
		s.writePage(w, r, http.StatusOK, s.deletedPage(c, d, []view.Notice{faultNotice(fault)}))
		return
	}
	path := strings.TrimSuffix(c.col.Path, "/") + "/" + d.UID + c.spec.ext
	if err == nil {
		sent.Path = path
		if err = c.create(r.Context(), path, sent); landedOnTheFirstTry(err) {
			err = nil
		}
	}
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.deletedPage(c, d, []view.Notice{conflictWriteFailure(s.writeFailure(c, "PUT", path, err))}))
		return
	}
	clearDraft(c)
	s.finishCreate(w, r, c, d.UID, sent)
}

// refusedCombination is a combination the record cannot hold (s-conflict-8):
// said in one line, the fields and why under it; the choice stays.
func refusedCombination(errs map[string]string) view.Notice {
	var lines []view.Text
	for _, name := range slices.Sorted(maps.Keys(errs)) {
		lines = append(lines, text(name+": "+errs[name]))
	}
	return view.Notice{Role: view.NoticeAlert, Title: "This combination cannot be written",
		Details: &view.Diagnostic{Summary: "Fields", Lines: lines}}
}

// conflictWriteFailure is a refused write on the conflict screen
// (s-conflict-9): there is no form to keep what was typed — the cards stay —,
// so it says only that nothing was written.
func conflictWriteFailure(n view.Notice) view.Notice {
	n.Body = "Nothing was written."
	return n
}

// chosenAgain is the screen drawn again over what the person chose (the
// answer to a combination refused, s-conflict-8, -9): each card on at the
// side the form sent, else as before; with a side for every card the
// combination no longer waits, and nothing is asked.
func chosenAgain(r *http.Request, page view.ConflictPage) view.ConflictPage {
	open := false
	for i := range page.Rows {
		page.Rows[i].Choice = chosenSide(r, i, page.Rows[i].Choice)
		open = open || page.Rows[i].Choice == view.ChoiceUnset
	}
	if !open && page.Form != nil {
		page.Ask = ""
		for i := range page.Form.Actions {
			page.Form.Actions[i].Waits, page.Form.Actions[i].Reason = false, ""
		}
	}
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
		s.writePage(w, r, http.StatusOK, s.deletedPage(c, d, nil))
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
	page := s.conflictPage(c, d, obj, theirs, alarms, nil)
	if len(page.Rows) == 0 && (d.Notebook == "" || d.Notebook == c.listPath()) {
		// Nothing differs (Р-69): the edit is on the server already. Nothing
		// is written, the draft goes, and the person is where a save leads.
		clearDraft(c)
		s.redirect(w, r, c.recordPath(uid))
		return
	}
	s.writePage(w, r, http.StatusOK, page)
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
	if notFoundErr(err) && op == "combine" {
		// Deleted on the server meanwhile (Р-67, s-conflict-7): the one card
		// writes the edit back or leaves the record deleted.
		switch view.ConflictChoice(r.FormValue(choiceField(0))) {
		case view.ChoiceMine:
			s.writeBack(w, r, c, d)
		case view.ChoiceTheirs:
			clearDraft(c)
			s.redirect(w, r, c.listPath())
		default:
			s.writePage(w, r, http.StatusOK, s.deletedPage(c, d, []view.Notice{{Role: view.NoticeAlert, Title: "Nothing was written", Body: chooseOne}}))
		}
		return
	}
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
	rows, _ := conflictCards(c.spec.kind, d, theirs, obj.Props())
	if op == "combine" {
		for i, cr := range rows {
			if chosenSide(r, i, cr.choice()) == view.ChoiceUnset {
				// A card with no side: nothing is written, and the choice is
				// shown again (Т1).
				s.writePage(w, r, http.StatusOK, chosenAgain(r, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{{
					Role: view.NoticeAlert, Title: "Nothing was written", Body: chooseEvery,
				}})))
				return
			}
		}
	}
	restored := map[string][]model.RawValue{}
	for i, cr := range rows {
		f := cr.field
		if cr.diff == view.DiffTheirs {
			// A field only the server changed: kept as the server holds it,
			// unless «Your edit» — what was opened — was chosen for it. «Use
			// only my edit» writes the person's changes, not this.
			if op == "mine" || chosenSide(r, i, cr.choice()) != view.ChoiceMine {
				continue
			}
			switch {
			case cr.prop != "":
				restored[cr.prop] = d.BaseProps[cr.prop]
			case cr.byLabel:
				putUnder(f, want, d.Base, cr.label)
			default:
				want[f.name] = d.Base[f.name]
			}
			continue
		}
		if op != "mine" && chosenSide(r, i, cr.choice()) != view.ChoiceMine {
			continue
		}
		if cr.byLabel {
			putUnder(f, want, d.Mine, cr.label)
			continue
		}
		want[f.name] = d.Mine[f.name]
		// What the form sent beside the field goes with it (changedValues).
		if line, ok := d.Mine[f.name+".add"]; ok {
			want[f.name+".add"] = line
		}
		if box, ok := d.Mine[taskDayBox]; ok && f.taskDay {
			want[taskDayBox] = box
		}
	}
	ch := s.buildChange(c, obj, alarms, theirs, want, time.Now())
	// A property outside the form goes back as the version opened held it.
	for _, name := range slices.Sorted(maps.Keys(restored)) {
		ch.ops = append(ch.ops, model.SetRaw{Prop: name, Values: restored[name]})
	}
	ch.restored = restored
	if len(ch.errors) > 0 {
		s.writePage(w, r, http.StatusOK, chosenAgain(r, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{refusedCombination(ch.errors)})))
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
		s.writePage(w, r, http.StatusOK, chosenAgain(r, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{faultNotice(fault)})))
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
		// The record read was patched in memory: the choice is shown again
		// over the record as the server still holds it (s-conflict-9).
		failure := conflictWriteFailure(s.writeFailure(c, "PUT", obj.Path, err))
		if obj, err = c.find(r.Context(), uid); err != nil {
			s.redirect(w, r, here)
			return
		}
		s.writePage(w, r, http.StatusOK, chosenAgain(r, s.conflictPage(c, d, obj, theirs, alarms, []view.Notice{failure})))
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
