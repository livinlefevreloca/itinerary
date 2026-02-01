package constraints

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"text/template"
	"time"
)

// HTTPHealthCheckConstraint makes an HTTP request and checks for a 200 response
type HTTPHealthCheckConstraint struct {
	name            string
	urlTemplate     *template.Template
	method          string
	headerTemplates map[string]*template.Template
	bodyTemplate    *template.Template
	timeout         time.Duration
	recheck         bool
}

// NewHTTPHealthCheckConstraint creates a new HTTPHealthCheckConstraint
func NewHTTPHealthCheckConstraint(
	name string,
	urlTemplate string,
	method string,
	headers map[string]string,
	body string,
	timeout time.Duration,
	recheck bool,
) (*HTTPHealthCheckConstraint, error) {
	urlTmpl, err := template.New("url").Parse(urlTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid URL template: %w", err)
	}

	headerTmpls := make(map[string]*template.Template)
	for key, value := range headers {
		tmpl, err := template.New(key).Parse(value)
		if err != nil {
			return nil, fmt.Errorf("invalid header template for %s: %w", key, err)
		}
		headerTmpls[key] = tmpl
	}

	var bodyTmpl *template.Template
	if body != "" {
		bodyTmpl, err = template.New("body").Parse(body)
		if err != nil {
			return nil, fmt.Errorf("invalid body template: %w", err)
		}
	}

	return &HTTPHealthCheckConstraint{
		name:            name,
		urlTemplate:     urlTmpl,
		method:          method,
		headerTemplates: headerTmpls,
		bodyTemplate:    bodyTmpl,
		timeout:         timeout,
		recheck:         recheck,
	}, nil
}

func (h *HTTPHealthCheckConstraint) Check(ctx *ExecutionContext) (ConstraintResult, error) {
	// Template data
	data := map[string]interface{}{
		"JobID":   ctx.Job.ID,
		"JobName": ctx.Job.Name,
		"RunID":   ctx.RunID,
	}

	// Execute URL template
	var urlBuf bytes.Buffer
	if err := h.urlTemplate.Execute(&urlBuf, data); err != nil {
		return ConstraintResult{}, err
	}
	url := urlBuf.String()

	// Render headers
	headers := make(map[string]string)
	for key, tmpl := range h.headerTemplates {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return ConstraintResult{}, err
		}
		headers[key] = buf.String()
	}

	// Make HTTP request
	reqCtx, cancel := context.WithTimeout(ctx.Context, h.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, h.method, url, nil)
	if err != nil {
		return ConstraintResult{}, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := ctx.HTTPClient.Do(req)
	if err != nil {
		return ConstraintResult{}, err
	}
	defer resp.Body.Close()

	met := resp.StatusCode == 200

	return ConstraintResult{
		Met: met,
		Message: fmt.Sprintf("HTTP %s %s returned %d",
			h.method, url, resp.StatusCode),
	}, nil
}

func (h *HTTPHealthCheckConstraint) Name() string {
	return h.name
}

func (h *HTTPHealthCheckConstraint) EvaluationTiming() []EvaluationPhase {
	return []EvaluationPhase{EvaluationPhasePreExecution}
}

func (h *HTTPHealthCheckConstraint) ShouldRecheckOnRetry() bool {
	return h.recheck
}
