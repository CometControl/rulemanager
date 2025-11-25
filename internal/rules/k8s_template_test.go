package rules_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK8sTemplateGeneration(t *testing.T) {
	// Paths to the template files
	baseDir := "../../templates"
	tmplPath := filepath.Join(baseDir, "go_templates", "k8s.tmpl")

	// Read the template file
	tmplContent, err := os.ReadFile(tmplPath)
	require.NoError(t, err)

	// Parse the template
	tmpl, err := template.New("k8s").Parse(string(tmplContent))
	require.NoError(t, err)

	// Define the input data matching the new schema
	inputJSON := `{
		"target": {
			"environment": "production",
			"namespace": "backend",
			"workload": "api-server"
		},
		"common": {
			"severity": "critical",
			"labels": {
				"team": "platform"
			},
			"annotations": {
				"runbook": "http://runbook.url"
			}
		},
		"rules": [
			{
				"rule_type": "cpu",
				"operator": ">",
				"threshold": 0.8
			},
			{
				"rule_type": "ram",
				"operator": ">",
				"threshold": 1024
			},
			{
				"rule_type": "service_up",
				"service_name": "auth-service"
			}
		]
	}`

	var data map[string]interface{}
	err = json.Unmarshal([]byte(inputJSON), &data)
	require.NoError(t, err)

	// Execute the template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, data)
	require.NoError(t, err)

	// Verify the output
	output := buf.String()

	// Check for CPU rule
	assert.Contains(t, output, "alert: HighCPUUsage_api-server")
	assert.Contains(t, output, "expr: sum(rate(container_cpu_usage_seconds_total{namespace=\"backend\", pod=~\"api-server-.*\"}[5m])) by (pod) > 0.8")

	// Check for RAM rule
	assert.Contains(t, output, "alert: HighMemoryUsage_api-server")
	assert.Contains(t, output, "expr: sum(container_memory_working_set_bytes{namespace=\"backend\", pod=~\"api-server-.*\"}) by (pod) > 1024")

	// Check for Service Up rule
	assert.Contains(t, output, "alert: ServiceDown_auth-service")
	assert.Contains(t, output, "expr: up{job=\"auth-service\", namespace=\"backend\"} == 0")

	// Check for common properties
	assert.Contains(t, output, "severity: critical")
	assert.Contains(t, output, "team: platform")
	assert.Contains(t, output, "runbook: http://runbook.url")
}
