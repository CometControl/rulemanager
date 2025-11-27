package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"
)

// Type aliases for dynamic JSON handling - improves readability while maintaining flexibility
type (
	// FieldValues represents user-provided field values (e.g., current form state in API requests)
	FieldValues map[string]interface{}
	// SchemaNode represents a node in the JSON schema tree during traversal
	SchemaNode map[string]interface{}
)

// DynamicOptionsConfig represents the x-dynamic-options configuration in a schema field.
// Uses Prometheus API structure directly - no parsing needed.
type DynamicOptionsConfig struct {
	Type         string   `json:"type"`
	Label        string   `json:"label"` // The Prometheus label to query values for
	Match        string   `json:"match"` // The match[] selector (can include filters and templates)
	Dependencies []string `json:"dependencies,omitempty"`
}

// GetOptions resolves dynamic options for a specific field in a template.
func (s *Service) GetOptions(ctx context.Context, templateName string, fieldPath string, currentValues FieldValues) ([]string, error) {
	// 1. Get Schema
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	// 2. Parse schema and extract dynamic options for the field
	dynamicOpts, err := extractDynamicOptions(schemaStr, fieldPath)
	if err != nil {
		return nil, err
	}

	if dynamicOpts.Type != "prometheus_query" {
		return nil, fmt.Errorf("unsupported dynamic options type: %s", dynamicOpts.Type)
	}

	if dynamicOpts.Label == "" {
		return nil, fmt.Errorf("label is empty")
	}

	if dynamicOpts.Match == "" {
		return nil, fmt.Errorf("match is empty")
	}

	// 3. Substitute variables in the match[] using Go templates
	match, err := substituteVariables(dynamicOpts.Match, currentValues)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute variables in match: %w", err)
	}

	// 4. Get datasource configuration
	var schemaObj struct {
		Datasource *DatasourceConfig `json:"datasource"`
	}
	if err := json.Unmarshal([]byte(schemaStr), &schemaObj); err != nil {
		return nil, fmt.Errorf("failed to parse schema for datasource: %w", err)
	}

	if schemaObj.Datasource == nil {
		return nil, fmt.Errorf("datasource not configured in template")
	}

	// 5. Query Prometheus directly with label and match[] (no parsing needed!)
	return s.queryLabelValues(ctx, schemaObj.Datasource, match, dynamicOpts.Label)
}

// extractDynamicOptions extracts the x-dynamic-options configuration for a specific field path.
func extractDynamicOptions(schemaStr string, fieldPath string) (*DynamicOptionsConfig, error) {
	var schema SchemaNode
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	fieldDef, err := navigateToField(schema, fieldPath)
	if err != nil {
		return nil, fmt.Errorf("field '%s' not found in schema: %w", fieldPath, err)
	}

	// Extract x-dynamic-options from the field definition
	dynOptsRaw, ok := fieldDef["x-dynamic-options"]
	if !ok {
		return nil, fmt.Errorf("field '%s' does not have dynamic options configured", fieldPath)
	}

	// Marshal and unmarshal to convert to our struct
	dynOptsBytes, err := json.Marshal(dynOptsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to process dynamic options: %w", err)
	}

	var dynOpts DynamicOptionsConfig
	if err := json.Unmarshal(dynOptsBytes, &dynOpts); err != nil {
		return nil, fmt.Errorf("failed to parse dynamic options: %w", err)
	}

	return &dynOpts, nil
}

// navigateToField traverses a JSON schema to find a field definition by dot-separated path.
func navigateToField(schema SchemaNode, path string) (SchemaNode, error) {
	if path == "" {
		return nil, fmt.Errorf("empty field path")
	}

	parts := strings.Split(path, ".")
	cursor := schema

	for i, part := range parts {
		props, ok := cursor["properties"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("no 'properties' field at level %d (path: %s)", i, strings.Join(parts[:i+1], "."))
		}

		next, ok := props[part].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("field '%s' not found at level %d", part, i)
		}
		cursor = next
	}

	return cursor, nil
}

// substituteVariables uses Go's text/template to replace template variables.
// Template syntax: {{.target.namespace}}
func substituteVariables(matchTemplate string, currentValues FieldValues) (string, error) {
	tmpl, err := template.New("match").Parse(matchTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse match template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, currentValues); err != nil {
		return "", fmt.Errorf("failed to execute match template: %w", err)
	}

	return buf.String(), nil
}

// PrometheusLabelValuesResponse represents the response from Prometheus /api/v1/label/<label>/values endpoint
type PrometheusLabelValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// queryLabelValues queries Prometheus for label values using the metadata API.
func (s *Service) queryLabelValues(ctx context.Context, datasource *DatasourceConfig, match string, label string) ([]string, error) {
	u, err := url.Parse(datasource.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid datasource URL: %w", err)
	}

	u.Path = fmt.Sprintf("/api/v1/label/%s/values", url.PathEscape(label))
	q := u.Query()
	q.Set("match[]", match)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("datasource returned status %d for URL %s", resp.StatusCode, u.String())
	}

	var result PrometheusLabelValuesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode datasource response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("datasource query failed with status: %s", result.Status)
	}

	return result.Data, nil
}
