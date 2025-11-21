package rules

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipelineProcessor_Execute(t *testing.T) {
	// Mock Datasource
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		if query == `count({__name__="existing_metric"})` {
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[123,"1"]}]}}`))
		} else {
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		}
	}))
	defer ts.Close()

	processor := NewPipelineProcessor()
	// Inject custom client to the runner to use the test server
	runner := processor.runners["validate_metric_exists"].(*ValidateMetricExistsRunner)
	runner.Client = ts.Client()

	datasource := &DatasourceConfig{
		Type: "prometheus",
		URL:  ts.URL,
	}

	tests := []struct {
		name        string
		pipelines   []PipelineStep
		ruleParams  json.RawMessage
		expectError bool
	}{
		{
			name: "Metric Exists",
			pipelines: []PipelineStep{
				{
					Name: "Check Metric",
					Type: "validate_metric_exists",
					Parameters: map[string]interface{}{
						"metric_name": "existing_metric",
					},
				},
			},
			ruleParams:  json.RawMessage(`{}`),
			expectError: false,
		},
		{
			name: "Metric Does Not Exist",
			pipelines: []PipelineStep{
				{
					Name: "Check Metric",
					Type: "validate_metric_exists",
					Parameters: map[string]interface{}{
						"metric_name": "non_existent_metric",
					},
				},
			},
			ruleParams:  json.RawMessage(`{}`),
			expectError: true,
		},
		{
			name: "Condition Met - Metric Exists",
			pipelines: []PipelineStep{
				{
					Name: "Check Metric",
					Type: "validate_metric_exists",
					Condition: &PipelineCondition{
						Property: "check",
						Equals:   "yes",
					},
					Parameters: map[string]interface{}{
						"metric_name": "existing_metric",
					},
				},
			},
			ruleParams:  json.RawMessage(`{"check": "yes"}`),
			expectError: false,
		},
		{
			name: "Condition Met - Metric Missing",
			pipelines: []PipelineStep{
				{
					Name: "Check Metric",
					Type: "validate_metric_exists",
					Condition: &PipelineCondition{
						Property: "check",
						Equals:   "yes",
					},
					Parameters: map[string]interface{}{
						"metric_name": "non_existent_metric",
					},
				},
			},
			ruleParams:  json.RawMessage(`{"check": "yes"}`),
			expectError: true,
		},
		{
			name: "Condition Not Met - Skip Step",
			pipelines: []PipelineStep{
				{
					Name: "Check Metric",
					Type: "validate_metric_exists",
					Condition: &PipelineCondition{
						Property: "check",
						Equals:   "yes",
					},
					Parameters: map[string]interface{}{
						"metric_name": "non_existent_metric",
					},
				},
			},
			ruleParams:  json.RawMessage(`{"check": "no"}`),
			expectError: false,
		},
		{
			name: "Templated Metric Name",
			pipelines: []PipelineStep{
				{
					Name: "Check Metric",
					Type: "validate_metric_exists",
					Parameters: map[string]interface{}{
						"metric_name": "{{ .metric }}",
					},
				},
			},
			ruleParams:  json.RawMessage(`{"metric": "existing_metric"}`),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.Execute(context.Background(), tt.pipelines, datasource, tt.ruleParams)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPipelineProcessor_TargetValidation(t *testing.T) {
	// Mock datasource that validates namespace metrics
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		// Simulate namespace validation - namespace "demo" exists, others don't
		if query == `count({__name__="kube_namespace_status_phase"})` {
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[123,"1"]}]}}`))
		} else {
			w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
		}
	}))
	defer ts.Close()

	processor := NewPipelineProcessor()
	runner := processor.runners["validate_metric_exists"].(*ValidateMetricExistsRunner)
	runner.Client = ts.Client()

	datasource := &DatasourceConfig{
		Type: "prometheus",
		URL:  ts.URL,
	}

	tests := []struct {
		name        string
		pipelines   []PipelineStep
		params      map[string]interface{}
		expectError bool
	}{
		{
			name: "Valid Target - Namespace Exists",
			pipelines: []PipelineStep{
				{
					Name: "validate_namespace_metrics",
					Type: "validate_metric_exists",
					Parameters: map[string]interface{}{
						"metric_name": "kube_namespace_status_phase",
					},
				},
			},
			params: map[string]interface{}{
				"target": map[string]string{
					"environment": "prod",
					"namespace":   "demo",
					"workload":    "app",
				},
			},
			expectError: false,
		},
		{
			name: "Invalid Target - Namespace Doesn't Exist",
			pipelines: []PipelineStep{
				{
					Name: "validate_namespace_metrics",
					Type: "validate_metric_exists",
					Parameters: map[string]interface{}{
						"metric_name": "kube_pod_info",
					},
				},
			},
			params: map[string]interface{}{
				"target": map[string]string{
					"environment": "prod",
					"namespace":   "nonexistent",
					"workload":    "app",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramsJSON, _ := json.Marshal(tt.params)
			err := processor.Execute(context.Background(), tt.pipelines, datasource, paramsJSON)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
