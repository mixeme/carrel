package user

import (
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
)

// EditDuplicates changes the person's decisions about duplicates (S-16) in
// the sealed blob. what names the decision in the journal with detail — the
// kind or the number, never an address; an empty what writes no journal line,
// for the silent upkeep of a member that is gone. A failed edit writes
// nothing.
func (s *Service) EditDuplicates(viewerID string, dataKey [crypt.KeyLen]byte, what, detail string, edit func(*sealed.Duplicates) error) error {
	apply := func(blob *sealed.Blob) error { return edit(&blob.Duplicates) }
	if what == "" {
		return s.editBlob(viewerID, dataKey, nil, apply)
	}
	return s.editCatalog(viewerID, dataKey, what, detail, apply)
}
