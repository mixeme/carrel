package user

import (
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Subscribe adds a feed (S-25.2) and returns its ID. Nothing is ticked: a new
// source does not tick itself (S-6.1). The journal names the subscription,
// not its address.
func (s *Service) Subscribe(viewerID string, dataKey [crypt.KeyLen]byte, f sealed.Subscription) (string, error) {
	id, err := NewFeedID()
	if err != nil {
		return "", err
	}
	f.ID = id
	err = s.editCatalog(viewerID, dataKey, "feed.subscribe", id, func(blob *sealed.Blob) error {
		blob.Subscriptions = append(blob.Subscriptions, f)
		return nil
	})
	return id, err
}

// Unsubscribe removes a feed and its ticks. The feed's owner is not asked
// anything: nothing of theirs was ever written.
func (s *Service) Unsubscribe(viewerID string, dataKey [crypt.KeyLen]byte, id string) error {
	return s.editCatalog(viewerID, dataKey, "feed.unsubscribe", id, func(blob *sealed.Blob) error {
		at := -1
		for i, f := range blob.Subscriptions {
			if f.ID == id {
				at = i
			}
		}
		if at < 0 {
			return ErrNotFound
		}
		blob.Subscriptions = append(blob.Subscriptions[:at], blob.Subscriptions[at+1:]...)
		var ticks []sealed.Selection
		for _, t := range blob.Selected {
			if t.AccountID != id {
				ticks = append(ticks, t)
			}
		}
		blob.Selected = ticks
		return nil
	})
}

// EditFeed renames a feed and sets its colour — both local, nothing goes to
// the feed's server — and, when kinds is not nil, records what it carried at
// its last check.
func (s *Service) EditFeed(viewerID string, dataKey [crypt.KeyLen]byte, id, name, colour string, kinds []view.Kind) error {
	return s.editCatalog(viewerID, dataKey, "feed.edit", id, func(blob *sealed.Blob) error {
		for i := range blob.Subscriptions {
			f := &blob.Subscriptions[i]
			if f.ID != id {
				continue
			}
			if name != "" {
				f.Name = name
			}
			if colour != "" {
				f.Colour = colour
			}
			if kinds != nil {
				f.Kinds = kinds
			}
			return nil
		}
		return ErrNotFound
	})
}
