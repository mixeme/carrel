// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// Form fields of the escrow screens.
const (
	fieldUserID         = "user_id"
	fieldMasterPassword = "master_password"
	fieldNewMaster      = "new_master_password"
	fieldConfirmMaster  = "confirm_master_password"
	fieldForbidOptOut   = "forbid_opt_out"
	fieldConfirmReset   = "confirm_reset"
)

// escrowNotice is what a user covered by key deposit is told at their first
// login. §5.4 asks for the honest wording rather than "key backup": what the
// scheme grants is read access to the account's saved credentials.
const escrowNotice = "Key escrow is active for this account. An administrator " +
	"who holds the escrow master password can decrypt the DAV credentials you " +
	"save here, and through them your contacts and calendars. Your profile " +
	"shows the current status."

// escrowStatus is what a user is shown about key deposit. Publishing it is not
// a courtesy: §5.4 makes this transparency the condition on which the option
// exists at all, so the fields cover both what the instance does and what has
// been done to this account.
type escrowStatus struct {
	// Configured reports whether a recovery key pair exists at all.
	Configured bool
	// Enabled reports whether new accounts are deposited. It can be false
	// while copies from an earlier period are still recoverable.
	Enabled bool
	// Deposited reports whether this account's own key can be recovered.
	Deposited bool
	// ForbidOptOut reports the administrator's policy, which the user sees
	// whether or not it currently affects them.
	ForbidOptOut bool
	// RecoveredAt is the last time an administrator used the scheme on this
	// account. It is never cleared.
	RecoveredAt time.Time
}

// CanOptIn reports whether the user may join the scheme voluntarily.
func (e escrowStatus) CanOptIn() bool { return e.Enabled && !e.Deposited }

// CanOptOut reports whether the user may delete their deposited copy.
func (e escrowStatus) CanOptOut() bool { return e.Deposited && !e.ForbidOptOut }

// escrowStatusOf reads the status of one account.
func escrowStatusOf(settings store.Settings, u *store.User) escrowStatus {
	st := escrowStatus{
		Configured:   settings.Escrow.Config != nil,
		Enabled:      settings.Escrow.Active(),
		ForbidOptOut: settings.Escrow.ForbidOptOut,
	}
	if u != nil {
		st.Deposited = len(u.EscrowDEK) > 0
		st.RecoveredAt = u.EscrowRecoveredAt
	}
	return st
}

// firstLoginEscrowNotice tells a user once that their account is covered. The
// profile carries the permanent status; this is the explicit notice §5.4 asks
// for at first login, and it is marked as delivered as soon as it is rendered.
func (s *Server) firstLoginEscrowNotice(r *http.Request, v *View) {
	sess := SessionFrom(r)
	if sess == nil || !sess.EscrowNotice() || v.Notice != "" {
		return
	}
	v.Notice = escrowNotice
	sess.ClearEscrowNotice()
	if err := s.Store.MarkEscrowNoticeSeen(sess.UserID); err != nil {
		s.logError("mark escrow notice seen", err)
	}
}

// ProfileEscrow takes the user's own decision about key deposit. Joining needs
// their password, because their DEK cannot be opened without it — that the
// server cannot do this on its own is the reason escrow is never retroactive
// (§5.4).
func (s *Server) ProfileEscrow(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	actor := store.Actor{ID: sess.UserID, Login: sess.Login, IP: ClientIP(r)}

	var err error
	notice := ""
	switch r.PostFormValue(fieldAction) {
	case "opt_in":
		err = s.Store.EscrowOptIn(actor, sess.UserID, r.PostFormValue(fieldPassword))
		notice = "Your key is now deposited. An administrator with the master password can recover this account."
	case "opt_out":
		err = s.Store.EscrowOptOut(actor, sess.UserID)
		notice = "The deposited copy of your key has been deleted. This account can no longer be recovered."
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	v := s.View(r, "Carrel")
	if err != nil {
		v.Error = escrowMessage(err)
		v.Data = s.buildAppView(r)
		s.RenderStatus(w, http.StatusBadRequest, "app.html", v)
		return
	}
	// The user has just decided the question the notice would ask about.
	sess.ClearEscrowNotice()

	v.Notice = notice
	v.Data = s.buildAppView(r)
	s.Render(w, "app.html", v)
}

// escrowMessage turns a store failure into something worth reading. Anything
// unrecognised is passed through: the escrow calls do not produce errors that
// disclose more than the administrator or the account owner already knows.
func escrowMessage(err error) string {
	switch {
	case errors.Is(err, store.ErrAuth):
		return "That is not your current password."
	case errors.Is(err, crypto.ErrWrongMasterPassword):
		return "That is not the escrow master password."
	case errors.Is(err, crypto.ErrMasterPasswordTooShort):
		return fmt.Sprintf("The master password must be at least %d characters.", crypto.MinMasterPasswordLength)
	case errors.Is(err, store.ErrEscrowNotConfigured):
		return "Key escrow is not active on this instance."
	case errors.Is(err, store.ErrEscrowConfigured):
		return "Key escrow already has a key pair. Change the master password instead of setting it up again."
	case errors.Is(err, store.ErrEscrowDeposited):
		return "This account is already covered by key escrow."
	case errors.Is(err, store.ErrEscrowNotDeposited):
		return "This account has no deposited key, so there is nothing to recover."
	case errors.Is(err, store.ErrEscrowOptOutForbidden):
		return "The administrator of this instance does not allow withdrawing a deposited key."
	case errors.Is(err, store.ErrNotFound):
		return "That account does not exist."
	default:
		return capitalize(err.Error()) + "."
	}
}

// throttleError carries a rate-limiter refusal out of an admin action, so the
// page comes back as 429 with a Retry-After rather than as a plain rejection.
type throttleError struct{ wait time.Duration }

func (e throttleError) Error() string {
	return "Too many attempts. Wait a moment and try again."
}

// adminEnableEscrow sets the scheme up for the first time. Everything about
// this is one-way: the key pair cannot be regenerated without orphaning the
// copies, and a forgotten master password makes the whole thing useless, which
// is why the form says so before the button (§5.4).
func (s *Server) adminEnableEscrow(r *http.Request, actor store.Actor) (adminView, error) {
	master := r.PostFormValue(fieldMasterPassword)
	if master != r.PostFormValue(fieldConfirmMaster) {
		return adminView{}, fmt.Errorf("the two master passwords do not match")
	}
	if err := s.Store.EnableEscrow(actor, master); err != nil {
		return adminView{}, errors.New(escrowMessage(err))
	}
	return adminView{}, nil
}

// adminResumeEscrow turns deposit back on with the key pair already on the
// volume, so the copies taken before it was switched off keep working.
func (s *Server) adminResumeEscrow(r *http.Request, actor store.Actor) (adminView, error) {
	if err := s.Store.ResumeEscrow(actor, r.PostFormValue(fieldMasterPassword)); err != nil {
		return adminView{}, errors.New(escrowMessage(err))
	}
	return adminView{}, nil
}

func (s *Server) adminDisableEscrow(_ *http.Request, actor store.Actor) (adminView, error) {
	if err := s.Store.DisableEscrow(actor); err != nil {
		return adminView{}, err
	}
	return adminView{}, nil
}

func (s *Server) adminEscrowPolicy(r *http.Request, actor store.Actor) (adminView, error) {
	forbid := r.PostFormValue(fieldForbidOptOut) != ""
	if err := s.Store.SetEscrowOptOutPolicy(actor, forbid); err != nil {
		return adminView{}, err
	}
	return adminView{}, nil
}

// adminChangeMaster re-seals the recovery private key. The public key is
// untouched, so nobody has to deposit their DEK again (§5.4).
func (s *Server) adminChangeMaster(r *http.Request, actor store.Actor) (adminView, error) {
	next := r.PostFormValue(fieldNewMaster)
	if next != r.PostFormValue(fieldConfirmMaster) {
		return adminView{}, fmt.Errorf("the two master passwords do not match")
	}
	if err := s.Store.ChangeEscrowMasterPassword(actor, r.PostFormValue(fieldMasterPassword), next); err != nil {
		return adminView{}, errors.New(escrowMessage(err))
	}
	return adminView{}, nil
}

// adminRecoverUser runs the recovery of §5.4 end to end: the master password
// opens the deposited copy of the user's DEK, the DEK is re-wrapped under a
// temporary password, the account's sessions are ended, and the user is told
// what happened whether the administrator likes it or not.
//
// Guessing the master password is throttled by client address: it is the one
// secret on the instance that unlocks somebody else's data.
func (s *Server) adminRecoverUser(r *http.Request, actor store.Actor) (adminView, error) {
	key := "escrow:" + actor.IP
	if ok, wait := s.RecoveryLimit.Allow(key); !ok {
		return adminView{}, throttleError{wait: wait}
	}

	user, err := s.Store.RecoverUser(actor,
		r.PostFormValue(fieldUserID),
		r.PostFormValue(fieldMasterPassword),
		r.PostFormValue(fieldTempPassword),
	)
	if err != nil {
		if errors.Is(err, crypto.ErrWrongMasterPassword) {
			s.RecoveryLimit.Fail(key)
		}
		return adminView{}, errors.New(escrowMessage(err))
	}
	s.RecoveryLimit.Reset(key)

	// The account's credentials changed underneath any live session, and the
	// person being recovered is by definition not the one holding it.
	s.Sessions.DestroyUser(user.ID)

	return adminView{Recovered: user.Login, MailWarning: s.notifyRecovery(user)}, nil
}

// notifyRecovery sends the mandatory notice and returns what the administrator
// must be told when it could not go out. §5.4 does not allow the notification
// to be skipped, so an instance that cannot send mail has to hand the message
// over in person instead.
func (s *Server) notifyRecovery(u *store.User) string {
	switch {
	case u.Email == "":
		return "There is no email address on file for " + u.Login +
			", so the notice could not be sent. Tell them about this recovery yourself."
	case s.Mail == nil || !s.Mail.QueueEscrowRecovery(u.Email, u.Login, u.EscrowRecoveredAt):
		return "Mail is not configured, so the notice to " + u.Email +
			" could not be sent. Tell " + u.Login + " about this recovery yourself."
	default:
		return ""
	}
}

// adminResetPassword is the destructive alternative of §5.5. It is refused for
// an account covered by escrow, where a recovery would keep the data, and the
// refusal says which form to use instead.
func (s *Server) adminResetPassword(r *http.Request, actor store.Actor) (adminView, error) {
	if r.PostFormValue(fieldConfirmReset) == "" {
		return adminView{}, fmt.Errorf("confirm that you understand the connections will be destroyed")
	}
	userID := r.PostFormValue(fieldUserID)
	if err := s.Store.ResetPassword(actor, userID, r.PostFormValue(fieldTempPassword)); err != nil {
		if errors.Is(err, store.ErrEscrowActive) {
			return adminView{}, fmt.Errorf("this account is covered by key escrow: " +
				"recover it with the master password instead, which keeps its data")
		}
		return adminView{}, errors.New(escrowMessage(err))
	}
	s.Sessions.DestroyUser(userID)
	return adminView{}, nil
}
