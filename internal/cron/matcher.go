package cron

import "time"

// matches checks if a time matches the cron schedule
func (cs *CronSchedule) matches(t time.Time) bool {
	return contains(cs.minutes, t.Minute()) &&
		contains(cs.hours, t.Hour()) &&
		cs.matchesDayConstraints(t) &&
		contains(cs.months, int(t.Month()))
}

// matchesDayConstraints handles the special day-of-month vs day-of-week logic
//
// Cron standard behavior:
// - If both day-of-month and day-of-week are restricted (not *): match if EITHER matches (OR logic)
// - If only one is restricted: match on that field only
// - If both are *: match any day
func (cs *CronSchedule) matchesDayConstraints(t time.Time) bool {
	// Check if fields are restricted (not all possible values)
	domRestricted := len(cs.daysOfMonth) < 31
	dowRestricted := len(cs.daysOfWeek) < 7

	if domRestricted && dowRestricted {
		// OR logic: either day-of-month OR day-of-week must match
		domMatch := contains(cs.daysOfMonth, t.Day())
		dowMatch := contains(cs.daysOfWeek, int(t.Weekday()))

		// Also need to validate the date is actually valid (Feb 29 in non-leap year)
		if domMatch && !isValidDate(t.Year(), int(t.Month()), t.Day()) {
			domMatch = false
		}

		return domMatch || dowMatch
	} else if domRestricted {
		// Only day-of-month is restricted
		if !contains(cs.daysOfMonth, t.Day()) {
			return false
		}
		// Validate the date is actually valid
		return isValidDate(t.Year(), int(t.Month()), t.Day())
	} else if dowRestricted {
		// Only day-of-week is restricted
		return contains(cs.daysOfWeek, int(t.Weekday()))
	}

	// Both unrestricted, match any day
	// Still need to validate the date exists (Feb 29 in non-leap year)
	return isValidDate(t.Year(), int(t.Month()), t.Day())
}

// contains checks if a slice contains a value
func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
