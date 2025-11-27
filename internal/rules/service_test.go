package rules

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"rulemanager/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTemplateProvider
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
	args := m.Called(ctx, name, content)
	return args.Error(0)
}

func (m *MockTemplateProvider) CreateTemplate(ctx context.Context, name, content string) error {
	args := m.Called(ctx, name, content)
	return args.Error(0)
}

func (m *MockTemplateProvider) DeleteSchema(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockTemplateProvider) DeleteTemplate(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

// MockSchemaValidator
type MockSchemaValidator struct {
	mock.Mock
}

func (m *MockSchemaValidator) Validate(schema string, data []byte) error {
	args := m.Called(schema, data)
	return args.Error(0)
}

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

func TestService_GenerateRule(t *testing.T) {
	// Setup
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	mockRS := new(MockRuleStore)
	service := NewService(mockTP, mockRS, mockVal)
	ctx := context.Background()

	templateName := "test_template"
	params := json.RawMessage(`{"name": "test"}`)
	schema := `{"type": "object"}`
	tmplContent := `alert: {{ .name }}`

	// Test Case 1: Success
	t.Run("Success", func(t *testing.T) {
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()
		mockTP.On("GetTemplate", ctx, templateName).Return(tmplContent, nil).Once()

		result, err := service.GenerateRule(ctx, templateName, params)

		assert.NoError(t, err)
		assert.Equal(t, "alert: test", result)
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	// Test Case 2: Schema Not Found
	t.Run("SchemaNotFound", func(t *testing.T) {
		mockTP.On("GetSchema", ctx, templateName).Return("", errors.New("schema not found")).Once()

		_, err := service.GenerateRule(ctx, templateName, params)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schema not found")
		mockTP.AssertExpectations(t)
	})

	// Test Case 3: Validation Error
	t.Run("ValidationError", func(t *testing.T) {
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(errors.New("invalid params")).Once()

		_, err := service.GenerateRule(ctx, templateName, params)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid params")
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	// Test Case 4: Template Not Found
	t.Run("TemplateNotFound", func(t *testing.T) {
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()
		mockTP.On("GetTemplate", ctx, templateName).Return("", errors.New("template not found")).Once()

		_, err := service.GenerateRule(ctx, templateName, params)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template not found")
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	// Test Case 5: Template Parse Error
	t.Run("TemplateParseError", func(t *testing.T) {
		invalidTmpl := `{{ .name ` // Invalid template syntax
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()
		mockTP.On("GetTemplate", ctx, templateName).Return(invalidTmpl, nil).Once()

		_, err := service.GenerateRule(ctx, templateName, params)

		assert.Error(t, err)
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	// Test Case 6: Template Execute Error
	t.Run("TemplateExecuteError", func(t *testing.T) {
		badTmpl := `{{ call .undefined }}` // Will error on execution
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()
		mockTP.On("GetTemplate", ctx, templateName).Return(badTmpl, nil).Once()

		_, err := service.GenerateRule(ctx, templateName, params)

		assert.Error(t, err)
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	// Test Case 7: Invalid JSON Parameters
	t.Run("InvalidJSONParameters", func(t *testing.T) {
		invalidParams := json.RawMessage(`{invalid}`)
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(invalidParams)).Return(nil).Once()
		mockTP.On("GetTemplate", ctx, templateName).Return(tmplContent, nil).Once()

		_, err := service.GenerateRule(ctx, templateName, invalidParams)

		assert.Error(t, err)
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})
}

func TestService_ValidateRule(t *testing.T) {
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	mockRS := new(MockRuleStore)
	service := NewService(mockTP, mockRS, mockVal)
	ctx := context.Background()

	templateName := "test_template"
	params := json.RawMessage(`{"target": {"namespace": "test"}}`)

	t.Run("Success_NoPipelines", func(t *testing.T) {
		schema := `{"type": "object"}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()

		err := service.ValidateRule(ctx, templateName, params)

		assert.NoError(t, err)
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	t.Run("Success_WithPipelines", func(t *testing.T) {
		schema := `{
			"type": "object",
			"datasource": {"type": "prometheus", "url": "http://localhost:9090"},
			"pipelines": []
		}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()

		err := service.ValidateRule(ctx, templateName, params)

		assert.NoError(t, err)
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	t.Run("SchemaError", func(t *testing.T) {
		mockTP.On("GetSchema", ctx, templateName).Return("", errors.New("schema error")).Once()

		err := service.ValidateRule(ctx, templateName, params)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "schema error")
		mockTP.AssertExpectations(t)
	})

	t.Run("ValidationError", func(t *testing.T) {
		schema := `{"type": "object"}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(errors.New("validation failed")).Once()

		err := service.ValidateRule(ctx, templateName, params)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "validation failed")
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})

	t.Run("InvalidSchemaJSON", func(t *testing.T) {
		schema := `{invalid json}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()

		err := service.ValidateRule(ctx, templateName, params)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse schema")
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})
}

func TestService_PlanRuleCreation(t *testing.T) {
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	mockRS := new(MockRuleStore)
	service := NewService(mockTP, mockRS, mockVal)
	ctx := context.Background()

	templateName := "test_template"
	params := json.RawMessage(`{
		"target": {"namespace": "test", "env": "prod"},
		"common": {"severity": "warning"},
		"rules": [{"rule_type": "cpu"}]
	}`)

	t.Run("DefaultUniqueness_Create", func(t *testing.T) {
		// No uniqueness_keys in schema -> fallback to target + rule_type
		schema := `{"type": "object"}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()

		// Expect search with target.* and rules.rule_type
		expectedFilter := database.RuleFilter{
			TemplateName: templateName,
			Parameters: map[string]string{
				"target.namespace": "test",
				"target.env":       "prod",
				"rules.rule_type":  "cpu",
			},
		}
		mockRS.On("SearchRules", ctx, expectedFilter).Return([]*database.Rule{}, nil).Once()

		plan, err := service.PlanRuleCreation(ctx, templateName, params)

		assert.NoError(t, err)
		assert.Equal(t, "create", plan.Action)
		mockTP.AssertExpectations(t)
		mockRS.AssertExpectations(t)
	})

	t.Run("CustomUniqueness_Update", func(t *testing.T) {
		// Custom keys: target.namespace and common.severity
		schema := `{
			"type": "object",
			"uniqueness_keys": ["target.namespace", "common.severity"]
		}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()

		expectedFilter := database.RuleFilter{
			TemplateName: templateName,
			Parameters: map[string]string{
				"target.namespace": "test",
				"common.severity":  "warning",
			},
		}
		existingRule := &database.Rule{ID: "123"}
		mockRS.On("SearchRules", ctx, expectedFilter).Return([]*database.Rule{existingRule}, nil).Once()

		plan, err := service.PlanRuleCreation(ctx, templateName, params)

		assert.NoError(t, err)
		assert.Equal(t, "update", plan.Action)
		assert.Equal(t, existingRule, plan.ExistingRule)
		mockTP.AssertExpectations(t)
		mockRS.AssertExpectations(t)
	})

	t.Run("TargetExpansion_Create", func(t *testing.T) {
		// "target" key should expand to all leaf fields
		schema := `{
			"type": "object",
			"uniqueness_keys": ["target"]
		}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()

		expectedFilter := database.RuleFilter{
			TemplateName: templateName,
			Parameters: map[string]string{
				"target.namespace": "test",
				"target.env":       "prod",
			},
		}
		mockRS.On("SearchRules", ctx, expectedFilter).Return([]*database.Rule{}, nil).Once()

		plan, err := service.PlanRuleCreation(ctx, templateName, params)

		assert.NoError(t, err)
		assert.Equal(t, "create", plan.Action)
		mockTP.AssertExpectations(t)
		mockRS.AssertExpectations(t)
	})
}

func TestService_PlanRuleUpdate(t *testing.T) {
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	mockRS := new(MockRuleStore)
	service := NewService(mockTP, mockRS, mockVal)
	ctx := context.Background()

	templateName := "test_template"
	ruleID := "rule1"
	existingParams := json.RawMessage(`{
		"target": {"namespace": "test"},
		"common": {"severity": "info"},
		"rules": [{"rule_type": "cpu"}]
	}`)
	existingRule := &database.Rule{
		ID:           ruleID,
		TemplateName: templateName,
		Parameters:   existingParams,
	}

	t.Run("Update_NoConflict", func(t *testing.T) {
		updateParams := json.RawMessage(`{"common": {"severity": "warning"}}`)

		// 1. Get Existing Rule
		mockRS.On("GetRule", ctx, ruleID).Return(existingRule, nil).Once()

		// 2. Schema Validation
		schema := `{"type": "object", "uniqueness_keys": ["target.namespace"]}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		// Validate merged params
		mockVal.On("Validate", schema, mock.Anything).Return(nil).Once()

		// 3. Search for conflicts
		// Expect search with target.namespace=test
		expectedFilter := database.RuleFilter{
			TemplateName: templateName,
			Parameters: map[string]string{
				"target.namespace": "test",
			},
		}
		// Return only the rule itself (no conflict)
		mockRS.On("SearchRules", ctx, expectedFilter).Return([]*database.Rule{existingRule}, nil).Once()

		plan, err := service.PlanRuleUpdate(ctx, ruleID, templateName, updateParams)

		assert.NoError(t, err)
		assert.Equal(t, "update", plan.Action)
		mockTP.AssertExpectations(t)
		mockRS.AssertExpectations(t)
	})

	t.Run("Update_Conflict", func(t *testing.T) {
		// Update target to match another existing rule
		updateParams := json.RawMessage(`{"target": {"namespace": "other"}}`)

		// 1. Get Existing Rule
		mockRS.On("GetRule", ctx, ruleID).Return(existingRule, nil).Once()

		// 2. Schema Validation
		schema := `{"type": "object", "uniqueness_keys": ["target.namespace"]}`
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, mock.Anything).Return(nil).Once()

		// 3. Search for conflicts
		expectedFilter := database.RuleFilter{
			TemplateName: templateName,
			Parameters: map[string]string{
				"target.namespace": "other",
			},
		}
		// Return ANOTHER rule
		otherRule := &database.Rule{ID: "rule2"}
		mockRS.On("SearchRules", ctx, expectedFilter).Return([]*database.Rule{otherRule}, nil).Once()

		plan, err := service.PlanRuleUpdate(ctx, ruleID, templateName, updateParams)

		assert.NoError(t, err)
		assert.Equal(t, "conflict", plan.Action)
		mockTP.AssertExpectations(t)
		mockRS.AssertExpectations(t)
	})
}
