package store

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
)

// EscrowActive reports whether instance escrow is configured.
func EscrowActive(st *State) bool {
	return st.Settings.EscrowEnabled && len(st.Settings.EscrowPublicKey) == crypt.KeyLen
}

// UserDeposited reports whether the user has a deposited data key copy.
func UserDeposited(u *User) bool {
	return len(u.EscrowDeposited) > 0
}

// EscrowPublicKey returns the instance escrow public key when active.
func EscrowPublicKey(st *State) ([crypt.KeyLen]byte, bool) {
	var pub [crypt.KeyLen]byte
	if !EscrowActive(st) {
		return pub, false
	}
	copy(pub[:], st.Settings.EscrowPublicKey)
	return pub, true
}

// TryDepositEscrow stores an encrypted copy of the data key when policy applies.
func TryDepositEscrow(st *State, u *User, dataKey [crypt.KeyLen]byte, now time.Time) error {
	if !u.EscrowOptIn || UserDeposited(u) {
		return nil
	}
	pub, ok := EscrowPublicKey(st)
	if !ok {
		return nil
	}
	deposited, err := crypt.DepositDataKey(pub, dataKey, u.Login)
	if err != nil {
		return err
	}
	u.EscrowDeposited = deposited
	u.EscrowDepositedAt = &now
	if u.PasswordLog == "" {
		var raw [32]byte
		if _, err := rand.Read(raw[:]); err != nil {
			return err
		}
		u.PasswordLog = base64.RawURLEncoding.EncodeToString(raw[:])
	}
	return nil
}

// PasswordEvent is one thing done to an account's password: what, when, by
// whom (Ад40).
type PasswordEvent struct {
	When time.Time `json:"when"`
	What string    `json:"what"` // the journal's code: escrow.recover, user.reset, password.change, user.wipe, user.password.set
	Who  string    `json:"who"`
}

// maxPasswordEvents bounds an account's password history; the oldest go
// first.
const maxPasswordEvents = 200

// NotePassword adds what was done to u's password to its history.
func NotePassword(u *User, when time.Time, what, who string) {
	u.PasswordEvents = append(u.PasswordEvents, PasswordEvent{When: when, What: what, Who: who})
	if n := len(u.PasswordEvents); n > maxPasswordEvents {
		u.PasswordEvents = append([]PasswordEvent(nil), u.PasswordEvents[n-maxPasswordEvents:]...)
	}
}

// ClearEscrowDeposit removes a deposited key copy.
func ClearEscrowDeposit(u *User) {
	u.EscrowDeposited = nil
	u.EscrowDepositedAt = nil
}

// SetMasterKDF stores master-password KDF parameters on instance settings.
func SetMasterKDF(s *InstanceSettings, p crypt.KDFParams) {
	s.EscrowMasterKDF = kdfFromCrypt(p)
}

// MasterKDFParams reads master-password KDF parameters.
func MasterKDFParams(s InstanceSettings) crypt.KDFParams {
	return s.EscrowMasterKDF.toCrypt()
}
