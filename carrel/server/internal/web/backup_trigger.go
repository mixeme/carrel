package web

import (
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// What the screen hands over for the external trigger is not the secret but
// the thing to paste where the clock already is (S-24.4, s-backup-2…5): for
// a job that writes to WebDAV — one step, start it — a crontab line; for one
// that downloads, and for Both, a short script — somebody has to take the
// stream — and the line that runs it; the fields of a GoSentry job, a Gitea
// workflow, the request alone, and what the secret opens. The secret is
// shown once, when it is issued.

// publicOrigin is Carrel's address as a script outside reaches it:
// CARREL_PUBLIC_URL, else the address this request came to.
func (s *Server) publicOrigin(r *http.Request) string {
	if o := strings.TrimSuffix(s.cfg.PublicURL, "/"); o != "" {
		return o
	}
	scheme := "http"
	if s.secureCookies(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// triggerSchedule is the schedule the handed-over lines carry, and its time
// as the card says it: it is changed there, on the machine that keeps it.
const (
	triggerSchedule = "0 3 * * *"
	triggerAt       = "03:00"
	triggerHeader   = "Authorization"
)

// triggerScript is the script a job that downloads needs: somebody has to be
// at the other end of the stream. Download takes the archive in the request
// that builds it; Both starts the copy, waits for it, and takes the file.
// The date lives in the script: cron reads % as a newline.
func triggerScript(base string, job sealed.BackupJob, secret string) string {
	q := url.Values{"job": {job.ID}}.Encode()
	head := []string{"#!/bin/sh", "set -eu", "h='" + triggerHeader + ": Bearer " + secret + "'", "b=" + base + "/api/backup", "out=/var/backups", ""}
	if job.Mode == sealed.BackupToDownload {
		return strings.Join(append(head, `curl -fsS -H "$h" "$b/download?`+q+`" -o "$out/carrel-`+job.ID+`-$(date +%F).zip"`), "\n")
	}
	return strings.Join(append(head,
		`id=$(curl -fsS -X POST -H "$h" "$b/run?`+q+`")`,
		`until [ "$(curl -fsS -H "$h" "$b/status?id=$id")" != running ]; do sleep 20; done`,
		`curl -fsS -H "$h" "$b/$id/download" -o "$out/carrel-`+job.ID+`-$(date +%F).zip"`,
	), "\n")
}

// triggerTabs fills the card with what each tab hands over for job, with
// secret, reaching base (s-backup-2…5): crontab — the line that starts a
// copy, or the script and the line that runs it; GoSentry — the fields of
// its job dialog; Gitea — a workflow of Gitea Actions on the same schedule,
// the secret by the name of a repository secret; curl — the request alone;
// and what the secret opens.
func triggerTabs(b *view.TriggerBlock, base string, job sealed.BackupJob, secret string) {
	q := url.Values{"job": {job.ID}}.Encode()
	runURL := base + "/api/backup/run?" + q
	bearer := triggerHeader + ": Bearer " + secret
	b.Shown, b.At, b.Header = true, triggerAt, triggerHeader
	b.Script = job.Mode == sealed.BackupToDownload || job.Mode == sealed.BackupToBoth
	b.Curl = text("curl -fsS -m 30 -X POST -H '" + bearer + "' '" + runURL + "'")
	b.GoSentry = view.GoSentryJob{Name: text("Carrel · " + job.Name), Folder: "Backup", Schedule: triggerSchedule}
	b.Status = text("GET /api/backup/status?" + q)
	// The repository keeps the password under the name the workflow gives
	// it, and the file under the job's name (s-backup-10).
	b.GiteaSecret = "CARREL_BACKUP"
	b.GiteaFile = text(".gitea/workflows/carrel-" + job.ID + ".yaml")
	fromRepo := "${{ secrets." + string(b.GiteaSecret) + " }}"
	gitea := []string{"name: Carrel backup · " + job.Name, "on:", "  schedule:", "    - cron: '" + triggerSchedule + "'",
		"jobs:", "  backup:", "    runs-on: ubuntu-latest", "    steps:"}
	if b.Script {
		name := "carrel-" + job.ID + ".sh"
		b.Command, b.ScriptName = text(triggerScript(base, job, secret)), text(name)
		b.CronLine = text(triggerSchedule + " /usr/local/bin/" + name)
		b.GoSentry.Command = text("/usr/local/bin/" + name)
		// The same script with the repository's secret; the run keeps the
		// archive it took.
		gitea = append(gitea, "      - run: |")
		for _, l := range strings.Split(triggerScript(base, job, fromRepo), "\n")[1:] {
			if l == "out=/var/backups" {
				l = "out=."
			}
			gitea = append(gitea, strings.TrimRight("          "+l, " "))
		}
		gitea = append(gitea, "      - uses: actions/upload-artifact@v3", "        with:", "          name: carrel-"+job.ID, "          path: carrel-*.zip")
	} else {
		b.Command = text(triggerSchedule + " " + string(b.Curl))
		b.GoSentry.Command = "curl"
		b.GoSentry.Arguments = []view.Text{"-fsS", "-m", "30", "-X", "POST", "-H", text(bearer), text(runURL)}
		gitea = append(gitea, `      - run: curl -fsS -m 30 -X POST -H "`+triggerHeader+": Bearer "+fromRepo+`" '`+runURL+"'")
	}
	b.Gitea = text(strings.Join(gitea, "\n"))
	// What the secret opens (s-backup-5): a job that downloads takes its
	// archive in one request; the others start, ask, take.
	if job.Mode == sealed.BackupToDownload {
		b.Endpoints = []view.Text{text("GET /api/backup/download?" + q)}
		return
	}
	b.Endpoints = []view.Text{text("POST /api/backup/run?" + q), b.Status, "GET /api/backup/{id}/download"}
}

// triggerPrice is what turning the trigger on costs, said before it is
// turned on (s-backup-14, S-24.4).
const triggerPrice = "Turning it on stores this job’s data key under the server’s key — less protection, for convenience: the job can then read without you signed in."

// triggerMasked is the secret in a line that does not show it (s-backup-12).
const triggerMasked = "cbk_••••"

// triggerSheet is the sheet of a job's external trigger over the list of
// jobs (Typecase's note 91): off — what turning it on costs and Turn on
// (s-backup-14); on — the password as a row, how it was used, New password
// and Revoke, and the line to paste with the password hidden (s-backup-12);
// just issued — the password and the line whole, this once (s-backup-13).
func (s *Server) triggerSheet(r *http.Request, sess *session.Session, job sealed.BackupJob, tab, secret string) view.BackupTriggerSheet {
	here := url.Values{"job": {job.ID}}
	q := func(op string) string {
		v := url.Values{"job": {job.ID}, "op": {op}}
		return s.postQuery("/app/settings/backup/trigger", v)
	}
	sheet := view.BackupTriggerSheet{Title: "External trigger", Subject: text(job.Name), Local: s.instanceLocal(),
		Form: s.plainForm(s.postQuery("/app/settings/backup/trigger", here), "POST", csrfField(sess))}
	closeTo := view.Action{ID: "backup.trigger.close", Label: "Close", Href: s.route("/app/settings/backup"), Method: "GET", Enabled: true}
	use, on := s.users.BackupTriggerUse(sess.AccountID(), job.ID)
	switch {
	case secret != "":
		// Just issued, the password is whole and has no hint (s-backup-13).
		sheet.Password = &view.TriggerPassword{Value: text(secret)}
		sheet.Line = s.triggerLine(r, job, tab, secret, line("Every use is logged, naming this password. The old one stopped working when this one was issued."))
		sheet.Form.Actions = []view.Action{closeTo}
	case on:
		tf := s.timeFor(sess)
		used := []any{"issued " + tf.dayOfYear(use.Issued)}
		if use.Runs > 0 {
			used = append(used, " · last run "+tf.dayOfYear(use.LastRun)+" "+tf.clock(use.LastRun)+" from ", monoRun(use.From), fmt.Sprintf(" · %s", plural(use.Runs, "run")))
		} else {
			used = append(used, " · not run yet")
		}
		// Outside the unlock window New password and Turn on answer with the
		// password's sheet (s-backup-14): they hand out the job's reading.
		renew := view.Action{ID: "backup.trigger.new", Label: "New password", Href: q("new"), Method: "POST", Enabled: true, Submits: true}
		sheet.Password = &view.TriggerPassword{Masked: text(use.Hint), Use: join(used...), Actions: []view.Action{renew,
			{ID: "backup.trigger.revoke", Label: "Revoke", Href: q("revoke"), Method: "POST", Enabled: true, Submits: true, Danger: true}}}
		sheet.Line = s.triggerLine(r, job, tab, triggerMasked, join("The password was shown once and is hidden here. ", strongRun("New password"), " shows the line whole; the old one stops working."))
		sheet.Form.Actions = []view.Action{closeTo}
	default:
		sheet.Warning = triggerPrice
		turnOn := view.Action{ID: "backup.trigger.on", Label: "Turn on", Href: q("on"), Method: "POST", Enabled: true, Primary: true, Submits: true}
		sheet.Form.Actions = []view.Action{turnOn,
			{ID: "backup.trigger.cancel", Label: "Cancel", Href: s.route("/app/settings/backup"), Method: "GET", Enabled: true}}
	}
	return sheet
}

// triggerLine is «The line to paste» of the sheet: the tabs as links, the
// open one's content, with secret — whole once, else hidden —, and note
// under it.
func (s *Server) triggerLine(r *http.Request, job sealed.BackupJob, tab, secret string, note []view.Run) *view.TriggerBlock {
	if tab == "" {
		tab = "crontab"
	}
	b := &view.TriggerBlock{Tabs: s.triggerTabLinks(job, tab)}
	triggerTabs(b, s.publicOrigin(r), job, secret)
	b.Shown, b.Note = secret != triggerMasked, note
	// A script and not a line, and why (s-backup-3).
	switch job.Mode {
	case sealed.BackupToBoth:
		b.Destination = join("Destination: ", strongRun("WebDAV and download"), " — two steps, so this is a file, not a line")
	case sealed.BackupToDownload:
		b.Destination = join("Destination: ", strongRun("download"), " — somebody has to take the stream, so this is a file, not a line")
	}
	if !job.Encrypted {
		b.Note = append(b.Note, view.Run{Text: " This job’s archive is not encrypted: whoever holds the password can take your data in the clear."})
	}
	return b
}

// getBackupTrigger is the sheet of a job's trigger (note 91): a page of its
// own over the list of jobs without scripts (U-8), and the password just
// issued, once.
func (s *Server) getBackupTrigger(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	job, has := s.catalogOf(sess).BackupJob(r.URL.Query().Get("job"))
	if !has {
		s.notFound(w, r)
		return
	}
	s.writeTriggerSheet(w, r, sess, job)
}

// writeTriggerSheet answers with the sheet of job's trigger: a fragment, or
// a page over the list of jobs; the password just issued, once.
func (s *Server) writeTriggerSheet(w http.ResponseWriter, r *http.Request, sess *session.Session, job sealed.BackupJob) {
	secret, _ := issuedTrigger(sess, job.ID)
	w.Header().Set("Cache-Control", "no-store")
	sheet := s.triggerSheet(r, sess, job, r.URL.Query().Get("tab"), secret)
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	sheet.Under = s.underOf(r, "/app/settings/backup", "/app/settings/backup")
	s.writePage(w, r, http.StatusOK, sheet)
}

// postBackupTrigger turns the trigger on, issues a new password — inside
// the unlock window, the answer the sheet with it whole, once; outside it,
// the password's sheet —, or revokes it: the password stops working and the
// trigger is off. A fragment is answered with the sheet in place, a plain
// form with a redirect to it.
func (s *Server) postBackupTrigger(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	job, has := s.catalogOf(sess).BackupJob(r.URL.Query().Get("job"))
	if !live || !has {
		s.notFound(w, r)
		return
	}
	back := "/app/settings/backup/trigger?" + url.Values{"job": {job.ID}}.Encode()
	switch r.URL.Query().Get("op") {
	case "on", "new":
		if sess.UnlockUntil(s.now()) == nil {
			if render.WantsFragment(r) {
				_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, "", back, unlockTrigger, url.Values{"job": {job.ID}}), nil)
				return
			}
			http.Redirect(w, r, s.triggerUnlock(job), http.StatusSeeOther)
			return
		}
		secret, err := s.users.EnableBackupTrigger(sess.AccountID(), key, job.ID)
		if err != nil {
			http.Error(w, "could not issue the trigger", http.StatusInternalServerError)
			return
		}
		sess.SetDraft(session.DraftIssuedTrigger, []byte(job.ID+"\x00"+secret))
	case "revoke":
		if err := s.users.DisableBackupTrigger(sess.AccountID(), job.ID); err != nil {
			http.Error(w, "could not revoke the trigger", http.StatusInternalServerError)
			return
		}
	default:
		s.notFound(w, r)
		return
	}
	if render.WantsFragment(r) {
		s.writeTriggerSheet(w, r, sess, job)
		return
	}
	s.redirect(w, r, back)
}

// triggerTabs are the tabs of a job's card of the line (s-backup-3): links
// to its page with ?trigger= — what is shown, not kept —, the one open on.
var triggerTabNames = []struct{ id, label string }{{"crontab", "crontab"}, {"gosentry", "GoSentry"}, {"gitea", "Gitea"}, {"curl", "curl"}}

func (s *Server) triggerTabLinks(job sealed.BackupJob, open string) []view.Action {
	var out []view.Action
	for _, t := range triggerTabNames {
		out = append(out, view.Action{ID: "backup.trigger.tab." + t.id, Label: text(t.label), Method: "GET", Enabled: true, On: t.id == open,
			Href: s.postQuery("/app/settings/backup/trigger", url.Values{"job": {job.ID}, "tab": {t.id}})})
	}
	return out
}

// instanceLocal is whether the addresses this instance hands out answer
// only from inside its own network (the badge «local», s-backup-2): no
// outside address is declared — the line is built from where the browser
// came — or the declared one is a local name or an address of a private
// network, a loopback, a link-local or a shared (CGNAT) one.
func (s *Server) instanceLocal() bool {
	u, err := url.Parse(strings.TrimSpace(s.cfg.PublicURL))
	if err != nil || u.Hostname() == "" {
		return true
	}
	return localHost(u.Hostname())
}

// localHost is whether a host does not answer from the internet: a private,
// loopback or link-local address, a name without a dot, or a name of a
// local or reserved suffix — one rule for the instance's address and
// davloom's (they told local apart differently).
func localHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || sharedAddressSpace.Contains(ip)
	}
	if !strings.Contains(host, ".") {
		return true
	}
	// .example and .invalid are reserved (RFC 2606) and never answer from
	// outside, as .test: the frames' instance, carrel.example, is local.
	for _, suffix := range []string{".localhost", ".local", ".lan", ".home", ".internal", ".intranet", ".corp", ".home.arpa", ".test", ".example", ".invalid"} {
		if strings.HasSuffix("."+host, suffix) {
			return true
		}
	}
	return false
}

// sharedAddressSpace is 100.64.0.0/10 (RFC 6598): carrier-grade NAT and the
// overlay networks that borrow it — reachable only from inside.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// triggerUnlock is the password's sheet over the trigger's sheet: turning
// the trigger on, and a new password, hand out the job's reading (193 p. 3,
// Typecase's notes 86 and 91).
func (s *Server) triggerUnlock(job sealed.BackupJob) string {
	return s.unlockHref(unlockTrigger, "/app/settings/backup/trigger?job="+url.QueryEscape(job.ID), "job", job.ID)
}
