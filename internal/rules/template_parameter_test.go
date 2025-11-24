package rules_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"rulemanager/internal/rules"
	"rulemanager/internal/validation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTemplateProvider is a mock implementation of database.TemplateProvider
type MockTemplateProvider struct {
	mock.Mock
}

func (m *MockTemplateProvider) GetSchema(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateProvider) GetTemplate(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateProvider) CreateSchema(ctx context.Context, name, content string) error {
	return nil
}
func (m *MockTemplateProvider) CreateTemplate(ctx context.Context, name, content string) error {
	return nil
}
func (m *MockTemplateProvider) DeleteSchema(ctx context.Context, name string) error   { return nil }
func (m *MockTemplateProvider) DeleteTemplate(ctx context.Context, name string) error { return nil }

func TestTemplateParameters(t *testing.T) {
	// Locate template files
	// Using k8s as the reference implementation for parameter testing
	schemaPath := "c:\\Dev\\rulemanager\\templates\\_base\\k8s.json"
	tmplPath := "c:\\Dev\\rulemanager\\templates\\go_templates\\k8s.tmpl"

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		wd, _ := os.Getwd()
		rootDir := filepath.Join(wd, "..", "..")
		schemaPath = filepath.Join(rootDir, "templates", "_base", "k8s.json")
		tmplPath = filepath.Join(rootDir, "templates", "go_templates", "k8s.tmpl")
		schemaBytes, err = os.ReadFile(schemaPath)
	}
	assert.NoError(t, err, "Failed to read schema file")

	tmplBytes, err := os.ReadFile(tmplPath)
	assert.NoError(t, err, "Failed to read template file")

	schemaContent := string(schemaBytes)
	tmplContent := string(tmplBytes)

	mockTP := new(MockTemplateProvider)
	mockTP.On("GetSchema", mock.Anything, "k8s").Return(schemaContent, nil)
	mockTP.On("GetTemplate", mock.Anything, "k8s").Return(tmplContent, nil)

	validator := validation.NewJSONSchemaValidator()
	svc := rules.NewService(mockTP, validator)

	tests := []struct {
		name       string
		params     map[string]interface{}
		wantErr    bool
		wantChecks []string
	}{
		{
			name: "Single CPU Rule",
			params: map[string]interface{}{
				"target": map[string]string{
					"environment": "prod",
					"namespace":   "backend",
					"workload":    "api-server",
				},
				"rule": map[string]interface{}{
					"rule_type": "cpu",
					"severity":  "critical",
					"operator":  ">",
					"threshold": 0.9,
				},
			},
			wantErr: false,
			wantChecks: []string{
				"severity: critical",
				"> 0.9",
				"HighCPUUsage_api-server",
			},
		},
		{
			name: "Single RAM Rule",
			params: map[string]interface{}{
				"target": map[string]string{
					"environment": "prod",
					"namespace":   "backend",
					"workload":    "api-server",
				},
				"rule": map[string]interface{}{
					"rule_type": "ram",
					"severity":  "critical",
					"operator":  ">",
					"threshold": 2000000000,
				},
			},
			wantErr: false,
			wantChecks: []string{
				"severity: critical",
				"> 2e+09",
				"HighMemoryUsage_api-server",
			},
		},
		{
			name: "With Labels and Annotations",
			params: map[string]interface{}{
				"target": map[string]string{
					"environment": "prod",
					"namespace":   "backend",
					"workload":    "api-server",
				},
				"rule": map[string]interface{}{
					"rule_type": "cpu",
					"severity":  "critical",
					"operator":  ">",
					"threshold": 0.9,
					"labels": map[string]string{
						"team": "platform",
					},
					"annotations": map[string]string{
						"runbook": "http://runbook.com/api-server",
					},
				},
			},
			wantErr: false,
			wantChecks: []string{
				"severity: critical",
				"team: platform",
				"runbook: http://runbook.com/api-server",
			},
		},
		{
			name: "Missing Target",
			params: map[string]interface{}{
				"rule": map[string]interface{}{
					"rule_type": "cpu",
					"severity":  "critical",
					"operator":  ">",
					"threshold": 0.9,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramBytes, _ := json.Marshal(tt.params)
			got, err := svc.GenerateRule(context.Background(), "k8s", paramBytes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, check := range tt.wantChecks {
					assert.Contains(t, got, check)
				}
				fmt.Printf("Generated Rule:\n%s\n", got)
			}
		})
	}
}
