# Actions Module Test Specification

## Test Organization

Tests are organized into the following categories:

1. **Action Unit Tests** - Test individual action types
2. **Template Rendering Tests** - Test template variable substitution
3. **Configuration Parsing Tests** - Test JSON configuration parsing and validation
4. **Error Handling Tests** - Test error conditions and recovery
5. **Context Cancellation Tests** - Test cancellation and timeout handling
6. **ExecutionContext Tests** - Test execution context setup and dependency injection
7. **Action Factory Tests** - Test the CreateAction factory function
8. **Thread Safety Tests** - Test concurrent action execution

## 1. Action Unit Tests

### DelayAction Tests

**TestDelayAction_Success**
- Create DelayAction with duration=100ms
- Create ExecutionContext with valid context
- Call Execute()
- Verify it returns nil after approximately 100ms
- Verify no error returned

**TestDelayAction_ContextCancelled**
- Create DelayAction with duration=1s
- Create ExecutionContext with cancellable context
- Start Execute() in goroutine
- Cancel context after 100ms
- Verify Execute() returns context.Canceled error quickly (not after 1s)

**TestDelayAction_Logging**
- Create DelayAction with duration=50ms
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with correct parameters
- Verify log message contains duration and runID

**TestDelayAction_Name**
- Create DelayAction
- Call Name()
- Assert returns "delay"

**TestDelayAction_ZeroDuration**
- Create DelayAction with duration=0
- Call Execute()
- Verify returns immediately with no error

**TestDelayAction_LongDuration**
- Create DelayAction with duration=5s
- Create ExecutionContext with context that times out after 100ms
- Call Execute()
- Verify returns context.DeadlineExceeded error

### WebhookAction Tests

**TestWebhookAction_Success**
- Create WebhookAction with valid URL and payload
- Create ExecutionContext with mock HTTP client
- Mock HTTP client returns 200 OK
- Call Execute()
- Verify HTTP POST was made to correct URL
- Verify payload was sent correctly
- Verify no error returned

**TestWebhookAction_HTTPError**
- Create WebhookAction
- Mock HTTP client returns 500 error
- Call Execute()
- Verify error is returned
- Verify error message contains status code

**TestWebhookAction_NetworkError**
- Create WebhookAction
- Mock HTTP client returns network error
- Call Execute()
- Verify error is returned
- Verify error message contains network error details

**TestWebhookAction_PayloadSerialization**
- Create WebhookAction with complex nested payload
- Create ExecutionContext with mock HTTP client
- Call Execute()
- Verify payload was correctly serialized to JSON
- Verify Content-Type header is application/json

**TestWebhookAction_Logging**
- Create WebhookAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with URL and runID

**TestWebhookAction_Name**
- Create WebhookAction
- Call Name()
- Assert returns "webhook"

**TestWebhookAction_ContextCancelled**
- Create WebhookAction
- Create ExecutionContext with cancelled context
- Call Execute()
- Verify returns quickly with error

**TestWebhookAction_EmptyPayload**
- Create WebhookAction with nil payload
- Call Execute()
- Verify sends empty JSON object {}

**TestWebhookAction_Timeout**
- Create WebhookAction
- Mock HTTP client with slow response (>context timeout)
- Call Execute()
- Verify returns timeout error

**TestWebhookAction_TemplateRendering**
- Create WebhookAction with templates in URL and payload
- Create ExecutionContext with Command, Args, Kwargs
- Call Execute()
- Verify templates rendered before sending webhook
- See detailed template tests in section 2

### LogAction Tests

**TestLogAction_Success**
- Create LogAction with message
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with correct message
- Verify log contains runID
- Verify no error returned

**TestLogAction_Name**
- Create LogAction
- Call Name()
- Assert returns "log"

**TestLogAction_EmptyMessage**
- Create LogAction with empty message
- Call Execute()
- Verify logger.Info still called
- Verify no error returned

**TestLogAction_MessageWithSpecialChars**
- Create LogAction with message containing quotes, newlines, unicode
- Call Execute()
- Verify message is logged correctly without corruption

**TestLogAction_NilLogger**
- Create LogAction
- Create ExecutionContext with nil Logger
- Call Execute()
- Verify panics (or returns error if we add defensive check)

### FailAction Tests

**TestFailAction_Success**
- Create FailAction with reason
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Error was called with reason and runID
- Verify returns error containing reason

**TestFailAction_ErrorMessage**
- Create FailAction with reason="disk full"
- Call Execute()
- Verify error message is "job failed: disk full"

**TestFailAction_Name**
- Create FailAction
- Call Name()
- Assert returns "fail"

**TestFailAction_EmptyReason**
- Create FailAction with empty reason
- Call Execute()
- Verify returns error (possibly "job failed: ")

**TestFailAction_Logging**
- Create FailAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Error was called (not Info)
- Verify log level is error

### NoOpAction Tests

**TestNoOpAction_Success**
- Create NoOpAction
- Call Execute()
- Verify returns nil immediately

**TestNoOpAction_Name**
- Create NoOpAction
- Call Name()
- Assert returns "noop"

**TestNoOpAction_ExecutionContext**
- Create NoOpAction
- Create ExecutionContext with all fields nil
- Call Execute()
- Verify returns nil (doesn't access any context fields)

### RetryAction Tests

**TestRetryAction_Success**
- Create RetryAction
- Create ExecutionContext with mock JobController
- Mock JobController.RetryJob returns nil
- Call Execute()
- Verify JobController.RetryJob was called with correct job ID
- Verify no error returned

**TestRetryAction_ControllerError**
- Create RetryAction
- Mock JobController.RetryJob returns error
- Call Execute()
- Verify error is returned
- Verify error is from controller

**TestRetryAction_Logging**
- Create RetryAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with jobID and runID

**TestRetryAction_Name**
- Create RetryAction
- Call Name()
- Assert returns "retry"

**TestRetryAction_NilJobController**
- Create RetryAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call RetryJob

### TriggerJobAction Tests

**TestTriggerJobAction_Success**
- Create TriggerJobAction with job_id and args
- Create ExecutionContext with mock JobController
- Mock JobController.TriggerJob returns nil
- Call Execute()
- Verify JobController.TriggerJob was called with correct job ID and args
- Verify no error returned

**TestTriggerJobAction_WithArgs**
- Create TriggerJobAction with complex args map
- Call Execute()
- Verify args passed correctly to JobController

**TestTriggerJobAction_EmptyArgs**
- Create TriggerJobAction with nil or empty args
- Call Execute()
- Verify succeeds with empty args

**TestTriggerJobAction_ControllerError**
- Create TriggerJobAction
- Mock JobController.TriggerJob returns error
- Call Execute()
- Verify error is returned

**TestTriggerJobAction_Logging**
- Create TriggerJobAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with target job ID and source runID

**TestTriggerJobAction_Name**
- Create TriggerJobAction
- Call Name()
- Assert returns "trigger_job"

**TestTriggerJobAction_NilJobController**
- Create TriggerJobAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call TriggerJob

### SlackAction Tests

**TestSlackAction_Success**
- Create SlackAction with all fields
- Create ExecutionContext with mock WebhookHandler
- Mock WebhookHandler.SendWebhook returns nil
- Call Execute()
- Verify webhook sent to correct Slack URL
- Verify payload contains text, channel, username, icon_emoji
- Verify no error returned

**TestSlackAction_PayloadStructure**
- Create SlackAction
- Call Execute()
- Verify webhook payload is correctly formatted for Slack API
- Verify all fields (text, channel, username, icon_emoji) in payload

**TestSlackAction_MinimalConfig**
- Create SlackAction with only webhook_url and text
- Call Execute()
- Verify succeeds with minimal fields

**TestSlackAction_WebhookError**
- Create SlackAction
- Mock WebhookHandler.SendWebhook returns error
- Call Execute()
- Verify error is returned

**TestSlackAction_Logging**
- Create SlackAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with channel and runID

**TestSlackAction_Name**
- Create SlackAction
- Call Name()
- Assert returns "slack"

**TestSlackAction_SpecialCharactersInMessage**
- Create SlackAction with special characters in text (quotes, newlines, emojis)
- Call Execute()
- Verify message is properly escaped/encoded

**TestSlackAction_TemplateRendering**
- Create SlackAction with templates in text, channel, username
- Create ExecutionContext with Command, Args, Kwargs
- Call Execute()
- Verify all templates rendered before sending webhook
- See detailed template tests in section 2

### PagerDutyAction Tests

**TestPagerDutyAction_Success**
- Create PagerDutyAction with all fields
- Create ExecutionContext with mock WebhookHandler
- Mock WebhookHandler.SendWebhook returns nil
- Call Execute()
- Verify webhook sent to PagerDuty events API
- Verify payload contains routing_key, event_action, severity, summary
- Verify no error returned

**TestPagerDutyAction_PayloadStructure**
- Create PagerDutyAction
- Call Execute()
- Verify webhook payload is correctly formatted for PagerDuty Events API v2
- Verify event_action is "trigger"
- Verify nested payload structure

**TestPagerDutyAction_WithCustomDetails**
- Create PagerDutyAction with custom_details
- Call Execute()
- Verify custom_details included in payload

**TestPagerDutyAction_WithoutCustomDetails**
- Create PagerDutyAction without custom_details
- Call Execute()
- Verify succeeds

**TestPagerDutyAction_WebhookError**
- Create PagerDutyAction
- Mock WebhookHandler.SendWebhook returns error
- Call Execute()
- Verify error is returned

**TestPagerDutyAction_Logging**
- Create PagerDutyAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with severity and runID

**TestPagerDutyAction_Name**
- Create PagerDutyAction
- Call Name()
- Assert returns "pagerduty"

**TestPagerDutyAction_SeverityLevels**
- Test different severity levels: "critical", "error", "warning", "info"
- Verify each is correctly included in payload

**TestPagerDutyAction_TemplateRendering**
- Create PagerDutyAction with templates in summary, source, custom_details
- Create ExecutionContext with Command, Args, Kwargs
- Call Execute()
- Verify all templates rendered before creating incident
- See detailed template tests in section 2

### KillAllInstancesAction Tests

**TestKillAllInstancesAction_Success**
- Create KillAllInstancesAction with job_id
- Create ExecutionContext with mock JobController
- Mock JobController.KillAllInstances returns nil
- Call Execute()
- Verify JobController.KillAllInstances was called with correct job ID
- Verify no error returned

**TestKillAllInstancesAction_ControllerError**
- Create KillAllInstancesAction
- Mock JobController.KillAllInstances returns error
- Call Execute()
- Verify error is returned

**TestKillAllInstancesAction_Logging**
- Create KillAllInstancesAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with jobID and runID

**TestKillAllInstancesAction_Name**
- Create KillAllInstancesAction
- Call Name()
- Assert returns "kill_all_instances"

**TestKillAllInstancesAction_NilJobController**
- Create KillAllInstancesAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call KillAllInstances

### KillLatestInstanceAction Tests

**TestKillLatestInstanceAction_Success**
- Create KillLatestInstanceAction with job_id
- Create ExecutionContext with mock JobController
- Mock JobController.KillLatestInstance returns nil
- Call Execute()
- Verify JobController.KillLatestInstance was called with correct job ID
- Verify no error returned

**TestKillLatestInstanceAction_ControllerError**
- Create KillLatestInstanceAction
- Mock JobController.KillLatestInstance returns error
- Call Execute()
- Verify error is returned

**TestKillLatestInstanceAction_Logging**
- Create KillLatestInstanceAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with jobID and runID

**TestKillLatestInstanceAction_Name**
- Create KillLatestInstanceAction
- Call Name()
- Assert returns "kill_latest_instance"

**TestKillLatestInstanceAction_NilJobController**
- Create KillLatestInstanceAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call KillLatestInstance

### SkipNextInstanceAction Tests

**TestSkipNextInstanceAction_Success**
- Create SkipNextInstanceAction with job_id
- Create ExecutionContext with mock JobController
- Mock JobController.SkipNextInstance returns nil
- Call Execute()
- Verify JobController.SkipNextInstance was called with correct job ID
- Verify no error returned

**TestSkipNextInstanceAction_ControllerError**
- Create SkipNextInstanceAction
- Mock JobController.SkipNextInstance returns error
- Call Execute()
- Verify error is returned

**TestSkipNextInstanceAction_Logging**
- Create SkipNextInstanceAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with jobID and runID

**TestSkipNextInstanceAction_Name**
- Create SkipNextInstanceAction
- Call Name()
- Assert returns "skip_next_instance"

**TestSkipNextInstanceAction_NilJobController**
- Create SkipNextInstanceAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call SkipNextInstance

### UpdateMetadataAction Tests

**TestUpdateMetadataAction_Success**
- Create UpdateMetadataAction with job_id and metadata
- Create ExecutionContext with mock MetadataUpdater
- Mock MetadataUpdater.UpdateMetadata returns nil
- Call Execute()
- Verify MetadataUpdater.UpdateMetadata was called with correct job ID and metadata
- Verify no error returned

**TestUpdateMetadataAction_WithTemplates**
- Create UpdateMetadataAction with templated metadata values
- ExecutionContext with JobID="job-123", RunID="run-456"
- Call Execute()
- Verify templates rendered in metadata before update

**TestUpdateMetadataAction_TemplateInJobID**
- Create UpdateMetadataAction with job_id="{{.JobID}}"
- ExecutionContext with JobID="job-123"
- Call Execute()
- Verify job_id rendered before calling UpdateMetadata

**TestUpdateMetadataAction_ComplexMetadata**
- Create UpdateMetadataAction with nested metadata structure
- Call Execute()
- Verify complex structure preserved after template rendering

**TestUpdateMetadataAction_KwargsInMetadata**
- Create UpdateMetadataAction with metadata={"env": "{{index .Kwargs \"env\"}}"}
- ExecutionContext with Kwargs={"env":"prod"}
- Call Execute()
- Verify kwargs rendered correctly in metadata

**TestUpdateMetadataAction_UpdaterError**
- Create UpdateMetadataAction
- Mock MetadataUpdater.UpdateMetadata returns error
- Call Execute()
- Verify error is returned

**TestUpdateMetadataAction_TemplateError**
- Create UpdateMetadataAction with invalid template in metadata
- Call Execute()
- Verify returns template error
- Verify metadata not updated

**TestUpdateMetadataAction_Logging**
- Create UpdateMetadataAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with jobID and runID

**TestUpdateMetadataAction_Name**
- Create UpdateMetadataAction
- Call Name()
- Assert returns "update_metadata"

**TestUpdateMetadataAction_NilMetadataUpdater**
- Create UpdateMetadataAction
- Create ExecutionContext with MetadataUpdater=nil
- Call Execute()
- Verify panics when trying to call UpdateMetadata

**TestUpdateMetadataAction_EmptyMetadata**
- Create UpdateMetadataAction with empty metadata map
- Call Execute()
- Verify still calls MetadataUpdater with empty map

**TestUpdateMetadataAction_AllFieldsTemplated**
- Create UpdateMetadataAction with all metadata fields using templates
- Call Execute()
- Verify all fields rendered correctly

### MetricAction Tests

**TestMetricAction_Success**
- Create MetricAction with name, value (float64), and tags
- Create ExecutionContext with mock MetricRecorder
- Mock MetricRecorder.RecordMetric returns nil
- Call Execute()
- Verify MetricRecorder.RecordMetric was called with correct name, value, tags
- Verify no error returned

**TestMetricAction_FloatValue**
- Create MetricAction with value=123.45
- Call Execute()
- Verify value passed as-is to RecordMetric

**TestMetricAction_StringValue**
- Create MetricAction with value="100.5"
- Call Execute()
- Verify string parsed to float64 and passed to RecordMetric

**TestMetricAction_TemplateInValue**
- Create MetricAction with value="{{index .Kwargs \"duration\"}}"
- ExecutionContext with Kwargs={"duration":"99.9"}
- Call Execute()
- Verify template rendered and parsed to float64

**TestMetricAction_TemplateInName**
- Create MetricAction with name="job.{{.JobName}}.duration"
- ExecutionContext with JobName="deploy"
- Call Execute()
- Verify metric name rendered to "job.deploy.duration"

**TestMetricAction_TemplateInTags**
- Create MetricAction with tags={"job": "{{.JobID}}", "env": "{{index .Kwargs \"env\"}}"}
- Call Execute()
- Verify all tag values rendered correctly

**TestMetricAction_EmptyTags**
- Create MetricAction with nil/empty tags
- Call Execute()
- Verify succeeds with empty tags map

**TestMetricAction_InvalidStringValue**
- Create MetricAction with value="not-a-number"
- Call Execute()
- Verify returns parse error

**TestMetricAction_InvalidTemplateInValue**
- Create MetricAction with value="{{.UndefinedField}}"
- Call Execute()
- Verify returns template error or parse error

**TestMetricAction_RecorderError**
- Create MetricAction
- Mock MetricRecorder.RecordMetric returns error
- Call Execute()
- Verify error is returned

**TestMetricAction_TemplateError**
- Create MetricAction with invalid template in name
- Call Execute()
- Verify returns template error
- Verify metric not recorded

**TestMetricAction_Logging**
- Create MetricAction
- Create ExecutionContext with mock logger
- Call Execute()
- Verify logger.Info was called with metric name and runID

**TestMetricAction_Name**
- Create MetricAction
- Call Name()
- Assert returns "metric"

**TestMetricAction_NilMetricRecorder**
- Create MetricAction
- Create ExecutionContext with MetricRecorder=nil
- Call Execute()
- Verify panics when trying to call RecordMetric

**TestMetricAction_ZeroValue**
- Create MetricAction with value=0.0
- Call Execute()
- Verify zero value is valid and recorded

**TestMetricAction_NegativeValue**
- Create MetricAction with value=-10.5
- Call Execute()
- Verify negative value is valid and recorded

**TestMetricAction_LargeValue**
- Create MetricAction with value=1e10
- Call Execute()
- Verify large value handled correctly

**TestMetricAction_IntegerValue**
- Create MetricAction with value=42 (will be JSON float64)
- Call Execute()
- Verify integer parsed as float64

**TestMetricAction_TemplateErrorInTags**
- Create MetricAction with invalid template in tag value
- Call Execute()
- Verify returns template error

## 2. Template Rendering Tests

### renderTemplate Function Tests

**TestRenderTemplate_SimpleVariable**
- Template: "Job {{.JobID}}"
- TemplateData with JobID="job-123"
- Verify renders to "Job job-123"

**TestRenderTemplate_MultipleVariables**
- Template: "Job {{.JobID}} Run {{.RunID}}"
- TemplateData with JobID="job-123", RunID="run-456"
- Verify renders correctly

**TestRenderTemplate_Command**
- Template: "Command: {{.Command}}"
- TemplateData with Command="deploy.sh"
- Verify renders to "Command: deploy.sh"

**TestRenderTemplate_KwargsIndex**
- Template: "Env: {{index .Kwargs \"env\"}}"
- TemplateData with Kwargs={"env":"prod"}
- Verify renders to "Env: prod"

**TestRenderTemplate_ArgsIndex**
- Template: "First: {{index .Args 0}}"
- TemplateData with Args=["arg1", "arg2"]
- Verify renders to "First: arg1"

**TestRenderTemplate_MissingKwarg**
- Template: "{{index .Kwargs \"missing\"}}"
- TemplateData with empty Kwargs
- Verify either renders empty or returns error

**TestRenderTemplate_OutOfBoundsArgs**
- Template: "{{index .Args 5}}"
- TemplateData with Args=["arg1"]
- Verify returns error (index out of bounds)

**TestRenderTemplate_InvalidSyntax**
- Template: "{{.JobID"
- Verify returns parse error
- Verify error message mentions syntax

**TestRenderTemplate_UndefinedVariable**
- Template: "{{.UndefinedField}}"
- Verify returns error or renders empty

**TestRenderTemplate_Timestamp**
- Template: "{{.Timestamp}}"
- Verify timestamp is rendered
- Verify format is valid

**TestRenderTemplate_ConstraintInfo**
- Template: "Constraint {{.ConstraintName}} is {{.ConstraintStatus}}"
- TemplateData with ConstraintName="deadline", ConstraintStatus="violated"
- Verify renders correctly

**TestRenderTemplate_NoTemplateVariables**
- Template: "Plain text with no variables"
- Verify renders unchanged

**TestRenderTemplate_EscapedBraces**
- Template with literal braces
- Verify handles escaping correctly

**TestRenderTemplate_URLEncoding**
- Template: "https://api.com?job={{.JobID}}&run={{.RunID}}"
- Verify renders valid URL

**TestRenderTemplate_SpecialCharacters**
- Template with special characters in variables
- TemplateData with values containing &, =, spaces
- Verify renders correctly

### renderPayload Function Tests

**TestRenderPayload_SimpleObject**
- Payload: {"job": "{{.JobID}}"}
- TemplateData with JobID="job-123"
- Verify payload becomes {"job": "job-123"}

**TestRenderPayload_NestedObject**
- Payload: {"job": {"id": "{{.JobID}}", "name": "{{.JobName}}"}}
- Verify nested fields rendered correctly

**TestRenderPayload_Array**
- Payload: ["{{.JobID}}", "{{.RunID}}"]
- Verify array elements rendered correctly

**TestRenderPayload_MixedTypes**
- Payload: {"id": "{{.JobID}}", "count": 123, "active": true}
- Verify only string fields rendered, other types unchanged

**TestRenderPayload_MultipleKwargs**
- Payload: {"env": "{{index .Kwargs \"env\"}}", "region": "{{index .Kwargs \"region\"}}"}
- TemplateData with Kwargs={"env":"prod", "region":"us-west-2"}
- Verify all kwargs rendered

**TestRenderPayload_NullPayload**
- Payload: nil
- Verify handles gracefully

**TestRenderPayload_EmptyObject**
- Payload: {}
- Verify returns empty object

**TestRenderPayload_InvalidJSON**
- Payload with circular reference or unmarshalable type
- Verify returns error

**TestRenderPayload_TemplateError**
- Payload: {"field": "{{.UndefinedField}}"}
- Verify returns template error

### buildTemplateData Function Tests

**TestBuildTemplateData_AllFields**
- Create ExecutionContext with all fields populated
- Call buildTemplateData()
- Verify TemplateData contains all expected fields
- Verify JobID, JobName, RunID, Command, Args, Kwargs copied correctly

**TestBuildTemplateData_EmptyKwargs**
- ExecutionContext with nil/empty Kwargs
- Verify TemplateData has empty Kwargs map

**TestBuildTemplateData_EmptyArgs**
- ExecutionContext with nil/empty Args
- Verify TemplateData has empty Args slice

**TestBuildTemplateData_Timestamp**
- Call buildTemplateData()
- Verify Timestamp is set to current time (approximately)

**TestBuildTemplateData_JobInfo**
- ExecutionContext with Job containing ID and Name
- Verify JobID and JobName extracted correctly

### WebhookAction Template Tests

**TestWebhookAction_TemplateInURL**
- WebhookAction with url="https://api.com/jobs/{{.JobID}}"
- ExecutionContext with JobID="job-123"
- Call Execute()
- Verify webhook sent to "https://api.com/jobs/job-123"

**TestWebhookAction_TemplateInPayload**
- WebhookAction with payload={"job": "{{.JobID}}", "run": "{{.RunID}}"}
- ExecutionContext with JobID="job-123", RunID="run-456"
- Call Execute()
- Verify payload rendered correctly

**TestWebhookAction_KwargsInURL**
- WebhookAction with url="https://api.com?env={{index .Kwargs \"env\"}}"
- ExecutionContext with Kwargs={"env":"prod"}
- Call Execute()
- Verify URL rendered correctly

**TestWebhookAction_ArgsInPayload**
- WebhookAction with payload={"first": "{{index .Args 0}}"}
- ExecutionContext with Args=["value1"]
- Call Execute()
- Verify payload rendered correctly

**TestWebhookAction_TemplateError**
- WebhookAction with invalid template in URL
- Call Execute()
- Verify returns template error
- Verify webhook not sent

**TestWebhookAction_NoTemplates**
- WebhookAction with plain URL and payload (no templates)
- Call Execute()
- Verify works normally

### SlackAction Template Tests

**TestSlackAction_TemplateInText**
- SlackAction with text="Job {{.JobName}} completed"
- ExecutionContext with JobName="deploy-prod"
- Call Execute()
- Verify text rendered in payload

**TestSlackAction_TemplateInChannel**
- SlackAction with channel="#{{index .Kwargs \"env\"}}-alerts"
- ExecutionContext with Kwargs={"env":"prod"}
- Call Execute()
- Verify channel rendered to "#prod-alerts"

**TestSlackAction_TemplateInUsername**
- SlackAction with username="Bot-{{.JobID}}"
- Call Execute()
- Verify username rendered correctly

**TestSlackAction_AllFieldsTemplated**
- SlackAction with templates in text, channel, username, webhook_url
- Call Execute()
- Verify all fields rendered correctly

**TestSlackAction_TemplateError**
- SlackAction with invalid template in text
- Call Execute()
- Verify returns error
- Verify webhook not sent

**TestSlackAction_MixedTemplateAndStatic**
- SlackAction with some templated fields, some static
- Call Execute()
- Verify static fields unchanged, templates rendered

### PagerDutyAction Template Tests

**TestPagerDutyAction_TemplateInSummary**
- PagerDutyAction with summary="Job {{.JobName}} failed"
- ExecutionContext with JobName="backup"
- Call Execute()
- Verify summary rendered in payload

**TestPagerDutyAction_TemplateInSource**
- PagerDutyAction with source="itinerary-{{index .Kwargs \"cluster\"}}"
- ExecutionContext with Kwargs={"cluster":"prod-us"}
- Call Execute()
- Verify source rendered correctly

**TestPagerDutyAction_TemplateInCustomDetails**
- PagerDutyAction with custom_details={"job": "{{.JobID}}", "command": "{{.Command}}"}
- Call Execute()
- Verify custom_details fully rendered

**TestPagerDutyAction_NestedTemplatesInCustomDetails**
- PagerDutyAction with nested custom_details structure containing templates
- Call Execute()
- Verify all nested templates rendered

**TestPagerDutyAction_TemplateError**
- PagerDutyAction with invalid template
- Call Execute()
- Verify returns error
- Verify incident not created

**TestPagerDutyAction_NoTemplates**
- PagerDutyAction with no template variables
- Call Execute()
- Verify works normally

### Integration Template Tests

**TestTemplateWithMultipleActions**
- Create multiple webhook actions with different templates
- Execute all with same ExecutionContext
- Verify each renders independently and correctly

**TestTemplateWithComplexKwargs**
- ExecutionContext with complex nested Kwargs
- Verify can access nested values in templates

**TestTemplateWithSpecialCharsInValues**
- ExecutionContext with values containing quotes, newlines, special chars
- Verify templates render safely without breaking JSON

**TestTemplateWithEmptyValues**
- ExecutionContext with empty strings in Command, Args, etc.
- Verify templates render empty strings correctly

**TestTemplateWithUnicodeValues**
- ExecutionContext with unicode characters in values
- Verify templates preserve unicode correctly

### UpdateMetadataAction Template Tests

**TestUpdateMetadataAction_TemplateInMetadata**
- UpdateMetadataAction with metadata={"last_run": "{{.RunID}}", "env": "{{index .Kwargs \"env\"}}"}
- ExecutionContext with RunID="run-123", Kwargs={"env":"prod"}
- Call Execute()
- Verify metadata values rendered correctly before update

**TestUpdateMetadataAction_TemplateInJobID**
- UpdateMetadataAction with job_id="{{.JobID}}"
- ExecutionContext with JobID="job-456"
- Call Execute()
- Verify job_id rendered before calling UpdateMetadata

**TestUpdateMetadataAction_NestedMetadata**
- UpdateMetadataAction with nested metadata containing templates
- Call Execute()
- Verify all nested values rendered

**TestUpdateMetadataAction_TemplateError**
- UpdateMetadataAction with invalid template
- Call Execute()
- Verify returns error
- Verify metadata not updated

### MetricAction Template Tests

**TestMetricAction_TemplateInName**
- MetricAction with name="job.{{.JobName}}.duration"
- ExecutionContext with JobName="deploy"
- Call Execute()
- Verify metric name rendered to "job.deploy.duration"

**TestMetricAction_TemplateInStringValue**
- MetricAction with value="{{index .Kwargs \"duration\"}}"
- ExecutionContext with Kwargs={"duration":"123.45"}
- Call Execute()
- Verify value rendered and parsed to float64

**TestMetricAction_TemplateInTags**
- MetricAction with tags={"job": "{{.JobID}}", "run": "{{.RunID}}"}
- Call Execute()
- Verify all tag values rendered correctly

**TestMetricAction_TemplateErrorInName**
- MetricAction with invalid template in name
- Call Execute()
- Verify returns error
- Verify metric not recorded

**TestMetricAction_TemplateErrorInValue**
- MetricAction with invalid template in value
- Call Execute()
- Verify returns error

**TestMetricAction_ValueNotParseable**
- MetricAction with value="{{.Command}}" rendering to non-numeric string
- Call Execute()
- Verify returns parse error

## 3. Configuration Parsing Tests

### CreateAction Factory Tests

**TestCreateAction_DelayAction**
- Create ActionConfig with type="delay", config={"duration":"1h"}
- Call CreateAction()
- Verify returns DelayAction with duration=1h
- Verify no error

**TestCreateAction_DelayAction_InvalidDuration**
- Create ActionConfig with type="delay", config={"duration":"invalid"}
- Call CreateAction()
- Verify returns error
- Verify error message mentions invalid duration

**TestCreateAction_WebhookAction**
- Create ActionConfig with type="webhook", config={"url":"http://example.com","payload":{"key":"value"}}
- Call CreateAction()
- Verify returns WebhookAction with correct URL and payload
- Verify no error

**TestCreateAction_WebhookAction_MissingURL**
- Create ActionConfig with type="webhook", config={"payload":{}}
- Call CreateAction()
- Verify returns error or WebhookAction with empty URL

**TestCreateAction_LogAction**
- Create ActionConfig with type="log", config={"message":"test"}
- Call CreateAction()
- Verify returns LogAction with correct message
- Verify no error

**TestCreateAction_FailAction**
- Create ActionConfig with type="fail", config={"reason":"test failure"}
- Call CreateAction()
- Verify returns FailAction with correct reason
- Verify no error

**TestCreateAction_NoOpAction**
- Create ActionConfig with type="noop", config={}
- Call CreateAction()
- Verify returns NoOpAction
- Verify no error

**TestCreateAction_RetryAction**
- Create ActionConfig with type="retry", config={}
- Call CreateAction()
- Verify returns RetryAction
- Verify no error

**TestCreateAction_TriggerJobAction**
- Create ActionConfig with type="trigger_job", config={"job_id":"job-123","args":{"key":"value"}}
- Call CreateAction()
- Verify returns TriggerJobAction with correct job_id and args
- Verify no error

**TestCreateAction_TriggerJobAction_MissingJobID**
- Create ActionConfig with type="trigger_job", config={"args":{}}
- Call CreateAction()
- Verify returns error
- Verify error message mentions job_id required

**TestCreateAction_SlackAction**
- Create ActionConfig with type="slack", config={"webhook_url":"https://hooks.slack.com/...","text":"alert"}
- Call CreateAction()
- Verify returns SlackAction with correct fields
- Verify no error

**TestCreateAction_SlackAction_MissingWebhookURL**
- Create ActionConfig with type="slack", config={"text":"alert"}
- Call CreateAction()
- Verify returns error
- Verify error message mentions webhook_url required

**TestCreateAction_PagerDutyAction**
- Create ActionConfig with type="pagerduty", config={"routing_key":"key","severity":"error","summary":"alert"}
- Call CreateAction()
- Verify returns PagerDutyAction with correct fields
- Verify no error

**TestCreateAction_PagerDutyAction_MissingRoutingKey**
- Create ActionConfig with type="pagerduty", config={"severity":"error"}
- Call CreateAction()
- Verify returns error
- Verify error message mentions routing_key required

**TestCreateAction_KillAllInstancesAction**
- Create ActionConfig with type="kill_all_instances", config={"job_id":"job-123"}
- Call CreateAction()
- Verify returns KillAllInstancesAction with correct job_id
- Verify no error

**TestCreateAction_KillAllInstancesAction_MissingJobID**
- Create ActionConfig with type="kill_all_instances", config={}
- Call CreateAction()
- Verify returns error
- Verify error message mentions job_id required

**TestCreateAction_KillLatestInstanceAction**
- Create ActionConfig with type="kill_latest_instance", config={"job_id":"job-123"}
- Call CreateAction()
- Verify returns KillLatestInstanceAction with correct job_id
- Verify no error

**TestCreateAction_KillLatestInstanceAction_MissingJobID**
- Create ActionConfig with type="kill_latest_instance", config={}
- Call CreateAction()
- Verify returns error
- Verify error message mentions job_id required

**TestCreateAction_SkipNextInstanceAction**
- Create ActionConfig with type="skip_next_instance", config={"job_id":"job-123"}
- Call CreateAction()
- Verify returns SkipNextInstanceAction with correct job_id
- Verify no error

**TestCreateAction_SkipNextInstanceAction_MissingJobID**
- Create ActionConfig with type="skip_next_instance", config={}
- Call CreateAction()
- Verify returns error
- Verify error message mentions job_id required

**TestCreateAction_UpdateMetadataAction**
- Create ActionConfig with type="update_metadata", config={"job_id":"job-123","metadata":{"key":"value"}}
- Call CreateAction()
- Verify returns UpdateMetadataAction with correct job_id and metadata
- Verify no error

**TestCreateAction_UpdateMetadataAction_MissingJobID**
- Create ActionConfig with type="update_metadata", config={"metadata":{}}
- Call CreateAction()
- Verify returns error
- Verify error message mentions job_id required

**TestCreateAction_UpdateMetadataAction_MissingMetadata**
- Create ActionConfig with type="update_metadata", config={"job_id":"job-123"}
- Call CreateAction()
- Verify returns error
- Verify error message mentions metadata required

**TestCreateAction_UpdateMetadataAction_EmptyMetadata**
- Create ActionConfig with type="update_metadata", config={"job_id":"job-123","metadata":{}}
- Call CreateAction()
- Verify returns error
- Verify error message mentions metadata must not be empty

**TestCreateAction_MetricAction**
- Create ActionConfig with type="metric", config={"name":"metric.name","value":123.45,"tags":{"env":"prod"}}
- Call CreateAction()
- Verify returns MetricAction with correct name, value, tags
- Verify no error

**TestCreateAction_MetricAction_StringValue**
- Create ActionConfig with type="metric", config={"name":"metric.name","value":"100"}
- Call CreateAction()
- Verify returns MetricAction
- Verify value stored as string
- Verify no error

**TestCreateAction_MetricAction_MissingName**
- Create ActionConfig with type="metric", config={"value":100}
- Call CreateAction()
- Verify returns error
- Verify error message mentions name required

**TestCreateAction_MetricAction_MissingValue**
- Create ActionConfig with type="metric", config={"name":"metric.name"}
- Call CreateAction()
- Verify returns error
- Verify error message mentions value required

**TestCreateAction_MetricAction_InvalidValueType**
- Create ActionConfig with type="metric", config={"name":"metric.name","value":true}
- Call CreateAction()
- Verify returns error
- Verify error message mentions value type

**TestCreateAction_MetricAction_EmptyTags**
- Create ActionConfig with type="metric", config={"name":"metric.name","value":100,"tags":{}}
- Call CreateAction()
- Verify returns MetricAction with empty tags map
- Verify no error

**TestCreateAction_UnknownType**
- Create ActionConfig with type="unknown"
- Call CreateAction()
- Verify returns error
- Verify error message is "unknown action type: unknown"

**TestCreateAction_InvalidJSON**
- Create ActionConfig with type="delay", config=invalid JSON
- Call CreateAction()
- Verify returns error
- Verify error mentions JSON parsing

**TestCreateAction_EmptyConfig**
- Create ActionConfig with type="delay", config={}
- Call CreateAction()
- Verify returns error (missing duration)

**TestCreateAction_ExtraFields**
- Create ActionConfig with extra unknown fields
- Call CreateAction()
- Verify succeeds (JSON unmarshaling ignores extra fields)

### parseDelayAction Tests

**TestParseDelayAction_ValidDurations**
- Test parsing various valid durations: "1s", "5m", "2h", "24h", "1h30m"
- Verify each parses correctly
- Verify resulting duration is accurate

**TestParseDelayAction_InvalidDuration**
- Test invalid durations: "invalid", "1x", "", "1.5h"
- Verify returns error for each

**TestParseDelayAction_NegativeDuration**
- Parse duration="-1h"
- Verify either errors or creates action with negative duration

**TestParseDelayAction_MissingDuration**
- Parse config without duration field
- Verify returns error

### parseWebhookAction Tests

**TestParseWebhookAction_SimplePayload**
- Parse webhook with string payload
- Verify payload stored correctly

**TestParseWebhookAction_ComplexPayload**
- Parse webhook with nested map/array payload
- Verify payload stored correctly

**TestParseWebhookAction_NullPayload**
- Parse webhook with null payload
- Verify payload is nil

**TestParseWebhookAction_MissingURL**
- Parse config without URL field
- Verify returns error or creates action with empty URL

### parseLogAction Tests

**TestParseLogAction_ValidMessage**
- Parse log action with message
- Verify message stored correctly

**TestParseLogAction_EmptyMessage**
- Parse log action with empty message
- Verify creates action (empty message is valid)

**TestParseLogAction_MissingMessage**
- Parse config without message field
- Verify returns error or creates action with empty message

### parseFailAction Tests

**TestParseFailAction_ValidReason**
- Parse fail action with reason
- Verify reason stored correctly

**TestParseFailAction_EmptyReason**
- Parse fail action with empty reason
- Verify creates action (empty reason is valid)

**TestParseFailAction_MissingReason**
- Parse config without reason field
- Verify returns error or creates action with empty reason

### parseTriggerJobAction Tests

**TestParseTriggerJobAction_ValidConfig**
- Parse trigger_job action with job_id and args
- Verify job_id stored correctly
- Verify args stored correctly

**TestParseTriggerJobAction_EmptyArgs**
- Parse trigger_job action with empty args
- Verify creates action with nil/empty args

**TestParseTriggerJobAction_ComplexArgs**
- Parse trigger_job action with nested map args
- Verify complex args structure preserved

**TestParseTriggerJobAction_MissingJobID**
- Parse config without job_id field
- Verify returns error
- Verify error message mentions job_id required

**TestParseTriggerJobAction_EmptyJobID**
- Parse config with job_id=""
- Verify returns error

### parseSlackAction Tests

**TestParseSlackAction_AllFields**
- Parse slack action with all fields
- Verify webhook_url, channel, username, text, icon_emoji stored correctly

**TestParseSlackAction_MinimalFields**
- Parse slack action with only webhook_url and text
- Verify creates action with minimal fields

**TestParseSlackAction_MissingWebhookURL**
- Parse config without webhook_url field
- Verify returns error
- Verify error message mentions webhook_url required

**TestParseSlackAction_EmptyWebhookURL**
- Parse config with webhook_url=""
- Verify returns error

**TestParseSlackAction_OptionalFields**
- Parse slack action without channel, username, icon_emoji
- Verify creates action with empty optional fields

### parsePagerDutyAction Tests

**TestParsePagerDutyAction_AllFields**
- Parse pagerduty action with all fields
- Verify routing_key, severity, summary, source, custom_details stored correctly

**TestParsePagerDutyAction_MinimalFields**
- Parse pagerduty action with only routing_key
- Verify creates action with minimal fields

**TestParsePagerDutyAction_MissingRoutingKey**
- Parse config without routing_key field
- Verify returns error
- Verify error message mentions routing_key required

**TestParsePagerDutyAction_EmptyRoutingKey**
- Parse config with routing_key=""
- Verify returns error

**TestParsePagerDutyAction_ComplexCustomDetails**
- Parse pagerduty action with nested custom_details
- Verify custom_details structure preserved

**TestParsePagerDutyAction_NullCustomDetails**
- Parse pagerduty action with custom_details=null
- Verify creates action with nil custom_details

### parseKillAllInstancesAction Tests

**TestParseKillAllInstancesAction_ValidJobID**
- Parse kill_all_instances action with job_id
- Verify job_id stored correctly

**TestParseKillAllInstancesAction_MissingJobID**
- Parse config without job_id field
- Verify returns error
- Verify error message mentions job_id required

**TestParseKillAllInstancesAction_EmptyJobID**
- Parse config with job_id=""
- Verify returns error

### parseKillLatestInstanceAction Tests

**TestParseKillLatestInstanceAction_ValidJobID**
- Parse kill_latest_instance action with job_id
- Verify job_id stored correctly

**TestParseKillLatestInstanceAction_MissingJobID**
- Parse config without job_id field
- Verify returns error
- Verify error message mentions job_id required

**TestParseKillLatestInstanceAction_EmptyJobID**
- Parse config with job_id=""
- Verify returns error

### parseSkipNextInstanceAction Tests

**TestParseSkipNextInstanceAction_ValidJobID**
- Parse skip_next_instance action with job_id
- Verify job_id stored correctly

**TestParseSkipNextInstanceAction_MissingJobID**
- Parse config without job_id field
- Verify returns error
- Verify error message mentions job_id required

**TestParseSkipNextInstanceAction_EmptyJobID**
- Parse config with job_id=""
- Verify returns error

### parseUpdateMetadataAction Tests

**TestParseUpdateMetadataAction_ValidConfig**
- Parse update_metadata action with job_id and metadata
- Verify job_id stored correctly
- Verify metadata stored correctly

**TestParseUpdateMetadataAction_ComplexMetadata**
- Parse update_metadata action with nested metadata
- Verify complex metadata structure preserved

**TestParseUpdateMetadataAction_MissingJobID**
- Parse config without job_id field
- Verify returns error
- Verify error message mentions job_id required

**TestParseUpdateMetadataAction_EmptyJobID**
- Parse config with job_id=""
- Verify returns error

**TestParseUpdateMetadataAction_MissingMetadata**
- Parse config without metadata field
- Verify returns error
- Verify error message mentions metadata required

**TestParseUpdateMetadataAction_EmptyMetadata**
- Parse config with metadata={}
- Verify returns error
- Verify error message mentions metadata must not be empty

**TestParseUpdateMetadataAction_NullMetadata**
- Parse config with metadata=null
- Verify returns error

**TestParseUpdateMetadataAction_MetadataWithTemplates**
- Parse update_metadata action with template strings in metadata
- Verify templates stored as-is (not rendered during parse)

### parseMetricAction Tests

**TestParseMetricAction_ValidFloat**
- Parse metric action with float value
- Verify name, value, tags stored correctly

**TestParseMetricAction_ValidString**
- Parse metric action with string value
- Verify string value stored correctly

**TestParseMetricAction_WithTags**
- Parse metric action with multiple tags
- Verify all tags stored correctly

**TestParseMetricAction_EmptyTags**
- Parse metric action without tags field
- Verify creates action with empty tags map

**TestParseMetricAction_MissingName**
- Parse config without name field
- Verify returns error
- Verify error message mentions name required

**TestParseMetricAction_EmptyName**
- Parse config with name=""
- Verify returns error

**TestParseMetricAction_MissingValue**
- Parse config without value field
- Verify returns error
- Verify error message mentions value required

**TestParseMetricAction_NullValue**
- Parse config with value=null
- Verify returns error

**TestParseMetricAction_InvalidValueType**
- Parse config with value=true (boolean)
- Verify returns error
- Verify error message mentions value type

**TestParseMetricAction_ArrayValueType**
- Parse config with value=[1,2,3]
- Verify returns error

**TestParseMetricAction_ObjectValueType**
- Parse config with value={"nested":"object"}
- Verify returns error

**TestParseMetricAction_ZeroValue**
- Parse metric action with value=0
- Verify creates action (zero is valid)

**TestParseMetricAction_NegativeValue**
- Parse metric action with value=-10.5
- Verify creates action (negative is valid)

**TestParseMetricAction_TemplateInValue**
- Parse metric action with value="{{.Variable}}"
- Verify template string stored as-is

## 3. Error Handling Tests

**TestAction_ExecuteWithNilContext**
- Create any action
- Call Execute(nil)
- Verify panics or returns error

**TestAction_ExecuteWithPartialContext**
- Create action that requires specific context fields
- Create ExecutionContext with some fields nil
- Call Execute()
- Verify handles gracefully (or errors appropriately)

**TestDelayAction_NilContextContext**
- Create DelayAction
- Create ExecutionContext with Context=nil
- Call Execute()
- Verify panics when trying to select on nil context

**TestWebhookAction_NilHTTPClient**
- Create WebhookAction
- Create ExecutionContext with HTTPClient=nil
- Call Execute()
- Verify panics or returns error

**TestLogAction_NilLogger**
- Create LogAction
- Create ExecutionContext with Logger=nil
- Call Execute()
- Verify panics when trying to log

**TestRetryAction_NilJobController**
- Create RetryAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call RetryJob

**TestTriggerJobAction_NilJobController**
- Create TriggerJobAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call TriggerJob

**TestKillAllInstancesAction_NilJobController**
- Create KillAllInstancesAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call KillAllInstances

**TestKillLatestInstanceAction_NilJobController**
- Create KillLatestInstanceAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call KillLatestInstance

**TestSkipNextInstanceAction_NilJobController**
- Create SkipNextInstanceAction
- Create ExecutionContext with JobController=nil
- Call Execute()
- Verify panics when trying to call SkipNextInstance

**TestSlackAction_NilWebhookHandler**
- Create SlackAction
- Create ExecutionContext with WebhookHandler=nil
- Call Execute()
- Verify panics or returns error

**TestPagerDutyAction_NilWebhookHandler**
- Create PagerDutyAction
- Create ExecutionContext with WebhookHandler=nil
- Call Execute()
- Verify panics or returns error

**TestUpdateMetadataAction_NilMetadataUpdater**
- Create UpdateMetadataAction
- Create ExecutionContext with MetadataUpdater=nil
- Call Execute()
- Verify panics when trying to call UpdateMetadata

**TestMetricAction_NilMetricRecorder**
- Create MetricAction
- Create ExecutionContext with MetricRecorder=nil
- Call Execute()
- Verify panics when trying to call RecordMetric

## 4. Context Cancellation Tests

**TestDelayAction_EarlyCancellation**
- Create DelayAction with long duration (10s)
- Cancel context after 10ms
- Verify Execute() returns within 50ms
- Verify returns context.Canceled

**TestDelayAction_CancellationBeforeStart**
- Create DelayAction
- Create already-cancelled context
- Call Execute()
- Verify returns immediately with context.Canceled

**TestWebhookAction_CancellationDuringRequest**
- Create WebhookAction
- Create context with timeout
- Make HTTP request that takes longer than timeout
- Verify Execute() respects context timeout

## 5. ExecutionContext Tests

**TestExecutionContext_Complete**
- Create ExecutionContext with all fields populated
- Verify all fields accessible
- Verify no nil pointers

**TestExecutionContext_MinimalForDelayAction**
- Create ExecutionContext with only fields needed for DelayAction
- Call DelayAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForWebhookAction**
- Create ExecutionContext with only fields needed for WebhookAction
- Call WebhookAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForLogAction**
- Create ExecutionContext with only fields needed for LogAction
- Call LogAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForRetryAction**
- Create ExecutionContext with only fields needed for RetryAction
- Call RetryAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForTriggerJobAction**
- Create ExecutionContext with only fields needed for TriggerJobAction
- Call TriggerJobAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForSlackAction**
- Create ExecutionContext with only fields needed for SlackAction
- Call SlackAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForPagerDutyAction**
- Create ExecutionContext with only fields needed for PagerDutyAction
- Call PagerDutyAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForKillAllInstancesAction**
- Create ExecutionContext with only fields needed for KillAllInstancesAction
- Call KillAllInstancesAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForKillLatestInstanceAction**
- Create ExecutionContext with only fields needed for KillLatestInstanceAction
- Call KillLatestInstanceAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForSkipNextInstanceAction**
- Create ExecutionContext with only fields needed for SkipNextInstanceAction
- Call SkipNextInstanceAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForUpdateMetadataAction**
- Create ExecutionContext with only fields needed for UpdateMetadataAction
- Call UpdateMetadataAction.Execute()
- Verify succeeds

**TestExecutionContext_MinimalForMetricAction**
- Create ExecutionContext with only fields needed for MetricAction
- Call MetricAction.Execute()
- Verify succeeds

**TestExecutionContext_JobInfo**
- Create ExecutionContext with job details
- Verify job ID, name, and runID are accessible
- Actions may use these in logging/payloads

**TestExecutionContext_CommandInfo**
- Create ExecutionContext with Command, Args, Kwargs
- Verify all command fields accessible
- Verify can be used in template rendering

**TestExecutionContext_EmptyArgs**
- Create ExecutionContext with nil Args
- Verify template rendering handles gracefully

**TestExecutionContext_EmptyKwargs**
- Create ExecutionContext with nil Kwargs
- Verify template rendering handles gracefully

**TestExecutionContext_ComplexKwargs**
- Create ExecutionContext with many kwargs
- Verify all accessible via index in templates

## 7. Action Factory Tests

**TestActionFactory_AllTypes**
- For each action type (delay, webhook, log, fail, noop, retry, trigger_job, slack, pagerduty, kill_all_instances, kill_latest_instance, skip_next_instance, update_metadata, metric)
- Create ActionConfig
- Call CreateAction()
- Verify correct concrete type returned
- Verify implements Action interface

**TestActionFactory_TypeCaseSensitivity**
- Test type="DELAY", "Delay", "DeLaY"
- Verify either case-insensitive or returns error

## 7. Thread Safety Tests

**TestConcurrentActionExecution**
- Create 100 different action instances
- Execute all concurrently in goroutines
- Verify all complete without panic
- Verify no data races (use go test -race)

**TestConcurrentSameActionExecution**
- Create single action instance
- Execute same instance 100 times concurrently
- Verify all complete without panic
- Verify no data races
- Actions should be stateless

**TestConcurrentCreateAction**
- Call CreateAction() 100 times concurrently
- Verify all succeed
- Verify no data races

## 8. Integration Tests

**TestActionsWithRealContext**
- Create ExecutionContext with real context.Context, real logger
- Test each action type executes successfully
- Verify logs appear correctly
- Verify webhooks can be sent (to test server)

**TestMultipleActionsSequence**
- Execute multiple actions in sequence
- Verify each completes independently
- Verify if one fails, others still execute

**TestActionExecutionTime**
- Create DelayAction with known duration
- Measure execution time
- Verify execution time is approximately correct (±10%)

## Test Utilities

### Mock HTTP Client
- Implement http.RoundTripper interface
- Allow configuring response status, body, error
- Track requests made (URL, method, body)

### Mock Logger
- Implement slog.Handler interface
- Track log calls (level, message, attributes)
- Allow asserting on log output

### Mock JobController
- Implement JobController interface
- Allow configuring return values for each method
- Track method calls (RetryJob, TriggerJob, KillAllInstances, KillLatestInstance, SkipNextInstance)
- Allow asserting on arguments passed to methods

### Mock MetadataUpdater
- Implement MetadataUpdater interface
- Allow configuring return values
- Track calls to UpdateMetadata
- Allow asserting on job ID and metadata passed

### Mock MetricRecorder
- Implement MetricRecorder interface
- Allow configuring return values
- Track calls to RecordMetric
- Allow asserting on metric name, value, and tags passed

### ExecutionContext Builder
- Helper to create ExecutionContext with sensible defaults
- Allow overriding specific fields for testing
- Include mock JobController, MetadataUpdater, MetricRecorder by default

### Action Test Helpers
- Helper to create basic ActionConfig JSON
- Helper to assert action interface compliance
- Helper to measure action execution time

## Coverage Goals

- 100% line coverage for all action implementations
- 100% branch coverage for CreateAction and parse functions
- All error paths tested
- All context cancellation paths tested
