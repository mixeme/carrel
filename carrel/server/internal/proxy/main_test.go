package proxy_test

import (
	"os"
	"testing"

	"carrel.dev/carrel/server/internal/crypt"
)

// TestMain runs this package's tests with the cheapest key derivation the
// store accepts, for the reason given in internal/web's: nothing here tests
// the cost of Argon2id, and at the production cost every account a test
// makes is most of the test's time. internal/crypt's own tests keep the
// production values.
func TestMain(m *testing.M) {
	cheap := crypt.KDFParams{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: crypt.KeyLen}
	crypt.LoginKDF, crypt.WrapKDF, crypt.MasterKDF = cheap, cheap, cheap
	os.Exit(m.Run())
}
