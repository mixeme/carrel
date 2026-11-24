package store

import (
	"strings"
)

// UserByID finds an account by ID.
func (s *State) UserByID(id string) *User {
	return s.userByID(id)
}

// UserByLogin finds an account by normalized login.
func (s *State) UserByLogin(login string) *User {
	return s.userByLogin(login)
}

// AdminCount returns live administrators.
func AdminCount(s *State) int {
	return adminCount(s)
}

// SoleAdmin is whether u is the only live administrator: nobody else could
// recover their deposited key (the owner's decision of 24.11.2026).
func SoleAdmin(s *State, u *User) bool {
	return u != nil && u.Role == RoleAdmin && !u.Disabled && adminCount(s) == 1
}

// NormalizeLogin lowercases and trims a login for comparison.
func NormalizeLogin(login string) string {
	return strings.ToLower(strings.TrimSpace(login))
}
