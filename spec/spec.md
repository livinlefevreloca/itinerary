# Itinerary

## Executive summary
itinerary highly customizable scheduler that orchestrates and monitors jobs running on a kubernetes cluster. It offers:
  * A feature rich UI used to control all aspects of your job running
  * Job "requirements" and "actions" which define condition about job and its metadata that must be met. These can 1 of
    - pre execution
    - execution time
    - post execution
* requirements and actions are also extensible and can be defined in code by the user.
    Some examples of built in "requirements"
      - Argument unique runs - no 2 instances of a given job can ever run with the same arguements
      - Sequential argument runs - An instance of a job cannot run unless all previous instances of a job defined by a given argument have been run
      - Job parrellism - no more than X of this job can be running at once
    Some examples of actions:
      - cancel the current job run
      - kill all running versions of the job
      - notify some external service of the failed requirement
  * Extensive tracking of job statistics run time, state change tracking,retries, failures and failure reasons
  * anomally detection for job statistics


## Scheduler component Components
This section defines the components of the scheduler and how they fit together

### Central scheduler
The central scheduler is the heart of the application and operates as an event loop.

#### Core Principles
* **Single Source of Truth**: The main loop owns all scheduling state. Any component that needs state must send a request to the inbox and wait for a response.
* **No I/O in Loop**: The loop never performs I/O operations. All external communication is delegated to other goroutines.
* **Request/Response Pattern**: Components communicate with the loop via inbox messages that can include response channels.

#### Startup
* Load configuration (intervals, windows, etc.)
* Load job definitions from database (read-only connection)
* Build initial ScheduledRunIndex
* Initialize state:
  - Active orchestrators map (runID → orchestrator state)
  - Inbox (buffered channel for events)
  - Shutdown signal channel

#### Configuration Parameters
* **PRE_SCHEDULE_INTERVAL**: Time before a job starts to launch its orchestrator (default: 10 seconds, configurable)
* **INDEX_REBUILD_INTERVAL**: How often to rebuild the index (default: 1 minute, configurable, must be < LOOKAHEAD_WINDOW)
* **LOOKAHEAD_WINDOW**: How far ahead to calculate runs (default: 10 minutes, configurable)
* **GRACE_PERIOD**: How far back to include runs in index to catch near-misses (default: 30 seconds, configurable)
* **LOOP_INTERVAL**: How often the main loop runs (default: 1 second)

#### Main Loop Iteration
Each iteration processes:

1. **Check for shutdown signal**
   - If received, cancel all active orchestrators and exit gracefully

2. **Rebuild index if needed**
   - If (now - lastRebuild) > INDEX_REBUILD_INTERVAL
   - Generate runs for window: (now - GRACE_PERIOD, now + LOOKAHEAD_WINDOW)
   - Atomically swap index
   - No I/O - uses in-memory job list

3. **Schedule new orchestrators**
   - Query index for jobs in (now, now + PRE_SCHEDULE_INTERVAL)
   - For each run not in activeOrchestrators map:
     - Generate unique runID
     - Launch orchestrator goroutine
     - Record in activeOrchestrators[runID] with metadata
   - No I/O - just goroutine spawning

4. **Process all inbox messages**
   - Handle every message in the inbox (not priority-based)
   - Message types:
     - State queries (respond with current state)
     - Cancellation requests (signal orchestrator)
     - Orchestrator completion notifications
     - Schedule update notifications (trigger rebuild)
     - Stats requests
   - Messages may include response channels for request/response pattern

5. **Clean up completed orchestrators**
   - Remove entries from activeOrchestrators where:
     - Orchestrator has completed AND
     - now > startTime + GRACE_PERIOD
   - This prevents re-running fast jobs that complete within grace period

6. **Record state changes**
   - Collect all state changes from this iteration
   - Send to syncer goroutines for database persistence
   - No I/O in main loop - syncers handle writes

7. **Update statistics**
   - Track loop iteration metrics
   - Send to stats syncer

#### State Management
The main loop maintains:
* **activeOrchestrators**: Map of runID → orchestrator metadata
  - Includes: jobID, scheduledTime, actualStartTime, status, cancel channel
  - Entries removed only after completion AND grace period expiration
* **jobDefinitions**: In-memory copy of all job schedules
* **index**: ScheduledRunIndex for efficient time-based queries
* **stats**: Current iteration statistics

#### Orchestrator Lifecycle
1. Main loop creates orchestrator goroutine
2. Orchestrator runs (pre-exec, exec, post-exec phases)
3. Orchestrator sends completion message to inbox
4. Main loop marks as completed in activeOrchestrators
5. After GRACE_PERIOD expires, main loop removes from map

#### Communication Patterns
* **External → Loop**: Watcher goroutine receives gRPC requests, sends to inbox
* **Loop → Orchestrators**: Via cancel channels
* **Orchestrators → Loop**: Via inbox messages (completion, errors)
* **Loop → Syncers**: Via syncer-specific channels (state changes)
* **Loop → External**: Via response channels in inbox messages


### Orchestrator
* An orchestrator is a go routine that is responsibile for a given run of a given job.
* It is passed the required information about the job and starts its pre execution loop. This involves
  - checking for cancel notifications from the main loop
  - checking if the starttime has passed
  - checking for a global shutdown signal
* Once starttime has passed it checks any pre execution requirements of the job and take a appropriate action
* If they are met it starts the job execution
* After that it starts its execution loop within the execution loop it:
  - waits on kuberenertes events from the job it started with a timeout
  - check any during execution time attributes and handle any failed
* Once the job is done if it has failed we simply notify the main loop, record the failure and exit
* Once the job concludes if its successful we check any post execution requirements. If the requirements do not hold then take the corresponding action. Afterwards we record the success and requirement results


### Writer
* the writer is the component responsible for syncing information required for the scheduler to the datastore
* It does not run any writes itself. It launches go routines to and tracks what writes are currently in progress.
* this prevents writes to the same tuple happening simulataneously. (we may need to reconsider this it might just be better to have other components write directly to the data store)
* The data store itself is configurable. The user can provide a database url for a postgres database, sqllite or mysql databases. We will need to use a library to abstract this away
* The recorder runs the following loop
  - check for completed writes by selecting over the list of "done" channel corresponding to each write. If any are found remove the write from the active writes
  - Check the write queue for writes waiting to be serviced. It any are waiting that do not conflict with an active write run them
  - Check for new writes in the inbox. If there are any check if they conflict, If they do add them to the write queue, otherwise run them

## Job requirements and actions
### Requirement
* requirements are conditions that are checked at a given point in the job life cycle
* Because go cannot dynamically load code there are 3 main options for implementing these plugins
  - dynamically load .so or dll files using go's [plugin](https://pkg.go.dev/plugin) module
  - Run compiled go binaries in separate processes or containers and communicate over some rpc method
  - Require users to write plugins via a scripting language such as lua and run them via [gopher-lua](https://github.com/yuin/gopher-lua)
* things to consider there
  * If the plugin method is blocking it will block the loop of the orchestrator responsible for the job
  * communuicating over ipc may be slow
  * What interface do we present to the plugin writer / what do we give them access to as far as state of the scheduler.
  * There is a shared state problem for many of these. Ultimatley the source of truth for the states of various jobs is the state held in the main scheduler loops memory context. The database is the long term store but there is no gurantee it will be fully up to date.
### Actions
* Actions are actions taken depending on the outcome of a requirement
* The will likely use the same dynamic loading method chosen for requirments

## Stats tracker
* The stats tracker records various metadata stats about components of the scheduler


## Database connection
### Database abstraction
* itinerary should be able to run with an relational store
* We will likely need to use a library to abstract over the database connection
* All connections besides the `Recorder` connection should be read only
* initial start up runs migrations on the database creating the neccessary schema
### Schema definition
* job
    - id pk
    - name
    - schedule
    - podSpec
  action
    - id pk
    - binary_path
  action_runs
    - id (pk)
    - action_id (fk action.id)
    - start_time
    - end_time
    - success
  requirement
    - id pk
    - binary path
  requirement_runs
    - id (pk)
    - requirment_id (fk requirement.id)
    - start_time
    - end_time
    - success
  job_requirements
    - id
    - job_id (fk job.id)
    - requirment_id (fk requirement.id)
  job_requirment_action
    - job_id ( fk job.id) pk with requirment_id
    - requirement_id (fk requirment.id)
    - action_id (fk action_id)
  job_run
    - id
    - job_id (fk job.id)
    - start_time
    - end time
    - state
  configuration
    -
  scheduler_stats
    - stats_period_id (pk)
    - start_time
    - end_time
    - iterations
    - run_jobs
    - late_jobs
    - time_passed_run_time
    - missed_jobs
    - time_passed_grace_period
    - jobs_cancelled
    - min_inbox_length
    - max_inbox_length
    - avg_inbox_length
    - empty_inbox_time
    - avg_time_in_inbox
    - min_time_in_inbox
    - max_time_in_inbox
  orchestrator_stats
   - run_id
   - stats_period_id
   - runtime
   - requirements_ran
   - actions_ran
  requirements_stats
   - run_id
   - requirement_run_id
   - stats_period
   - requirement_id
   - runtime
  actions_stats
   - run_id
   - action_run_id
   - stats_period
   - requirement_id
   - runtime
  writer_stats
    - stats_period_id
    - total_writes
    - writes_succeeded
    - writes_failed
    - avg_writes_in_flight
    - max_writes_in_flight
    - min_writes_in_flight
    - avg_queued_writes
    - max_queued_writes
    - min_queued_writes
    - avg_inbox_length
    - max_inbox_length
    - min_inbox_length
    - avg_time_in_write_queue
    - max_time_in_write_queue
    - min_time_in_write_queue
    - avg_time_in_inbox
    - max_time_in_inbox
    - min_time_in_inbox



## UI components
* The UI is a web based UI. The UI should be in typescript with react using vite
### Currently running jobs screen
* A screen displaying running jobs.
* Each job has a card that displays stats about it (job name, start time, etc)
* The job card links to each individual job run page

### Full Job run history
* Job run history displayed as a [gantt chart](https://en.wikipedia.org/wiki/Gantt_chart)
* There should be a several filters on this page
  - time range in which to display jobs (starttime, endtime) this should be able to be set relativley (since 5 mins ago) or precisley (2026-01-01T00:00:00 to 2026-01-01T00:05:00)
  - job run length (min and max)
  - job name
  - job tags
* Jobs on gantt chart should be color coded by success, failure and still running


### Individual job run history
* Job Run hisotry displaying simimlar to the the cronitor page which can be seen in these screenshots
  - ![top of page](/Users/adam/Projects/personal/golang/itinerary/spec/top.png)
  - ![bottom of page](/Users/adam/Projects/personal/golang/itinerary/spec/bottom.png)

### Job run page
* A page displaying information specific to a job run
* start time, current run time, args, requirements and results of those requirments
* linkes to Job Page

### Manually run a job page
* Run a job (if allowed) with the arguements passed overwritten

### Job page
* Information on a job (not a specific run)
* All job configuration

### Job definitions page
* A page listing all define jobs.
* paginated
* allow search based on
  - name
  - tags
  - schedule

### New Job page
* A page for defining a new job
* A job definition takes
  - name
  - namespace
  - schedule
  - image(s)
  - command(s)
  - serviceaccount(s)
  - env
  - envSecret
  - podSpec
  - tags
  - requirements
  - actions
  - resources

### Edit job page
  * Edit all of the above attributes in the newJob page



## UI web API
* The UI talks to a backend web API which defines the following endpoints
  - GET /runs/<run_id> - Get information on a specific run of a job
  - GET /runs/<job_id> - Get information on runs of a given job given an id
    params:
      - start_time
      - end_time
  - GET /runs - Get a list of runs satisfying the criteria
      params:
        - job_id (optional)
        - name (optional)
        - tags (optional)
        - requirements (optional)
        - actions (optional)
        - started_after_time (optional)
        - ended_before_time (optional)
        - running_after_time (optional)
        - running_before_time (optional)
  - POST /runs/<job_id> - kick off a run of a given job with the provided args
      params:
        - args (optional)
        - resources (optional)
        - start-time (optional)
        - requirements (optional)
        - actions (optional)
  - DELETE /runs/<run_id> - Cancel the run of a job matching the ID
  - GET /jobs/<job_id> - get information on a specific job
  - GET /jobs - get information on all jobs
    params:
      - name_pattern (optional)
      - tags (optional)
      - tag_pattern (optional)
      - actions (optional)
      - requirements (optional)
      - schedule (optional)
  - POST /jobs - create a new job with the given parameters
    params:
      - name
      - schedule
      - podSpec (optional)
      * namespace (optional)
      * images (optional)
      * commands
      * serviceaccounts (optional)
      * env (optional)
      * envSecret (optional)
      - tags (optional)
      - requirements
      - actions
      - is manually runnable
      - is modifiable for manual run
  - PUT /jobs/<job-id> - update an existing job
    params:
      - name
      - schedule
      - podSpec (optional)
      * namespace (optional)
      * images (optional)
      * commands
      * serviceaccounts (optional)
      * env (optional)
      * envSecret (optional)
      - tags (optional)
      - requirements
      - actions
      - is manually runnable
      - manual run modifiable fields
