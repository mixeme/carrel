package store

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"strings"
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
	Users    []User           `json:"users"`
	Audit    []AuditEntry     `json:"audit"`
	Settings InstanceSettings `json:"settings"`
	Invites  []Invite         `json:"invites"`
	Tokens   []SecretToken    `json:"tokens"`
	// Published are the links out (S-25). A link reads without a session, so
	// its target and an envelope with its connection's credentials live here
	// — never the secret, only its hash.
	Published []PublishedLink `json:"published,omitempty"`
}

// PublishedLink is one secret link to one thing, read-only (S-25).
type PublishedLink struct {
	Hash      []byte `json:"hash"` // SHA-256 of the 32-byte secret
	UserID    string `json:"userID"`
	AccountID string `json:"accountID"`
	Kind      string `json:"kind"` // view.Kind of the thing
	Col       string `json:"col"`  // the collection
	UID       string `json:"uid,omitempty"`
	// Path is a file or a folder under a file collection; Folder marks one.
	Path   string `json:"path,omitempty"`
	Folder bool   `json:"folder,omitempty"`
	// Title is the label the link was made under: the collection's name or
	// the record's title as it stood. The page shows the live one.
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"` // nil: no expiry, chosen explicitly
	LastSeen  *time.Time `json:"lastSeen,omitempty"`
	// Envelope holds the connection's address and credentials, sealed again
	// under the server key and bound to Hash (crypt.SealLink).
	Envelope []byte `json:"envelope"`
}

// InstanceSettings holds instance-wide configuration stored in state.enc.
type InstanceSettings struct {
	SelfRegEnabled bool `json:"selfRegEnabled,omitempty"`
	EscrowEnabled  bool `json:"escrowEnabled,omitempty"`

	EscrowPublicKey     []byte    `json:"escrowPublicKey,omitempty"`
	EscrowPrivateWrap   []byte    `json:"escrowPrivateWrap,omitempty"`
	EscrowMasterSalt    []byte    `json:"escrowMasterSalt,omitempty"`
	EscrowMasterKDF     kdfRecord `json:"escrowMasterKdf,omitempty"`
	EscrowCoverNewUsers bool      `json:"escrowCoverNewUsers,omitempty"`
}

// Invite is a pending account invitation (S-10.2).
type Invite struct {
	ID         string    `json:"id"`
	Login      string    `json:"login"`
	Email      string    `json:"email,omitempty"`
	SecretHash []byte    `json:"secretHash"`
	Secret     string    `json:"secret,omitempty"` // raw token; lives only inside encrypted state.enc for the admin panel link (S-10.3)
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Revoked    bool      `json:"revoked,omitempty"` // withdrawn by an administrator or replaced by a resend
	Used       bool      `json:"used,omitempty"`    // accepted: the invitee set a password
}

// SecretToken covers email confirmation, forgot/console wipe links (S-10).
type SecretToken struct {
	Kind      string    `json:"kind"`
	Hash      []byte    `json:"hash"`
	UserID    string    `json:"userID"`
	Step      string    `json:"step,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Used      bool      `json:"used,omitempty"`
}

// User is one account on the instance. ConnectionCount is stored outside the
// sealed blob so administration can read it without the data key (S-3).
type User struct {
	ID        string    `json:"id"`
	Login     string    `json:"login"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	Disabled  bool      `json:"disabled,omitempty"`

	Email              string `json:"email,omitempty"`
	EmailConfirmed     bool   `json:"emailConfirmed,omitempty"`
	MustChangePassword bool   `json:"mustChangePassword,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	// WeekStart and Clock12 are the account's C-10 display settings: the first
	// day of the week ("sunday", "saturday"; empty is Monday) and 12 hours.
	WeekStart           string     `json:"weekStart,omitempty"`
	Clock12             bool       `json:"clock12,omitempty"`
	EscrowOptIn         bool       `json:"escrowOptIn,omitempty"`
	EscrowDeposited     []byte     `json:"escrowDeposited,omitempty"`
	EscrowDepositedAt   *time.Time `json:"escrowDepositedAt,omitempty"`
	EscrowLastRecovery  *time.Time `json:"escrowLastRecovery,omitempty"`
	EscrowNoticePending bool       `json:"escrowNoticePending,omitempty"`

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

func (s *State) userByLogin(login string) *User {
	login = strings.ToLower(strings.TrimSpace(login))
	for i := range s.Users {
		if strings.ToLower(s.Users[i].Login) == login {
			return &s.Users[i]
		}
	}
	return nil
}

func (s *State) inviteByHash(hash []byte) *Invite {
	for i := range s.Invites {
		if len(s.Invites[i].SecretHash) == len(hash) &&
			subtle.ConstantTimeCompare(s.Invites[i].SecretHash, hash) == 1 {
			return &s.Invites[i]
		}
	}
	return nil
}

func (s *State) tokenByHash(kind string, hash []byte) *SecretToken {
	for i := range s.Tokens {
		if s.Tokens[i].Kind != kind {
			continue
		}
		if len(s.Tokens[i].Hash) == len(hash) &&
			subtle.ConstantTimeCompare(s.Tokens[i].Hash, hash) == 1 {
			return &s.Tokens[i]
		}
	}
	return nil
}

func adminCount(s *State) int {
	n := 0
	for _, u := range s.Users {
		if u.Role == RoleAdmin && !u.Disabled {
			n++
		}
	}
	return n
}
