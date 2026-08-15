// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// Queue sends mail asynchronously with retries and a growing backoff (§5.3).
// A failed delivery is logged and recorded; the invite or confirmation link
// stays valid.
type Queue struct {
	Store  *store.Store
	Logger *slog.Logger
	// ServiceName appears in outbound messages.
	ServiceName string

	mu     sync.Mutex
	jobs   chan job
	closed bool
}

type job struct {
	kind       string
	inviteID   string
	to         string
	msg        Message
	maxRetries int
}

// Start spins up the background sender. Call Close before shutdown.
func (q *Queue) Start(ctx context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	q.mu.Lock()
	if q.jobs == nil {
		q.jobs = make(chan job, 64)
	}
	q.closed = false
	q.mu.Unlock()

	for i := 0; i < workers; i++ {
		go q.worker(ctx)
	}
}

// Close stops accepting new jobs and drains the queue.
func (q *Queue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.jobs != nil && !q.closed {
		close(q.jobs)
		q.closed = true
	}
}

func (q *Queue) enqueue(j job) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.jobs == nil || q.closed {
		return
	}
	select {
	case q.jobs <- j:
	default:
		// The queue is full: send synchronously in a goroutine so the HTTP
		// handler is not blocked, but the work is not dropped.
		go q.deliver(j, 0)
	}
}

func (q *Queue) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-q.jobs:
			if !ok {
				return
			}
			q.deliver(j, 0)
		}
	}
}

func (q *Queue) deliver(j job, attempt int) {
	cfg, err := q.loadConfig()
	if err != nil {
		q.logError("load smtp config", err)
		q.recordInvite(j, store.SendFailed, err.Error())
		return
	}
	if cfg.Host == "" || cfg.FromAddress == "" {
		q.recordInvite(j, store.SendNotConfigured, "smtp not configured")
		return
	}

	j.msg.To = j.to
	res := Send(cfg, j.msg)
	if res.OK {
		q.recordInvite(j, store.SendOK, "")
		return
	}

	if attempt < j.maxRetries {
		delay := time.Duration(1<<attempt) * time.Second
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		time.Sleep(delay)
		q.deliver(j, attempt+1)
		return
	}

	q.logError("send mail", nil)
	if q.Logger != nil {
		q.Logger.Error("mail delivery failed", "kind", j.kind, "to", j.to, "diagnostic", res.Diagnostic)
	}
	q.recordInvite(j, store.SendFailed, truncate(res.Diagnostic, 500))
}

func (q *Queue) recordInvite(j job, status store.SendStatus, detail string) {
	if j.kind != "invite" || j.inviteID == "" {
		return
	}
	if err := q.Store.RecordInviteSend(j.inviteID, status, detail); err != nil && q.Logger != nil {
		q.Logger.Error("record invite send", "error", err)
	}
}

func (q *Queue) loadConfig() (Config, error) {
	settings := q.Store.Settings()
	password, err := q.Store.SMTPPassword()
	if err != nil {
		return Config{}, err
	}
	s := settings.SMTP
	return Config{
		Host:        s.Host,
		Port:        s.Port,
		TLS:         s.TLS,
		Username:    s.Username,
		Password:    password,
		FromAddress: s.FromAddress,
		FromName:    s.FromName,
	}, nil
}

// SendTest delivers one message synchronously and returns the full diagnostic
// transcript for the admin UI (§5.3).
func (q *Queue) SendTest(to string) Result {
	cfg, err := q.loadConfig()
	if err != nil {
		return Result{Diagnostic: "load smtp config: " + err.Error()}
	}
	msg := Message{
		Subject: "Carrel test message",
		Text:    "This is a test message from Carrel. If you received it, SMTP is working.",
		HTML:    "<p>This is a test message from <strong>Carrel</strong>. If you received it, SMTP is working.</p>",
	}
	res := Send(cfg, Message{
		To:      to,
		Subject: msg.Subject,
		Text:    msg.Text,
		HTML:    msg.HTML,
	})
	return res
}

// QueueInvite enqueues an invitation email. When SMTP is unset the store already
// marks the invite accordingly; nothing is queued.
func (q *Queue) QueueInvite(inviteID, to, invitedBy, inviteURL string, expires time.Time) {
	if to == "" {
		return
	}
	settings := q.Store.Settings()
	if !settings.SMTP.Configured() {
		return
	}
	name := q.ServiceName
	if name == "" {
		name = "Carrel"
	}
	msg := InviteContent(name, invitedBy, inviteURL, expires)
	q.enqueue(job{kind: "invite", inviteID: inviteID, to: to, msg: msg, maxRetries: 4})
}

// QueueEmailChange enqueues a confirmation message for a profile email change.
func (q *Queue) QueueEmailChange(to, login, confirmURL string, expires time.Time) {
	if to == "" {
		return
	}
	settings := q.Store.Settings()
	if !settings.SMTP.Configured() {
		return
	}
	name := q.ServiceName
	if name == "" {
		name = "Carrel"
	}
	msg := EmailChangeContent(name, login, confirmURL, expires)
	q.enqueue(job{kind: "email_change", to: to, msg: msg, maxRetries: 4})
}

// QueueRegistration enqueues the confirmation message for a public sign-up.
func (q *Queue) QueueRegistration(to, login, confirmURL string, expires time.Time) {
	if to == "" {
		return
	}
	settings := q.Store.Settings()
	if !settings.SMTP.Configured() {
		return
	}
	name := q.ServiceName
	if name == "" {
		name = "Carrel"
	}
	msg := RegisterContent(name, login, confirmURL, expires)
	q.enqueue(job{kind: "email_change", to: to, msg: msg, maxRetries: 4})
}

// QueueEscrowRecovery tells a user that their account was recovered. Notifying
// them is not the administrator's decision to make, so this has no opt-out
// (§5.4); it reports whether the message could be handed to the queue at all,
// because an instance with no relay leaves the administrator to say it in
// person.
func (q *Queue) QueueEscrowRecovery(to, login string, recoveredAt time.Time) bool {
	if to == "" {
		return false
	}
	settings := q.Store.Settings()
	if !settings.SMTP.Configured() {
		return false
	}
	name := q.ServiceName
	if name == "" {
		name = "Carrel"
	}
	msg := EscrowRecoveryContent(name, login, recoveredAt.UTC().Format(time.RFC1123))
	// More attempts than the other messages get: an unsent invitation is an
	// inconvenience, an unsent recovery notice is a transparency failure.
	q.enqueue(job{kind: "escrow_recovery", to: to, msg: msg, maxRetries: 6})
	return true
}

func (q *Queue) logError(msg string, err error) {
	if q.Logger != nil {
		q.Logger.Error(msg, "error", err)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
