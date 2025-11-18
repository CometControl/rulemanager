package rules

import (
	"context"
	"encoding/json"
	"rulemanager/internal/database"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestService_GenerateVMAlertConfig(t *testing.T) {
	// Setup
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	service := NewService(mockTP, mockVal)
	ctx := context.Background()

	templateName := "test_template"
	params := json.RawMessage(`{"name": "test"}`)
	schema := `{"type": "object"}`
	tmplContent := `alert: {{ .name }}`

	rules := []*database.Rule{
		{TemplateName: templateName, Parameters: params},
		{TemplateName: templateName, Parameters: params},
	}

	// Expectations
	mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Twice()
	mockVal.On("Validate", schema, []byte(params)).Return(nil).Twice()
	mockTP.On("GetTemplate", ctx, templateName).Return(tmplContent, nil).Twice()

	// Execute
	config, err := service.GenerateVMAlertConfig(ctx, rules)

	// Assert
	assert.NoError(t, err)
	expectedConfig := `groups:
  - name: test_template
    rules:
      alert: test
      alert: test
`
	assert.Equal(t, expectedConfig, config)
	mockTP.AssertExpectations(t)
	mockVal.AssertExpectations(t)
}
