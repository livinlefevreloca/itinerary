package actions

import (
	"context"
	"log/slog"

	"github.com/livinlefevreloca/itinerary/internal/model"
	"github.com/livinlefevreloca/itinerary/internal/testutil"
)

// ExecutionContextBuilder helps create ExecutionContext for testing
type ExecutionContextBuilder struct {
	job             *model.Job
	runID           string
	command         string
	args            []string
	kwargs          map[string]string
	webhookHandler  model.WebhookSender
	jobController   model.JobController
	metadataUpdater model.MetadataUpdater
	metricRecorder  model.MetricRecorder
	logger          *slog.Logger
	ctx             context.Context
}

func NewExecutionContextBuilder() *ExecutionContextBuilder {
	return &ExecutionContextBuilder{
		job: &model.Job{
			ID:   "test-job-id",
			Name: "test-job",
		},
		runID:           "test-run-id",
		command:         "test-command",
		args:            []string{},
		kwargs:          make(map[string]string),
		webhookHandler:  testutil.NewMockWebhookHandler(),
		jobController:   testutil.NewMockJobController(),
		metadataUpdater: testutil.NewMockMetadataUpdater(),
		metricRecorder:  testutil.NewMockMetricRecorder(),
		logger:          slog.Default(),
		ctx:             context.Background(),
	}
}

func (b *ExecutionContextBuilder) WithJob(job *model.Job) *ExecutionContextBuilder {
	b.job = job
	return b
}

func (b *ExecutionContextBuilder) WithRunID(runID string) *ExecutionContextBuilder {
	b.runID = runID
	return b
}

func (b *ExecutionContextBuilder) WithCommand(command string) *ExecutionContextBuilder {
	b.command = command
	return b
}

func (b *ExecutionContextBuilder) WithArgs(args []string) *ExecutionContextBuilder {
	b.args = args
	return b
}

func (b *ExecutionContextBuilder) WithKwargs(kwargs map[string]string) *ExecutionContextBuilder {
	b.kwargs = kwargs
	return b
}

func (b *ExecutionContextBuilder) WithWebhookHandler(handler model.WebhookSender) *ExecutionContextBuilder {
	b.webhookHandler = handler
	return b
}

func (b *ExecutionContextBuilder) WithJobController(controller model.JobController) *ExecutionContextBuilder {
	b.jobController = controller
	return b
}

func (b *ExecutionContextBuilder) WithMetadataUpdater(updater model.MetadataUpdater) *ExecutionContextBuilder {
	b.metadataUpdater = updater
	return b
}

func (b *ExecutionContextBuilder) WithMetricRecorder(recorder model.MetricRecorder) *ExecutionContextBuilder {
	b.metricRecorder = recorder
	return b
}

func (b *ExecutionContextBuilder) WithLogger(logger *slog.Logger) *ExecutionContextBuilder {
	b.logger = logger
	return b
}

func (b *ExecutionContextBuilder) WithContext(ctx context.Context) *ExecutionContextBuilder {
	b.ctx = ctx
	return b
}

func (b *ExecutionContextBuilder) Build() *model.ExecutionContext {
	return &model.ExecutionContext{
		Job:             b.job,
		RunID:           b.runID,
		Command:         b.command,
		Args:            b.args,
		Kwargs:          b.kwargs,
		WebhookHandler:  b.webhookHandler,
		JobController:   b.jobController,
		MetadataUpdater: b.metadataUpdater,
		MetricRecorder:  b.metricRecorder,
		Logger:          b.logger,
		Context:         b.ctx,
	}
}
