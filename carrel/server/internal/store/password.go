package store

import (
	"context"

	"carrel.dev/carrel/server/internal/crypt"
)

// ApplyPassword sets login hash, wrap key, empty sealed blob, and KDF records
// for a new or reset account (S-10).
func ApplyPassword(ctx context.Context, u *User, login, password string) error {
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
	return nil
}

// WipeCredentials removes decryptable material while keeping identity (S-10.4).
func WipeCredentials(u *User) {
	u.LoginSalt = nil
	u.LoginHash = nil
	u.LoginKDF = kdfRecord{}
	u.WrapSalt = nil
	u.WrappedKey = nil
	u.WrapKDF = kdfRecord{}
	u.SealedBlob = nil
	u.ConnCount = 0
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

func key32(b []byte) [crypt.KeyLen]byte {
	var k [crypt.KeyLen]byte
	copy(k[:], b)
	return k
}
