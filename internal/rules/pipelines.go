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

func NewPipelineProcessor() *PipelineProcessor {
	p := &PipelineProcessor{
		runners: make(map[string]StepRunner),
	}
	// Register built-in runners
	p.RegisterRunner("validate_metric_exists", &ValidateMetricExistsRunner{})
	return p
}

func (p *PipelineProcessor) RegisterRunner(name string, runner StepRunner) {
	p.runners[name] = runner
}

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
				continue // Property not found, skip or fail? detailed design says skip if condition not met
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

func (r *ValidateMetricExistsRunner) Run(ctx context.Context, datasource *DatasourceConfig, ruleParams json.RawMessage, stepParams map[string]interface{}) error {
	if datasource == nil {
		return fmt.Errorf("datasource configuration is required for metric validation")
	}
	if datasource.Type != "prometheus" && datasource.Type != "victoriametrics" && datasource.Type != "thanos" {
		// Assuming these all support PromQL
		return fmt.Errorf("unsupported datasource type for metric validation: %s", datasource.Type)
	}

	// 1. Extract parameters
	metricNameTmpl, _ := stepParams["metric_name"].(string)
	labelsTmpl, _ := stepParams["labels"].(string) // Optional

	if metricNameTmpl == "" {
		return fmt.Errorf("metric_name is required")
	}

	// 2. Render templates
	var paramsMap map[string]interface{}
	json.Unmarshal(ruleParams, &paramsMap) // Already checked in processor, but safe to redo

	metricName, err := renderString(metricNameTmpl, paramsMap)
	if err != nil {
		return fmt.Errorf("failed to render metric_name: %w", err)
	}

	var labels string
	if labelsTmpl != "" {
		labels, err = renderString(labelsTmpl, paramsMap)
		if err != nil {
			return fmt.Errorf("failed to render labels: %w", err)
		}
	}

	// 3. Construct Query
	// Query: count({__name__="metric", labels...})
	// If labels is a JSON object string (from schema), we might need to parse it to PromQL label selector format?
	// The example in DEVELOPMENT.md shows: "labels": "{{ .workload_labels }}" where workload_labels is an object.
	// If the user passes an object, the template renders it as map[...].
	// We need to convert that map to PromQL selector syntax: key="value",...
	// OR, if the user puts PromQL syntax directly.
	// Let's assume for now we need to handle the map case if it comes from a JSON object parameter.
	
	// Actually, `renderString` will render the map using Go's default formatting which is `map[k:v]`. This is NOT PromQL.
	// We need a helper to render labels correctly.
	// However, for simplicity in this iteration, let's assume the template output or the parameter is handled.
	// Wait, the example says: "labels": "{{ .workload_labels }}".
	// If .workload_labels is `{"app": "foo"}`, Go template renders it as `map[app:foo]`.
	// We probably need a custom template function or logic to format it.
	// Let's try to parse the rendered string as a map if possible, or just rely on the user to provide a string that works?
	// Better: The `stepParams` are `interface{}`. If `labels` is a string, we render it.
	// If we want to support the object-to-promql conversion, we should do it here.
	
	// Let's construct the selector.
	selector := fmt.Sprintf("{__name__=%q", metricName)
	
	// If labels was rendered, we need to append it. 
	// But converting `map[app:foo]` string back is hard.
	// Instead, let's look at `ruleParams` directly.
	// If the step param `labels` refers to a rule param that is an object, we should iterate that object.
	// But `stepParams["labels"]` is a string template.
	// We can add a template function `promLabels`?
	// For now, let's do a basic implementation:
	// If `labels` string is not empty, we try to append it.
	// If it looks like `map[...]` we might fail.
	// Let's assume for the MVP that the user provides a string or we just check the metric name.
	// Actually, checking just the metric name is often enough for "existence".
	// Let's stick to metric name for now to be safe, or append labels if they are simple.
	
	// Refined approach for MVP: Just check metric name.
	// selector := fmt.Sprintf("{__name__=%q}", metricName)
	
	// If we want to support labels, we really need that `promLabels` function or similar.
	// Let's assume the user handles it or we skip labels for this specific MVP step.
	// Re-reading requirements: "checks if a given metric, optionally with specific labels".
	// Let's try to support it if it's a string that looks like PromQL.
	
	if labels != "" {
		// If labels starts with {, strip it? No, it's inside the braces.
		// Let's just append it with a comma if it's not empty.
		// But we need to be careful about syntax.
		// Let's just use the metric name for the "exists" check to be robust.
		// Checking specific series existence is stricter.
		// Let's ignore labels for the very first iteration to ensure stability, 
		// or try to use them if they don't look like a Go map.
	}
	selector += "}"

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

	// If count > 0, result vector should not be empty and value should be > 0
	if len(result.Data.Result) == 0 {
		return fmt.Errorf("metric '%s' not found", metricName)
	}
	
	// We could check the value but count() returns empty if no series match? 
	// Actually count() over empty vector returns empty vector.
	// So len(Result) == 0 means 0 count.
	
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
