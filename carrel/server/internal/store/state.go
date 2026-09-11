package store

import (
	"encoding/json"
	"fmt"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
)

// Role is an account role (S-10.1).
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// State is the decrypted on-disk instance state.
type State struct {
	Users []User       `json:"users"`
	Audit []AuditEntry `json:"audit"`
}

// User is one account on the instance. ConnectionCount is stored outside the
// sealed blob so administration can read it without the data key (S-3).
type User struct {
	ID        string    `json:"id"`
	Login     string    `json:"login"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	Disabled  bool      `json:"disabled,omitempty"`

	LoginSalt  []byte     `json:"loginSalt,omitempty"`
	LoginHash  []byte     `json:"loginHash,omitempty"`
	LoginKDF   kdfRecord  `json:"loginKdf,omitempty"`
	WrapSalt   []byte     `json:"wrapSalt,omitempty"`
	WrappedKey []byte     `json:"wrappedKey,omitempty"`
	WrapKDF    kdfRecord  `json:"wrapKdf,omitempty"`
	SealedBlob []byte     `json:"sealedBlob,omitempty"`
	ConnCount  int        `json:"connCount"`
	LastLogin  *time.Time `json:"lastLogin,omitempty"`
}

type kdfRecord struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

func kdfFromCrypt(p crypt.KDFParams) kdfRecord {
	return kdfRecord{Time: p.Time, Memory: p.Memory, Threads: p.Threads, KeyLen: p.KeyLen}
}

func (k kdfRecord) toCrypt() crypt.KDFParams {
	return crypt.KDFParams{Time: k.Time, Memory: k.Memory, Threads: k.Threads, KeyLen: k.KeyLen}
}

func validateUserKDF(u *User) error {
	hasLogin := len(u.LoginHash) > 0
	if err := crypt.ValidateKDFParams(u.LoginKDF.toCrypt(), hasLogin); err != nil {
		return fmt.Errorf("login KDF: %w", err)
	}
	hasWrap := len(u.WrappedKey) > 0
	if err := crypt.ValidateKDFParams(u.WrapKDF.toCrypt(), hasWrap); err != nil {
		return fmt.Errorf("wrap KDF: %w", err)
	}
	return nil
}

func validateStateKDF(st *State) error {
	for i := range st.Users {
		if err := validateUserKDF(&st.Users[i]); err != nil {
			return err
		}
	}
	return nil
}

// AuditEntry is one audit-log line. Passwords, tokens, and record bodies never
// belong here (S-3, S-29).
type AuditEntry struct {
	When   time.Time `json:"when"`
	Who    string    `json:"who"`
	What   string    `json:"what"`
	Whom   string    `json:"whom,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

func emptyState() State {
	return State{Users: nil, Audit: nil}
}

func (s State) clone() State {
	b, err := json.Marshal(s)
	if err != nil {
		panic("store: clone marshal: " + err.Error())
	}
	var out State
	if err := json.Unmarshal(b, &out); err != nil {
		panic("store: clone unmarshal: " + err.Error())
	}
	return out
}

func (s *State) userByID(id string) *User {
	for i := range s.Users {
		if s.Users[i].ID == id {
			return &s.Users[i]
		}
	}
	return nil
}
