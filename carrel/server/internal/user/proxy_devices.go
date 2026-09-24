package user

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// davloom devices (S-27.2). A device has a password of its own, issued by
// the server and shown once, and from it a wrapper of the data key under
// its own AAD: it opens the person's credentials without their password.
// Changing the sign-in password leaves the wrappers alone — the data key
// does not change (S-2) — and revoking a device deletes its wrapper.

var (
	// ErrDeviceName is a name the person would not recognise later.
	ErrDeviceName = errors.New("Give the device a name of up to 60 characters: it is how you will know what you are revoking.")
	// ErrDeviceScope is an empty scope: a device that sees nothing is refused,
	// not taken for one that sees everything.
	ErrDeviceScope = errors.New("Choose at least one collection the device may see.")
)

// NewDeviceID returns d- + 10 Crockford chars from 6 random bytes.
func NewDeviceID() (string, error) {
	return newEntityID("d-")
}

// DeviceScope is what a device sees: kinds, and collections of the catalog.
type DeviceScope struct {
	Kinds []view.Kind
	Cols  []store.DeviceScope
	// ReadOnly is the device's right, chosen when it is issued: read only,
	// or read and write.
	ReadOnly bool
}

// cleanScope keeps the collections the catalog holds, live, of a kind the
// scope names — the intersection (S-27.2); nothing left is a refusal.
func (s *Service) cleanScope(userID string, dataKey [crypt.KeyLen]byte, sc DeviceScope) ([]string, []store.DeviceScope, error) {
	blob, ok := s.Catalog(userID, dataKey)
	if !ok {
		return nil, nil, ErrNotFound
	}
	kinds := map[view.Kind]bool{}
	var kindNames []string
	for _, k := range sc.Kinds {
		if !kinds[k] {
			kinds[k] = true
			kindNames = append(kindNames, string(k))
		}
	}
	var cols []store.DeviceScope
	seen := map[string]bool{}
	for _, c := range sc.Cols {
		col, ok := blob.Collection(c.AccountID, c.Path)
		conn, hasConn := blob.Connection(c.AccountID)
		if !ok || !hasConn || conn.Feed || col.Gone || seen[c.AccountID+"|"+c.Path] {
			continue
		}
		for _, k := range col.Kinds {
			if kinds[k] {
				cols = append(cols, c)
				seen[c.AccountID+"|"+c.Path] = true
				break
			}
		}
	}
	if len(cols) == 0 {
		return nil, nil, ErrDeviceScope
	}
	return kindNames, cols, nil
}

// IssueDevice makes a device and its password, returned this once. The
// caller has checked the unlock window (S-10.7).
func (s *Service) IssueDevice(ctx context.Context, userID string, dataKey [crypt.KeyLen]byte, name string, sc DeviceScope) (id, password string, err error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 60 || strings.ContainsAny(name, "\r\n") {
		return "", "", ErrDeviceName
	}
	kinds, cols, err := s.cleanScope(userID, dataKey, sc)
	if err != nil {
		return "", "", err
	}
	password, _, err = newSecret()
	if err != nil {
		return "", "", err
	}
	id, err = NewDeviceID()
	if err != nil {
		return "", "", err
	}
	salt, err := crypt.NewSalt()
	if err != nil {
		return "", "", err
	}
	wrapKey, kdf, err := crypt.DeriveWrapKey(ctx, password, salt)
	if err != nil {
		return "", "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		wrapped, err := crypt.WrapDeviceKey(key32(wrapKey), dataKey, u.Login, id)
		if err != nil {
			return nil, err
		}
		d := store.ProxyDevice{ID: id, Name: name, Salt: salt, WrappedKey: wrapped, Created: now, ScopeKinds: kinds, ScopeCols: cols, ReadOnly: sc.ReadOnly}
		d.SetDeviceKDF(kdf)
		u.ProxyDevices = append(u.ProxyDevices, d)
		return []store.AuditEntry{{When: now, Who: u.Login, What: "device.issue", Whom: id, Detail: name}}, nil
	})
	crypt.Wipe(wrapKey)
	if err != nil {
		return "", "", err
	}
	return id, password, nil
}

// RevokeDevice deletes the device and its wrapper: from this request on it
// opens nothing.
func (s *Service) RevokeDevice(userID, id string) error {
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, ErrNotFound
		}
		kept := u.ProxyDevices[:0]
		found := false
		for _, d := range u.ProxyDevices {
			if d.ID == id {
				found = true
				continue
			}
			kept = append(kept, d)
		}
		if !found {
			return nil, ErrNotFound
		}
		u.ProxyDevices = kept
		return []store.AuditEntry{{When: s.now(), Who: u.Login, What: "device.revoke", Whom: id}}, nil
	})
	if err == nil {
		s.devicesChanged(userID)
	}
	return err
}

// SetDeviceScope changes what a device sees, without a new password: the
// scope belongs to the device record, not to its password.
func (s *Service) SetDeviceScope(userID string, dataKey [crypt.KeyLen]byte, id string, sc DeviceScope) error {
	kinds, cols, err := s.cleanScope(userID, dataKey, sc)
	if err != nil {
		return err
	}
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, ErrNotFound
		}
		for i := range u.ProxyDevices {
			if u.ProxyDevices[i].ID == id {
				u.ProxyDevices[i].ScopeKinds, u.ProxyDevices[i].ScopeCols = kinds, cols
				return []store.AuditEntry{{When: s.now(), Who: u.Login, What: "device.scope", Whom: id}}, nil
			}
		}
		return nil, ErrNotFound
	})
	if err == nil {
		s.devicesChanged(userID)
	}
	return err
}

// SetDeviceAccess makes a device read only or read and write, without a new
// password: like its scope, the right belongs to the device record.
func (s *Service) SetDeviceAccess(userID, id string, readOnly bool) error {
	detail := "read and write"
	if readOnly {
		detail = "read only"
	}
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, ErrNotFound
		}
		for i := range u.ProxyDevices {
			if u.ProxyDevices[i].ID == id {
				u.ProxyDevices[i].ReadOnly = readOnly
				return []store.AuditEntry{{When: s.now(), Who: u.Login, What: "device.access", Whom: id, Detail: detail}}, nil
			}
		}
		return nil, ErrNotFound
	})
	if err == nil {
		s.devicesChanged(userID)
	}
	return err
}

// Devices are the person's devices, oldest first.
func (s *Service) Devices(userID string) []store.ProxyDevice {
	snap := s.st.Snapshot()
	if u := snap.UserByID(userID); u != nil {
		return u.ProxyDevices
	}
	return nil
}

// DeviceOf is one live device of a live account: what the proxy checks on
// every request, so a revoke or a disable holds from the next one.
func (s *Service) DeviceOf(userID, id string) (store.ProxyDevice, bool) {
	snap := s.st.Snapshot()
	u := snap.UserByID(userID)
	if u == nil || u.Disabled {
		return store.ProxyDevice{}, false
	}
	for _, d := range u.ProxyDevices {
		if d.ID == id {
			return d, true
		}
	}
	return store.ProxyDevice{}, false
}

// UnlockDevice opens the data key with a device password: it tries the
// login's devices in turn. An unknown login, a disabled account and an
// account without devices cost one derivation, like a wrong password —
// sign-in answers alike (S-29.1).
func (s *Service) UnlockDevice(ctx context.Context, login, password string) (userID, deviceID string, dataKey [crypt.KeyLen]byte, ok bool) {
	snap := s.st.Snapshot()
	u := snap.UserByLogin(normalizeLogin(login))
	if u == nil || u.Disabled || len(u.ProxyDevices) == 0 {
		s.verifyDummy(ctx, password)
		return "", "", dataKey, false
	}
	for _, d := range u.ProxyDevices {
		wrapKey, err := crypt.DeriveKey(ctx, password, d.Salt, d.DeviceKDF())
		if err != nil {
			continue
		}
		key, err := crypt.UnwrapDeviceKey(key32(wrapKey), d.WrappedKey, u.Login, d.ID)
		crypt.Wipe(wrapKey)
		if err == nil {
			return u.ID, d.ID, key, true
		}
	}
	return "", "", dataKey, false
}

// Device activity: the last request of each device, in the process only —
// a PROPFIND is no reason to write state.enc (plan, S-27). Empty after a
// restart.
type deviceSeen struct {
	mu     sync.Mutex
	at     map[string]time.Time
	onGone []func(userID string)
}

// TouchDevice notes a request of a device.
func (s *Service) TouchDevice(id string, at time.Time) {
	s.seen.mu.Lock()
	defer s.seen.mu.Unlock()
	if s.seen.at == nil {
		s.seen.at = map[string]time.Time{}
	}
	s.seen.at[id] = at
}

// DeviceLastSeen is a device's last request since the start, or zero.
func (s *Service) DeviceLastSeen(id string) time.Time {
	s.seen.mu.Lock()
	defer s.seen.mu.Unlock()
	return s.seen.at[id]
}

// OnDevicesChanged registers what to do when an account's devices change:
// the proxy wipes its cache of opened keys for that account.
func (s *Service) OnDevicesChanged(fn func(userID string)) {
	s.seen.mu.Lock()
	defer s.seen.mu.Unlock()
	s.seen.onGone = append(s.seen.onGone, fn)
}

func (s *Service) devicesChanged(userID string) {
	s.seen.mu.Lock()
	fns := append([]func(string){}, s.seen.onGone...)
	s.seen.mu.Unlock()
	for _, fn := range fns {
		fn(userID)
	}
}

func key32(b []byte) [crypt.KeyLen]byte {
	var k [crypt.KeyLen]byte
	copy(k[:], b)
	return k
}
