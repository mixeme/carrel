package user

import (
	"fmt"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/store"
)

// InstanceUpdate is the administrator's form: the name, the lifetimes, and
// people's mail with its ceilings. Zero is the default.
type InstanceUpdate struct {
	Name              string
	IdleHours         int
	SessionDays       int
	InviteHours       int
	PeopleMail        bool
	MailMaxAttachMB   int
	MailMaxMessageMB  int
	MailMaxRecipients int
	MailMaxPerDay     int
}

// ErrSetting is a value out of its range; Field names the form row.
type ErrSetting = ErrSMTP

func inRange(field string, v, lo, hi int, what string) error {
	if v != 0 && (v < lo || v > hi) {
		return &ErrSetting{field, fmt.Sprintf("%s from %d to %d.", what, lo, hi)}
	}
	return nil
}

// SetInstance stores the settings and applies the lifetimes to the session
// manager at once. The journal names what changed, as values.
func (s *Service) SetInstance(actorID string, in InstanceUpdate) error {
	in.Name = strings.TrimSpace(in.Name)
	if len([]rune(in.Name)) > 60 || strings.ContainsAny(in.Name, "\r\n<>") {
		return &ErrSetting{"name", "A name of up to 60 characters, on one line."}
	}
	for _, err := range []error{
		inRange("idle", in.IdleHours, 1, 168, "Hours"),
		inRange("session", in.SessionDays, 1, 365, "Days"),
		inRange("inviteTTL", in.InviteHours, 1, 720, "Hours"),
		inRange("attach", in.MailMaxAttachMB, 1, 100, "Megabytes"),
		inRange("letter", in.MailMaxMessageMB, 1, 100, "Megabytes"),
		inRange("recipients", in.MailMaxRecipients, 1, 100, "Recipients"),
		inRange("perDay", in.MailMaxPerDay, 1, 1000, "Letters"),
	} {
		if err != nil {
			return err
		}
	}
	if in.MailMaxAttachMB != 0 && in.MailMaxMessageMB != 0 && in.MailMaxAttachMB > in.MailMaxMessageMB {
		return &ErrSetting{"attach", "An attachment cannot be larger than the letter."}
	}
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		set := &st.Settings
		set.Name, set.IdleHours, set.SessionDays, set.InviteHours = in.Name, in.IdleHours, in.SessionDays, in.InviteHours
		people := in.PeopleMail
		set.PeopleMail = &people
		set.MailMaxAttachMB, set.MailMaxMessageMB = in.MailMaxAttachMB, in.MailMaxMessageMB
		set.MailMaxRecipients, set.MailMaxPerDay = in.MailMaxRecipients, in.MailMaxPerDay
		return []store.AuditEntry{{When: s.now(), Who: actorLogin(st, actorID), What: "settings.instance",
			Detail: fmt.Sprintf("name %q, idle %dh, session %dd, invite %dh, people's mail %v, attachment %d MB, letter %d MB, %d recipients, %d a day (0 = default)",
				in.Name, in.IdleHours, in.SessionDays, in.InviteHours, in.PeopleMail, in.MailMaxAttachMB, in.MailMaxMessageMB, in.MailMaxRecipients, in.MailMaxPerDay)}}, nil
	})
	if err != nil {
		return err
	}
	s.applyLifetimes()
	return nil
}

// Settings is the instance's settings as stored.
func (s *Service) Settings() store.InstanceSettings {
	snap := s.st.Snapshot()
	return snap.Settings
}

// applyLifetimes hands the stored lifetimes to the session manager.
func (s *Service) applyLifetimes() {
	if s.mgr == nil {
		return
	}
	set := s.Settings()
	s.mgr.SetLifetimes(time.Duration(set.IdleHours)*time.Hour, time.Duration(set.SessionDays)*24*time.Hour)
}

// inviteTTL is how long an invitation lives.
func inviteTTL(st *store.State) time.Duration {
	if st.Settings.InviteHours > 0 {
		return time.Duration(st.Settings.InviteHours) * time.Hour
	}
	return conf.InviteTTL
}

// InviteTTL is how long an invitation made now lives.
func (s *Service) InviteTTL() time.Duration {
	snap := s.st.Snapshot()
	return inviteTTL(&snap)
}

// Journal writes one entry of the actor's: what was done and on what, for
// actions that change nothing of the instance itself (a check of somebody's
// server, S-6).
func (s *Service) Journal(actorID, what, detail string) {
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		return []store.AuditEntry{{When: s.now(), Who: actorLogin(st, actorID), What: what, Detail: detail}}, nil
	})
}
