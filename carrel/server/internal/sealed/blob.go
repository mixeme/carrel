// Package sealed holds JSON structures stored inside a user's sealed blob.
package sealed

import (
	"encoding/json"
	"fmt"
	"strings"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/view"
)

// Blob is the plaintext inside a user's SealedBlob ciphertext. Re-discovery is
// an explicit action (S-6.1), so the collection catalog lives here and not in
// the session cache: it survives restarts, and a new collection on the server
// does not join it until the person applies the difference.
type Blob struct {
	Connections []Connection `json:"connections,omitempty"`
	Collections []Collection `json:"collections,omitempty"`
	Selected    []Selection  `json:"selected,omitempty"`
	// Attachments is the folder chosen once in settings (S-20); nil until then.
	Attachments *AttachmentFolder `json:"attachments,omitempty"`
	// AttachNames is how a large attachment is named in that folder
	// (S-20, attachments-3): AttachNames…; empty is by date, as a blob
	// written before this field reads.
	AttachNames string `json:"attachNames,omitempty"`
	// LastUsed is where a record of each kind was last created, one entry per
	// kind. The quick sheet opened from the shell has no collection to go by
	// and starts there (U-6.16). Empty until the first record is made, and a
	// blob written before this field reads as empty — nothing to migrate.
	LastUsed []Selection `json:"lastUsed,omitempty"`
	// Duplicates are the person's decisions about duplicates and their own
	// threshold (S-16): the only knowledge of what foreign collections hold
	// that lies on disk, and so sealed like the credentials. Metadata only —
	// addresses and UIDs, never a record. Empty in a blob written before it.
	Duplicates Duplicates `json:"duplicates,omitzero"`
	// DeviceExcluded are the connections left out of every device set-up
	// (S-23): not in an Apple profile, not in the tables of the other tabs.
	DeviceExcluded []string `json:"deviceExcluded,omitempty"`
	// Subscriptions are feeds read on every visit, never kept (S-25.2). A
	// subscription is not a collection of any server: Connection and
	// Collection answer for it as a read-only one, so addresses, ownership
	// and ticks work the same, and nothing that walks Connections or
	// Collections — devices, re-discovery, the connection cards — sees it.
	Subscriptions []Subscription `json:"subscriptions,omitempty"`
	// BackupJobs are the person's copy jobs (S-24.1): what, where, how many
	// to keep, sealed or not, and the job's own password — under the data
	// key, like the credentials.
	BackupJobs []BackupJob `json:"backupJobs,omitempty"`
	// Links are the person's published links whole, by ID (S-25.1): the
	// instance keeps only their hashes, so the link itself is kept here,
	// under the data key, for Copy and for a letter (owner's decision
	// 19.09.2026). A link made before this field has no entry.
	Links map[string]string `json:"links,omitempty"`
	// UploadTo is where "Upload file" of the create menu puts files, once
	// chosen on its sheet; until then the attachment folder (U-6.4).
	UploadTo *AttachmentFolder `json:"uploadTo,omitempty"`
	// Mailboxes are the person's own mailboxes (carrel-mailbox.md M-3):
	// IMAP and SMTP, sealed like the DAV credentials. A blob written before
	// this field reads as none; the state file's format went up with it
	// (store.FormatVersion 2), so an older build refuses the file rather
	// than drop the mailboxes with their passwords.
	Mailboxes []Mailbox `json:"mailboxes,omitempty"`
}

// Destinations of a backup job (S-24.1).
const (
	BackupToWebDAV   = "webdav"
	BackupToDownload = "download"
	BackupToBoth     = "both"
)

// BackupJob is one copy job.
type BackupJob struct {
	ID      string      `json:"id"` // "j-…"
	Name    string      `json:"name"`
	Sources []Selection `json:"sources"` // catalog collections, by address
	Mode    string      `json:"mode"`    // BackupTo…
	// DestAccount and DestPath are the file collection the archive goes to,
	// DestFolder the folder under its root; created on first use.
	DestAccount string   `json:"destAccount,omitempty"`
	DestPath    string   `json:"destPath,omitempty"`
	DestFolder  []string `json:"destFolder,omitempty"`
	Keep        int      `json:"keep"`
	Encrypted   bool     `json:"encrypted"`
	Password    string   `json:"password,omitempty"`
	// OnSignInDays runs the job at sign-in when its last whole copy is older
	// than that many days; 0 is never. Carrel keeps no clock of its own.
	OnSignInDays int `json:"onSignInDays,omitempty"`
	// Off is «Job is on» unticked (s-backup-2): the job runs from the screen
	// only — not at sign-in, not from outside. Zero is on, as every job
	// made before the switch.
	Off bool `json:"off,omitempty"`
}

// BackupJob returns the job with id.
func (b Blob) BackupJob(id string) (BackupJob, bool) {
	for _, j := range b.BackupJobs {
		if j.ID == id {
			return j, true
		}
	}
	return BackupJob{}, false
}

// FeedPath is the {col} of every subscription: a feed is one file, one
// collection.
const FeedPath = "feed"

// Subscription is one feed: its address, the local name and colour, the
// optional credentials (sealed with the rest of the blob) and the kinds of
// record it carried when it was checked.
type Subscription struct {
	ID       string      `json:"id"` // "f-…", the {account} of its addresses
	URL      string      `json:"url"`
	Name     string      `json:"name"`
	Colour   string      `json:"colour,omitempty"`
	Login    string      `json:"login,omitempty"`
	Password string      `json:"password,omitempty"`
	Kinds    []view.Kind `json:"kinds"`
}

// Subscription returns the feed with id.
func (b Blob) Subscription(id string) (Subscription, bool) {
	for _, f := range b.Subscriptions {
		if f.ID == id {
			return f, true
		}
	}
	return Subscription{}, false
}

// FeedCollection is a subscription as a read-only catalog entry.
func (f Subscription) FeedCollection() Collection {
	return Collection{AccountID: f.ID, Path: FeedPath, Name: f.Name, Kinds: f.Kinds, Colour: f.Colour, ReadOnly: true, Feed: true}
}

// FeedConnection is a subscription as the connection it is read through.
func (f Subscription) FeedConnection() Connection {
	return Connection{ID: f.ID, Server: f.URL, Login: f.Login, Password: f.Password, Feed: true}
}

// IsDeviceExcluded reports whether the connection is left out of devices.
func (b Blob) IsDeviceExcluded(accountID string) bool {
	for _, id := range b.DeviceExcluded {
		if id == accountID {
			return true
		}
	}
	return false
}

// Duplicates are the decisions of S-16. Nothing here is written to a server.
type Duplicates struct {
	// Threshold is the person's own number of points; 0 until they choose
	// one, and the instance value applies (CARREL_DUPLICATES_THRESHOLD).
	Threshold int `json:"threshold,omitempty"`
	// Weights are the person's own points per signal, by the signal's name
	// ("email", "phone", "uid", "name", "birthday", "title-date"); a signal
	// not here weighs what the instance says. 0 is a weight: the signal off.
	Weights map[string]int `json:"weights,omitempty"`
	// Linked are groups shown as one entity. Linking changes nothing on any
	// server and is undone at any time.
	Linked []LinkedGroup `json:"linked,omitempty"`
	// Ignored are the groups marked "not duplicates": such a group is not
	// offered again until it gains a member. Its members are kept with its
	// signature so the decision can be named and taken back while nothing
	// is loaded (s-dup-6, «Decided earlier»).
	Ignored []IgnoredGroup `json:"ignored,omitempty"`
}

// IgnoredGroup is a group the person marked "not duplicates".
type IgnoredGroup struct {
	// ID is the group's signature (dupes.Signature)
	ID      string      `json:"id"`
	Kind    view.Kind   `json:"kind"`
	Members []RecordRef `json:"members"`
	// Title is as LinkedGroup's: «Анна Ковалёва · Алексей Смирнов»
	Title string `json:"title,omitempty"`
}

// LinkedGroup is a set of records the person linked.
type LinkedGroup struct {
	ID      string      `json:"id"`
	Kind    view.Kind   `json:"kind"`
	Members []RecordRef `json:"members"`
	// Title is what the group was called when it was decided: its records'
	// names, each once, «Анна Ковалёва». The decision is named by it while
	// no record of it is loaded (s-dup-6, «Decided earlier»).
	Title string `json:"title,omitempty"`
	// Prefs answer a single-valued property the members disagree on: the
	// member whose value the group shows, by property name. Asked once, when
	// linking, and kept (S-16).
	Prefs map[string]RecordRef `json:"prefs,omitempty"`
	// Dropped answer a many-valued property (TEL, EMAIL, CATEGORIES): the
	// values the person unticked, by property name, as the group's question
	// names them. Everything else the members hold stays — a value that
	// arrives later was never unticked (from-typecase note 117 p. 1).
	Dropped map[string][]string `json:"dropped,omitempty"`
}

// RecordRef is a record by its connection, collection address and UID.
type RecordRef struct {
	AccountID string `json:"accountID"`
	Col       string `json:"col"`
	UID       string `json:"uid"`
}

// IsIgnored reports whether the group with this signature is marked "not
// duplicates".
func (d Duplicates) IsIgnored(id string) bool {
	for _, g := range d.Ignored {
		if g.ID == id {
			return true
		}
	}
	return false
}

// Unignored is the ignored groups without the one of this signature.
func (d Duplicates) Unignored(id string) []IgnoredGroup {
	var out []IgnoredGroup
	for _, g := range d.Ignored {
		if g.ID != id {
			out = append(out, g)
		}
	}
	return out
}

// LastFor is where a record of this kind was last created.
func (b Blob) LastFor(k view.Kind) (Selection, bool) {
	for _, s := range b.LastUsed {
		if s.Kind == k {
			return s, true
		}
	}
	return Selection{}, false
}

// How a large attachment is named in the folder (S-20): the date of the
// record, then the file's own name; the file's own name alone; the record's
// title, then the file's own name.
const (
	AttachNamesDated    = "dated"
	AttachNamesOriginal = "original"
	AttachNamesTitled   = "titled"
)

// AttachmentFolder is where an attachment too large for its record goes: a
// folder of a file collection, bound to the collection address (S-6.1).
type AttachmentFolder struct {
	AccountID string   `json:"accountID"`
	Path      string   `json:"path"`             // the file collection
	Folder    []string `json:"folder,omitempty"` // segments under its root; created on first use
	// Monthly puts each attachment in a subfolder of its month, «2026-08»,
	// made on first use (attachments-1); the month is the one the file's
	// dated name carries. Off, the folder is flat. Only the attachment folder
	// reads it.
	Monthly bool `json:"monthly,omitempty"`
}

// Connection stores DAV credentials, the homes discovery found, and an
// optional accepted leaf certificate fingerprint (F-13).
type Connection struct {
	ID string `json:"id"`
	// Label is the person's name for the connection; empty, it goes by its
	// host. A label, never an identifier (C-4).
	Label           string `json:"label,omitempty"`
	Server          string `json:"server"`
	Login           string `json:"login"`
	Password        string `json:"password"`
	CertFingerprint string `json:"certFingerprint,omitempty"`
	CalendarHome    string `json:"calendarHome,omitempty"`
	AddressBookHome string `json:"addressBookHome,omitempty"`
	// FileRoot is the entered address when the server named no principal:
	// that address is the file collection (S-6).
	FileRoot string `json:"fileRoot,omitempty"`
	// NoPassword says the server asks no password (s-profile-14): the
	// password is empty and stays empty. An empty password field cannot say
	// it — empty means «unchanged» on edit.
	NoPassword bool `json:"noPassword,omitempty"`
	// Scheduling is what discovery learned of the principal (RFC 6638):
	// "yes" when it announced a schedule-outbox-URL, "no" when it answered
	// without one, "" when there is no principal or it was never asked — a
	// connection made before the question stays unknown until it is edited
	// or discovered again (S-15.4).
	Scheduling string `json:"scheduling,omitempty"`
	// Addresses are the person's own calendar addresses on this server
	// (calendar-user-address-set), lower-cased and without mailto:. A
	// meeting organised at one of them is the person's own.
	Addresses []string `json:"addresses,omitempty"`
	// Feed marks a subscription answered as a connection; never stored.
	Feed bool `json:"-"`
}

// Collection is one catalog entry. Path is the address on the server, the key
// everything else binds to (S-6.1); Name is only a label.
type Collection struct {
	AccountID       string      `json:"accountID"`
	Path            string      `json:"path"`
	Name            string      `json:"name"`
	Kinds           []view.Kind `json:"kinds"`
	Colour          string      `json:"colour,omitempty"`
	ReadOnly        bool        `json:"readOnly,omitempty"`
	Components      []string    `json:"components,omitempty"`
	ChangeTag       string      `json:"changeTag,omitempty"`
	MaxResourceSize int64       `json:"maxResourceSize,omitempty"`
	// Gone marks an entry re-discovery no longer found and the person applied
	// (S-6.1). It stays so a selection holding it says gone (C-8.2).
	Gone bool `json:"gone,omitempty"`
	// Feed marks a subscription answered as a collection; never stored.
	Feed bool `json:"-"`
	// Twin marks a collection whose connection holds another under the same
	// name — Nextcloud allows it — so that its label carries its address name
	// too (C-10, owner's decision 24.09.2026). Worked out on Open; never
	// stored.
	Twin bool `json:"-"`
}

// markTwins sets Twin on every collection that shares its name with another
// of the same connection holding a kind it holds too: two calendars called
// alike are told apart in the same rail and the same column, while a
// calendar and an address book of one name never meet in either. Letter case
// does not tell two names apart on a screen, and does not here.
func (b *Blob) markTwins() {
	same := func(x, y Collection) bool {
		if x.AccountID != y.AccountID || x.Name == "" || !strings.EqualFold(x.Name, y.Name) {
			return false
		}
		for _, k := range x.Kinds {
			if y.Has(k) {
				return true
			}
		}
		return false
	}
	for i := range b.Collections {
		b.Collections[i].Twin = false
		for j := range b.Collections {
			if i != j && same(b.Collections[i], b.Collections[j]) {
				b.Collections[i].Twin = true
				break
			}
		}
	}
}

// Has reports whether the collection holds records of kind k.
func (c Collection) Has(k view.Kind) bool {
	for _, have := range c.Kinds {
		if have == k {
			return true
		}
	}
	return false
}

// Selection is one tick of the combined view or the files root, bound to the
// collection address, not its name (S-6.1, S-14).
type Selection struct {
	Kind      view.Kind `json:"kind"`
	AccountID string    `json:"accountID"`
	Path      string    `json:"path"`
}

// ConnectionCount is the number of connections S-3 counts: the servers and
// the mailboxes (carrel-mailbox.md M-3), so every screen that shows the
// number asks this. Subscriptions are not connections and are not counted.
func (b Blob) ConnectionCount() int {
	return len(b.Connections) + len(b.Mailboxes)
}

// Connection returns the connection with id.
func (b Blob) Connection(id string) (Connection, bool) {
	for _, c := range b.Connections {
		if c.ID == id {
			return c, true
		}
	}
	if f, ok := b.Subscription(id); ok {
		return f.FeedConnection(), true
	}
	return Connection{}, false
}

// Collection returns the catalog entry at path under the connection.
func (b Blob) Collection(accountID, path string) (Collection, bool) {
	for _, c := range b.Collections {
		if c.AccountID == accountID && c.Path == path {
			return c, true
		}
	}
	if f, ok := b.Subscription(accountID); ok && path == FeedPath {
		return f.FeedCollection(), true
	}
	return Collection{}, false
}

// CollectionsOf returns the catalog entries of one connection, in catalog order.
func (b Blob) CollectionsOf(accountID string) []Collection {
	var out []Collection
	for _, c := range b.Collections {
		if c.AccountID == accountID {
			out = append(out, c)
		}
	}
	return out
}

// Open decrypts a user's sealed blob.
func Open(dataKey [crypt.KeyLen]byte, ciphertext []byte, login string) (Blob, error) {
	var out Blob
	if len(ciphertext) == 0 {
		return out, nil
	}
	plain, err := crypt.DecryptBlob(dataKey, ciphertext, login)
	if err != nil {
		return out, err
	}
	if len(plain) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(plain, &out); err != nil {
		return out, fmt.Errorf("sealed: decode blob: %w", err)
	}
	out.markTwins()
	return out, nil
}

// Seal encrypts a user's sealed blob.
func Seal(dataKey [crypt.KeyLen]byte, b Blob, login string) ([]byte, error) {
	plain, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}
	if len(plain) == 0 {
		plain = []byte("{}")
	}
	return crypt.EncryptBlob(dataKey, plain, login)
}
