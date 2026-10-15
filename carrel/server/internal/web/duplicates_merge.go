package web

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// A merge on the server (S-16) is one address: GET shows what will be
// written, what deleted and what the kept record gains; POST does it. The
// page and its confirmation name the group (group=), the kept member by its
// place in the group (into=), the answers to the questions (choice.<PROP>=
// the option's index, as the MergeRow lists them) and the version of every
// member shown (v.<place>=). A member that changed between the two is not
// merged over: the person confirmed what they saw.
//
// The order is the safety. The kept record is written with If-Match and read
// back; only a write the server kept whole lets the others go, each deleted
// with its own If-Match, and the first refusal stops the rest. Nothing is
// rolled back: undoing the write would lose what was merged into it.

// mergeMember is one member of the group as read for the merge.
type mergeMember struct {
	place int // in the group's refs
	ref   sealed.RecordRef
	c     *collectionCtx
	obj   *model.Object
}

func (m mergeMember) label() string {
	return mergeTitle(m.c, m.obj) + " · " + string(m.c.stamp())
}

func mergeTitle(c *collectionCtx, obj *model.Object) string {
	switch c.spec.kind {
	case view.KindContact:
		return contactName(obj)
	case view.KindNote:
		return noteTitle(cal.Text(obj, "SUMMARY"), cal.Description(obj))
	default:
		return titleOr(cal.Text(obj, "SUMMARY"), "Untitled "+c.spec.noun)
	}
}

// mergeRead is a group read fresh from its servers.
type mergeRead struct {
	st      dupState
	g       dupGroup
	members []mergeMember
	target  int // in members; -1 when none can be kept
	down    []view.Notice
}

// readMerge reads every member of the group named by the request. A member
// its server says is not there leaves the group silently; a server that does
// not answer is a notice, and its member is not gone (S-16).
func (s *Server) readMerge(r *http.Request, sess *session.Session) (mergeRead, *view.Notice) {
	mr := mergeRead{st: s.readDuplicates(sess, true), target: -1}
	g, ok := mr.st.group(r.FormValue("group"))
	if !ok {
		return mr, &view.Notice{
			Role: view.NoticeAlert, Title: "This group is not among the loaded records",
			Body: "The lists it came from are no longer loaded, or a record in it changed. Nothing was written. Open the lists again, then the duplicates.",
		}
	}
	mr.g = g
	spec := recordKinds[segmentOf(g.kind)]
	var gone []sealed.RecordRef
	for i, ref := range g.refs {
		c, ok := s.collectionOf(sess, spec, ref.AccountID, ref.Col)
		if !ok {
			gone = append(gone, ref)
			continue
		}
		obj, err := c.find(r.Context(), ref.UID)
		switch {
		case notFoundErr(err):
			gone = append(gone, ref)
		case err != nil:
			mr.down = append(mr.down, s.unavailableNotice(c, s.mergeHref(g.id, -1, nil), err))
		default:
			mr.members = append(mr.members, mergeMember{place: i, ref: ref, c: c, obj: obj})
		}
	}
	if len(gone) > 0 {
		s.forgetMembers(sess, "", gone)
	}
	into, err := strconv.Atoi(r.FormValue("into"))
	if err != nil {
		into = -1
	}
	for i, m := range mr.members {
		if m.c.col.ReadOnly {
			continue
		}
		if mr.target < 0 || m.place == into {
			mr.target = i
		}
	}
	return mr, nil
}

// forgetMembers takes records out of every linked group, and a group left
// with one member is dissolved.
func (s *Server) forgetMembers(sess *session.Session, what string, refs []sealed.RecordRef) {
	key, _ := sess.DataKey()
	drop := map[sealed.RecordRef]bool{}
	for _, r := range refs {
		drop[r] = true
	}
	detail := ""
	if what != "" {
		detail = strconv.Itoa(len(refs))
	}
	_ = s.users.EditDuplicates(sess.AccountID(), key, what, detail, func(d *sealed.Duplicates) error {
		var linked []sealed.LinkedGroup
		for _, g := range d.Linked {
			var kept []sealed.RecordRef
			for _, m := range g.Members {
				if !drop[m] {
					kept = append(kept, m)
				}
			}
			if len(kept) >= 2 {
				g.Members = kept
				linked = append(linked, g)
			}
		}
		d.Linked = linked
		return nil
	})
}

func (s *Server) mergeHref(group string, into int, extra url.Values) string {
	q := url.Values{"group": {group}}
	if into >= 0 {
		q.Set("into", strconv.Itoa(into))
	}
	for k, v := range extra {
		q[k] = v
	}
	return s.postQuery("/app/duplicates/merge", q)
}

// picksOf are the values chosen on the card a merge came from: each
// property's member by its place in the group (pick.<PROP>=; Дб9).
func picksOf(r *http.Request, mr mergeRead) map[string]sealed.RecordRef {
	out := map[string]sealed.RecordRef{}
	_ = r.ParseForm()
	for k, v := range r.Form {
		prop, ok := strings.CutPrefix(k, "pick.")
		if !ok || len(v) == 0 {
			continue
		}
		if n, err := strconv.Atoi(v[0]); err == nil && n >= 0 && n < len(mr.g.refs) {
			out[strings.ToUpper(prop)] = mr.g.refs[n]
		}
	}
	return out
}

func choicesOf(r *http.Request) map[string]int {
	out := map[string]int{}
	_ = r.ParseForm()
	for k, v := range r.Form {
		prop, ok := strings.CutPrefix(k, "choice.")
		if !ok || len(v) == 0 {
			continue
		}
		if n, err := strconv.Atoi(v[0]); err == nil {
			out[strings.ToUpper(prop)] = n
		}
	}
	return out
}

func (mr mergeRead) others() []mergeMember {
	var out []mergeMember
	for i, m := range mr.members {
		if i != mr.target {
			out = append(out, m)
		}
	}
	return out
}

// mergeBlocked is why this merge cannot be written now, or "".
func (mr mergeRead) mergeBlocked() string {
	switch {
	case len(mr.down) > 0:
		return "A server holding a member did not answer. Nothing is merged while a member cannot be read."
	case len(mr.members) < 2:
		return "Fewer than two of these records are still there, so there is nothing to merge."
	case mr.target < 0:
		return "Every member is in a read-only collection."
	}
	refs := make([]sealed.RecordRef, len(mr.members))
	for i, m := range mr.members {
		refs[i] = m.ref
	}
	return mergeBlock(mr.st.blob, refs)
}

var calUnescape = strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`)

// valueText is a value as the page names it: a photo or anything long by
// where it comes from, the rest as written.
func valueText(calendar bool, prop string, v model.RawValue, from mergeMember) string {
	t := v.Text
	if calendar {
		t = calUnescape.Replace(t)
	}
	if prop == "PHOTO" || prop == "ATTACH" || len(t) > 120 {
		return prop + " (from " + string(from.c.stamp()) + ")"
	}
	return t
}

// whenText is the time of a member as the question «When» offers it (А6 of
// the audit of 11.10.2026): the day and the hours, the zone when it is not
// the instance's, the repetition in words, how many occurrences were
// rewritten by hand — and whose it is, which also tells apart two times
// that read the same.
func whenText(tf timeFmt, m mergeMember) string {
	var parts []string
	start := time.Time{}
	if m.c.spec.kind == view.KindTask {
		if due, err := cal.DueOf(m.obj, tf.tz); err == nil && !due.Start.IsZero() {
			parts = append(parts, "Due "+tf.timeOrDate(due))
			start = due.Start
		}
		if tm, err := cal.TimingOf(m.obj, tf.tz); err == nil && !tm.Start.IsZero() {
			parts = append(parts, "starts "+tf.timeOrDate(tm))
			start = tm.Start
		}
	} else if tm, err := cal.TimingOf(m.obj, tf.tz); err == nil && !tm.Start.IsZero() {
		when := tf.when(tm)
		if z := tf.zone(tm.Zone); z != "" {
			when += " (" + z + ")"
		}
		parts = append(parts, when)
		start = tm.Start
	}
	if len(parts) == 0 {
		parts = append(parts, "No time")
	}
	if raw := cal.RawRRULE(m.obj); raw != "" {
		parts = append(parts, tf.recurrence(raw, start))
	}
	if over, err := cal.Overrides(m.obj, tf.tz); err == nil && len(over) > 0 {
		parts = append(parts, plural(len(over), "rewritten occurrence"))
	}
	return strings.Join(parts, ", ") + " (from " + string(m.c.stamp()) + ")"
}

func gainLine(calendar bool, g dupes.Gain, from mergeMember) string {
	v := valueText(calendar, g.Prop, g.Value, from)
	if strings.HasPrefix(v, g.Prop+" (") {
		return v
	}
	name := g.Prop
	keys := make([]string, 0, len(g.Value.Params))
	for k := range g.Value.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k == "VALUE" || k == "ENCODING" {
			continue
		}
		name += ";" + k + "=" + g.Value.Params[k]
	}
	// The value says itself; only what shows no value (a photo) is told by
	// where it comes from (s-dup-2, Дб15).
	return name + ": " + v
}

// mergePlan is the merged record and the page that shows it.
func (s *Server) mergePlan(sess *session.Session, mr mergeRead, choices map[string]int, picks map[string]sealed.RecordRef) (*model.Object, view.DuplicatesMergePage) {
	page := view.DuplicatesMergePage{Shell: s.buildShell(sess, "Merge on the server"), GroupID: mr.g.id}
	page.Rail = view.Rail{Nav: s.navRows(sess, mr.st.blob, "duplicates"), About: s.aboutAction(), Davloom: s.davloomMark(sess)}
	// The merge stands on the same ground as the screen it came from: every
	// collection compared (s-dup-2, -4, -5: «12 / 12»; Дб20).
	page.Rail.Sections = dupSources(mr.st, "")
	page.Notices = append(page.Notices, mr.down...)
	back := view.Action{ID: "duplicates.merge.cancel", Label: "Cancel", Href: s.route("/app/duplicates"), Method: "GET", Enabled: true}
	confirm := view.Action{ID: "duplicates.merge.confirm", Label: "Write the merge and delete the rest", Method: "POST", Danger: true}
	if mr.target >= 0 {
		t := mr.members[mr.target]
		page.Target = s.member(mr.st, mr.g.kind, t.ref)
		page.Target.Title = text(mergeTitle(t.c, t.obj))
		// Whom the merge is of, and how many records (s-dup-2, Дб13).
		page.Subtitle = page.Target.Title + text(" · "+plural(len(mr.members), "record"))
	}
	if reason := mr.mergeBlocked(); reason != "" {
		confirm.Reason = text(reason)
		page.Warning = view.Notice{Role: view.NoticeAlert, Title: "Nothing can be merged now", Body: text(reason)}
		page.Actions = []view.Action{confirm, back}
		return nil, page
	}

	target, others := mr.members[mr.target], mr.others()
	objs := make([]*model.Object, len(others))
	for i, o := range others {
		objs[i] = o.obj
	}
	// A linked group's answers are the merge's defaults: asked once (S-16);
	// what was chosen on the card the merge came from goes over them (Дб9).
	prefs := map[string]sealed.RecordRef{}
	for p, ref := range mr.st.linkedPrefs(mr.g.id) {
		prefs[p] = ref
	}
	for p, ref := range picks {
		prefs[p] = ref
	}
	if len(prefs) > 0 {
		if _, probe, err := dupes.Merge(target.obj, objs, nil); err == nil {
			all := append([]mergeMember{target}, others...)
			for _, qn := range probe.Questions {
				ref, ok := prefs[qn.Prop]
				if _, answered := choices[qn.Prop]; answered || !ok {
					continue
				}
				for i, o := range qn.Options {
					if all[o.From].ref == ref {
						choices[qn.Prop] = i
					}
				}
			}
		}
	}
	merged, plan, err := dupes.Merge(target.obj, objs, choices)
	if err != nil {
		confirm.Reason = text("These records could not be put together: " + err.Error())
		page.Warning = view.Notice{Role: view.NoticeAlert, Title: "Nothing can be merged", Body: confirm.Reason}
		page.Actions = []view.Action{confirm, back}
		return nil, page
	}
	calendar := target.obj.IsCalendar()
	tf := s.timeFor(sess)
	from := func(i int) mergeMember {
		if i == 0 {
			return target
		}
		return others[i-1]
	}

	q := url.Values{}
	for _, qn := range plan.Questions {
		row := view.MergeRow{Property: text(choiceLabel(qn.Prop)), Name: "choice." + qn.Prop}
		chosen := 0
		if n, ok := choices[qn.Prop]; ok && n >= 0 && n < len(qn.Options) {
			chosen = n
		}
		for _, o := range qn.Options {
			if qn.Prop == dupes.When {
				row.Options = append(row.Options, text(whenText(tf, from(o.From))))
				continue
			}
			row.Options = append(row.Options, text(valueText(calendar, qn.Prop, o.Value, from(o.From))))
		}
		row.Chosen = chosen
		page.Rows = append(page.Rows, row)
		q.Set("choice."+qn.Prop, strconv.Itoa(chosen))
	}
	for _, prop := range plan.Union {
		row := view.MergeRow{Property: text(choiceLabel(prop)), Union: true}
		for _, v := range merged.Props()[prop] {
			row.Options = append(row.Options, text(valueText(calendar, prop, v, target)))
		}
		page.Rows = append(page.Rows, row)
	}
	for _, m := range mr.members {
		q.Set("v."+strconv.Itoa(m.place), m.obj.ETag)
	}

	// The plan, each part under its rubric (s-dup-2, -4; Дб14, Дб15).
	page.StaysNote = text(mergeTitle(target.c, target.obj) + " there is rewritten with the values you chose.")
	for _, o := range others {
		d := s.member(mr.st, mr.g.kind, o.ref)
		d.Title = text(mergeTitle(o.c, o.obj))
		page.Deleted = append(page.Deleted, d)
	}
	for _, g := range plan.Gains {
		page.Gains = append(page.Gains, text(gainLine(calendar, g, from(g.From))))
	}
	if plan.Time > 0 {
		page.Gains = append(page.Gains, text("the time of "+from(plan.Time).label()+": its start and end, its repetition and its rewritten occurrences"))
	}
	if plan.Alarms > 0 {
		page.Gains = append(page.Gains, text(plural(plan.Alarms, "reminder")+" from the others"))
	}
	if plan.Dropped > 0 {
		page.NotCarried = append(page.NotCarried, text(fmt.Sprintf("%s of the kept record, which %s to the time it gives up",
			plural(plan.Dropped, "rewritten occurrence"), verbs(plan.Dropped, "belongs", "belong"))))
	}
	for i := 1; i <= len(others); i++ {
		if n := plan.Instances[i]; n > 0 {
			// The verb agrees with the count (s-dup-4, Дб18): «1 rewritten
			// occurrence …, which belongs to its series and goes with it».
			page.NotCarried = append(page.NotCarried, text(fmt.Sprintf("%s of %s, which %s to its series and %s with it",
				plural(n, "rewritten occurrence"), others[i-1].label(), verbs(n, "belongs", "belong"), verbs(n, "goes", "go"))))
		}
	}
	// The plan is said, not alarmed (s-dup-2, -4; Дб16): the button that
	// deletes is the alarm.
	page.Warning = view.Notice{
		Role:  view.NoticeInfo,
		Title: text(fmt.Sprintf("One record is rewritten and %s deleted.", plural(len(others), "other"))),
		Body: "The kept record is rewritten with the values you chose, and only once the server has kept it are the others deleted. " +
			"This cannot be undone from here.",
	}
	confirm.Enabled, confirm.Href = true, s.mergeHref(mr.g.id, target.place, q)
	page.Actions = []view.Action{confirm, back}
	// Where it stays: a segment of the sources (s-dup-2), the kept one on,
	// another the same page with it kept; a read-only one cannot be
	// rewritten, and says so.
	for _, m := range mr.members {
		a := view.Action{ID: "duplicates.merge.into", Label: text(string(m.c.stamp())), Method: "GET", Enabled: true, On: m.place == target.place}
		switch {
		case a.On:
		case m.c.col.ReadOnly:
			a.Enabled, a.Reason = false, text(collectionLabel(m.c.col)+" is read-only here, so the merged record cannot be written there.")
		default:
			a.Href = s.mergeHref(mr.g.id, m.place, nil)
		}
		page.Stays = append(page.Stays, a)
	}
	return merged, page
}

func (s *Server) getDuplicatesMerge(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	mr, missing := s.readMerge(r, sess)
	if missing != nil {
		s.writePage(w, r, http.StatusOK, s.lostGroupPage(sess, mr, *missing))
		return
	}
	_, page := s.mergePlan(sess, mr, choicesOf(r), picksOf(r, mr))
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) lostGroupPage(sess *session.Session, mr mergeRead, n view.Notice) view.DuplicatesMergePage {
	page := view.DuplicatesMergePage{Shell: s.buildShell(sess, "Merge on the server"), Warning: n}
	page.Rail = view.Rail{Nav: s.navRows(sess, mr.st.blob, "duplicates"), About: s.aboutAction(), Davloom: s.davloomMark(sess),
		Sections: dupSources(mr.st, "")}
	page.Actions = []view.Action{{ID: "duplicates.back", Label: "Back to duplicates", Href: s.route("/app/duplicates"), Method: "GET", Enabled: true}}
	return page
}

// postDuplicatesMerge writes a confirmed merge in the order of S-16.
func (s *Server) postDuplicatesMerge(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	mr, missing := s.readMerge(r, sess)
	if missing != nil {
		s.writePage(w, r, http.StatusOK, s.lostGroupPage(sess, mr, *missing))
		return
	}
	merged, page := s.mergePlan(sess, mr, choicesOf(r), nil)
	if merged == nil {
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	shown := func(n view.Notice) {
		page.Notices = append(page.Notices, n)
		s.writePage(w, r, http.StatusOK, page)
	}
	for _, m := range mr.members {
		if v := r.FormValue("v." + strconv.Itoa(m.place)); v != m.obj.ETag {
			shown(view.Notice{
				Role: view.NoticeAlert, Title: "A record changed since it was shown",
				Body: text(m.label() + " is not the version this page showed. Nothing was written and nothing deleted; look at the merge again."),
			})
			return
		}
	}

	ctx := r.Context()
	target, others := mr.members[mr.target], mr.others()
	kept := target.c
	uid := target.ref.UID
	if err := kept.update(ctx, merged); err != nil {
		n := s.writeFailure(kept, "PUT", target.obj.Path, err)
		n.Body = "Nothing was written and nothing deleted."
		if dav.IsStatus(err, http.StatusPreconditionFailed) {
			n.Title, n.Body = "The kept record changed on its server", "It changed after this page read it. Nothing was written and nothing deleted; look at the merge again."
		}
		shown(n)
		return
	}
	got, err := kept.find(ctx, uid)
	if err != nil {
		merged.ETag = ""
		kept.noteWrite(merged.Path, merged)
		s.mergeResult(w, r, sess, mr, nil, others, view.Notice{
			Role: view.NoticeAlert, Title: "Written, but nothing was deleted",
			Body: "The merged record could not be read back to check it, so the others were kept. Look at it, then delete them yourself if it holds everything.",
		})
		return
	}
	kept.noteWrite(got.Path, got)
	if loss := writeLoss(kept.conn.ID, merged, got, sess.Losses(), false); loss != nil {
		s.mergeResult(w, r, sess, mr, nil, others, view.Notice{
			Role: view.NoticeAlert, Title: "Written, but nothing was deleted",
			Body: text("The server of " + collectionLabel(kept.col) + " did not keep " + strings.Join(loss.Properties, ", ") +
				". The others stay, so nothing is lost; delete them yourself if the kept record is enough."),
		})
		return
	}
	// Before anything is deleted, what pointed at the others is made to point
	// at the kept record: a task or a note made from a person's timeline is
	// tied to the card by its UID alone, and a deleted card's UID names
	// nothing (relink.go). Done first so that a delete refused halfway still
	// leaves every link on a record that exists.
	gone := make([]string, 0, len(others))
	for _, o := range others {
		gone = append(gone, o.ref.UID)
	}
	rep := s.relink(ctx, sess, s.catalogOf(sess), gone, uid)
	var deleted []mergeMember
	for i, o := range others {
		if err := o.c.remove(ctx, o.obj.Path, o.obj.ETag); err != nil {
			n := moveFailure(o.c, "Written, but not every other record was deleted.", "DELETE", o.obj.Path, err)
			n.Body = text(fmt.Sprintf("The merge was written and %s deleted; %s refused, and the rest were not tried. Nothing is undone.",
				plural(len(deleted), "record"), mergeTitle(o.c, o.obj)))
			if dav.IsStatus(err, http.StatusPreconditionFailed) {
				n.Body = text(o.label() + " changed after it was read, so it was kept, and the rest were not tried. The merge was written; nothing is undone.")
			}
			if n.Details != nil {
				n.Details.Lines = append(n.Details.Lines, rep.lines()...)
			}
			s.mergeResult(w, r, sess, mr, deleted, others[i:], n)
			return
		}
		o.c.noteWrite(o.obj.Path, nil)
		// A link to a record merged away opens the kept one (Б41).
		s.followLinks(sess, placeOf(o.c, o.ref.UID), kept, uid)
		deleted = append(deleted, o)
	}
	s.forgetDeleted(sess, mr.g.kind, deleted)
	if rep.nothing() {
		s.redirect(w, r, kept.recordPath(uid))
		return
	}
	// Something pointed at the records that are gone: what became of it is
	// said here, on the kept record, rather than lost behind a redirect.
	role := view.NoticeInfo
	if len(rep.skipped) > 0 || len(rep.unread) > 0 {
		role = view.NoticeAlert
	}
	s.writePage(w, r, http.StatusOK, s.recordPage(kept, uid, got, view.Notice{
		Role: role, Title: "Merged",
		Body:    text("The others were deleted." + rep.sentence()),
		Details: &view.Diagnostic{Summary: "Linked records", Lines: rep.lines()},
	}))
}

func (s *Server) forgetDeleted(sess *session.Session, kind view.Kind, deleted []mergeMember) {
	if len(deleted) == 0 {
		return
	}
	refs := make([]sealed.RecordRef, len(deleted))
	for i, d := range deleted {
		refs[i] = d.ref
	}
	s.forgetMembers(sess, "duplicates.merge", refs)
}

// mergeResult is a merge that did not end in one record: what was deleted and
// what was kept, said plainly, and the kept record one link away.
func (s *Server) mergeResult(w http.ResponseWriter, r *http.Request, sess *session.Session, mr mergeRead, deleted, left []mergeMember, n view.Notice) {
	s.forgetDeleted(sess, mr.g.kind, deleted)
	target := mr.members[mr.target]
	// What happened, under its rubric (s-dup-5, Дб19): the kept record, what
	// went, what is still there; what the server answered stays in Details.
	page := view.DuplicatesMergePage{Shell: s.buildShell(sess, "Merge on the server"), GroupID: mr.g.id, Warning: n}
	page.Happened = append(page.Happened, text("Kept and rewritten: "+target.label()))
	for _, d := range deleted {
		page.Happened = append(page.Happened, text("Deleted: "+d.label()))
	}
	for _, l := range left {
		page.Happened = append(page.Happened, text("Still there: "+l.label()))
	}
	page.Subtitle = text(mergeTitle(target.c, target.obj) + " · " + plural(len(mr.members), "record"))
	page.Rail = view.Rail{Nav: s.navRows(sess, s.catalogOf(sess), "duplicates"), About: s.aboutAction(), Davloom: s.davloomMark(sess),
		Sections: dupSources(mr.st, "")}
	page.Target = s.member(mr.st, mr.g.kind, target.ref)
	kept := s.openThere(target.c, target.ref.UID, "Open the kept record")
	kept.Primary = true // s-dup-5
	page.Actions = []view.Action{
		kept,
		{ID: "duplicates.back", Label: "Back to duplicates", Href: s.route("/app/duplicates"), Method: "GET", Enabled: true},
	}
	s.writePage(w, r, http.StatusOK, page)
}
