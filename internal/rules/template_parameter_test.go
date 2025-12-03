package rules_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"testing"

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

func (m *MockTemplateProvider) ListSchemas(ctx context.Context) ([]*database.Schema, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*database.Schema), args.Error(1)
}

func (m *MockTemplateProvider) CreateSchema(ctx context.Context, name, content string) error {
	return nil
}

func (m *MockTemplateProvider) CreateTemplate(ctx context.Context, name, content string) error {
	return nil
}
func (m *MockTemplateProvider) DeleteSchema(ctx context.Context, name string) error   { return nil }
func (m *MockTemplateProvider) DeleteTemplate(ctx context.Context, name string) error { return nil }

// MockRuleStore
type MockRuleStore struct {
	mock.Mock
}

func (m *MockRuleStore) CreateRule(ctx context.Context, rule *database.Rule) error {
	args := m.Called(ctx, rule)
	return args.Error(0)
}

func (m *MockRuleStore) GetRule(ctx context.Context, id string) (*database.Rule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Rule), args.Error(1)
}

func (m *MockRuleStore) ListRules(ctx context.Context, offset, limit int) ([]*database.Rule, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*database.Rule), args.Error(1)
}

func (m *MockRuleStore) UpdateRule(ctx context.Context, id string, rule *database.Rule) error {
	args := m.Called(ctx, id, rule)
	return args.Error(0)
}

func (m *MockRuleStore) DeleteRule(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRuleStore) SearchRules(ctx context.Context, filter database.RuleFilter) ([]*database.Rule, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*database.Rule), args.Error(1)
}

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
	mockRS := new(MockRuleStore)
	svc := rules.NewService(mockTP, mockRS, validator)

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
				"common": map[string]interface{}{
					"severity": "critical",
				},
				"rules": []map[string]interface{}{
					{
						"rule_type": "cpu",
						"operator":  ">",
						"threshold": 0.9,
					},
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
				"common": map[string]interface{}{
					"severity": "critical",
				},
				"rules": []map[string]interface{}{
					{
						"rule_type": "ram",
						"operator":  ">",
						"threshold": 2000000000,
					},
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
				"common": map[string]interface{}{
					"severity": "critical",
					"labels": map[string]string{
						"team": "platform",
					},
					"annotations": map[string]string{
						"runbook": "http://runbook.com/api-server",
					},
				},
				"rules": []map[string]interface{}{
					{
						"rule_type": "cpu",
						"operator":  ">",
						"threshold": 0.9,
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
				"common": map[string]interface{}{
					"severity": "critical",
				},
				"rules": []map[string]interface{}{
					{
						"rule_type": "cpu",
						"operator":  ">",
						"threshold": 0.9,
					},
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

func TestCustomTemplate(t *testing.T) {
	// Locate template files
	schemaPath := "c:\\Dev\\rulemanager\\templates\\_base\\custom.json"
	tmplPath := "c:\\Dev\\rulemanager\\templates\\go_templates\\custom.tmpl"

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		wd, _ := os.Getwd()
		rootDir := filepath.Join(wd, "..", "..")
		schemaPath = filepath.Join(rootDir, "templates", "_base", "custom.json")
		tmplPath = filepath.Join(rootDir, "templates", "go_templates", "custom.tmpl")
		schemaBytes, err = os.ReadFile(schemaPath)
	}
	assert.NoError(t, err, "Failed to read schema file")

	tmplBytes, err := os.ReadFile(tmplPath)
	assert.NoError(t, err, "Failed to read template file")

	schemaContent := string(schemaBytes)
	tmplContent := string(tmplBytes)

	mockTP := new(MockTemplateProvider)
	mockTP.On("GetSchema", mock.Anything, "custom").Return(schemaContent, nil)
	mockTP.On("GetTemplate", mock.Anything, "custom").Return(tmplContent, nil)

	validator := validation.NewJSONSchemaValidator()
	mockRS := new(MockRuleStore)
	svc := rules.NewService(mockTP, mockRS, validator)

	tests := []struct {
		name       string
		params     map[string]interface{}
		wantErr    bool
		wantChecks []string
	}{
		{
			name: "Valid Custom Rule",
			params: map[string]interface{}{
				"target": map[string]string{
					"custom_rule_name": "MyRuleGroup",
				},
				"common": map[string]interface{}{
					"severity": "warning",
				},
				"rules": []map[string]interface{}{
					{
						"alert": "MyCustomAlert",
						"expr":  "up == 0",
						"for":   "10m",
						"labels": map[string]string{
							"custom_label": "value",
						},
						"annotations": map[string]string{
							"summary": "Instance down",
						},
					},
				},
			},
			wantErr: false,
			wantChecks: []string{
				"alert: MyCustomAlert",
				"expr: up == 0",
				"for: 10m",
				"severity: warning",
				"custom_rule_name: MyRuleGroup",
				"custom_label: value",
				"summary: Instance down",
			},
		},
		{
			name: "Valid Custom Rule No For",
			params: map[string]interface{}{
				"target": map[string]string{
					"custom_rule_name": "MyRuleGroup",
				},
				"rules": []map[string]interface{}{
					{
						"alert": "MyCustomAlert",
						"expr":  "up == 0",
					},
				},
			},
			wantErr: false,
			wantChecks: []string{
				"alert: MyCustomAlert",
				"expr: up == 0",
				"custom_rule_name: MyRuleGroup",
			},
		},
		{
			name: "Missing Required Expr",
			params: map[string]interface{}{
				"target": map[string]string{
					"custom_rule_name": "MyRuleGroup",
				},
				"rules": []map[string]interface{}{
					{
						"alert": "InvalidAlert",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramBytes, _ := json.Marshal(tt.params)
			got, err := svc.GenerateRule(context.Background(), "custom", paramBytes)
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
