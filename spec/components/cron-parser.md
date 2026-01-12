# Cron Parser & Calculator Implementation Plan

## Overview
Implement a cron expression parser and schedule calculator for the Itinerary scheduler. This component parses standard 5-field cron expressions and calculates job run times within specified windows.

## Scope

### Cron Format
Standard 5-field cron expression:
```
minute hour day-of-month month day-of-week
(0-59) (0-23) (1-31)      (1-12) (0-6, Sunday=0)
```

### Supported Syntax
- `*` - Any value (all valid values for the field)
- `,` - Value list (e.g., `1,3,5`)
- `-` - Range (e.g., `1-5`)
- `/` - Step (e.g., `*/5` or `1-10/2`)

### Examples
- `0 0 * * *` - Daily at midnight
- `*/15 * * * *` - Every 15 minutes
- `0 9-17 * * 1-5` - Hourly 9 AM to 5 PM, Monday through Friday
- `0 0 1,15 * *` - Twice a month on the 1st and 15th

## Data Structures

```go
package cron

import "time"

// CronSchedule represents a parsed cron expression
type CronSchedule struct {
    // Each field stores all valid values for that field
    minutes     []int  // 0-59
    hours       []int  // 0-23
    daysOfMonth []int  // 1-31
    months      []int  // 1-12
    daysOfWeek  []int  // 0-6 (0=Sunday)

    // Store original expression for debugging
    original string
}
```

## Public API

```go
// Parse parses a cron expression and validates all constraints
// Returns error if:
// - Format is invalid (not 5 fields)
// - Any field contains invalid syntax
// - Impossible dates are specified (e.g., Feb 31st)
func Parse(expr string) (*CronSchedule, error)

// Next calculates the next N occurrences of this schedule after the given time
// The time is evaluated in the provided timezone
// Returns empty slice if no occurrences found (should not happen for valid schedules)
func (cs *CronSchedule) Next(after time.Time, count int) []time.Time

// Between calculates all occurrences within the given time window [start, end)
// Start is inclusive, end is exclusive
// Returns all matching times in chronological order
func (cs *CronSchedule) Between(start, end time.Time) []time.Time
```

## Implementation Details

### Parsing Algorithm

1. **Split and validate structure**
   - Split expression on whitespace
   - Verify exactly 5 fields
   - Return error if format is invalid

2. **Parse each field**
   - Call `parseField(field string, min, max int)` for each field
   - Returns sorted slice of valid values or error
   - Parsing priority:
     1. Check for `*` → return all values in [min, max]
     2. Check for `/` → parse step expression
     3. Check for `,` → split and parse each part
     4. Check for `-` → parse range
     5. Single number → validate and return

3. **Validate impossible dates**
   - Check if schedule includes impossible date combinations
   - Examples: Feb 30, Feb 31, Apr 31, June 31, Sep 31, Nov 31
   - Return error if impossible date is explicitly required

### Field Parsing Details

```go
// parseField parses a single cron field
func parseField(field string, min, max int) ([]int, error) {
    // Handle wildcard
    if field == "*" {
        return expandRange(min, max), nil
    }

    // Handle step values: */N or M-N/S
    if strings.Contains(field, "/") {
        return parseStep(field, min, max)
    }

    // Handle lists: 1,3,5
    if strings.Contains(field, ",") {
        return parseList(field, min, max)
    }

    // Handle ranges: 1-5
    if strings.Contains(field, "-") {
        return parseRange(field, min, max)
    }

    // Single value
    return parseSingle(field, min, max)
}
```

### Occurrence Calculation

```go
// Next implementation
func (cs *CronSchedule) Next(after time.Time, count int) []time.Time {
    results := make([]time.Time, 0, count)

    // Start at the next minute (truncate to minute, add 1 minute)
    current := after.Truncate(time.Minute).Add(time.Minute)

    for len(results) < count {
        if cs.matches(current) {
            results = append(results, current)
        }
        current = current.Add(time.Minute)
    }

    return results
}

// Between implementation
func (cs *CronSchedule) Between(start, end time.Time) []time.Time {
    results := []time.Time{}

    // Start at start time truncated to minute
    current := start.Truncate(time.Minute)

    for current.Before(end) {
        if cs.matches(current) {
            results = append(results, current)
        }
        current = current.Add(time.Minute)
    }

    return results
}

// matches checks if a time matches the cron schedule
func (cs *CronSchedule) matches(t time.Time) bool {
    return contains(cs.minutes, t.Minute()) &&
           contains(cs.hours, t.Hour()) &&
           cs.matchesDayConstraints(t) &&
           contains(cs.months, int(t.Month()))
}
```

### Day-of-Month vs Day-of-Week Logic

Special handling for day constraints (cron standard behavior):
- If **both** day-of-month and day-of-week are restricted (not `*`): match if **either** matches (OR logic)
- If only one is restricted: match on that field only
- If both are `*`: match any day

```go
func (cs *CronSchedule) matchesDayConstraints(t time.Time) bool {
    // Check if fields are restricted (not all possible values)
    domRestricted := len(cs.daysOfMonth) < 31
    dowRestricted := len(cs.daysOfWeek) < 7

    if domRestricted && dowRestricted {
        // OR logic: either day-of-month OR day-of-week must match
        return contains(cs.daysOfMonth, t.Day()) ||
               contains(cs.daysOfWeek, int(t.Weekday()))
    } else if domRestricted {
        return contains(cs.daysOfMonth, t.Day())
    } else if dowRestricted {
        return contains(cs.daysOfWeek, int(t.Weekday()))
    }

    // Both unrestricted, match any day
    return true
}
```

### Impossible Date Detection

Check for impossible date combinations at parse time:
- Month 2 (February): Only days 1-29 valid (29 only on leap years, but we allow it)
- Months 4, 6, 9, 11 (Apr, Jun, Sep, Nov): Only days 1-30 valid
- Months 1, 3, 5, 7, 8, 10, 12: Days 1-31 valid

If a schedule explicitly requires an impossible date (not via wildcard), return parse error.

```go
func validateImpossibleDates(daysOfMonth, months []int) error {
    // For each month, check if any day-of-month is impossible
    for _, month := range months {
        maxDay := daysInMonth(month)
        for _, day := range daysOfMonth {
            if day > maxDay {
                return fmt.Errorf("impossible date: day %d does not exist in month %d", day, month)
            }
        }
    }
    return nil
}

func daysInMonth(month int) int {
    switch month {
    case 2:  // February
        return 29  // Allow leap year
    case 4, 6, 9, 11:  // Apr, Jun, Sep, Nov
        return 30
    default:  // Jan, Mar, May, Jul, Aug, Oct, Dec
        return 31
    }
}
```

## Configuration

### Timezone Handling
- Cron schedules are evaluated in a configurable timezone (global setting)
- All schedules are stored internally in UTC
- When calculating occurrences:
  1. Convert input times to configured timezone
  2. Evaluate schedule in that timezone
  3. Return results in the input time's location

## Error Handling

Parse errors returned for:
- Wrong number of fields (not 5)
- Invalid field syntax (malformed ranges, steps, etc.)
- Values out of range (e.g., minute=70, hour=25)
- Invalid step values (e.g., `*/0`)
- Empty fields
- Impossible dates (e.g., `0 0 31 2 *`)

## Dependencies

**Standard library only:**
- `time` - Time manipulation
- `strings` - String parsing
- `strconv` - Number parsing
- `fmt` - Error formatting
- `sort` - Sorting value lists

## File Structure

```
lib/cron/
├── cron.go           # Main CronSchedule type and public API
├── parser.go         # Parsing logic
├── matcher.go        # Occurrence matching logic
└── cron_test.go      # Tests
```

## Testing Strategy

See `cron-parser-tests.md` for detailed test plan.

## Future Enhancements

Not included in initial implementation:
- Seconds field (6-field cron)
- Special strings (@daily, @hourly, etc.)
- Non-standard characters (L, W, #)
- Multiple timezone support per job
