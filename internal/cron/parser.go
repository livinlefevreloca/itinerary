package cron

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// parse parses a cron expression into a CronSchedule
func parse(expr string) (*CronSchedule, error) {
	// Split on whitespace
	fields := strings.Fields(expr)

	// Verify exactly 5 fields
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 fields, got %d", len(fields))
	}

	// Parse each field
	minutes, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}

	hours, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}

	daysOfMonth, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-month field: %w", err)
	}

	months, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}

	daysOfWeek, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid day-of-week field: %w", err)
	}

	// Validate impossible dates
	if err := validateImpossibleDates(daysOfMonth, months); err != nil {
		return nil, err
	}

	return &CronSchedule{
		minutes:     minutes,
		hours:       hours,
		daysOfMonth: daysOfMonth,
		months:      months,
		daysOfWeek:  daysOfWeek,
		original:    expr,
	}, nil
}

// parseField parses a single cron field
func parseField(field string, min, max int) ([]int, error) {
	if field == "" {
		return nil, fmt.Errorf("empty field")
	}

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

// expandRange returns all values from min to max inclusive
func expandRange(min, max int) []int {
	result := make([]int, max-min+1)
	for i := range result {
		result[i] = min + i
	}
	return result
}

// parseStep parses step expressions like */5 or 1-10/2
func parseStep(field string, min, max int) ([]int, error) {
	parts := strings.Split(field, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid step syntax")
	}

	// Parse step value
	step, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid step value: %w", err)
	}
	if step <= 0 {
		return nil, fmt.Errorf("step must be greater than 0")
	}

	// Parse range part (either * or a range like 1-10)
	var rangeVals []int
	if parts[0] == "*" {
		rangeVals = expandRange(min, max)
	} else if strings.Contains(parts[0], "-") {
		rangeVals, err = parseRange(parts[0], min, max)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid step range")
	}

	result := []int{}
	for i := 0; i < len(rangeVals); i += step {
		result = append(result, rangeVals[i])
	}

	return result, nil
}

// parseList parses comma-separated values like 1,3,5
func parseList(field string, min, max int) ([]int, error) {
	parts := strings.Split(field, ",")
	result := []int{}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("empty value in list")
		}

		// Each part can be a single value or a range
		var vals []int
		var err error
		if strings.Contains(part, "-") {
			vals, err = parseRange(part, min, max)
		} else {
			vals, err = parseSingle(part, min, max)
		}
		if err != nil {
			return nil, err
		}

		result = append(result, vals...)
	}

	// Sort and deduplicate
	sort.Ints(result)
	return deduplicate(result), nil
}

// parseRange parses a range like 1-5
func parseRange(field string, min, max int) ([]int, error) {
	parts := strings.Split(field, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range syntax")
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid range start: %w", err)
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid range end: %w", err)
	}

	if start < min || start > max {
		return nil, fmt.Errorf("range start %d out of bounds [%d, %d]", start, min, max)
	}

	if end < min || end > max {
		return nil, fmt.Errorf("range end %d out of bounds [%d, %d]", end, min, max)
	}

	if start > end {
		return nil, fmt.Errorf("invalid range: start %d > end %d", start, end)
	}

	return expandRange(start, end), nil
}

// parseSingle parses a single integer value
func parseSingle(field string, min, max int) ([]int, error) {
	val, err := strconv.Atoi(field)
	if err != nil {
		return nil, fmt.Errorf("invalid value: %w", err)
	}

	if val < min || val > max {
		return nil, fmt.Errorf("value %d out of bounds [%d, %d]", val, min, max)
	}

	return []int{val}, nil
}

// deduplicate removes duplicate values from a sorted slice
func deduplicate(vals []int) []int {
	if len(vals) == 0 {
		return vals
	}

	result := []int{vals[0]}
	for i := 1; i < len(vals); i++ {
		if vals[i] != vals[i-1] {
			result = append(result, vals[i])
		}
	}
	return result
}

// validateImpossibleDates checks for impossible date combinations
// Only error if the schedule can never run (all months have no valid days)
func validateImpossibleDates(daysOfMonth, months []int) error {
	// Check if at least one month has at least one valid day
	for _, month := range months {
		maxDay := daysInMonth(month)
		for _, day := range daysOfMonth {
			if day <= maxDay {
				// Found at least one valid day in at least one month
				return nil
			}
		}
	}
	// No valid day/month combinations found - schedule will never run
	return fmt.Errorf("impossible date: no valid days exist for specified days %v in months %v", daysOfMonth, months)
}

// daysInMonth returns the maximum number of days in a given month
func daysInMonth(month int) int {
	switch month {
	case 2: // February
		return 29 // Allow leap year
	case 4, 6, 9, 11: // Apr, Jun, Sep, Nov
		return 30
	default: // Jan, Mar, May, Jul, Aug, Oct, Dec
		return 31
	}
}

// isLeapYear checks if a year is a leap year
func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// isValidDate checks if a date is valid (handles Feb 29 in non-leap years)
func isValidDate(year, month, day int) bool {
	if month == 2 && day == 29 {
		return isLeapYear(year)
	}
	return day <= daysInMonth(month)
}
