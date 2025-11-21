package rules

import (
	"context"
	"encoding/json"
	"errors"
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

	t.Run("Success", func(t *testing.T) {
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
	})

	t.Run("SkipsErrorRules", func(t *testing.T) {
		rules := []*database.Rule{
			{ID: "1", TemplateName: "bad_template", Parameters: params},
			{ID: "2", TemplateName: templateName, Parameters: params},
		}

		// First rule will fail (template not found)
		mockTP.On("GetSchema", ctx, "bad_template").Return("", errors.New("not found")).Once()
		// Second rule will succeed
		mockTP.On("GetSchema", ctx, templateName).Return(schema, nil).Once()
		mockVal.On("Validate", schema, []byte(params)).Return(nil).Once()
		mockTP.On("GetTemplate", ctx, templateName).Return(tmplContent, nil).Once()

		// Execute
		config, err := service.GenerateVMAlertConfig(ctx, rules)

		// Assert - no error, but only valid rule is included
		assert.NoError(t, err)
		assert.Contains(t, config, "alert: test")
		assert.Contains(t, config, "test_template")
		mockTP.AssertExpectations(t)
		mockVal.AssertExpectations(t)
	})
}

func TestService_ValidateTemplate(t *testing.T) {
	mockTP := new(MockTemplateProvider)
	mockVal := new(MockSchemaValidator)
	service := NewService(mockTP, mockVal)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		tmplContent := `alert: HighCPU
expr: sum(rate(cpu[5m])) > 0.9`
		params := json.RawMessage(`{"threshold": 0.9}`)

		rendered, err := service.ValidateTemplate(ctx, tmplContent, params)

		assert.NoError(t, err)
		assert.Contains(t, rendered, "sum(rate(cpu[5m])) > 0.9")
	})

	t.Run("RenderError", func(t *testing.T) {
		tmplContent := `{{ .invalid syntax`
		params := json.RawMessage(`{}`)

		rendered, err := service.ValidateTemplate(ctx, tmplContent, params)

		assert.Error(t, err)
		assert.Empty(t, rendered)
	})

	t.Run("InvalidQuery", func(t *testing.T) {
		tmplContent := `alert: Test
expr: ""`
		params := json.RawMessage(`{}`)

		_, err := service.ValidateTemplate(ctx, tmplContent, params)

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "invalid rule content")
		}
	})
}

func TestService_ValidateRuleContent(t *testing.T) {
	service := &Service{}

	t.Run("ValidRule", func(t *testing.T) {
		ruleYaml := `alert: HighCPU
expr: sum(rate(cpu_usage[5m])) > 0.9
for: 5m`

		err := service.ValidateRuleContent(ruleYaml)

		assert.NoError(t, err)
	})

	t.Run("ValidRuleWithQuotes", func(t *testing.T) {
		ruleYaml := `alert: HighCPU
expr: "sum(rate(cpu_usage[5m])) > 0.9"
for: 5m`

		err := service.ValidateRuleContent(ruleYaml)

		assert.NoError(t, err)
	})

	t.Run("InvalidQuery", func(t *testing.T) {
		ruleYaml := `alert: Test
expr: rate(cpu[5m` // Missing closing paren

		err := service.ValidateRuleContent(ruleYaml)

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "invalid MetricsQL expression")
		}
	})

	t.Run("InvalidMetricsQLSyntax", func(t *testing.T) {
		ruleYaml := `alert: Test
expr: "this is not valid!"`

		err := service.ValidateRuleContent(ruleYaml)

		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "invalid MetricsQL expression")
		}
	})

	t.Run("MultilineStringIndicator", func(t *testing.T) {
		ruleYaml := `alert: Test
expr: |
  sum(rate(cpu[5m]))`

		// The "|" is valid yaml
		err := service.ValidateRuleContent(ruleYaml)

		assert.NoError(t, err)
	})

	t.Run("MissingExpr", func(t *testing.T) {
		ruleYaml := `alert: Test
for: 5m
labels:
  severity: warning`

		err := service.ValidateRuleContent(ruleYaml)

		assert.Error(t, err) // Alert rules must have an expr
	})
}
