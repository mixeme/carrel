package store

import (
	"context"
	"errors"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
)

// ApplyPassword sets login hash, wrap key, empty sealed blob, and KDF records
// for a new or reset account (S-10). When escrow is active and the user is
// covered, a copy of the data key is deposited in the same mutation.
func ApplyPassword(ctx context.Context, u *User, login, password string, st *State, now time.Time) error {
	loginSalt, err := crypt.NewSalt()
	if err != nil {
		return err
	}
	wrapSalt, err := crypt.NewSalt()
	if err != nil {
		return err
	}
	loginHash, loginKDF, err := crypt.HashLogin(ctx, password, loginSalt)
	if err != nil {
		return err
	}
	wrapKey, wrapKDF, err := crypt.DeriveWrapKey(ctx, password, wrapSalt)
	if err != nil {
		return err
	}
	dataKey, err := crypt.NewDataKey()
	if err != nil {
		crypt.Wipe(wrapKey)
		return err
	}
	wrapped, err := crypt.WrapDataKey(key32(wrapKey), dataKey, login)
	crypt.Wipe(wrapKey)
	if err != nil {
		return err
	}
	blob, err := crypt.EncryptBlob(dataKey, []byte("{}"), login)
	if err != nil {
		return err
	}
	u.LoginSalt = loginSalt
	u.LoginHash = loginHash
	u.LoginKDF = kdfFromCrypt(loginKDF)
	u.WrapSalt = wrapSalt
	u.WrappedKey = wrapped
	u.WrapKDF = kdfFromCrypt(wrapKDF)
	u.SealedBlob = blob
	u.MustChangePassword = false
	if st != nil {
		if err := TryDepositEscrow(st, u, dataKey, now); err != nil {
			return err
		}
	}
	return nil
}

// ClearEscrowDeposit removes decryptable material while keeping identity (S-10.4).
func WipeCredentials(u *User) {
	u.LoginSalt = nil
	u.LoginHash = nil
	u.LoginKDF = kdfRecord{}
	u.WrapSalt = nil
	u.WrappedKey = nil
	u.WrapKDF = kdfRecord{}
	u.SealedBlob = nil
	u.ConnCount = 0
	u.ProxyDevices = nil // a device opens nothing once the data key is gone
	ClearEscrowDeposit(u)
}

// WipeAccount is the wipe of S-10.4 whole: the credentials, and with them
// everything of the state that read the connections without the person's
// password — the published links, whose envelopes hold the connections'
// own credentials under server.key, and the external backup triggers,
// whose envelopes hold the old data key. Without this a link made by
// whoever held the account went on reading after the owner wiped it
// (audit №3, Ф3-6).
func WipeAccount(st *State, u *User) {
	WipeCredentials(u)
	kept := st.Published[:0]
	for _, l := range st.Published {
		if l.UserID == u.ID {
			crypt.Wipe(l.Envelope)
			continue
		}
		kept = append(kept, l)
	}
	st.Published = kept
	backups := st.Backups[:0]
	for _, b := range st.Backups {
		if b.UserID == u.ID {
			crypt.Wipe(b.KeyEnvelope)
			continue
		}
		backups = append(backups, b)
	}
	st.Backups = backups
}

// VerifyPassword checks a password against the login hash without touching
// the wrap key: the S-10.7 confirmation sheet needs a yes or no, not key
// material. Accounts without a password answer false.
func VerifyPassword(ctx context.Context, u *User, password string) bool {
	if len(u.LoginHash) == 0 {
		return false
	}
	ok, err := crypt.VerifyLogin(ctx, password, u.LoginSalt, u.LoginHash, u.LoginKDF.toCrypt())
	return err == nil && ok
}

// DataKeyForLogin verifies password and returns the data key.
func DataKeyForLogin(ctx context.Context, u *User, password string) ([crypt.KeyLen]byte, bool) {
	var zero [crypt.KeyLen]byte
	if len(u.LoginHash) == 0 || len(u.WrappedKey) == 0 {
		return zero, false
	}
	ok, err := crypt.VerifyLogin(ctx, password, u.LoginSalt, u.LoginHash, u.LoginKDF.toCrypt())
	if err != nil || !ok {
		return zero, false
	}
	wrapKey, _, err := crypt.DeriveWrapKey(ctx, password, u.WrapSalt)
	if err != nil {
		return zero, false
	}
	key, err := crypt.UnwrapDataKey(key32(wrapKey), u.WrappedKey, u.Login)
	crypt.Wipe(wrapKey)
	if err != nil {
		return zero, false
	}
	return key, true
}

// RewrapLogin re-seals the wrapped data key and user blob under a new login
// AAD in one step (F-14). Call inside Mutate after verifying the actor.
func RewrapLogin(u *User, newLogin string, wrapKey, dataKey [crypt.KeyLen]byte) error {
	if u.Login == newLogin {
		return nil
	}
	plain, err := crypt.DecryptBlob(dataKey, u.SealedBlob, u.Login)
	if err != nil {
		return err
	}
	wrapped, err := crypt.WrapDataKey(wrapKey, dataKey, newLogin)
	if err != nil {
		return err
	}
	blob, err := crypt.EncryptBlob(dataKey, plain, newLogin)
	if err != nil {
		return err
	}
	u.WrappedKey = wrapped
	u.SealedBlob = blob
	u.Login = newLogin
	return nil
}

// ChangePassword re-wraps the data key under a new password without touching
// the sealed blob (S-2). ApplyPassword must not be used here: it issues a new
// data key and makes existing ciphertext unreadable.
func ChangePassword(ctx context.Context, u *User, login, currentPassword, newPassword string) error {
	if _, ok := DataKeyForLogin(ctx, u, currentPassword); !ok {
		return errInvalidPassword
	}
	oldWrapBytes, _, err := crypt.DeriveWrapKey(ctx, currentPassword, u.WrapSalt)
	if err != nil {
		return err
	}
	var oldWrap [crypt.KeyLen]byte
	copy(oldWrap[:], oldWrapBytes)

	loginSalt, err := crypt.NewSalt()
	if err != nil {
		crypt.Wipe(oldWrapBytes)
		return err
	}
	wrapSalt, err := crypt.NewSalt()
	if err != nil {
		crypt.Wipe(oldWrapBytes)
		return err
	}
	loginHash, loginKDF, err := crypt.HashLogin(ctx, newPassword, loginSalt)
	if err != nil {
		crypt.Wipe(oldWrapBytes)
		return err
	}
	newWrapBytes, wrapKDF, err := crypt.DeriveWrapKey(ctx, newPassword, wrapSalt)
	if err != nil {
		crypt.Wipe(oldWrapBytes)
		return err
	}
	var newWrap [crypt.KeyLen]byte
	copy(newWrap[:], newWrapBytes)

	rewrapped, err := crypt.RewrapDataKey(oldWrap, newWrap, u.WrappedKey, login)
	crypt.Wipe(oldWrapBytes)
	crypt.Wipe(newWrapBytes)
	if err != nil {
		return err
	}

	u.LoginSalt = loginSalt
	u.LoginHash = loginHash
	u.LoginKDF = kdfFromCrypt(loginKDF)
	u.WrapSalt = wrapSalt
	u.WrappedKey = rewrapped
	u.WrapKDF = kdfFromCrypt(wrapKDF)
	u.MustChangePassword = false
	return nil
}

var errInvalidPassword = errors.New("invalid password")

// RewrapPasswordWithKey sets new login credentials while keeping the data key
// and sealed blob intact (escrow recovery, S-10.5).
func RewrapPasswordWithKey(ctx context.Context, u *User, login, password string, dataKey [crypt.KeyLen]byte) error {
	loginSalt, err := crypt.NewSalt()
	if err != nil {
		return err
	}
	wrapSalt, err := crypt.NewSalt()
	if err != nil {
		return err
	}
	loginHash, loginKDF, err := crypt.HashLogin(ctx, password, loginSalt)
	if err != nil {
		return err
	}
	wrapKey, wrapKDF, err := crypt.DeriveWrapKey(ctx, password, wrapSalt)
	if err != nil {
		return err
	}
	wrapped, err := crypt.WrapDataKey(key32(wrapKey), dataKey, login)
	crypt.Wipe(wrapKey)
	if err != nil {
		return err
	}
	u.LoginSalt = loginSalt
	u.LoginHash = loginHash
	u.LoginKDF = kdfFromCrypt(loginKDF)
	u.WrapSalt = wrapSalt
	u.WrappedKey = wrapped
	u.WrapKDF = kdfFromCrypt(wrapKDF)
	u.MustChangePassword = true
	return nil
}

func key32(b []byte) [crypt.KeyLen]byte {
	var k [crypt.KeyLen]byte
	copy(k[:], b)
	return k
}
