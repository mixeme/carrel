package store

import (
	"carrel.dev/carrel/server/internal/crypt"
)

// ApplyPassword sets login hash, wrap key, empty sealed blob, and KDF records
// for a new or reset account (S-10).
func ApplyPassword(u *User, login, password string) error {
	loginSalt, err := crypt.NewSalt()
	if err != nil {
		return err
	}
	wrapSalt, err := crypt.NewSalt()
	if err != nil {
		return err
	}
	loginHash, loginKDF := crypt.HashLogin(password, loginSalt)
	wrapKey, wrapKDF := crypt.DeriveWrapKey(password, wrapSalt)
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
func DataKeyForLogin(u *User, password string) ([crypt.KeyLen]byte, bool) {
	var zero [crypt.KeyLen]byte
	if len(u.LoginHash) == 0 || len(u.WrappedKey) == 0 {
		return zero, false
	}
	if !crypt.VerifyLogin(password, u.LoginSalt, u.LoginHash, u.LoginKDF.toCrypt()) {
		return zero, false
	}
	wrapKey, _ := crypt.DeriveWrapKey(password, u.WrapSalt)
	key, err := crypt.UnwrapDataKey(key32(wrapKey), u.WrappedKey, u.Login)
	crypt.Wipe(wrapKey)
	if err != nil {
		return zero, false
	}
	return key, true
}

func key32(b []byte) [crypt.KeyLen]byte {
	var k [crypt.KeyLen]byte
	copy(k[:], b)
	return k
}
