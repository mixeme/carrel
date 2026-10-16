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

func inRange(field string, v, lo, hi int, what string) *ErrSetting {
	if v != 0 && (v < lo || v > hi) {
		return &ErrSetting{field, fmt.Sprintf("%s from %d to %d.", what, lo, hi)}
	}
	return nil
}

// checkInstance is every refusal of the instance's values, by the row it
// stands at.
func checkInstance(in *InstanceUpdate) []*ErrSetting {
	in.Name = strings.TrimSpace(in.Name)
	var out []*ErrSetting
	if len([]rune(in.Name)) > 60 || strings.ContainsAny(in.Name, "\r\n<>") {
		out = append(out, &ErrSetting{"name", "A name of up to 60 characters, on one line."})
	}
	for _, err := range []*ErrSetting{
		inRange("inviteTTL", in.InviteHours, 1, 720, "Hours"),
		inRange("idle", in.IdleHours, 1, 168, "Hours"),
		inRange("session", in.SessionDays, 1, 365, "Days"),
		inRange("attach", in.MailMaxAttachMB, 1, 100, "Megabytes"),
		inRange("letter", in.MailMaxMessageMB, 1, 100, "Megabytes"),
		inRange("recipients", in.MailMaxRecipients, 1, 100, "Recipients"),
		inRange("perDay", in.MailMaxPerDay, 1, 1000, "Letters"),
	} {
		if err != nil {
			out = append(out, err)
		}
	}
	if in.MailMaxAttachMB != 0 && in.MailMaxMessageMB != 0 && in.MailMaxAttachMB > in.MailMaxMessageMB {
		out = append(out, &ErrSetting{"attach", "An attachment cannot be larger than the letter."})
	}
	return out
}

// SaveSettings stores the one form of the instance's settings (s-admin-3,
// Ад17): the name, the lifetimes, the relay (S-26) and people's mail with
// its ceilings — all of it, or nothing and every refusal at its row at once.
// The lifetimes apply to the session manager at once. The journal has one
// entry naming the rows that changed — never a value of the relay's
// password (Ад38).
func (s *Service) SaveSettings(actorID string, in InstanceUpdate, relay store.SMTPSettings) ([]*ErrSetting, error) {
	refused := append(checkInstance(&in), CheckSMTP(&relay)...)
	if len(refused) > 0 {
		return refused, nil
	}
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if relay.Password == "" && relay.Username != "" {
			relay.Password = st.Settings.SMTP.Password
		}
		if relay.Username == "" {
			relay.Password = ""
		}
		set := &st.Settings
		changed := settingsChanged(*set, in, relay)
		set.Name, set.IdleHours, set.SessionDays, set.InviteHours = in.Name, in.IdleHours, in.SessionDays, in.InviteHours
		people := in.PeopleMail
		set.PeopleMail = &people
		set.MailMaxAttachMB, set.MailMaxMessageMB = in.MailMaxAttachMB, in.MailMaxMessageMB
		set.MailMaxRecipients, set.MailMaxPerDay = in.MailMaxRecipients, in.MailMaxPerDay
		set.SMTP = relay
		if len(changed) == 0 {
			return nil, nil
		}
		return []store.AuditEntry{{When: s.now(), Who: actorLogin(st, actorID), What: "settings.instance", Detail: strings.Join(changed, ", ")}}, nil
	})
	if err != nil {
		return nil, err
	}
	s.applyLifetimes()
	return nil, nil
}

// settingsChanged names the rows of the form a save changes, in the form's
// order (s-admin-7: «smtp_host, smtp_port»).
func settingsChanged(was store.InstanceSettings, in InstanceUpdate, relay store.SMTPSettings) []string {
	people := was.PeopleMail != nil && *was.PeopleMail
	var out []string
	for _, c := range []struct {
		name string
		diff bool
	}{
		{"inviteTTL", was.InviteHours != in.InviteHours},
		{"idle", was.IdleHours != in.IdleHours},
		{"session", was.SessionDays != in.SessionDays},
		{"name", was.Name != in.Name},
		{"host", was.SMTP.Host != relay.Host},
		{"port", was.SMTP.Port != relay.Port},
		{"security", was.SMTP.Security != relay.Security},
		{"username", was.SMTP.Username != relay.Username},
		// The journal holds no word of a secret (store.containsSecret).
		{"credentials", was.SMTP.Password != relay.Password},
		{"from", was.SMTP.From != relay.From},
		{"fromName", was.SMTP.FromName != relay.FromName},
		// Unset, people's letters are as the start set them; the box shows
		// that, and only a box ticked over it is a change.
		{"people", was.PeopleMail == nil && in.PeopleMail || was.PeopleMail != nil && people != in.PeopleMail},
		{"sender", was.SMTP.Sender != relay.Sender},
		{"attach", was.MailMaxAttachMB != in.MailMaxAttachMB},
		{"letter", was.MailMaxMessageMB != in.MailMaxMessageMB},
		{"recipients", was.MailMaxRecipients != in.MailMaxRecipients},
		{"perDay", was.MailMaxPerDay != in.MailMaxPerDay},
	} {
		if c.diff {
			out = append(out, c.name)
		}
	}
	return out
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
