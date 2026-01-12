# Cron Parser Test Plan

## Test Categories

### 1. Parse - Valid Expressions

#### 1.1 Basic Wildcards
- `* * * * *` - Every minute
- `0 * * * *` - Every hour at minute 0
- `0 0 * * *` - Every day at midnight
- `0 0 * * 0` - Every Sunday at midnight
- `0 0 1 * *` - First day of every month at midnight

#### 1.2 Single Values
- `15 10 5 6 3` - Specific time: June 5th at 10:15, if it's a Wednesday
- `0 0 1 1 *` - January 1st at midnight
- `30 14 * * *` - Every day at 14:30

#### 1.3 Lists (Comma-separated)
- `0,30 * * * *` - Every hour at 0 and 30 minutes
- `0 9,12,15,18 * * *` - At 9 AM, noon, 3 PM, 6 PM
- `0 0 1,15 * *` - Twice a month: 1st and 15th
- `0 0 * * 1,3,5` - Monday, Wednesday, Friday at midnight
- `0 0 * 1,6,12 *` - First day of Jan, Jun, Dec

#### 1.4 Ranges (Hyphen)
- `0-59 * * * *` - Every minute (equivalent to `*`)
- `0 9-17 * * *` - Every hour from 9 AM to 5 PM
- `0 0 * * 1-5` - Weekdays at midnight
- `0 0 1-7 * *` - First week of every month at midnight
- `0 0 * 1-3 *` - Every day in Q1 (Jan-Mar)

#### 1.5 Steps (Slash)
- `*/5 * * * *` - Every 5 minutes
- `0 */2 * * *` - Every 2 hours
- `0 0 */3 * *` - Every 3 days
- `0 0 * */2 *` - Every 2 months (Jan, Mar, May, etc.)
- `0 0 * * */2` - Every 2 days of week (Sun, Tue, Thu, Sat)
- `5-59/10 * * * *` - Minutes 5, 15, 25, 35, 45, 55
- `0 2-22/4 * * *` - Hours 2, 6, 10, 14, 18, 22

#### 1.6 Complex Combinations
- `*/15 9-17 * * 1-5` - Every 15 minutes, 9 AM-5 PM, weekdays
- `0,30 8-18 * * *` - Every 30 minutes from 8 AM to 6 PM
- `0 0 1,15 * 1` - 1st and 15th of month, plus every Monday at midnight
- `0 */6 * * *` - Every 6 hours (0:00, 6:00, 12:00, 18:00)

### 2. Parse - Invalid Expressions

#### 2.1 Format Errors
- `` (empty string)
- `* * * *` (only 4 fields)
- `* * * * * *` (6 fields)
- `*  * * * *` (extra whitespace should still work - strict format means single space)

#### 2.2 Syntax Errors
- `* * * * x` (non-numeric)
- `60 * * * *` (minute out of range)
- `* 24 * * *` (hour out of range)
- `* * 32 * *` (day out of range)
- `* * 0 * *` (day 0 invalid, must be 1-31)
- `* * * 13 * *` (month out of range)
- `* * * 0 *` (month 0 invalid, must be 1-12)
- `* * * * 7` (day-of-week out of range)
- `5-2 * * * *` (invalid range, start > end)
- `*/0 * * * *` (step of 0)
- `*/ * * * *` (incomplete step)
- `- * * * *` (incomplete range)
- `, * * * *` (incomplete list)
- `1,2, * * * *` (trailing comma)
- `1,,2 * * * *` (double comma)

#### 2.3 Impossible Dates
- `0 0 31 2 *` (Feb 31st doesn't exist)
- `0 0 30 2 *` (Feb 30th doesn't exist)
- `0 0 31 4 *` (April 31st doesn't exist)
- `0 0 31 6 *` (June 31st doesn't exist)
- `0 0 31 9 *` (September 31st doesn't exist)
- `0 0 31 11 *` (November 31st doesn't exist)

### 3. Next() - Calculate Next N Occurrences

#### 3.1 Every Minute
- Expression: `* * * * *`
- After: `2024-01-01 10:30:45`
- Count: 3
- Expected: `[2024-01-01 10:31:00, 2024-01-01 10:32:00, 2024-01-01 10:33:00]`

#### 3.2 Every Hour
- Expression: `0 * * * *`
- After: `2024-01-01 10:30:45`
- Count: 3
- Expected: `[2024-01-01 11:00:00, 2024-01-01 12:00:00, 2024-01-01 13:00:00]`

#### 3.3 Daily at Specific Time
- Expression: `30 14 * * *`
- After: `2024-01-01 10:00:00`
- Count: 3
- Expected: `[2024-01-01 14:30:00, 2024-01-02 14:30:00, 2024-01-03 14:30:00]`

#### 3.4 Daily at Specific Time (After Target Time)
- Expression: `30 14 * * *`
- After: `2024-01-01 15:00:00`
- Count: 2
- Expected: `[2024-01-02 14:30:00, 2024-01-03 14:30:00]`

#### 3.5 Weekdays Only
- Expression: `0 9 * * 1-5`
- After: `2024-01-01 00:00:00` (Monday)
- Count: 5
- Expected: Mon-Fri of that week at 9 AM

#### 3.6 Specific Days of Month
- Expression: `0 0 1,15 * *`
- After: `2024-01-01 00:00:00`
- Count: 4
- Expected: `[2024-01-01 00:00:00, 2024-01-15 00:00:00, 2024-02-01 00:00:00, 2024-02-15 00:00:00]`

#### 3.7 Month Boundaries
- Expression: `0 0 * * *`
- After: `2024-01-30 10:00:00`
- Count: 3
- Expected: `[2024-01-31 00:00:00, 2024-02-01 00:00:00, 2024-02-02 00:00:00]`

#### 3.8 Year Boundaries
- Expression: `0 0 * * *`
- After: `2024-12-30 10:00:00`
- Count: 3
- Expected: `[2024-12-31 00:00:00, 2025-01-01 00:00:00, 2025-01-02 00:00:00]`

#### 3.9 Leap Year - Feb 29
- Expression: `0 0 29 2 *`
- After: `2024-01-01 00:00:00`
- Count: 1
- Expected: `[2024-02-29 00:00:00]` (2024 is a leap year)

- Expression: `0 0 29 2 *`
- After: `2025-01-01 00:00:00`
- Count: 1
- Expected: `[2028-02-29 00:00:00]` (next leap year)

#### 3.10 Last Day of Month
- Expression: `0 0 31 * *`
- After: `2024-01-01 00:00:00`
- Count: 7
- Expected: Only months with 31 days (Jan, Mar, May, Jul, Aug, Oct, Dec)

### 4. Between() - Calculate Occurrences in Window

#### 4.1 Every Minute in 1 Hour Window
- Expression: `* * * * *`
- Start: `2024-01-01 10:00:00`
- End: `2024-01-01 11:00:00`
- Expected: 60 occurrences (10:00 through 10:59)

#### 4.2 Every Hour in 1 Day Window
- Expression: `0 * * * *`
- Start: `2024-01-01 00:00:00`
- End: `2024-01-02 00:00:00`
- Expected: 24 occurrences

#### 4.3 Daily in 1 Week Window
- Expression: `0 0 * * *`
- Start: `2024-01-01 00:00:00`
- End: `2024-01-08 00:00:00`
- Expected: 7 occurrences

#### 4.4 Weekdays in 2 Week Window
- Expression: `0 9 * * 1-5`
- Start: `2024-01-01 00:00:00` (Monday)
- End: `2024-01-15 00:00:00`
- Expected: 10 occurrences (2 weeks × 5 weekdays)

#### 4.5 Empty Result - No Matches
- Expression: `0 0 31 2 *` (Feb 31st - already tested as parse error)
- Use: `0 0 29 2 *` in non-leap year
- Start: `2025-02-01 00:00:00`
- End: `2025-03-01 00:00:00`
- Expected: 0 occurrences (2025 is not a leap year)

#### 4.6 Window Boundaries - Inclusive Start, Exclusive End
- Expression: `0 * * * *`
- Start: `2024-01-01 10:00:00`
- End: `2024-01-01 13:00:00`
- Expected: `[10:00, 11:00, 12:00]` (3 occurrences, end is exclusive)

#### 4.7 Large Window - Every Minute for 1 Week
- Expression: `* * * * *`
- Start: `2024-01-01 00:00:00`
- End: `2024-01-08 00:00:00`
- Expected: 10,080 occurrences (7 days × 24 hours × 60 minutes)

#### 4.8 Sparse Schedule - Monthly
- Expression: `0 0 1 * *`
- Start: `2024-01-01 00:00:00`
- End: `2024-12-31 23:59:59`
- Expected: 12 occurrences (first of each month)

### 5. Day-of-Month vs Day-of-Week Logic

#### 5.1 Only Day-of-Month Restricted
- Expression: `0 0 15 * *` (15th of every month)
- After: `2024-01-01 00:00:00`
- Count: 3
- Expected: 15th of Jan, Feb, Mar regardless of day-of-week

#### 5.2 Only Day-of-Week Restricted
- Expression: `0 0 * * 1` (Every Monday)
- After: `2024-01-01 00:00:00`
- Count: 3
- Expected: Next 3 Mondays, regardless of date

#### 5.3 Both Restricted - OR Logic
- Expression: `0 0 1 * 1` (1st of month OR Monday)
- After: `2024-01-01 00:00:00` (Monday the 1st)
- Count: 5
- Expected: Should include:
  - 2024-01-01 (Mon, 1st) - matches both
  - 2024-01-08 (Mon) - matches day-of-week
  - 2024-01-15 (Mon) - matches day-of-week
  - 2024-01-22 (Mon) - matches day-of-week
  - 2024-01-29 (Mon) - matches day-of-week
  - NOTE: 2024-02-01 (Thu, 1st) - would also match but later

- Expression: `0 0 15 * 5` (15th of month OR Friday)
- Start: `2024-01-01 00:00:00`
- End: `2024-02-01 00:00:00`
- Expected: Should include every Friday AND the 15th of January

#### 5.4 Both Unrestricted
- Expression: `0 0 * * *` (Every day)
- After: `2024-01-01 00:00:00`
- Count: 3
- Expected: Jan 1, 2, 3

### 6. Edge Cases

#### 6.1 Midnight Boundary
- Expression: `0 0 * * *`
- After: `2024-01-01 23:59:59`
- Count: 1
- Expected: `[2024-01-02 00:00:00]`

#### 6.2 Time Truncation
- Expression: `30 14 * * *`
- After: `2024-01-01 14:30:30.500` (has seconds and subseconds)
- Count: 1
- Expected: `[2024-01-01 14:30:00]` (truncated to minute)

#### 6.3 Already at Scheduled Time
- Expression: `30 14 * * *`
- After: `2024-01-01 14:30:00` (exactly at scheduled time)
- Count: 1
- Expected: `[2024-01-02 14:30:00]` (next occurrence, not current)

#### 6.4 Step That Doesn't Align
- Expression: `*/7 * * * *` (every 7 minutes)
- After: `2024-01-01 10:00:00`
- Count: 5
- Expected: `[10:00, 10:07, 10:14, 10:21, 10:28]`

#### 6.5 End of Month with 30 Days
- Expression: `0 0 31 * *`
- Start: `2024-04-01 00:00:00`
- End: `2024-05-01 00:00:00`
- Expected: 0 occurrences (April has only 30 days)

#### 6.6 Between with Start After End
- Expression: `* * * * *`
- Start: `2024-01-02 00:00:00`
- End: `2024-01-01 00:00:00`
- Expected: 0 occurrences (invalid window)

#### 6.7 Between with Start Equal to End
- Expression: `* * * * *`
- Start: `2024-01-01 10:00:00`
- End: `2024-01-01 10:00:00`
- Expected: 0 occurrences (empty window)

### 7. Timezone Handling

#### 7.1 UTC Timezone
- Expression: `0 0 * * *`
- After: `2024-01-01 10:00:00 UTC`
- Count: 1
- Expected: `[2024-01-02 00:00:00 UTC]`

#### 7.2 Different Timezone (EST)
- Expression: `0 0 * * *` (midnight)
- After: `2024-01-01 10:00:00 EST` (which is 15:00 UTC)
- Count: 1
- Expected: `[2024-01-02 00:00:00 EST]` (which is 05:00 UTC)

#### 7.3 Timezone with DST Transition
- Test during DST transitions (spring forward, fall back)
- Ensure cron times are evaluated correctly in local time
- This is complex - may defer to future testing

## Test Organization

```
lib/cron/cron_test.go

TestParse_Valid
TestParse_Invalid
TestNext_Simple
TestNext_EdgeCases
TestBetween_Simple
TestBetween_EdgeCases
TestDayLogic_DOMOnly
TestDayLogic_DOWOnly
TestDayLogic_Both
TestTimezone_UTC
TestTimezone_NonUTC
```

## Benchmarks

```
BenchmarkParse
BenchmarkNext_EveryMinute
BenchmarkNext_Daily
BenchmarkBetween_OneWeek
BenchmarkBetween_OneYear
```

## Test Data Helpers

Create helper functions for common test scenarios:
- `mustParse(t *testing.T, expr string) *CronSchedule`
- `assertTimes(t *testing.T, expected, actual []time.Time)`
- `makeTime(year, month, day, hour, minute int) time.Time`

## Coverage Goals

- 100% coverage of public API (Parse, Next, Between)
- 100% coverage of error paths
- All edge cases documented and tested
