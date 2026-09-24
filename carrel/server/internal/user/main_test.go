package user_test

import (
	"os"
	"testing"

	"carrel.dev/carrel/server/internal/crypt"
)

// TestMain runs this package's tests with the cheapest key derivation the
// store accepts, for the reason given in internal/web's: every account made
// and every sign-in is Argon2id at the production cost behind one semaphore
// (F-7), and nothing here tests the cost itself. The production values stay
// what internal/crypt's own tests check.
func TestMain(m *testing.M) {
	cheap := crypt.KDFParams{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: crypt.KeyLen}
	crypt.LoginKDF, crypt.WrapKDF, crypt.MasterKDF = cheap, cheap, cheap
	os.Exit(m.Run())
}
