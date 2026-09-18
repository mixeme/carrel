// Package sealed holds JSON structures stored inside a user's sealed blob.
package sealed

import (
	"encoding/json"
	"fmt"

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
	// Ignored are the signatures of groups marked "not duplicates": such a
	// group is not offered again until it gains a member.
	Ignored []string `json:"ignored,omitempty"`
}

// LinkedGroup is a set of records the person linked.
type LinkedGroup struct {
	ID      string      `json:"id"`
	Kind    view.Kind   `json:"kind"`
	Members []RecordRef `json:"members"`
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
	for _, s := range d.Ignored {
		if s == id {
			return true
		}
	}
	return false
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

// AttachmentFolder is where an attachment too large for its record goes: a
// folder of a file collection, bound to the collection address (S-6.1).
type AttachmentFolder struct {
	AccountID string   `json:"accountID"`
	Path      string   `json:"path"`             // the file collection
	Folder    []string `json:"folder,omitempty"` // segments under its root; created on first use
}

// Connection stores DAV credentials, the homes discovery found, and an
// optional accepted leaf certificate fingerprint (F-13).
type Connection struct {
	ID              string `json:"id"`
	Server          string `json:"server"`
	Login           string `json:"login"`
	Password        string `json:"password"`
	CertFingerprint string `json:"certFingerprint,omitempty"`
	CalendarHome    string `json:"calendarHome,omitempty"`
	AddressBookHome string `json:"addressBookHome,omitempty"`
	// FileRoot is the entered address when the server named no principal:
	// that address is the file collection (S-6).
	FileRoot string `json:"fileRoot,omitempty"`
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

// Connection returns the connection with id.
func (b Blob) Connection(id string) (Connection, bool) {
	for _, c := range b.Connections {
		if c.ID == id {
			return c, true
		}
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
