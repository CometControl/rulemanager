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

func TestService_GenerateRule(t *testing.T) {
	// Setup
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	service := NewService(mockTP, mockVal)
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
	service := NewService(mockTP, mockVal)
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
