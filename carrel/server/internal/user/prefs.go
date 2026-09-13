package user

import (
	"errors"
	"time"

	"carrel.dev/carrel/server/internal/store"
)

// WeekStarts are the first days of the week an account may choose (C-10). An
// account that never chose one reads Monday.
var WeekStarts = map[string]time.Weekday{
	"monday":   time.Monday,
	"sunday":   time.Sunday,
	"saturday": time.Saturday,
}

// ErrWeekStart is a first day of the week outside WeekStarts.
var ErrWeekStart = errors.New("unknown first day of the week")

// SetTimePrefs stores the account's C-10 display settings: the first day of
// the week and the 12-hour clock. They are not secrets and live outside the
// sealed blob, beside the display name.
func (s *Service) SetTimePrefs(accountID, weekStart string, clock12 bool) error {
	if _, ok := WeekStarts[weekStart]; !ok {
		return ErrWeekStart
	}
	if weekStart == "monday" {
		weekStart = ""
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(accountID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		u.WeekStart, u.Clock12 = weekStart, clock12
		return nil, nil
	})
}
