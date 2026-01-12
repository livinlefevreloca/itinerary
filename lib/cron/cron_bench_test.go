package cron

import (
	"testing"
	"time"
)

// Benchmarks

func BenchmarkParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Parse("*/5 9-17 * * 1-5")
	}
}

func BenchmarkNext_EveryMinute(b *testing.B) {
	cs, _ := Parse("* * * * *")
	after := makeTime(2024, 1, 1, 0, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cs.Next(after, 100)
	}
}

func BenchmarkNext_Daily(b *testing.B) {
	cs, _ := Parse("0 0 * * *")
	after := makeTime(2024, 1, 1, 0, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cs.Next(after, 365)
	}
}

func BenchmarkBetween_OneWeek(b *testing.B) {
	cs, _ := Parse("0 * * * *")
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2024, 1, 8, 0, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cs.Between(start, end)
	}
}

func BenchmarkBetween_OneYear(b *testing.B) {
	cs, _ := Parse("0 0 * * *")
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2025, 1, 1, 0, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cs.Between(start, end)
	}
}

// Comprehensive parsing benchmarks at scale

var testCronExpressions = []string{
	"* * * * *",           // Every minute
	"0 * * * *",           // Every hour
	"0 0 * * *",           // Daily
	"*/5 * * * *",         // Every 5 minutes
	"*/15 9-17 * * 1-5",   // Business hours
	"0 0 1 * *",           // Monthly
	"0 0 1 1 *",           // Yearly
	"30 2 * * 0",          // Weekly on Sunday
	"0,30 * * * *",        // Twice an hour
	"0 0,12 * * *",        // Twice a day
	"0 9-17 * * 1-5",      // Hourly on weekdays
	"*/10 8-18 * * 1-5",   // Every 10 min business hours
	"0 0 1,15 * *",        // Twice a month
	"0 0 * * 1",           // Every Monday
	"0 0 31 * *",          // Last day (where applicable)
	"0 0 * 1,6,12 *",      // Quarterly
	"15 10 * * *",         // Daily at 10:15
	"0 */4 * * *",         // Every 4 hours
	"30 3 * * *",          // Daily at 3:30am
	"0 0 * * 0,6",         // Weekends only
}

func BenchmarkParse_10_Schedules(b *testing.B) {
	expressions := testCronExpressions[:10]
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			_, _ = Parse(expr)
		}
	}
}

func BenchmarkParse_100_Schedules(b *testing.B) {
	// Repeat expressions to get 100
	expressions := make([]string, 100)
	for i := 0; i < 100; i++ {
		expressions[i] = testCronExpressions[i%len(testCronExpressions)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			_, _ = Parse(expr)
		}
	}
}

func BenchmarkParse_1000_Schedules(b *testing.B) {
	expressions := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		expressions[i] = testCronExpressions[i%len(testCronExpressions)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			_, _ = Parse(expr)
		}
	}
}

func BenchmarkParse_10000_Schedules(b *testing.B) {
	expressions := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		expressions[i] = testCronExpressions[i%len(testCronExpressions)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			_, _ = Parse(expr)
		}
	}
}

func BenchmarkParse_100000_Schedules(b *testing.B) {
	expressions := make([]string, 100000)
	for i := 0; i < 100000; i++ {
		expressions[i] = testCronExpressions[i%len(testCronExpressions)]
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range expressions {
			_, _ = Parse(expr)
		}
	}
}

// Comprehensive matching benchmarks at scale

func preParsedSchedules(count int) []*CronSchedule {
	schedules := make([]*CronSchedule, count)
	for i := 0; i < count; i++ {
		schedules[i], _ = Parse(testCronExpressions[i%len(testCronExpressions)])
	}
	return schedules
}

// 1 hour window matching benchmarks

func BenchmarkMatch_10_Schedules_1Hour(b *testing.B) {
	schedules := preParsedSchedules(10)
	start := makeTime(2024, 1, 1, 10, 0)
	end := start.Add(1 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_100_Schedules_1Hour(b *testing.B) {
	schedules := preParsedSchedules(100)
	start := makeTime(2024, 1, 1, 10, 0)
	end := start.Add(1 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_1000_Schedules_1Hour(b *testing.B) {
	schedules := preParsedSchedules(1000)
	start := makeTime(2024, 1, 1, 10, 0)
	end := start.Add(1 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_10000_Schedules_1Hour(b *testing.B) {
	schedules := preParsedSchedules(10000)
	start := makeTime(2024, 1, 1, 10, 0)
	end := start.Add(1 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

// 1 day window matching benchmarks

func BenchmarkMatch_10_Schedules_1Day(b *testing.B) {
	schedules := preParsedSchedules(10)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_100_Schedules_1Day(b *testing.B) {
	schedules := preParsedSchedules(100)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_1000_Schedules_1Day(b *testing.B) {
	schedules := preParsedSchedules(1000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_10000_Schedules_1Day(b *testing.B) {
	schedules := preParsedSchedules(10000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

// 1 week window matching benchmarks

func BenchmarkMatch_10_Schedules_1Week(b *testing.B) {
	schedules := preParsedSchedules(10)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(7 * 24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_100_Schedules_1Week(b *testing.B) {
	schedules := preParsedSchedules(100)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(7 * 24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_1000_Schedules_1Week(b *testing.B) {
	schedules := preParsedSchedules(1000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(7 * 24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_10000_Schedules_1Week(b *testing.B) {
	schedules := preParsedSchedules(10000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := start.Add(7 * 24 * time.Hour)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

// 1 month window matching benchmarks

func BenchmarkMatch_10_Schedules_1Month(b *testing.B) {
	schedules := preParsedSchedules(10)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2024, 2, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_100_Schedules_1Month(b *testing.B) {
	schedules := preParsedSchedules(100)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2024, 2, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_1000_Schedules_1Month(b *testing.B) {
	schedules := preParsedSchedules(1000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2024, 2, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_10000_Schedules_1Month(b *testing.B) {
	schedules := preParsedSchedules(10000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2024, 2, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

// 1 year window matching benchmarks

func BenchmarkMatch_10_Schedules_1Year(b *testing.B) {
	schedules := preParsedSchedules(10)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2025, 1, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_100_Schedules_1Year(b *testing.B) {
	schedules := preParsedSchedules(100)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2025, 1, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_1000_Schedules_1Year(b *testing.B) {
	schedules := preParsedSchedules(1000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2025, 1, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}

func BenchmarkMatch_10000_Schedules_1Year(b *testing.B) {
	schedules := preParsedSchedules(10000)
	start := makeTime(2024, 1, 1, 0, 0)
	end := makeTime(2025, 1, 1, 0, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		current := start
		for current.Before(end) {
			for _, cs := range schedules {
				_ = cs.matches(current)
			}
			current = current.Add(time.Minute)
		}
	}
}
