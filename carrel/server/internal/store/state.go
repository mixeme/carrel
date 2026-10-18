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
	// Backups are what a copy job leaves outside the sealed blob (S-24): its
	// runs — so a status can be answered without the data key — and, when
	// the person turns the external trigger on, its secret's hash and an
	// envelope of their data key.
	Backups []BackupState `json:"backups,omitempty"`
}

// BackupState is one job's state outside the blob.
type BackupState struct {
	JobID  string      `json:"jobID"`
	UserID string      `json:"userID"`
	Runs   []BackupRun `json:"runs,omitempty"` // newest last, at most BackupRunsKept
	// TriggerHash is the hash of the external trigger's secret; empty when
	// the trigger is off.
	TriggerHash []byte `json:"triggerHash,omitempty"`
	// KeyEnvelope is the person's data key sealed under server.key, bound to
	// the job (crypt.SealBackupKey) — the named loss of protection that lets
	// an external trigger read without a session.
	KeyEnvelope []byte `json:"keyEnvelope,omitempty"`
}

// BackupRunsKept is how many runs a job's history keeps.
const BackupRunsKept = 20

// BackupRun is one run of a job: when, what went in, how it ended, how big.
type BackupRun struct {
	ID          string     `json:"id"`
	Started     time.Time  `json:"started"`
	Finished    *time.Time `json:"finished,omitempty"`
	State       string     `json:"state"` // running, done, partial, failed
	Trigger     string     `json:"trigger"`
	Mode        string     `json:"mode"`
	File        string     `json:"file,omitempty"`
	Size        int64      `json:"size,omitempty"`
	Collections int        `json:"collections"`
	Whole       int        `json:"whole"`
	Verified    bool       `json:"verified,omitempty"`
	Error       string     `json:"error,omitempty"`
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
	// From and To are the days a published calendar shows, 2006-01-02 and
	// both in it; empty means the whole calendar, week by week (S-25.1).
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
	// NoDownload keeps the .ics of a published calendar from being handed
	// over at all — the page stays, the file and the subscription go.
	NoDownload bool `json:"noDownload,omitempty"`
	// HideDescriptions leaves the text of events out of the public page and
	// the file: the times are shared, what was said at them is not.
	HideDescriptions bool `json:"hideDescriptions,omitempty"`
	// Shows is what of a contact, or of every contact of a book, the visitor
	// sees beside the name and the organisation: "phones", "emails",
	// "photo", "note" — each asked for apart when the link is made
	// (S-25.1, s-publish-3). A link without it shows none of the four.
	Shows []string `json:"shows,omitempty"`
	// HideOrganisation leaves the visitor of a contact or a book the name
	// alone: «Name and organisation» of the sheet was taken off.
	HideOrganisation bool `json:"hideOrganisation,omitempty"`
	// Colour is the collection's colour as it stood when the link was made:
	// the mark of each row of its page (Пс7, Пс12).
	Colour string `json:"colour,omitempty"`
	// Title is the label the link was made under: the collection's name or
	// the record's title as it stood. The page shows the live one.
	Title     string     `json:"title"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"` // nil: no expiry, chosen explicitly
	LastSeen  *time.Time `json:"lastSeen,omitempty"`
	// Opens is how many times the link was opened, as of the last time
	// LastSeen was written: the one way to see that a link went further than
	// it was meant to (s-publish-1). The opens since are in memory.
	Opens int `json:"opens,omitempty"`
	// Gone says that what the link opens is no longer there and Carrel could
	// not follow it: a record deleted, one moved where its new place is not
	// known, a collection removed (Б41). The link stays until it expires or
	// is revoked; the owner's list says it is dead.
	Gone bool `json:"gone,omitempty"`
	// Parent is the publish.ID of the link this one was made with: an
	// attachment of a published note (Пб3, s-publish-4). It expires with
	// that link, is revoked with it and is not a row of the owner's list;
	// its secret is derived from the parent's (user.ChildToken).
	Parent string `json:"parent,omitempty"`
	// Children is how many links were made with this one, numbered from 0.
	Children int `json:"children,omitempty"`
	// Attach is the attachment of the record UID this link opens, its
	// position among the ATTACH values plus one: a file written into the
	// note. 0 — the link opens the record itself.
	Attach int `json:"attach,omitempty"`
	// AttachSize is the size that attachment was published at: with its
	// name, what tells it from a file a removal shifted into its place.
	AttachSize int64 `json:"attachSize,omitempty"`
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

	// SMTP is the outgoing relay (S-26): an administrator's setting, not an
	// environment variable. The password lives only here, inside state.enc.
	SMTP SMTPSettings `json:"smtp,omitempty"`

	// The administrator's settings of the instance (owner's decision
	// 19.09.2026): zero is the default — the constants of conf for the
	// lifetimes, the CARREL_MAIL_* of the first start for mail.
	Name              string `json:"name,omitempty"`
	IdleHours         int    `json:"idleHours,omitempty"`
	SessionDays       int    `json:"sessionDays,omitempty"`
	InviteHours       int    `json:"inviteHours,omitempty"`
	PeopleMail        *bool  `json:"peopleMail,omitempty"`
	MailMaxAttachMB   int    `json:"mailMaxAttachMB,omitempty"`
	MailMaxMessageMB  int    `json:"mailMaxMessageMB,omitempty"`
	MailMaxRecipients int    `json:"mailMaxRecipients,omitempty"`
	MailMaxPerDay     int    `json:"mailMaxPerDay,omitempty"`
}

// InstanceName is the name the instance goes by: "Carrel" unless set.
func (s InstanceSettings) InstanceName() string {
	if s.Name == "" {
		return "Carrel"
	}
	return s.Name
}

// SMTPSettings is the relay and the instance's sender address.
type SMTPSettings struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Security string `json:"security,omitempty"` // mail.StartTLS, mail.TLS, mail.None
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	From     string `json:"from,omitempty"`
	FromName string `json:"fromName,omitempty"`
	// Sender is the From of people's letters: mail.SenderService (empty) or
	// mail.SenderLogin.
	Sender string `json:"sender,omitempty"`
}

// Configured reports whether letters can be sent: a relay and a sender.
func (s SMTPSettings) Configured() bool {
	return s.Host != "" && s.Port > 0 && s.From != ""
}

// Invite is a pending account invitation (S-10.2).
type Invite struct {
	ID string `json:"id"`
	// Login is the account the invitation is for; empty — the invited
	// person chooses it with the password (s-admin-1, s-auth-12). Named
	// only by invitations made before v0.2.68.
	Login string `json:"login"`
	Email string `json:"email,omitempty"`
	// Role is what the account becomes when the invitation is accepted
	// (s-admin-8); empty is a user.
	Role       Role      `json:"role,omitempty"`
	SecretHash []byte    `json:"secretHash"`
	Secret     string    `json:"secret,omitempty"` // raw token; lives only inside encrypted state.enc for the admin panel link (S-10.3)
	CreatedAt  time.Time `json:"createdAt"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Revoked    bool      `json:"revoked,omitempty"` // withdrawn by an administrator or replaced by a resend
	Used       bool      `json:"used,omitempty"`    // accepted: the invitee set a password
	// Mailed is how the last letter went: "sent", "failed", or empty when
	// none was tried (no address, or no relay).
	Mailed string `json:"mailed,omitempty"`
	// MailReply is what the relay answered to a letter that did not go,
	// «550 mailbox unavailable» (s-admin-1 names a failure by it); empty
	// once a letter went.
	MailReply string `json:"mailReply,omitempty"`
	// By is the login of the administrator who made it: «mix invited
	// you.» (s-auth-12). Empty for invitations made before v0.2.71.
	By string `json:"by,omitempty"`
}

// ProxyDevice is one davloom device: its wrapper and what it may see.
type ProxyDevice struct {
	ID         string        `json:"id"` // "d-…"
	Name       string        `json:"name"`
	Salt       []byte        `json:"salt"`
	KDF        kdfRecord     `json:"kdf"`
	WrappedKey []byte        `json:"wrappedKey"`
	Created    time.Time     `json:"created"`
	ScopeKinds []string      `json:"scopeKinds"`
	ScopeCols  []DeviceScope `json:"scopeCols"`
	// ReadOnly: the device reads and never writes, whatever its collections
	// allow (owner's decision 24.09.2026, revisiting S-27). A collection
	// read-only for its connection stays read-only either way.
	ReadOnly bool `json:"readOnly,omitempty"`
}

// DeviceScope is one collection a device sees: the connection and the
// collection's path, as the catalog names them.
type DeviceScope struct {
	AccountID string `json:"accountID"`
	Path      string `json:"path"`
}

// DeviceKDF is the KDF record of a device wrapper as crypt wants it.
func (d ProxyDevice) DeviceKDF() crypt.KDFParams { return d.KDF.toCrypt() }

// SetDeviceKDF records the parameters a device wrapper was derived with.
func (d *ProxyDevice) SetDeviceKDF(p crypt.KDFParams) { d.KDF = kdfFromCrypt(p) }

// SecretToken covers email confirmation, forgot/console wipe links (S-10).
type SecretToken struct {
	Kind   string `json:"kind"`
	Hash   []byte `json:"hash"`
	UserID string `json:"userID"`
	Step   string `json:"step,omitempty"`
	// Email is the address an email-confirmation token confirms; empty for
	// the address the account already has (registration).
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Used      bool      `json:"used,omitempty"`
	// MailedAt is when the relay took the letter carrying this link; nil
	// while no letter went. Only an email-confirmation token has it.
	MailedAt *time.Time `json:"mailedAt,omitempty"`
}

// User is one account on the instance. ConnectionCount is stored outside the
// sealed blob so administration can read it without the data key (S-3).
type User struct {
	ID        string    `json:"id"`
	Login     string    `json:"login"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	Disabled  bool      `json:"disabled,omitempty"`

	Email          string `json:"email,omitempty"`
	EmailConfirmed bool   `json:"emailConfirmed,omitempty"`
	// EmailConfirmedAt is when Email was proven (the link opened, an
	// invitation by letter completed). Nil on an address confirmed before
	// v0.2.60: the day was not written then.
	EmailConfirmedAt *time.Time `json:"emailConfirmedAt,omitempty"`
	// PendingEmail is a new address waiting for its link; Email stays in use
	// until then.
	PendingEmail string `json:"pendingEmail,omitempty"`
	// ProxyDevices are the davloom devices (S-27.2): each wraps the data key
	// under a password of its own. They live here, not in the blob — the
	// wrapper is opened before there is a data key to open a blob with.
	ProxyDevices []ProxyDevice `json:"proxyDevices,omitempty"`
	// MailDay and MailSent count the letters sent on one UTC day (S-26.2,
	// C-12): the day as 2006-01-02 and the letters so far.
	MailDay            string `json:"mailDay,omitempty"`
	MailSent           int    `json:"mailSent,omitempty"`
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
	// PasswordLog is the token of the link to this account's password
	// history (the owner's decision of 16.10.2026, Ад40): made at the first
	// deposit, shown in Key escrow, never changed or revoked — by nobody.
	// PasswordEvents are what that page shows: kept here, not read from the
	// journal, which is trimmed.
	PasswordLog    string          `json:"passwordLog,omitempty"`
	PasswordEvents []PasswordEvent `json:"passwordEvents,omitempty"`

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
