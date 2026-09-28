package web

import (
	"net/http"
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

// triggerBlock is the job's trigger card: its tick, and the command and the
// secret when they have just been issued, otherwise whether it is on and how
// to renew it.
func (s *Server) triggerBlock(sess *session.Session, job sealed.BackupJob, issued map[string]string) view.TriggerBlock {
	q := func(op string) string {
		return s.postQuery("/app/settings/backup", url.Values{"op": {op}, "job": {job.ID}})
	}
	on := s.users.BackupTriggerOn(sess.AccountID(), job.ID)
	b := view.TriggerBlock{
		Password: view.SecretField{Reveal: view.Action{ID: "backup.trigger", Label: "Regenerate", Href: q("trigger"), Method: "POST", Enabled: true}},
		Clock:    view.Action{ID: "backup.trigger.clock", Label: triggerClock, Href: q("trigger"), Method: "POST", Enabled: true},
	}
	switch {
	case issued[job.ID] != "":
		b.Command = text(issued[job.ID])
		b.Password.Shown = true
		b.Note = "Visible only now. Every use is logged; Regenerate replaces the secret, and every line already handed out stops working."
	case on:
		b.Note = "The external trigger is on. The secret was shown once; Regenerate issues a new one and the old lines stop working."
	default:
		b.Password.Reveal.Href, b.Password.Reveal.Enabled = "", false
		b.Password.Reveal.Reason = "The external trigger is off: there is no line to renew."
		b.Note = "A script outside can start this job. Turning it on stores your data key for this job under the server's key — a loss of protection, for convenience: the job can then read without you signed in."
	}
	if on || issued[job.ID] != "" {
		// Cleared, the tick turns the trigger off: the lines handed out stop.
		b.Clock.On, b.Clock.Href = true, q("trigger-off")
	}
	// Outside the unlock window what issues a secret opens the password
	// sheet; what it is for is said in the note, not as a reason of a
	// working button (C-6.2). Clearing the tick issues nothing.
	if sess.UnlockUntil(time.Now()) == nil {
		if b.Password.Reveal.Enabled {
			b.Password.Reveal.Method, b.Password.Reveal.Href = "GET", s.route("/app/settings/unlock")
		}
		if !b.Clock.On {
			b.Clock.Method, b.Clock.Href = "GET", s.route("/app/settings/unlock")
		}
		b.Note = "Confirm your password first: the trigger hands out this job's reading. " + b.Note
	}
	if (on || issued[job.ID] != "") && !job.Encrypted {
		b.Note += " This job's archive is not encrypted: whoever holds the secret can take your data in the clear."
	}
	return b
}

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
