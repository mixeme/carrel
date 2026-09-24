package web_test

import (
	"os"
	"testing"

	"carrel.dev/carrel/server/internal/crypt"
)

// TestMain runs this package's tests with the cheapest key derivation the
// store accepts. Every fixture makes an account and signs in, which is four
// Argon2id runs at the production cost of 64 MiB — and the escrow master key
// is 256 MiB — all behind one process-wide semaphore (F-7). That was most of
// the package's time: 337 tests at about a second and a third each, and on
// 24.09.2026 a full run crossed go test's ten-minute ceiling. Nothing here
// tests the cost itself; the parameters travel with every stored record, so a
// cheaper one is verified the same way. The production values stay what
// internal/crypt's own tests check.
func TestMain(m *testing.M) {
	cheap := crypt.KDFParams{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: crypt.KeyLen}
	crypt.LoginKDF, crypt.WrapKDF, crypt.MasterKDF = cheap, cheap, cheap
	os.Exit(m.Run())
}
