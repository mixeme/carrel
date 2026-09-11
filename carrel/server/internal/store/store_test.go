package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
)

func testKey(t *testing.T) [crypt.KeyLen]byte {
	key, err := crypt.CreateServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestFirstRunEmptyVolume(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	st, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	if !st.FirstRun() {
		t.Fatal("expected first-run mode")
	}
	if len(st.Snapshot().Users) != 0 {
		t.Fatal("expected empty users")
	}
}

func TestMutatePersistsAndReloads(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	st, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		s.Users = []User{{
			ID:         "a-test00001",
			Login:      "alice",
			Role:       RoleAdmin,
			CreatedAt:  when,
			ConnCount:  2,
			SealedBlob: []byte{1, 2, 3},
		}}
		return []AuditEntry{{
			When: when,
			Who:  "system",
			What: "setup.first_admin",
			Whom: "alice",
		}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.FirstRun() {
		t.Fatal("first-run should be false after write")
	}

	st2, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	snap := st2.Snapshot()
	if len(snap.Users) != 1 || snap.Users[0].Login != "alice" {
		t.Fatalf("users = %+v", snap.Users)
	}
	if len(snap.Audit) != 1 || snap.Audit[0].What != "setup.first_admin" {
		t.Fatalf("audit = %+v", snap.Audit)
	}
}

func TestFailedWriteLeavesStateAndAuditUnchanged(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	path := filepath.Join(dir, StateFile)
	st, err := openAt(path, key, atomicWrite)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().UTC().Truncate(time.Second)
	if err := st.Mutate(func(s *State) ([]AuditEntry, error) {
		s.Users = []User{{ID: "a-1", Login: "bob", Role: RoleUser, CreatedAt: when}}
		return []AuditEntry{{When: when, Who: "system", What: "user.create", Whom: "bob"}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	before := st.Snapshot()

	failWrite := errors.New("disk full")
	st.writeFn = func(string, []byte) error { return failWrite }
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		s.Users[0].ConnCount = 9
		return []AuditEntry{{When: when, Who: "admin", What: "conn.set", Whom: "bob", Detail: "count=9"}}, nil
	})
	if !errors.Is(err, failWrite) {
		t.Fatalf("expected write error, got %v", err)
	}
	after := st.Snapshot()
	if after.Users[0].ConnCount != before.Users[0].ConnCount {
		t.Fatalf("conn count changed in memory: %d -> %d", before.Users[0].ConnCount, after.Users[0].ConnCount)
	}
	if len(after.Audit) != len(before.Audit) {
		t.Fatalf("audit grew: %d -> %d", len(before.Audit), len(after.Audit))
	}

	st3, err := openAt(path, key, atomicWrite)
	if err != nil {
		t.Fatal(err)
	}
	reloaded := st3.Snapshot()
	if reloaded.Users[0].ConnCount != before.Users[0].ConnCount {
		t.Fatalf("disk state changed: conn=%d", reloaded.Users[0].ConnCount)
	}
	if len(reloaded.Audit) != 1 {
		t.Fatalf("disk audit = %d entries", len(reloaded.Audit))
	}
}

func TestRejectNewerVersion(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	path := filepath.Join(dir, StateFile)

	payload, err := encodeFile(FormatVersion+1, key, emptyState())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Open(dir, key)
	if !errors.Is(err, ErrNewerVersion) {
		t.Fatalf("expected ErrNewerVersion, got %v", err)
	}
}

func TestConnectionCountWithoutOpeningBlob(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	st, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	dataKey, err := crypt.NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	blob, err := crypt.EncryptBlob(dataKey, []byte(`{"connections":[{"id":"c1"}]}`), "carol")
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().UTC()
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		s.Users = []User{{
			ID:         "a-carol0001",
			Login:      "carol",
			Role:       RoleUser,
			CreatedAt:  when,
			SealedBlob: blob,
			ConnCount:  3,
		}}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.ConnectionCount("a-carol0001") != 3 {
		t.Fatalf("count = %d", st.ConnectionCount("a-carol0001"))
	}
	// Prove we never needed the data key: reload and read count only.
	st2, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	if st2.ConnectionCount("a-carol0001") != 3 {
		t.Fatal("reloaded count wrong")
	}
	u := st2.Snapshot().Users[0]
	if len(u.SealedBlob) == 0 {
		t.Fatal("blob missing")
	}
}

func TestAuditRejectsSecrets(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	st, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().UTC()
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		return []AuditEntry{{When: when, Who: "admin", What: "login.fail", Detail: "bad password"}}, nil
	})
	if !errors.Is(err, ErrAuditSecret) {
		t.Fatalf("expected ErrAuditSecret, got %v", err)
	}
}

func TestAtomicWriteUsesTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.enc")
	data := []byte("payload")
	if err := atomicWrite(path, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Fatalf("file = %q", got)
	}
	if _, err := os.Stat(path + tmpSuffix); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("temp file should be gone")
	}
}

// The secret scan looks at the free-form field. A person may legitimately be
// called "secretary" or "tokarev", and a name must never make an account
// unauditable — a rejected audit entry fails the whole Mutate, which would
// leave that account unmanageable.
func TestAuditAllowsIdentitiesThatLookLikeKeywords(t *testing.T) {
	dir := t.TempDir()
	key := testKey(t)
	st, err := Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().UTC()
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		return []AuditEntry{
			{When: when, Who: "admin", What: "password.change", Whom: "secretary@corp.example"},
			{When: when, Who: "tokarev", What: "user.create", Whom: "passwordless@corp.example"},
		}, nil
	})
	if err != nil {
		t.Fatalf("legitimate audit entries rejected: %v", err)
	}
	if got := len(st.Snapshot().Audit); got != 2 {
		t.Fatalf("audit = %d entries, want 2", got)
	}
}

// What is a programmer-chosen event code, not prose: nothing a person typed
// arrives through it.
func TestAuditWhatMustBeAnEventCode(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir, testKey(t))
	if err != nil {
		t.Fatal(err)
	}
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		return []AuditEntry{{When: time.Now().UTC(), Who: "admin", What: "changed the password to hunter2"}}, nil
	})
	if !errors.Is(err, ErrAuditWhat) {
		t.Fatalf("expected ErrAuditWhat, got %v", err)
	}
}

func TestMutateRejectsInvalidKDF(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir, testKey(t))
	if err != nil {
		t.Fatal(err)
	}
	when := time.Now().UTC()
	err = st.Mutate(func(s *State) ([]AuditEntry, error) {
		s.Users = []User{{
			ID:        "a-1",
			Login:     "alice",
			Role:      RoleAdmin,
			CreatedAt: when,
			LoginHash: []byte{1, 2, 3},
			LoginKDF:  kdfRecord{},
		}}
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected KDF validation error")
	}
}
