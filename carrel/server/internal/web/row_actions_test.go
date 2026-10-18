package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// from-typecase notes 5, 8, 11 and 17 (owner's rule): what a row is opened
// for stands on it, the rare rest is under its «⋯»; an action that cannot be
// taken is shown switched off with its reason, and one that can carries none.
// The buttons of a row and the line beside a deletion are read from the
// frames.

var (
	frameRowActs = regexp.MustCompile(`(?s)<td class="tc-acts">(.*?)</td>`)
	frameDanger  = regexp.MustCompile(`(?s)<div class="tc-danger"><button[^>]*>(.*?)</button>(.*?)</div>`)
	// A row's «⋯» with its menu (Typecase's task 117, ac658b0) is a details
	// after the row's buttons: its items are their twins, not buttons of the
	// row.
	frameRowDrop = regexp.MustCompile(`(?s)<details class="tc-drop">.*?</details>`)
)

// frameRows are the labelled buttons of each row of a frame's table, the
// «⋯» and the twins in its menu left out.
func frameRows(t *testing.T, id string) [][]string {
	t.Helper()
	var out [][]string
	for _, m := range frameRowActs.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		row := []string{}
		for _, b := range frameButton.FindAllStringSubmatch(frameRowDrop.ReplaceAllString(m[1], ""), -1) {
			if label := frameText(b[1]); label != "" {
				row = append(row, label)
			}
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		t.Fatalf("%s draws no row buttons: the frame changed, read it again", id)
	}
	return out
}

var (
	frameTableRow = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
	frameCell     = regexp.MustCompile(`(?s)<td(?:\s[^>]*)?>(.*?)</td>`)
)

// frameCells are the texts of the cells of each body row of a frame's table.
func frameCells(t *testing.T, id string) [][]string {
	t.Helper()
	var out [][]string
	for _, r := range frameTableRow.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		var row []string
		for _, c := range frameCell.FindAllStringSubmatch(r[1], -1) {
			row = append(row, frameText(c[1]))
		}
		if len(row) > 0 {
			out = append(out, row)
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s draws no table rows", id)
	}
	return out
}

// frameDangerNote is the line a frame writes beside its danger button.
func frameDangerNote(t *testing.T, id, button string) string {
	t.Helper()
	for _, m := range frameDanger.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		if frameText(m[1]) == button {
			return frameText(m[2])
		}
	}
	t.Fatalf("%s draws no %q with a line beside it", id, button)
	return ""
}

func labels(actions []view.Action) []string {
	out := []string{}
	for _, a := range actions {
		out = append(out, string(a.Label))
	}
	return out
}

// sayWhy is C-6.2 on a list of actions: off means a reason, on means none.
func sayWhy(t *testing.T, what string, actions ...view.Action) {
	t.Helper()
	for _, a := range actions {
		switch {
		case a.Enabled && a.Reason != "":
			t.Errorf("%s: %s «%s» can be taken and carries a reason %q", what, a.ID, a.Label, a.Reason)
		case !a.Enabled && a.Reason == "":
			t.Errorf("%s: %s «%s» is off without a reason", what, a.ID, a.Label)
		}
	}
}

// s-publish-1: Copy, Send, Revoke on the row; the dates absolute, a link
// nobody opened says so in words.
func TestPublishedRowIsTheFrame(t *testing.T) {
	f, _ := letterFixture(t)
	decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	if len(page.Rows) != 1 {
		t.Fatalf("rows = %+v", page.Rows)
	}
	row := page.Rows[0]
	if want := frameRows(t, "s-publish-1")[0]; !equalStrings(labels(row.Actions), want) {
		t.Errorf("row actions = %q, want %q", labels(row.Actions), want)
	}
	if got := ids(row.Actions); !equalStrings(got, []string{"publish.copy", "publish.mail", "publish.revoke"}) {
		t.Errorf("row action ids = %v", got)
	}
	sayWhy(t, "published row", row.Actions...)
	if !strings.Contains(mockupFrame(t, "s-publish-1"), ">never opened<") || row.LastSeen != "never opened" {
		t.Errorf("last opened = %q, want the frame's words", row.LastSeen)
	}
	if strings.Contains(string(row.Created), "ago") || row.Created == "" {
		t.Errorf("created = %q, want the day", row.Created)
	}
}

// s-backup-1: Run and Edit on every job, the one that downloads too — its
// Run streams the archive. s-backup-2: the job page has Cancel, Run now and
// Save, «Job is on», and the deletion at its foot with the frame's line.
func TestBackupRowsAndJobPageAreTheFrame(t *testing.T) {
	f := backupFixture(t)
	dl := saveJob(t, f, jobForm(f, sealed.BackupToDownload, "8", "pw"))
	dav := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	list := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if len(list.Jobs) != 2 || list.Delete != nil {
		t.Fatalf("jobs = %d, delete %+v: the list deletes nothing", len(list.Jobs), list.Delete)
	}
	for i, row := range frameRows(t, "s-backup-1") {
		if i >= len(list.Jobs) {
			break
		}
		if got := labels(list.Jobs[i].Actions); !equalStrings(got, row) {
			t.Errorf("job %d actions = %q, want %q", i, got, row)
		}
		sayWhy(t, "job row", list.Jobs[i].Actions...)
	}
	run := actionByID(list.Jobs[0].Actions, "backup.run")
	if run == nil || run.Method != http.MethodGet || !strings.Contains(run.Href, "op=download") {
		t.Fatalf("Run of a download job = %+v, want the download", run)
	}
	if rec := f.h.do(get(run.Href, f.sess)); rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), ".zip") {
		t.Errorf("Run streamed %d %v", rec.Code, rec.Header())
	}
	if run := actionByID(list.Jobs[1].Actions, "backup.run"); run == nil || run.Method != http.MethodPost {
		t.Errorf("Run of a WebDAV job = %+v", run)
	}

	// The job's page is headed by its name and what its last copy was
	// (s-backup-2: «Job · last copy 2 days ago»).
	sub := regexp.MustCompile(`<div class="tc-head__sub">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, "s-backup-2"))
	if sub == nil || !strings.HasPrefix(frameText(sub[1]), "Job · last copy ") {
		t.Fatalf("s-backup-2 head = %v: read the frame again", sub)
	}
	dlPage := decode[view.BackupPage](t, f.h.do(get(actionHref(t, list.Jobs[0].Actions, "backup.edit"), f.sess)))
	// A destination other than WebDAV alone is named (s-backup-3).
	if dlPage.Title != text(dl.Name) || saidRuns(dlPage.Subtitle) != "Job · destination ⟨strong:download⟩ · last copy just now" {
		t.Errorf("head = %q / %q", dlPage.Title, saidRuns(dlPage.Subtitle))
	}
	// The list counts its jobs under the title (s-backup-1: «Three jobs ·
	// their own sources, destinations and triggers»).
	if saidRuns(list.Subtitle) != "Two jobs · their own sources, destinations and triggers" || list.Title != "Backup" {
		t.Errorf("list head = %q / %q", list.Title, saidRuns(list.Subtitle))
	}
	page := decode[view.BackupPage](t, f.h.do(get(actionHref(t, list.Jobs[1].Actions, "backup.edit"), f.sess)))
	if saidRuns(page.Subtitle) != "Job · no copy yet" {
		t.Errorf("never run: subtitle %q", saidRuns(page.Subtitle))
	}
	drawn(t, "job page", page.Add.Actions, "s-backup-2")
	if got := ids(page.Add.Actions); !equalStrings(got, []string{"backup.save", "backup.cancel", "backup.run"}) {
		t.Errorf("job page actions = %v", got)
	}
	sayWhy(t, "job page", page.Add.Actions...)
	if page.Delete == nil || page.Delete.Label != "Delete job" || !page.Delete.Danger || page.Delete.Reason != "" {
		t.Fatalf("delete = %+v", page.Delete)
	}
	if want := frameDangerNote(t, "s-backup-2", "Delete job"); string(page.DeleteNote) != want {
		t.Errorf("delete note = %q, want %q", page.DeleteNote, want)
	}
	var on *view.FormRow
	for _, set := range page.Add.Sets {
		for i := range set.Rows {
			if set.Rows[i].Name == "on" {
				on = &set.Rows[i]
			}
		}
	}
	if on == nil || len(on.Options) != 1 || on.Options[0].Label != "Job is on" || !on.Options[0].Selected {
		t.Fatalf("Job is on = %+v", on)
	}

	// A job that is off does not run by itself: not at sign-in.
	v := jobForm(f, sealed.BackupToWebDAV, "8", "pw")
	v.Del("on")
	v.Set("signin", "1")
	if rec := f.h.do(postForm(page.Add.Action, f.sess, v)); rec.Code != http.StatusSeeOther {
		t.Fatalf("save off: %d %s", rec.Code, rec.Body)
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	if job, _ := blob.BackupJob(dav.ID); !job.Off || job.OnSignInDays != 1 {
		t.Fatalf("saved without «Job is on» = %+v", job)
	}
	if rec := f.h.do(f.h.anonPostForm("/login", "/login", url.Values{"login": {"owner"}, "password": {"password123"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("login: %d", rec.Code)
	}
	if runs := f.h.users.BackupRuns(f.sess.AccountID())[dav.ID]; len(runs) != 0 {
		t.Errorf("a job that is off ran at sign-in: %+v", runs)
	}

	if rec := f.h.do(postForm(page.Delete.Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d", rec.Code)
	}
	blob, _ = f.h.users.Catalog(f.sess.AccountID(), key)
	if _, has := blob.BackupJob(dav.ID); has || len(blob.BackupJobs) != 1 || blob.BackupJobs[0].ID != dl.ID {
		t.Errorf("jobs after the deletion = %+v", blob.BackupJobs)
	}
}

// The external trigger is turned on and off by its own tick, «On a clock of
// your own» (s-backup-2, to-typecase task 18a) — there is no separate
// switch, and none on the row; a job that is off is not started by it
// either. The tick is the job's; the card of the line is there only while
// the trigger is on (owner's decision, note 19): off, nothing to show or
// renew, and no card is sent.
func TestTriggerIsTurnedOffByItsTick(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	tick := regexp.MustCompile(`(?s)<label class="tc-check">(?:<input[^>]*>)?(?:<span[^>]*></span>)?(On a clock of your own.*?)</label>`).FindStringSubmatch(mockupFrame(t, "s-backup-2"))
	if tick == nil {
		t.Fatal("s-backup-2 draws no «On a clock of your own»: read the frame again")
	}
	// The badge «local» is the instance's, not the tick's; «cron» is set in
	// mono and followed by its comma.
	label := strings.ReplaceAll(frameText(regexp.MustCompile(`(?s)<span class="tc-badge[^"]*"[^>]*>.*?</span>`).ReplaceAllString(tick[1], "")), " ,", ",")
	clock := func() view.Action {
		page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
		if a := actionByID(page.Jobs[0].Actions, "backup.trigger.off"); a != nil {
			t.Errorf("the row turns the trigger off: %+v", a)
		}
		j := openJob(t, f)
		c := j.Clock
		sayWhy(t, "trigger", c)
		if tb := j.Trigger; (tb != nil) != c.On {
			t.Errorf("the tick is on %v, the card %+v", c.On, tb)
		} else if tb != nil {
			sayWhy(t, "trigger", tb.Password.Reveal)
		}
		return c
	}
	if c := clock(); c.Label != view.Text(label) || c.On || !c.Enabled {
		t.Fatalf("clock before = %+v, want %q unticked", c, label)
	}
	secret, _ := issueTrigger(t, f, job)
	on := clock()
	if !on.On || !on.Enabled || on.Method != http.MethodPost {
		t.Fatalf("clock on = %+v", on)
	}
	raw := decode[map[string]any](t, f.h.do(get("/app/settings/backup?job="+job.ID, f.sess)))
	if trig := raw["job"].(map[string]any)["trigger"].(map[string]any); trig["off"] != nil {
		t.Errorf("the trigger card still has a switch of its own: %v", trig["off"])
	}

	v := jobForm(f, sealed.BackupToWebDAV, "8", "pw")
	v.Del("on")
	if rec := f.h.do(postForm("/app/settings/backup?op=save&job="+job.ID, f.sess, v)); rec.Code != http.StatusSeeOther {
		t.Fatalf("save off: %d", rec.Code)
	}
	if rec := f.h.do(script(http.MethodPost, "/api/backup/run?job="+job.ID, secret)); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "off") {
		t.Errorf("a job that is off started from outside: %d %q", rec.Code, rec.Body.String())
	}
	if rec := f.h.do(postForm(on.Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("untick: %d", rec.Code)
	}
	if rec := f.h.do(script(http.MethodGet, "/api/backup/status", secret)); rec.Code != http.StatusNotFound {
		t.Errorf("a secret unticked still works: %d", rec.Code)
	}
	if c := clock(); c.On {
		t.Errorf("unticked, the clock is still on: %+v", c)
	}
}

// s-admin-1, s-admin-11: Resend (for an invitation by letter) and Copy link
// on the row; Extend and Revoke under «⋯». A letter that did not go says
// the relay's reply.
func TestInviteRowIsTheFrame(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	relay := h.relayed()
	relay.Refuse = "anna@example.org"
	for _, action := range []string{"link", "send"} {
		v := url.Values{"email": {"anna@example.org"}}
		if rec := h.do(postForm("/admin/invites?action="+action, alice, v)); rec.Code != http.StatusSeeOther {
			t.Fatalf("create %s: %d %s", action, rec.Code, rec.Body)
		}
	}
	page := decode[view.AdminInvitesPage](t, h.do(get("/admin/invites", alice)))
	// Neither names a login: the person chooses it, and the first column is
	// the address — the letter's, or the frame's «—» for a link. The
	// address typed beside a link goes as a link.
	if head := regexp.MustCompile(`(?s)<thead><tr><th[^>]*>(.*?)</th>`).FindStringSubmatch(mockupFrame(t, "s-admin-1")); head == nil || head[1] != "Address" {
		t.Fatalf("s-admin-1 heads its first column %q: read the frame again", head)
	}
	rows := map[string]view.InviteRow{}
	for _, r := range page.Rows {
		rows[map[bool]string{true: "anna", false: "link"}[r.Delivery == "email"]] = r
	}
	frame := frameRows(t, "s-admin-1")
	cells := frameCells(t, "s-admin-1")
	for i, login := range []string{"link", "anna"} {
		row := rows[login]
		// Address, Delivery and Status are the frame's columns, in its words.
		if string(row.Address) != cells[i][0] || string(row.Delivery) != cells[i][1] || string(row.Status) != cells[i][2] {
			t.Errorf("%s address, delivery, status = %q, %q, %q, want %q, %q, %q (s-admin-1)", login, row.Address, row.Delivery, row.Status, cells[i][0], cells[i][1], cells[i][2])
		}
		if got := labels(row.Actions); !equalStrings(got, frame[i]) {
			t.Errorf("%s actions = %q, want %q", login, got, frame[i])
		}
		sameMenu(t, login+" invitation", row.Menu, "s-admin-11")
		sayWhy(t, login, append(row.Actions, menuActions(row.Menu)...)...)
		copyLink := actionByID(row.Actions, "admin.invite.copy")
		if copyLink == nil || copyLink.Method != http.MethodGet || copyLink.Href != string(row.Link) || row.Link == "" {
			t.Errorf("%s Copy link = %+v, link %q", login, copyLink, row.Link)
		}
	}
	if sent := string(rows["anna"].Sent); !strings.HasPrefix(sent, "failed — 550 ") {
		t.Errorf("mail column = %q, want the relay's reply", sent)
	}
	if sent := string(rows["link"].Sent); sent != cells[0][3] {
		t.Errorf("a link's mail column = %q, want the frame's %q", sent, cells[0][3])
	}
	if menu := rows["anna"].Menu; len(menu) == 0 || menu[len(menu)-1].ID != "admin.invite.revoke" || !menu[len(menu)-1].Danger {
		t.Errorf("menu = %+v, want Revoke last, marked", menu)
	}
}

// s-admin-12, s-admin-2: a person's row has only «⋯» — the role as one
// toggle (Make administrator, Make user), Kill sessions, Disable (Enable for
// one disabled), and Delete under the line. There is no sheet of roles and
// no reset in the row: the password is reset from the head (s-admin-13).
// The role is the word of s-admin-8.
func TestUserMenuIsTheFrame(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	bobID := h.invite(adminID, "bob", "bobpass123")
	alice := h.login("alice", "password123")

	users := func() map[string]view.AdminUserRow {
		page := decode[view.AdminUsersPage](t, h.do(get("/admin/", alice)))
		out := map[string]view.AdminUserRow{}
		for _, r := range page.Rows {
			out[string(r.Login)] = r
			sayWhy(t, string(r.Login), menuActions(r.Menu)...)
		}
		return out
	}
	rows := users()
	sameMenu(t, "bob", rows["bob"].Menu, "s-admin-12")
	head := mockupFrame(t, "s-admin-2")
	for login, role := range map[string]string{"alice": "Administrator", "bob": "User"} {
		if string(rows[login].Role) != role || !strings.Contains(head, "<td>"+role+"</td>") {
			t.Errorf("%s role = %q, want %q as s-admin-2 writes it", login, rows[login].Role, role)
		}
	}
	item := func(row view.AdminUserRow, id string) view.Action {
		for _, it := range row.Menu {
			if it.ID == id {
				return it.Action
			}
		}
		t.Fatalf("%s: no %s in %v", row.Login, id, serverMenu(row.Menu))
		return view.Action{}
	}
	// The last administrator can be neither demoted, disabled nor deleted:
	// the items are there, off, with the reason; the role's is the
	// figcaption's.
	for _, id := range []string{"admin.user.role", "admin.user.disable", "admin.user.delete"} {
		if a := item(rows["alice"], id); a.Enabled {
			t.Errorf("last admin: %s is on", id)
		}
	}
	if a := item(rows["alice"], "admin.user.role"); a.Label != "Make user" || a.Reason != "Someone has to stay an administrator." {
		t.Errorf("last admin's role = %+v", a)
	}
	// Ад7: Delete sends no login, and the store refuses it every time — off
	// for everyone, with the reason, until its dialogue is drawn (task 195).
	if a := item(rows["bob"], "admin.user.delete"); a.Enabled || a.Reason == "" {
		t.Errorf("bob's Delete = %+v, want off with its reason", a)
	}
	// Bob has not signed in: nothing to kill.
	if a := item(rows["bob"], "admin.user.sessions"); a.Enabled || a.Label != "Kill sessions" {
		t.Errorf("bob's sessions = %+v", a)
	}
	if rec := h.do(get("/admin/role?target="+bobID, alice)); strings.Contains(rec.Body.String(), "admin.user.role.save") {
		t.Errorf("GET /admin/role still answers with the sheet of roles: %d", rec.Code)
	}

	// The toggle is the role's one move: Bob becomes an administrator, his
	// item says Make user, and Alice is no longer the last one.
	toggle := item(rows["bob"], "admin.user.role")
	if toggle.Method != http.MethodPost {
		t.Fatalf("Make administrator = %+v", toggle)
	}
	if rec := h.do(postForm(toggle.Href, alice, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("make administrator: %d %s", rec.Code, rec.Body)
	}
	if st := h.st.Snapshot(); st.UserByID(bobID) == nil || st.UserByID(bobID).Role != "admin" {
		t.Fatalf("bob = %+v", st.UserByID(bobID))
	}
	rows = users()
	if a := item(rows["bob"], "admin.user.role"); a.Label != "Make user" || !a.Enabled {
		t.Errorf("bob's role after = %+v", a)
	}
	if a := item(rows["alice"], "admin.user.role"); a.Label != "Make user" || !a.Enabled {
		t.Errorf("alice's role with two admins = %+v", a)
	}
	if rec := h.do(postForm(item(rows["bob"], "admin.user.role").Href, alice, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("make user: %d", rec.Code)
	}
	if st := h.st.Snapshot(); st.UserByID(bobID).Role != "user" {
		t.Fatalf("bob after Make user = %+v", st.UserByID(bobID))
	}
	rows = users()

	// Disable, then Enable in its place.
	if rec := h.do(postForm(item(rows["bob"], "admin.user.disable").Href, alice, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("disable: %d", rec.Code)
	}
	rows = users()
	want := frameMenu(t, "s-admin-12")
	for i := range want {
		if want[i] == "Disable []" {
			want[i] = "Enable []" // the figcaption: «У отключённого вместо Disable — Enable»
		}
	}
	if got := serverMenu(rows["bob"].Menu); !equalStrings(got, want) {
		t.Errorf("disabled bob «⋯» = %q, want %q", got, want)
	}
	enable := item(rows["bob"], "admin.user.enable")
	if enable.Label != "Enable" || !rows["bob"].Disabled {
		t.Fatalf("enable = %+v", enable)
	}
	if rec := h.do(postForm(enable.Href, alice, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("enable: %d", rec.Code)
	}
	snap := h.st.Snapshot()
	if u := snap.UserByID(bobID); u == nil || u.Disabled {
		t.Fatalf("bob after Enable = %+v", u)
	}
	if last := snap.Audit[len(snap.Audit)-1]; last.What != "user.enable" || last.Whom != "bob" || last.Who != "alice" {
		t.Errorf("journal = %+v", last)
	}
}
