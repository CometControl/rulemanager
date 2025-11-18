package rules

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

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
}
