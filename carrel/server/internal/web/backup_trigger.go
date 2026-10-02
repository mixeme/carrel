package web

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
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

// triggerClock is the tick of s-backup-2 that turns the external trigger on
// and, cleared, off: there is no switch of its own (to-typecase task 18a).
const triggerClock = "On a clock of your own — cron, GoSentry, a job in Gitea"

// triggerPrice is the hint under the tick, ticked or not: what turning the
// trigger on costs is said before it is turned on (s-backup-2, S-24.4).
const triggerPrice = "Turning it on stores this job's data key under the server's key — less protection, for convenience: the job can then read without you signed in."

// triggerTick is the job's tick «On a clock of your own» (s-backup-2):
// ticked, it issues the trigger — through the password sheet outside the
// unlock window, since it hands out the job's reading — and cleared, it
// turns the trigger off: the lines handed out stop. Clearing issues
// nothing and asks for no password.
func (s *Server) triggerTick(sess *session.Session, job sealed.BackupJob, issued map[string]string) view.Action {
	q := func(op string) string {
		return s.postQuery("/app/settings/backup", url.Values{"op": {op}, "job": {job.ID}})
	}
	tick := view.Action{ID: "backup.trigger.clock", Label: triggerClock, Href: q("trigger"), Method: "POST", Enabled: true}
	switch {
	case s.users.BackupTriggerOn(sess.AccountID(), job.ID) || issued[job.ID] != "":
		tick.On, tick.Href = true, q("trigger-off")
	case sess.UnlockUntil(time.Now()) == nil:
		tick.Method, tick.Href = "GET", s.route("/app/settings/unlock")
	}
	return tick
}

// triggerBlock is the job's card of the line: the command and the secret
// when they have just been issued, otherwise that it is on and how to renew
// it. While the trigger is off there is nothing to show or renew, and no
// card (owner's decision, from-typecase note 19).
func (s *Server) triggerBlock(sess *session.Session, job sealed.BackupJob, issued, secrets map[string]string) *view.TriggerBlock {
	on := s.users.BackupTriggerOn(sess.AccountID(), job.ID)
	if !on && issued[job.ID] == "" {
		return nil
	}
	b := &view.TriggerBlock{
		Password: view.SecretField{Reveal: view.Action{ID: "backup.trigger", Label: "New password", Method: "POST", Enabled: true,
			Hint: "The old password stops working; whatever runs this job has to be given the new one.",
			Href: s.postQuery("/app/settings/backup", url.Values{"op": {"trigger"}, "job": {job.ID}})}},
	}
	if issued[job.ID] != "" && secrets[job.ID] != "" {
		triggerTabs(b, issued[job.ID], job, secrets[job.ID])
		b.Password.Shown, b.Password.Value = true, text(secrets[job.ID])
		// What turning it on cost is said under the tick (triggerPrice), not
		// a second time here.
		b.Note = "Visible only now. Every use is logged; New password replaces it on the spot, and every line already handed out stops working."
	} else {
		b.Note = "The external trigger is on. The password was shown once; New password issues another, and the old lines stop working."
	}
	// Outside the unlock window New password opens the password sheet; what
	// it is for is said in the note, not as a reason of a working button
	// (C-6.2).
	if sess.UnlockUntil(time.Now()) == nil {
		b.Password.Reveal.Method, b.Password.Reveal.Href = "GET", s.route("/app/settings/unlock")
		b.Note = "Confirm your password first: the trigger hands out this job's reading. " + b.Note
	}
	if !job.Encrypted {
		b.Note += " This job's archive is not encrypted: whoever holds the secret can take your data in the clear."
	}
	return b
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
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || sharedAddressSpace.Contains(ip)
	}
	if !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".lan", ".home", ".internal", ".intranet", ".corp", ".home.arpa", ".test"} {
		if strings.HasSuffix("."+host, suffix) {
			return true
		}
	}
	return false
}

// sharedAddressSpace is 100.64.0.0/10 (RFC 6598): carrier-grade NAT and the
// overlay networks that borrow it — reachable only from inside.
var sharedAddressSpace = netip.MustParsePrefix("100.64.0.0/10")

// enableTrigger issues the job's trigger, inside the unlock window only, and
// shows the page with the command and the secret, once.
func (s *Server) enableTrigger(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, job sealed.BackupJob) {
	if sess.UnlockUntil(time.Now()) == nil {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, view.Notice{
			Role: view.NoticeAlert, Title: "Confirm your password first",
			Body:    "The external trigger lets a job read without you signed in, so it is issued only inside the unlock window.",
			Actions: []view.Action{{ID: "settings.unlock", Label: "Confirm password", Href: s.route("/app/settings/unlock"), Method: "GET", Enabled: true}},
		}))
		return
	}
	secret, err := s.users.EnableBackupTrigger(sess.AccountID(), key, job.ID)
	if err != nil {
		http.Error(w, "could not issue the trigger", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	page := s.backupPageIssued(sess, map[string]string{job.ID: s.publicOrigin(r)}, map[string]string{job.ID: secret})
	s.writePage(w, r, http.StatusOK, page)
}
