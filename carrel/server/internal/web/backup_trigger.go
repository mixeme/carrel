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
// the thing to paste where the clock already is (S-24.4): a crontab line for
// a job that writes to WebDAV — one step, start it — a line with -o for a
// job that downloads, and a short script for Both, which has two steps: start
// it, wait, take the file. The secret is shown once, when it is issued.

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

// triggerCommand is what to paste for job with secret, reaching base.
func triggerCommand(base string, job sealed.BackupJob, secret string) string {
	h := "Authorization: Bearer " + secret
	q := url.Values{"job": {job.ID}}.Encode()
	switch job.Mode {
	case sealed.BackupToDownload:
		// cron reads % as a newline: \% keeps the date in the line.
		return "0 3 * * * curl -fsS -H '" + h + "' -o /var/backups/carrel-" + job.ID + "-$(date +\\%F).zip '" + base + "/api/backup/download?" + q + "'"
	case sealed.BackupToBoth:
		return strings.Join([]string{
			"#!/bin/sh",
			"set -eu",
			"h='" + h + "'",
			"b=" + base + "/api/backup",
			"out=/var/backups",
			`id=$(curl -fsS -X POST -H "$h" "$b/run?` + q + `")`,
			`until [ "$(curl -fsS -H "$h" "$b/status?id=$id")" != running ]; do sleep 20; done`,
			`curl -fsS -H "$h" "$b/$id/download" -o "$out/carrel-` + job.ID + `-$(date +%F).zip"`,
		}, "\n")
	default:
		return "0 3 * * * curl -fsS -m 30 -X POST -H '" + h + "' '" + base + "/api/backup/run?" + q + "'"
	}
}

// triggerClock is the tick of s-backup-2 that turns the external trigger on
// and, cleared, off: there is no switch of its own (to-typecase task 18a).
const triggerClock = "On a clock of your own — cron, GoSentry, a job in Gitea"

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
func (s *Server) triggerBlock(sess *session.Session, job sealed.BackupJob, issued map[string]string) *view.TriggerBlock {
	on := s.users.BackupTriggerOn(sess.AccountID(), job.ID)
	if !on && issued[job.ID] == "" {
		return nil
	}
	b := &view.TriggerBlock{
		Password: view.SecretField{Reveal: view.Action{ID: "backup.trigger", Label: "Regenerate", Method: "POST", Enabled: true,
			Href: s.postQuery("/app/settings/backup", url.Values{"op": {"trigger"}, "job": {job.ID}})}},
	}
	if issued[job.ID] != "" {
		b.Command = text(issued[job.ID])
		b.Password.Shown = true
		b.Note = "Visible only now. Every use is logged; Regenerate replaces the secret, and every line already handed out stops working."
	} else {
		b.Note = "The external trigger is on. The secret was shown once; Regenerate issues a new one and the old lines stop working."
	}
	// Outside the unlock window Regenerate opens the password sheet; what
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
	page := s.backupPageIssued(sess, map[string]string{job.ID: triggerCommand(s.publicOrigin(r), job, secret)}, map[string]string{job.ID: secret})
	s.writePage(w, r, http.StatusOK, page)
}
