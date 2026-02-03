# Changelog

All notable changes to the Itinerary project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Changed
- Updated spec.md to reflect current implementation state
  - Revised all component descriptions to match actual code in internal/
  - Added lock-free design principles for scheduler and index
  - Documented Job State Syncer architecture with dual-trigger flushing
  - Updated orchestrator state machine and lifecycle documentation
  - Added implementation status section tracking component completion
  - Clarified constraint/action system architecture

- Consolidated database migrations into single comprehensive schema
  - Split job configuration into jobs, constraints, and actions tables
  - Renamed constraint_violations to constraint_runs for clarity
  - Added trigger column to job_runs (scheduled, manual, retry, action)
  - Updated action_runs to reference constraint_runs and actions tables
  - Maintained all statistics tables (scheduler, orchestrator, syncer, stats_collector)
  - Included future tables (webhook_deliveries, webhook_handler_stats)

### Technical Details
- Migration changes ensure schema matches spec.md and internal/db/spec.md
- Conservative indexing: only proven necessary indexes included
- Foreign keys have appropriate ON DELETE actions
- Dimension tables use manually assigned IDs (never reused or deleted)
