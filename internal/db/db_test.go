package db

import (
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Test Fixtures and Helpers

// NewTestDB creates an in-memory SQLite database for testing
func NewTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	// Initialize schema
	if err := initTestSchema(db); err != nil {
		db.Close()
		t.Fatalf("failed to initialize test schema: %v", err)
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
	jobs := []*Job{
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
		if err := db.CreateJob(job); err != nil {
			t.Fatalf("failed to seed job: %v", err)
		}
	}
}

// MakeTestJob creates a job with default test values
func MakeTestJob(id string) *Job {
	return &Job{
		ID:       id,
		Name:     "Test Job " + id,
		Schedule: "0 0 * * *",
		PodSpec:  `{"image": "test:latest"}`,
	}
}

// MakeTestJobRun creates a job run with default test values
func MakeTestJobRun(jobID string, scheduledAt time.Time) *JobRun {
	return &JobRun{
		JobID:       jobID,
		RunID:       "run-" + jobID + "-" + scheduledAt.Format("20060102150405"),
		ScheduledAt: scheduledAt,
		Status:      "pending",
	}
}

// Helper functions removed - dimension tables are pre-seeded in schema_test.go

// Connection Tests

func TestOpen(t *testing.T) {
	tests := []struct {
		name        string
		driver      string
		dsn         string
		wantErr     bool
		errContains string
	}{
		{
			name:    "sqlite in-memory",
			driver:  "sqlite3",
			dsn:     ":memory:",
			wantErr: false,
		},
		{
			name:        "invalid driver",
			driver:      "invalid",
			dsn:         "",
			wantErr:     true,
			errContains: "unknown driver",
		},
		{
			name:    "empty dsn",
			driver:  "sqlite3",
			dsn:     "",
			wantErr: false,
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

func TestPing(t *testing.T) {
	db := NewTestDB(t)

	if err := db.Ping(); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

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

// Job Tests

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

// Job Run Tests

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

func TestCreateJobRun_Duplicate(t *testing.T) {
	db := NewTestDB(t)
	SeedTestData(t, db)

	scheduledAt := time.Now()
	run := MakeTestJobRun("job-1", scheduledAt)

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

// Transaction Tests

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

// Error Handling Tests

func TestIsNotFound(t *testing.T) {
	db := NewTestDB(t)

	_, err := db.GetJob("nonexistent")

	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound = true for nonexistent job")
	}
}

func TestIsDuplicate(t *testing.T) {
	db := NewTestDB(t)

	job := MakeTestJob("test-job")
	db.CreateJob(job)

	err := db.CreateJob(job)

	if !IsDuplicate(err) {
		t.Errorf("expected IsDuplicate = true for duplicate job")
	}
}

func TestIsForeignKey(t *testing.T) {
	db := NewTestDB(t)

	run := MakeTestJobRun("nonexistent-job", time.Now())
	err := db.CreateJobRun(run)

	if !IsForeignKey(err) {
		t.Errorf("expected IsForeignKey = true for invalid job_id")
	}
}

// Constraint Violation Tests

func TestCreateConstraintViolation(t *testing.T) {
	db := NewTestDB(t)
	SeedTestData(t, db)

	// Constraint types are pre-seeded in schema
	run := MakeTestJobRun("job-1", time.Now())
	if err := db.CreateJobRun(run); err != nil {
		t.Fatalf("CreateJobRun failed: %v", err)
	}

	actionTaken := "retry"
	violation := &ConstraintViolation{
		ID:               "violation-1",
		RunID:            run.RunID,
		ConstraintTypeID: 7, // maxAllowedRunTime
		ViolationTime:    time.Now(),
		ActionTaken:      &actionTaken,
		Details:          `{"message": "Job exceeded maxAllowedRunTime", "threshold": "2h"}`,
	}

	if err := db.CreateConstraintViolation(violation); err != nil {
		t.Fatalf("CreateConstraintViolation failed: %v", err)
	}
}

func TestGetConstraintViolations(t *testing.T) {
	db := NewTestDB(t)
	SeedTestData(t, db)

	// Constraint types are pre-seeded in schema
	run := MakeTestJobRun("job-1", time.Now())
	if err := db.CreateJobRun(run); err != nil {
		t.Fatalf("CreateJobRun failed: %v", err)
	}

	// Create multiple violations
	actionTaken := "webhook"
	violation1 := &ConstraintViolation{
		ID:               "violation-1",
		RunID:            run.RunID,
		ConstraintTypeID: 6, // maxExpectedRunTime
		ViolationTime:    time.Now(),
		ActionTaken:      &actionTaken,
		Details:          `{"message": "Job exceeded maxExpectedRunTime"}`,
	}
	violation2 := &ConstraintViolation{
		ID:               "violation-2",
		RunID:            run.RunID,
		ConstraintTypeID: 7, // maxAllowedRunTime
		ViolationTime:    time.Now().Add(time.Minute),
		ActionTaken:      &actionTaken,
		Details:          `{"message": "Job exceeded maxAllowedRunTime"}`,
	}

	if err := db.CreateConstraintViolation(violation1); err != nil {
		t.Fatalf("CreateConstraintViolation failed: %v", err)
	}
	if err := db.CreateConstraintViolation(violation2); err != nil {
		t.Fatalf("CreateConstraintViolation failed: %v", err)
	}

	// Retrieve all violations for the run
	violations, err := db.GetConstraintViolations(run.RunID)
	if err != nil {
		t.Fatalf("GetConstraintViolations failed: %v", err)
	}

	if len(violations) != 2 {
		t.Errorf("got %d violations, want 2", len(violations))
	}
}

func TestGetConstraintViolationsByType(t *testing.T) {
	db := NewTestDB(t)
	SeedTestData(t, db)

	// Constraint types are pre-seeded in schema
	run1 := MakeTestJobRun("job-1", time.Now())
	run2 := MakeTestJobRun("job-1", time.Now().Add(time.Hour))
	db.CreateJobRun(run1)
	db.CreateJobRun(run2)

	actionTaken := "killAllInstances"
	violation1 := &ConstraintViolation{
		ID:               "violation-1",
		RunID:            run1.RunID,
		ConstraintTypeID: 7, // maxAllowedRunTime
		ViolationTime:    time.Now(),
		ActionTaken:      &actionTaken,
		Details:          `{"message": "Job exceeded maxAllowedRunTime"}`,
	}
	violation2 := &ConstraintViolation{
		ID:               "violation-2",
		RunID:            run2.RunID,
		ConstraintTypeID: 7, // maxAllowedRunTime
		ViolationTime:    time.Now().Add(time.Hour),
		ActionTaken:      &actionTaken,
		Details:          `{"message": "Job exceeded maxAllowedRunTime"}`,
	}

	db.CreateConstraintViolation(violation1)
	db.CreateConstraintViolation(violation2)

	// Retrieve violations for specific constraint type
	violations, err := db.GetConstraintViolationsByType(7, 10) // maxAllowedRunTime
	if err != nil {
		t.Fatalf("GetConstraintViolationsByType failed: %v", err)
	}

	if len(violations) != 2 {
		t.Errorf("got %d violations, want 2", len(violations))
	}

	for _, v := range violations {
		if v.ConstraintTypeID != 7 {
			t.Errorf("ConstraintTypeID = %d, want %d", v.ConstraintTypeID, 7)
		}
	}
}

func TestJobWithConstraints(t *testing.T) {
	db := NewTestDB(t)

	// Create a job with constraints (stored as JSON)
	constraints := `{"1": {"value": 5}, "6": {"value": "1h"}, "7": {"value": "2h"}}`
	job := &Job{
		ID:          "job-with-constraints",
		Name:        "Test Job with Constraints",
		Schedule:    "0 * * * *",
		PodSpec:     `{"image": "test:latest"}`,
		Constraints: &constraints,
	}

	if err := db.CreateJob(job); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}

	retrieved, err := db.GetJob("job-with-constraints")
	if err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}

	if retrieved.Constraints == nil || *retrieved.Constraints != constraints {
		t.Errorf("Constraints = %v, want %s", retrieved.Constraints, constraints)
	}
}

// Statistics Tests

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

// Multi-Database Compatibility Tests

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

func TestSQLiteCompatibility(t *testing.T) {
	db := NewTestDB(t)
	testBasicCRUD(t, db)
}

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
