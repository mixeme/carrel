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
