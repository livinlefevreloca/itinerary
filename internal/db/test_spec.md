# Database Layer Test Suite

## Overview

Comprehensive test suite for the database layer that validates connection management, query operations, transaction handling, and multi-database compatibility.

## Test Organization

Tests are organized into the following categories:

1. **Connection Tests** - Database connection and configuration
2. **Schema Tests** - Table structure and constraints
3. **Query Tests** - CRUD operations for all entity types
4. **Transaction Tests** - Transaction management and rollback
5. **Error Handling Tests** - Error detection and classification
6. **Multi-Database Tests** - Compatibility across PostgreSQL, MySQL, SQLite

## Test Fixtures

### Test Database Helper

```go
// NewTestDB creates an in-memory SQLite database for testing
func NewTestDB(t *testing.T) *DB {
    t.Helper()

    db, err := Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("failed to create test database: %v", err)
    }

    t.Cleanup(func() {
        db.Close()
    })

    return db
}

// SeedTestData populates the database with test data
func SeedTestData(t *testing.T, db *DB) {
    t.Helper()

    // Insert test jobs
    jobs := []Job{
        {
            ID:       "job-1",
            Name:     "Test Job 1",
            Schedule: "0 0 * * *",
            PodSpec:  `{"image": "test:latest"}`,
        },
        {
            ID:       "job-2",
            Name:     "Test Job 2",
            Schedule: "*/5 * * * *",
            PodSpec:  `{"image": "test2:latest"}`,
        },
    }

    for _, job := range jobs {
        if err := db.CreateJob(&job); err != nil {
            t.Fatalf("failed to seed job: %v", err)
        }
    }
}
```

### Test Data Helpers

```go
// MakeTestJob creates a job with default test values
func MakeTestJob(id string) *Job

// MakeTestJobRun creates a job run with default test values
func MakeTestJobRun(jobID string, scheduledAt time.Time) *JobRun

// MakeTestConstraint creates a constraint with default test values
func MakeTestConstraint(id string) *Constraint

// MakeTestAction creates an action with default test values
func MakeTestAction(id string) *Action

// MakeTestConstraintViolation creates a constraint violation with default test values
func MakeTestConstraintViolation(runID string, constraintName string) *ConstraintViolation
```

## Connection Tests

### TestOpen

```go
func TestOpen(t *testing.T) {
    tests := []struct {
        name       string
        driver     string
        dsn        string
        wantErr    bool
        errContains string
    }{
        {
            name:    "sqlite in-memory",
            driver:  "sqlite3",
            dsn:     ":memory:",
            wantErr: false,
        },
        {
            name:       "invalid driver",
            driver:     "invalid",
            dsn:        "",
            wantErr:    true,
            errContains: "unknown driver",
        },
        {
            name:       "empty dsn",
            driver:     "sqlite3",
            dsn:        "",
            wantErr:    false, // sqlite allows empty DSN
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            db, err := Open(tt.driver, tt.dsn)
            if tt.wantErr {
                if err == nil {
                    t.Errorf("expected error containing %q, got nil", tt.errContains)
                }
                return
            }

            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            defer db.Close()

            // Verify driver
            if db.Driver() != tt.driver {
                t.Errorf("driver = %q, want %q", db.Driver(), tt.driver)
            }
        })
    }
}
```

### TestOpenWithConfig

```go
func TestOpenWithConfig(t *testing.T) {
    config := Config{
        Driver:          "sqlite3",
        DSN:             ":memory:",
        MaxOpenConns:    10,
        MaxIdleConns:    5,
        ConnMaxLifetime: 5 * time.Minute,
        ConnMaxIdleTime: 2 * time.Minute,
    }

    db, err := OpenWithConfig(config)
    if err != nil {
        t.Fatalf("failed to open database: %v", err)
    }
    defer db.Close()

    // Verify connection parameters were applied
    stats := db.Stats()
    if stats.MaxOpenConnections != 10 {
        t.Errorf("MaxOpenConnections = %d, want 10", stats.MaxOpenConnections)
    }
}
```

### TestPing

```go
func TestPing(t *testing.T) {
    db := NewTestDB(t)

    if err := db.Ping(); err != nil {
        t.Errorf("Ping failed: %v", err)
    }
}
```

### TestClose

```go
func TestClose(t *testing.T) {
    db, err := Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("failed to open: %v", err)
    }

    if err := db.Close(); err != nil {
        t.Errorf("Close failed: %v", err)
    }

    // Verify database is closed
    if err := db.Ping(); err == nil {
        t.Error("expected Ping to fail after Close")
    }
}
```

## Job Tests

### TestCreateJob

```go
func TestCreateJob(t *testing.T) {
    db := NewTestDB(t)

    job := &Job{
        ID:       "test-job-1",
        Name:     "Test Job",
        Schedule: "0 0 * * *",
        PodSpec:  `{"image": "test:latest"}`,
    }

    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    // Verify job was created with timestamps
    if job.CreatedAt.IsZero() {
        t.Error("CreatedAt was not set")
    }
    if job.UpdatedAt.IsZero() {
        t.Error("UpdatedAt was not set")
    }
}
```

### TestCreateJob_Duplicate

```go
func TestCreateJob_Duplicate(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("test-job-1")

    // Create first time - should succeed
    if err := db.CreateJob(job); err != nil {
        t.Fatalf("first CreateJob failed: %v", err)
    }

    // Create second time - should fail
    err := db.CreateJob(job)
    if err == nil {
        t.Fatal("expected duplicate error, got nil")
    }

    if !IsDuplicate(err) {
        t.Errorf("expected IsDuplicate(err) = true, got false: %v", err)
    }
}
```

### TestGetJob

```go
func TestGetJob(t *testing.T) {
    db := NewTestDB(t)

    original := MakeTestJob("test-job-1")
    if err := db.CreateJob(original); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    retrieved, err := db.GetJob("test-job-1")
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }

    // Verify all fields match
    if retrieved.ID != original.ID {
        t.Errorf("ID = %q, want %q", retrieved.ID, original.ID)
    }
    if retrieved.Name != original.Name {
        t.Errorf("Name = %q, want %q", retrieved.Name, original.Name)
    }
    if retrieved.Schedule != original.Schedule {
        t.Errorf("Schedule = %q, want %q", retrieved.Schedule, original.Schedule)
    }
    if retrieved.PodSpec != original.PodSpec {
        t.Errorf("PodSpec = %q, want %q", retrieved.PodSpec, original.PodSpec)
    }
}
```

### TestGetJob_NotFound

```go
func TestGetJob_NotFound(t *testing.T) {
    db := NewTestDB(t)

    job, err := db.GetJob("nonexistent")
    if err == nil {
        t.Fatal("expected error for nonexistent job, got nil")
    }

    if !IsNotFound(err) {
        t.Errorf("expected IsNotFound(err) = true, got false: %v", err)
    }

    if job != nil {
        t.Errorf("expected nil job, got %v", job)
    }
}
```

### TestGetAllJobs

```go
func TestGetAllJobs(t *testing.T) {
    db := NewTestDB(t)

    // Create multiple jobs
    jobs := []*Job{
        MakeTestJob("job-1"),
        MakeTestJob("job-2"),
        MakeTestJob("job-3"),
    }

    for _, job := range jobs {
        if err := db.CreateJob(job); err != nil {
            t.Fatalf("CreateJob failed: %v", err)
        }
    }

    retrieved, err := db.GetAllJobs()
    if err != nil {
        t.Fatalf("GetAllJobs failed: %v", err)
    }

    if len(retrieved) != len(jobs) {
        t.Errorf("got %d jobs, want %d", len(retrieved), len(jobs))
    }
}
```

### TestGetAllJobs_Empty

```go
func TestGetAllJobs_Empty(t *testing.T) {
    db := NewTestDB(t)

    jobs, err := db.GetAllJobs()
    if err != nil {
        t.Fatalf("GetAllJobs failed: %v", err)
    }

    if len(jobs) != 0 {
        t.Errorf("expected empty slice, got %d jobs", len(jobs))
    }
}
```

### TestUpdateJob

```go
func TestUpdateJob(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("test-job")
    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    // Wait to ensure UpdatedAt changes
    time.Sleep(10 * time.Millisecond)

    // Modify job
    job.Name = "Updated Name"
    job.Schedule = "*/10 * * * *"

    if err := db.UpdateJob(job); err != nil {
        t.Fatalf("UpdateJob failed: %v", err)
    }

    // Retrieve and verify
    updated, err := db.GetJob("test-job")
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }

    if updated.Name != "Updated Name" {
        t.Errorf("Name = %q, want %q", updated.Name, "Updated Name")
    }

    if !updated.UpdatedAt.After(job.CreatedAt) {
        t.Error("UpdatedAt should be after CreatedAt")
    }
}
```

### TestUpdateJob_NotFound

```go
func TestUpdateJob_NotFound(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("nonexistent")
    err := db.UpdateJob(job)

    if err == nil {
        t.Fatal("expected error for nonexistent job, got nil")
    }

    if !IsNotFound(err) {
        t.Errorf("expected IsNotFound(err) = true, got false: %v", err)
    }
}
```

### TestDeleteJob

```go
func TestDeleteJob(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("test-job")
    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    if err := db.DeleteJob("test-job"); err != nil {
        t.Fatalf("DeleteJob failed: %v", err)
    }

    // Verify deletion
    _, err := db.GetJob("test-job")
    if !IsNotFound(err) {
        t.Error("expected job to be deleted")
    }
}
```

### TestDeleteJob_NotFound

```go
func TestDeleteJob_NotFound(t *testing.T) {
    db := NewTestDB(t)

    err := db.DeleteJob("nonexistent")
    if err == nil {
        t.Fatal("expected error for nonexistent job, got nil")
    }

    if !IsNotFound(err) {
        t.Errorf("expected IsNotFound(err) = true, got false: %v", err)
    }
}
```

### TestDeleteJob_CascadeToRuns

```go
func TestDeleteJob_CascadeToRuns(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("test-job")
    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    // Create job runs
    run := MakeTestJobRun("test-job", time.Now())
    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    // Delete job
    if err := db.DeleteJob("test-job"); err != nil {
        t.Fatalf("DeleteJob failed: %v", err)
    }

    // Verify runs were cascaded
    runs, err := db.GetJobRuns("test-job", 100)
    if err != nil {
        t.Fatalf("GetJobRuns failed: %v", err)
    }

    if len(runs) != 0 {
        t.Errorf("expected 0 runs after cascade delete, got %d", len(runs))
    }
}
```

## Job Run Tests

### TestCreateJobRun

```go
func TestCreateJobRun(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("test-job")
    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    scheduledAt := time.Now()
    run := &JobRun{
        JobID:       "test-job",
        RunID:       "run-123",
        ScheduledAt: scheduledAt,
        Status:      "pending",
    }

    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }
}
```

### TestCreateJobRun_Duplicate

```go
func TestCreateJobRun_Duplicate(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    scheduledAt := time.Now()
    run := MakeTestJobRun("test-job", scheduledAt)

    // Create first time
    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("first CreateJobRun failed: %v", err)
    }

    // Create second time with same (job_id, scheduled_at)
    err := db.CreateJobRun(run)
    if err == nil {
        t.Fatal("expected duplicate error, got nil")
    }

    if !IsDuplicate(err) {
        t.Errorf("expected IsDuplicate(err) = true, got false: %v", err)
    }
}
```

### TestCreateJobRun_InvalidJobID

```go
func TestCreateJobRun_InvalidJobID(t *testing.T) {
    db := NewTestDB(t)

    run := MakeTestJobRun("nonexistent-job", time.Now())
    err := db.CreateJobRun(run)

    if err == nil {
        t.Fatal("expected foreign key error, got nil")
    }

    if !IsForeignKey(err) {
        t.Errorf("expected IsForeignKey(err) = true, got false: %v", err)
    }
}
```

### TestGetJobRun

```go
func TestGetJobRun(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    scheduledAt := time.Now().Truncate(time.Second)
    original := MakeTestJobRun("job-1", scheduledAt)

    if err := db.CreateJobRun(original); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    retrieved, err := db.GetJobRun("job-1", scheduledAt)
    if err != nil {
        t.Fatalf("GetJobRun failed: %v", err)
    }

    if retrieved.JobID != original.JobID {
        t.Errorf("JobID = %q, want %q", retrieved.JobID, original.JobID)
    }
    if retrieved.RunID != original.RunID {
        t.Errorf("RunID = %q, want %q", retrieved.RunID, original.RunID)
    }
    if !retrieved.ScheduledAt.Equal(original.ScheduledAt) {
        t.Errorf("ScheduledAt = %v, want %v", retrieved.ScheduledAt, original.ScheduledAt)
    }
}
```

### TestGetJobRunByRunID

```go
func TestGetJobRunByRunID(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    original := MakeTestJobRun("job-1", time.Now())
    original.RunID = "unique-run-id"

    if err := db.CreateJobRun(original); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    retrieved, err := db.GetJobRunByRunID("unique-run-id")
    if err != nil {
        t.Fatalf("GetJobRunByRunID failed: %v", err)
    }

    if retrieved.RunID != "unique-run-id" {
        t.Errorf("RunID = %q, want %q", retrieved.RunID, "unique-run-id")
    }
}
```

### TestGetJobRuns

```go
func TestGetJobRuns(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create multiple runs for the same job
    now := time.Now()
    for i := 0; i < 5; i++ {
        run := MakeTestJobRun("job-1", now.Add(time.Duration(i)*time.Minute))
        if err := db.CreateJobRun(run); err != nil {
            t.Fatalf("CreateJobRun %d failed: %v", i, err)
        }
    }

    runs, err := db.GetJobRuns("job-1", 100)
    if err != nil {
        t.Fatalf("GetJobRuns failed: %v", err)
    }

    if len(runs) != 5 {
        t.Errorf("got %d runs, want 5", len(runs))
    }

    // Verify ordered by scheduled_at DESC
    for i := 1; i < len(runs); i++ {
        if runs[i].ScheduledAt.After(runs[i-1].ScheduledAt) {
            t.Error("runs not ordered by scheduled_at DESC")
        }
    }
}
```

### TestGetJobRuns_WithLimit

```go
func TestGetJobRuns_WithLimit(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create 10 runs
    now := time.Now()
    for i := 0; i < 10; i++ {
        run := MakeTestJobRun("job-1", now.Add(time.Duration(i)*time.Minute))
        if err := db.CreateJobRun(run); err != nil {
            t.Fatalf("CreateJobRun %d failed: %v", i, err)
        }
    }

    // Request only 3
    runs, err := db.GetJobRuns("job-1", 3)
    if err != nil {
        t.Fatalf("GetJobRuns failed: %v", err)
    }

    if len(runs) != 3 {
        t.Errorf("got %d runs, want 3", len(runs))
    }
}
```

### TestUpdateJobRunStatus

```go
func TestUpdateJobRunStatus(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    run := MakeTestJobRun("job-1", time.Now())
    run.Status = "pending"

    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    if err := db.UpdateJobRunStatus(run.RunID, "running"); err != nil {
        t.Fatalf("UpdateJobRunStatus failed: %v", err)
    }

    // Verify status updated
    retrieved, err := db.GetJobRunByRunID(run.RunID)
    if err != nil {
        t.Fatalf("GetJobRunByRunID failed: %v", err)
    }

    if retrieved.Status != "running" {
        t.Errorf("Status = %q, want %q", retrieved.Status, "running")
    }
}
```

### TestCompleteJobRun_Success

```go
func TestCompleteJobRun_Success(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    run := MakeTestJobRun("job-1", time.Now())
    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    if err := db.CompleteJobRun(run.RunID, true, nil); err != nil {
        t.Fatalf("CompleteJobRun failed: %v", err)
    }

    // Verify completion
    retrieved, err := db.GetJobRunByRunID(run.RunID)
    if err != nil {
        t.Fatalf("GetJobRunByRunID failed: %v", err)
    }

    if retrieved.Status != "completed" {
        t.Errorf("Status = %q, want %q", retrieved.Status, "completed")
    }

    if retrieved.Success == nil || !*retrieved.Success {
        t.Error("Success should be true")
    }

    if retrieved.CompletedAt == nil {
        t.Error("CompletedAt should be set")
    }

    if retrieved.Error != nil {
        t.Errorf("Error should be nil, got %q", *retrieved.Error)
    }
}
```

### TestCompleteJobRun_Failure

```go
func TestCompleteJobRun_Failure(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    run := MakeTestJobRun("job-1", time.Now())
    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    errMsg := "pod failed with exit code 1"
    if err := db.CompleteJobRun(run.RunID, false, &errMsg); err != nil {
        t.Fatalf("CompleteJobRun failed: %v", err)
    }

    // Verify failure recorded
    retrieved, err := db.GetJobRunByRunID(run.RunID)
    if err != nil {
        t.Fatalf("GetJobRunByRunID failed: %v", err)
    }

    if retrieved.Status != "failed" {
        t.Errorf("Status = %q, want %q", retrieved.Status, "failed")
    }

    if retrieved.Success == nil || *retrieved.Success {
        t.Error("Success should be false")
    }

    if retrieved.Error == nil {
        t.Fatal("Error should be set")
    }

    if *retrieved.Error != errMsg {
        t.Errorf("Error = %q, want %q", *retrieved.Error, errMsg)
    }
}
```

## Transaction Tests

### TestBeginCommit

```go
func TestBeginCommit(t *testing.T) {
    db := NewTestDB(t)

    tx, err := db.Begin()
    if err != nil {
        t.Fatalf("Begin failed: %v", err)
    }

    // Create job within transaction
    job := MakeTestJob("test-job")
    if err := tx.CreateJob(job); err != nil {
        tx.Rollback()
        t.Fatalf("CreateJob failed: %v", err)
    }

    if err := tx.Commit(); err != nil {
        t.Fatalf("Commit failed: %v", err)
    }

    // Verify job exists after commit
    retrieved, err := db.GetJob("test-job")
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }

    if retrieved.ID != "test-job" {
        t.Errorf("Job not found after commit")
    }
}
```

### TestRollback

```go
func TestRollback(t *testing.T) {
    db := NewTestDB(t)

    tx, err := db.Begin()
    if err != nil {
        t.Fatalf("Begin failed: %v", err)
    }

    // Create job within transaction
    job := MakeTestJob("test-job")
    if err := tx.CreateJob(job); err != nil {
        tx.Rollback()
        t.Fatalf("CreateJob failed: %v", err)
    }

    // Rollback instead of commit
    if err := tx.Rollback(); err != nil {
        t.Fatalf("Rollback failed: %v", err)
    }

    // Verify job does not exist
    _, err = db.GetJob("test-job")
    if !IsNotFound(err) {
        t.Error("job should not exist after rollback")
    }
}
```

### TestWithTransaction_Success

```go
func TestWithTransaction_Success(t *testing.T) {
    db := NewTestDB(t)

    err := db.WithTransaction(func(tx *Tx) error {
        job := MakeTestJob("test-job")
        return tx.CreateJob(job)
    })

    if err != nil {
        t.Fatalf("WithTransaction failed: %v", err)
    }

    // Verify job exists
    _, err = db.GetJob("test-job")
    if err != nil {
        t.Errorf("job should exist after successful transaction: %v", err)
    }
}
```

### TestWithTransaction_Rollback

```go
func TestWithTransaction_Rollback(t *testing.T) {
    db := NewTestDB(t)

    testErr := errors.New("test error")

    err := db.WithTransaction(func(tx *Tx) error {
        job := MakeTestJob("test-job")
        if err := tx.CreateJob(job); err != nil {
            return err
        }

        // Trigger rollback by returning error
        return testErr
    })

    if err != testErr {
        t.Fatalf("expected testErr, got %v", err)
    }

    // Verify job does not exist
    _, err = db.GetJob("test-job")
    if !IsNotFound(err) {
        t.Error("job should not exist after rollback")
    }
}
```

### TestWithTransaction_Nested

```go
func TestWithTransaction_Nested(t *testing.T) {
    db := NewTestDB(t)

    // SQLite doesn't support nested transactions
    // This test verifies we get appropriate error
    err := db.WithTransaction(func(tx *Tx) error {
        return tx.WithTransaction(func(tx2 *Tx) error {
            job := MakeTestJob("test-job")
            return tx2.CreateJob(job)
        })
    })

    // Should error or handle gracefully
    if err == nil && db.Driver() == "sqlite3" {
        t.Error("expected error for nested transaction in SQLite")
    }
}
```

## Error Handling Tests

### TestIsNotFound

```go
func TestIsNotFound(t *testing.T) {
    db := NewTestDB(t)

    _, err := db.GetJob("nonexistent")

    if !IsNotFound(err) {
        t.Errorf("expected IsNotFound = true for nonexistent job")
    }
}
```

### TestIsDuplicate

```go
func TestIsDuplicate(t *testing.T) {
    db := NewTestDB(t)

    job := MakeTestJob("test-job")
    db.CreateJob(job)

    err := db.CreateJob(job)

    if !IsDuplicate(err) {
        t.Errorf("expected IsDuplicate = true for duplicate job")
    }
}
```

### TestIsForeignKey

```go
func TestIsForeignKey(t *testing.T) {
    db := NewTestDB(t)

    run := MakeTestJobRun("nonexistent-job", time.Now())
    err := db.CreateJobRun(run)

    if !IsForeignKey(err) {
        t.Errorf("expected IsForeignKey = true for invalid job_id")
    }
}
```

## Constraints Tests

### TestCreateConstraint

```go
func TestCreateConstraint(t *testing.T) {
    db := NewTestDB(t)

    maxConcurrentRuns := 5
    catchUp := true
    catchUpWindow := "5m"
    maxExpectedRunTime := "1h"
    maxAllowedRunTime := "2h"

    constraint := &Constraint{
        ID:                 "constraint-1",
        MaxConcurrentRuns:  &maxConcurrentRuns,
        CatchUp:            &catchUp,
        CatchUpWindow:      &catchUpWindow,
        MaxExpectedRunTime: &maxExpectedRunTime,
        MaxAllowedRunTime:  &maxAllowedRunTime,
    }

    if err := db.CreateConstraint(constraint); err != nil {
        t.Fatalf("CreateConstraint failed: %v", err)
    }

    if constraint.CreatedAt.IsZero() {
        t.Error("CreatedAt not set")
    }
    if constraint.UpdatedAt.IsZero() {
        t.Error("UpdatedAt not set")
    }
}
```

### TestGetConstraint

```go
func TestGetConstraint(t *testing.T) {
    db := NewTestDB(t)

    maxConcurrentRuns := 3
    original := &Constraint{
        ID:                "constraint-1",
        MaxConcurrentRuns: &maxConcurrentRuns,
    }

    if err := db.CreateConstraint(original); err != nil {
        t.Fatalf("CreateConstraint failed: %v", err)
    }

    retrieved, err := db.GetConstraint("constraint-1")
    if err != nil {
        t.Fatalf("GetConstraint failed: %v", err)
    }

    if retrieved.ID != original.ID {
        t.Errorf("ID = %q, want %q", retrieved.ID, original.ID)
    }
    if *retrieved.MaxConcurrentRuns != *original.MaxConcurrentRuns {
        t.Errorf("MaxConcurrentRuns = %d, want %d", *retrieved.MaxConcurrentRuns, *original.MaxConcurrentRuns)
    }
}
```

### TestUpdateConstraint

```go
func TestUpdateConstraint(t *testing.T) {
    db := NewTestDB(t)

    maxConcurrentRuns := 5
    constraint := &Constraint{
        ID:                "constraint-1",
        MaxConcurrentRuns: &maxConcurrentRuns,
    }

    if err := db.CreateConstraint(constraint); err != nil {
        t.Fatalf("CreateConstraint failed: %v", err)
    }

    // Update
    newMax := 10
    constraint.MaxConcurrentRuns = &newMax

    if err := db.UpdateConstraint(constraint); err != nil {
        t.Fatalf("UpdateConstraint failed: %v", err)
    }

    // Verify
    updated, err := db.GetConstraint("constraint-1")
    if err != nil {
        t.Fatalf("GetConstraint failed: %v", err)
    }

    if *updated.MaxConcurrentRuns != 10 {
        t.Errorf("MaxConcurrentRuns = %d, want 10", *updated.MaxConcurrentRuns)
    }
}
```

### TestDeleteConstraint

```go
func TestDeleteConstraint(t *testing.T) {
    db := NewTestDB(t)

    maxConcurrentRuns := 5
    constraint := &Constraint{
        ID:                "constraint-1",
        MaxConcurrentRuns: &maxConcurrentRuns,
    }

    if err := db.CreateConstraint(constraint); err != nil {
        t.Fatalf("CreateConstraint failed: %v", err)
    }

    if err := db.DeleteConstraint("constraint-1"); err != nil {
        t.Fatalf("DeleteConstraint failed: %v", err)
    }

    _, err := db.GetConstraint("constraint-1")
    if !IsNotFound(err) {
        t.Error("constraint should be deleted")
    }
}
```

## Actions Tests

### TestCreateAction

```go
func TestCreateAction(t *testing.T) {
    db := NewTestDB(t)

    config := `{"url": "https://hooks.slack.com/...", "channel": "#alerts"}`
    action := &Action{
        ID:         "action-1",
        Name:       "Notify Slack",
        ActionType: "webhook",
        Config:     &config,
    }

    if err := db.CreateAction(action); err != nil {
        t.Fatalf("CreateAction failed: %v", err)
    }

    if action.CreatedAt.IsZero() {
        t.Error("CreatedAt not set")
    }
}
```

### TestGetAction

```go
func TestGetAction(t *testing.T) {
    db := NewTestDB(t)

    config := `{"jobID": "cleanup-job"}`
    original := &Action{
        ID:         "action-1",
        Name:       "Kick off cleanup",
        ActionType: "kickOffJob",
        Config:     &config,
    }

    if err := db.CreateAction(original); err != nil {
        t.Fatalf("CreateAction failed: %v", err)
    }

    retrieved, err := db.GetAction("action-1")
    if err != nil {
        t.Fatalf("GetAction failed: %v", err)
    }

    if retrieved.ID != original.ID {
        t.Errorf("ID = %q, want %q", retrieved.ID, original.ID)
    }
    if retrieved.Name != original.Name {
        t.Errorf("Name = %q, want %q", retrieved.Name, original.Name)
    }
}
```

### TestGetAllActions

```go
func TestGetAllActions(t *testing.T) {
    db := NewTestDB(t)

    action1 := &Action{
        ID:         "action-1",
        Name:       "Retry",
        ActionType: "retry",
    }
    action2 := &Action{
        ID:         "action-2",
        Name:       "Webhook",
        ActionType: "webhook",
    }

    db.CreateAction(action1)
    db.CreateAction(action2)

    actions, err := db.GetAllActions()
    if err != nil {
        t.Fatalf("GetAllActions failed: %v", err)
    }

    if len(actions) != 2 {
        t.Errorf("got %d actions, want 2", len(actions))
    }
}
```

### TestDeleteAction

```go
func TestDeleteAction(t *testing.T) {
    db := NewTestDB(t)

    action := &Action{
        ID:         "action-1",
        Name:       "Test Action",
        ActionType: "retry",
    }

    if err := db.CreateAction(action); err != nil {
        t.Fatalf("CreateAction failed: %v", err)
    }

    if err := db.DeleteAction("action-1"); err != nil {
        t.Fatalf("DeleteAction failed: %v", err)
    }

    _, err := db.GetAction("action-1")
    if !IsNotFound(err) {
        t.Error("action should be deleted")
    }
}
```

## Job Actions Tests

### TestCreateJobAction

```go
func TestCreateJobAction(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create action
    action := &Action{
        ID:         "action-1",
        Name:       "Retry on failure",
        ActionType: "retry",
    }
    if err := db.CreateAction(action); err != nil {
        t.Fatalf("CreateAction failed: %v", err)
    }

    jobAction := &JobAction{
        JobID:    "job-1",
        ActionID: "action-1",
        Trigger:  "on_failure",
    }

    if err := db.CreateJobAction(jobAction); err != nil {
        t.Fatalf("CreateJobAction failed: %v", err)
    }
}
```

### TestGetJobActions

```go
func TestGetJobActions(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create actions
    action1 := &Action{ID: "action-1", Name: "Retry", ActionType: "retry"}
    action2 := &Action{ID: "action-2", Name: "Webhook", ActionType: "webhook"}
    db.CreateAction(action1)
    db.CreateAction(action2)

    // Associate with job
    jobAction1 := &JobAction{JobID: "job-1", ActionID: "action-1", Trigger: "on_failure"}
    jobAction2 := &JobAction{JobID: "job-1", ActionID: "action-2", Trigger: "on_violation"}
    db.CreateJobAction(jobAction1)
    db.CreateJobAction(jobAction2)

    // Retrieve
    jobActions, err := db.GetJobActions("job-1")
    if err != nil {
        t.Fatalf("GetJobActions failed: %v", err)
    }

    if len(jobActions) != 2 {
        t.Errorf("got %d job actions, want 2", len(jobActions))
    }
}
```

### TestDeleteJobAction

```go
func TestDeleteJobAction(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create action
    action := &Action{ID: "action-1", Name: "Retry", ActionType: "retry"}
    db.CreateAction(action)

    // Associate with job
    jobAction := &JobAction{JobID: "job-1", ActionID: "action-1", Trigger: "on_failure"}
    db.CreateJobAction(jobAction)

    // Delete
    if err := db.DeleteJobAction("job-1", "action-1"); err != nil {
        t.Fatalf("DeleteJobAction failed: %v", err)
    }

    // Verify
    jobActions, err := db.GetJobActions("job-1")
    if err != nil {
        t.Fatalf("GetJobActions failed: %v", err)
    }

    if len(jobActions) != 0 {
        t.Errorf("got %d job actions, want 0", len(jobActions))
    }
}
```

### TestJobActionCascadeDelete

```go
func TestJobActionCascadeDelete(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create action
    action := &Action{ID: "action-1", Name: "Retry", ActionType: "retry"}
    db.CreateAction(action)

    // Associate with job
    jobAction := &JobAction{JobID: "job-1", ActionID: "action-1", Trigger: "on_failure"}
    db.CreateJobAction(jobAction)

    // Delete job
    if err := db.DeleteJob("job-1"); err != nil {
        t.Fatalf("DeleteJob failed: %v", err)
    }

    // Job actions should be cascade deleted
    jobActions, err := db.GetJobActions("job-1")
    if err != nil {
        t.Fatalf("GetJobActions failed: %v", err)
    }

    if len(jobActions) != 0 {
        t.Errorf("job actions should be cascade deleted")
    }
}
```

## Constraint Violation Tests

### TestCreateConstraintViolation

```go
func TestCreateConstraintViolation(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    run := MakeTestJobRun("job-1", time.Now())
    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    actionTaken := "retry"
    violation := &ConstraintViolation{
        ID:             "violation-1",
        RunID:          run.RunID,
        ConstraintName: "maxAllowedRunTime",
        ViolationTime:  time.Now(),
        ActionTaken:    &actionTaken,
        Details:        `{"message": "Job exceeded maxAllowedRunTime", "threshold": "2h"}`,
    }

    if err := db.CreateConstraintViolation(violation); err != nil {
        t.Fatalf("CreateConstraintViolation failed: %v", err)
    }
}
```

### TestGetConstraintViolations

```go
func TestGetConstraintViolations(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    run := MakeTestJobRun("job-1", time.Now())
    if err := db.CreateJobRun(run); err != nil {
        t.Fatalf("CreateJobRun failed: %v", err)
    }

    // Create multiple violations
    actionTaken := "webhook"
    violation1 := &ConstraintViolation{
        ID:             "violation-1",
        RunID:          run.RunID,
        ConstraintName: "maxConcurrentRuns",
        ViolationTime:  time.Now(),
        ActionTaken:    &actionTaken,
        Details:        `{"message": "Too many concurrent runs"}`,
    }

    violation2 := &ConstraintViolation{
        ID:             "violation-2",
        RunID:          run.RunID,
        ConstraintName: "maxExpectedRunTime",
        ViolationTime:  time.Now(),
        ActionTaken:    nil,
        Details:        `{"message": "Job behind schedule"}`,
    }

    db.CreateConstraintViolation(violation1)
    db.CreateConstraintViolation(violation2)

    violations, err := db.GetConstraintViolations(run.RunID)
    if err != nil {
        t.Fatalf("GetConstraintViolations failed: %v", err)
    }

    if len(violations) != 2 {
        t.Errorf("got %d violations, want 2", len(violations))
    }
}
```

### TestGetConstraintViolationsByName

```go
func TestGetConstraintViolationsByName(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    // Create multiple runs with violations
    for i := 0; i < 3; i++ {
        run := MakeTestJobRun("job-1", time.Now().Add(time.Duration(i)*time.Minute))
        db.CreateJobRun(run)

        actionTaken := "killAllInstances"
        violation := &ConstraintViolation{
            ID:             fmt.Sprintf("violation-%d", i),
            RunID:          run.RunID,
            ConstraintName: "maxAllowedRunTime",
            ViolationTime:  time.Now(),
            ActionTaken:    &actionTaken,
            Details:        `{"message": "Exceeded time limit"}`,
        }
        db.CreateConstraintViolation(violation)
    }

    violations, err := db.GetConstraintViolationsByName("maxAllowedRunTime", 10)
    if err != nil {
        t.Fatalf("GetConstraintViolationsByName failed: %v", err)
    }

    if len(violations) != 3 {
        t.Errorf("got %d violations, want 3", len(violations))
    }

    // Verify all are for the correct constraint
    for _, v := range violations {
        if v.ConstraintName != "maxAllowedRunTime" {
            t.Errorf("ConstraintName = %q, want %q", v.ConstraintName, "maxAllowedRunTime")
        }
    }
}
```

### TestGetConstraintViolations_CascadeDelete

```go
func TestGetConstraintViolations_CascadeDelete(t *testing.T) {
    db := NewTestDB(t)
    SeedTestData(t, db)

    run := MakeTestJobRun("job-1", time.Now())
    db.CreateJobRun(run)

    actionTaken := "retry"
    violation := &ConstraintViolation{
        ID:             "violation-1",
        RunID:          run.RunID,
        ConstraintName: "preRunHook",
        ViolationTime:  time.Now(),
        ActionTaken:    &actionTaken,
        Details:        `{"message": "Hook returned 500"}`,
    }
    db.CreateConstraintViolation(violation)

    // Delete the job (should cascade to job_runs and constraint_violations)
    if err := db.DeleteJob("job-1"); err != nil {
        t.Fatalf("DeleteJob failed: %v", err)
    }

    // Verify violations were cascaded
    violations, err := db.GetConstraintViolations(run.RunID)
    if err != nil {
        t.Fatalf("GetConstraintViolations failed: %v", err)
    }

    if len(violations) != 0 {
        t.Errorf("expected 0 violations after cascade delete, got %d", len(violations))
    }
}
```

## Job Constraints Tests

### TestJobWithConstraints

```go
func TestJobWithConstraints(t *testing.T) {
    db := NewTestDB(t)

    constraints := `{
        "maxConcurrentRuns": 5,
        "catchUp": true,
        "catchUpWindow": "5m",
        "maxExpectedRunTime": "1h",
        "maxAllowedRunTime": "2h",
        "preRunHook": {
            "type": "slack",
            "url": "https://hooks.slack.com/...",
            "channel": "#notifications"
        }
    }`

    job := &Job{
        ID:          "test-job",
        Name:        "Test Job",
        Schedule:    "0 0 * * *",
        PodSpec:     `{"image": "test:latest"}`,
        Constraints: constraints,
    }

    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    retrieved, err := db.GetJob("test-job")
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }

    if retrieved.Constraints != constraints {
        t.Errorf("Constraints not preserved")
    }
}
```

### TestJobWithNullConstraints

```go
func TestJobWithNullConstraints(t *testing.T) {
    db := NewTestDB(t)

    job := &Job{
        ID:          "test-job",
        Name:        "Test Job",
        Schedule:    "0 0 * * *",
        PodSpec:     `{"image": "test:latest"}`,
        Constraints: "",
    }

    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    retrieved, err := db.GetJob("test-job")
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }

    if retrieved.Constraints != "" {
        t.Errorf("Expected empty constraints, got %q", retrieved.Constraints)
    }
}
```

## Statistics Tests

### TestCreateSchedulerStats

```go
func TestCreateSchedulerStats(t *testing.T) {
    db := NewTestDB(t)

    stats := &SchedulerStats{
        StatsPeriodID: "period-1",
        StartTime:     time.Now().Add(-1 * time.Hour),
        EndTime:       time.Now(),
        Iterations:    3600,
        RunJobs:       45,
        LateJobs:      2,
    }

    if err := db.CreateSchedulerStats(stats); err != nil {
        t.Fatalf("CreateSchedulerStats failed: %v", err)
    }
}
```

### TestGetSchedulerStats

```go
func TestGetSchedulerStats(t *testing.T) {
    db := NewTestDB(t)

    now := time.Now()
    stats1 := &SchedulerStats{
        StatsPeriodID: "period-1",
        StartTime:     now.Add(-2 * time.Hour),
        EndTime:       now.Add(-1 * time.Hour),
        Iterations:    3600,
    }
    stats2 := &SchedulerStats{
        StatsPeriodID: "period-2",
        StartTime:     now.Add(-1 * time.Hour),
        EndTime:       now,
        Iterations:    3600,
    }

    db.CreateSchedulerStats(stats1)
    db.CreateSchedulerStats(stats2)

    retrieved, err := db.GetSchedulerStats(now.Add(-90*time.Minute), now)
    if err != nil {
        t.Fatalf("GetSchedulerStats failed: %v", err)
    }

    // Should get both stats that overlap the time range
    if len(retrieved) != 2 {
        t.Errorf("got %d stats, want 2", len(retrieved))
    }
}
```

## Multi-Database Compatibility Tests

These tests verify the database layer works correctly with PostgreSQL, MySQL, and SQLite.

### TestPostgreSQLCompatibility

```go
func TestPostgreSQLCompatibility(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping PostgreSQL integration test")
    }

    dsn := os.Getenv("POSTGRES_TEST_DSN")
    if dsn == "" {
        t.Skip("POSTGRES_TEST_DSN not set")
    }

    db, err := Open("postgres", dsn)
    if err != nil {
        t.Fatalf("failed to connect: %v", err)
    }
    defer db.Close()

    // Run basic CRUD operations
    testBasicCRUD(t, db)
}
```

### TestMySQLCompatibility

```go
func TestMySQLCompatibility(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping MySQL integration test")
    }

    dsn := os.Getenv("MYSQL_TEST_DSN")
    if dsn == "" {
        t.Skip("MYSQL_TEST_DSN not set")
    }

    db, err := Open("mysql", dsn)
    if err != nil {
        t.Fatalf("failed to connect: %v", err)
    }
    defer db.Close()

    testBasicCRUD(t, db)
}
```

### TestSQLiteCompatibility

```go
func TestSQLiteCompatibility(t *testing.T) {
    db := NewTestDB(t)
    testBasicCRUD(t, db)
}
```

### testBasicCRUD Helper

```go
// testBasicCRUD runs basic CRUD operations to verify compatibility
func testBasicCRUD(t *testing.T, db *DB) {
    t.Helper()

    // Create job
    job := MakeTestJob("test-job")
    if err := db.CreateJob(job); err != nil {
        t.Fatalf("CreateJob failed: %v", err)
    }

    // Read job
    retrieved, err := db.GetJob("test-job")
    if err != nil {
        t.Fatalf("GetJob failed: %v", err)
    }

    // Update job
    retrieved.Name = "Updated Job"
    if err := db.UpdateJob(retrieved); err != nil {
        t.Fatalf("UpdateJob failed: %v", err)
    }

    // Delete job
    if err := db.DeleteJob("test-job"); err != nil {
        t.Fatalf("DeleteJob failed: %v", err)
    }
}
```

## Placeholder Tests

### TestPlaceholder

```go
func TestPlaceholder(t *testing.T) {
    tests := []struct {
        driver string
        n      int
        want   string
    }{
        {"postgres", 1, "$1"},
        {"postgres", 5, "$5"},
        {"mysql", 1, "?"},
        {"sqlite3", 1, "?"},
    }

    for _, tt := range tests {
        t.Run(fmt.Sprintf("%s_%d", tt.driver, tt.n), func(t *testing.T) {
            db := &DB{driver: tt.driver}
            got := db.placeholder(tt.n)
            if got != tt.want {
                t.Errorf("placeholder(%d) = %q, want %q", tt.n, got, tt.want)
            }
        })
    }
}
```

## Test Coverage Goals

The test suite should achieve:
- **Line coverage**: > 90%
- **Branch coverage**: > 85%
- **All error paths tested**: Every error return should have a test
- **All database drivers tested**: PostgreSQL, MySQL, SQLite

## Test Execution

```bash
# Run all tests
go test ./internal/db/...

# Run with coverage
go test -cover ./internal/db/...

# Run with race detector
go test -race ./internal/db/...

# Run only unit tests (skip integration tests)
go test -short ./internal/db/...

# Run specific test
go test -run TestCreateJob ./internal/db/...
```

## Integration Test Setup

Integration tests require running database instances. Use Docker for local testing:

```bash
# PostgreSQL
docker run -d --name postgres-test \
  -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=itinerary_test \
  -p 5432:5432 \
  postgres:15

export POSTGRES_TEST_DSN="postgres://postgres:test@localhost:5432/itinerary_test?sslmode=disable"

# MySQL
docker run -d --name mysql-test \
  -e MYSQL_ROOT_PASSWORD=test \
  -e MYSQL_DATABASE=itinerary_test \
  -p 3306:3306 \
  mysql:8

export MYSQL_TEST_DSN="root:test@tcp(localhost:3306)/itinerary_test"
```

## Non-Goals

1. **Performance benchmarks** - Focus on correctness first
2. **Stress testing** - Connection pool limits tested elsewhere
3. **Migration testing** - Covered by migrator test suite
4. **Schema validation** - Migrations create correct schema
