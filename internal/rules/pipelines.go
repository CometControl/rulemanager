package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"text/template"
	"time"
)

// PipelineStep defines a single step in the rule creation pipeline.
type PipelineStep struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Condition  *PipelineCondition     `json:"condition,omitempty"`
	Parameters map[string]interface{} `json:"parameters"`
}

// PipelineCondition defines a condition for executing a pipeline step.
type PipelineCondition struct {
	Property string `json:"property"`
	Equals   string `json:"equals"`
}

// DatasourceConfig defines the connection details for a datasource.
type DatasourceConfig struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// StepRunner defines the interface for a pipeline step runner.
type StepRunner interface {
	Run(ctx context.Context, datasource *DatasourceConfig, ruleParams json.RawMessage, stepParams map[string]interface{}) error
}

// PipelineProcessor manages the execution of pipeline steps.
type PipelineProcessor struct {
	runners map[string]StepRunner
}

// NewPipelineProcessor creates a new PipelineProcessor with built-in runners.
func NewPipelineProcessor() *PipelineProcessor {
	p := &PipelineProcessor{
		runners: make(map[string]StepRunner),
	}
	// Register built-in runners
	p.RegisterRunner("validate_metric_exists", &ValidateMetricExistsRunner{})
	return p
}

// RegisterRunner registers a new step runner with the given name.
func (p *PipelineProcessor) RegisterRunner(name string, runner StepRunner) {
	p.runners[name] = runner
}

// Execute runs a sequence of pipeline steps.
func (p *PipelineProcessor) Execute(ctx context.Context, schemaPipelines []PipelineStep, datasource *DatasourceConfig, ruleParams json.RawMessage) error {
	var paramsMap map[string]interface{}
	if err := json.Unmarshal(ruleParams, &paramsMap); err != nil {
		return fmt.Errorf("failed to unmarshal rule parameters: %w", err)
	}

	for _, step := range schemaPipelines {
		// Check condition
		if step.Condition != nil {
			val, ok := paramsMap[step.Condition.Property]
			if !ok {
				continue
			}
			valStr, ok := val.(string)
			if !ok || valStr != step.Condition.Equals {
				continue // Condition not met
			}
		}

		// Find runner
		runner, ok := p.runners[step.Type]
		if !ok {
			return fmt.Errorf("unknown pipeline step type: %s", step.Type)
		}

		// Run step
		if err := runner.Run(ctx, datasource, ruleParams, step.Parameters); err != nil {
			return fmt.Errorf("pipeline step '%s' failed: %w", step.Name, err)
		}
	}
	return nil
}

// ValidateMetricExistsRunner checks if a metric exists in the datasource.
type ValidateMetricExistsRunner struct {
	Client *http.Client
}

// Run executes the metric validation step.
func (r *ValidateMetricExistsRunner) Run(ctx context.Context, datasource *DatasourceConfig, ruleParams json.RawMessage, stepParams map[string]interface{}) error {
	if datasource == nil {
		return fmt.Errorf("datasource configuration is required for metric validation")
	}
	if datasource.Type != "prometheus" && datasource.Type != "victoriametrics" && datasource.Type != "thanos" {
		// Assuming these all support PromQL
		return fmt.Errorf("unsupported datasource type for metric validation: %s", datasource.Type)
	}

	// Extract parameters
	metricNameTmpl, _ := stepParams["metric_name"].(string)

	if metricNameTmpl == "" {
		return fmt.Errorf("metric_name is required")
	}

	// Render templates
	var paramsMap map[string]interface{}
	if err := json.Unmarshal(ruleParams, &paramsMap); err != nil {
		return fmt.Errorf("failed to unmarshal rule parameters: %w", err)
	}

	metricName, err := renderString(metricNameTmpl, paramsMap)
	if err != nil {
		return fmt.Errorf("failed to render metric_name: %w", err)
	}

	// Construct selector and query
	selector := fmt.Sprintf("{__name__=%q}", metricName)

	query := fmt.Sprintf("count(%s)", selector)

	// 4. Execute Query
	u, err := url.Parse(datasource.URL)
	if err != nil {
		return fmt.Errorf("invalid datasource URL: %w", err)
	}
	u.Path = "/api/v1/query" // Instant query is enough
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to query datasource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("datasource returned status %d", resp.StatusCode)
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string        `json:"resultType"`
			Result     []interface{} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode datasource response: %w", err)
	}

	if result.Status != "success" {
		return fmt.Errorf("datasource query failed")
	}

	// Metric exists if query returns results
	if len(result.Data.Result) == 0 {
		return fmt.Errorf("metric '%s' not found", metricName)
	}

	return nil
}

func renderString(tmplStr string, data interface{}) (string, error) {
	tmpl, err := template.New("pipeline").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
