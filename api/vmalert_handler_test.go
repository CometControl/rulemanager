package api

import (
	"context"
	"errors"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRuleHandlers_GetVMAlertConfig(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	mockRS := new(MockRuleStore)
	ruleService := rules.NewService(mockTP, mockRS, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		rules := []*database.Rule{
			{TemplateName: "test", Parameters: []byte(`{"name":"alert1"}`)},
		}

		schema := `{"type": "object"}`
		tmpl := `alert: {{ .name }}`

		mockStore.On("ListRules", ctx, 0, 10000).Return(rules, nil).Once()
		mockTP.On("GetSchema", ctx, "test").Return(schema, nil).Once()
		mockTP.On("GetTemplate", ctx, "test").Return(tmpl, nil).Once()

		output, err := handlers.GetVMAlertConfig(ctx, &struct{}{})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Contains(t, string(output.Body), "groups:")
		mockStore.AssertExpectations(t)
		mockTP.AssertExpectations(t)
	})

	t.Run("ListRulesError", func(t *testing.T) {
		mockStore.On("ListRules", ctx, 0, 10000).Return(([]*database.Rule)(nil), errors.New("database error")).Once()

		output, err := handlers.GetVMAlertConfig(ctx, &struct{}{})

		assert.Error(t, err)
		assert.Nil(t, output)
		mockStore.AssertExpectations(t)
	})

	t.Run("GenerateConfigError", func(t *testing.T) {
		rules := []*database.Rule{
			{TemplateName: "bad_template", Parameters: []byte(`{}`)},
		}

		mockStore.On("ListRules", ctx, 0, 10000).Return(rules, nil).Once()
		mockTP.On("GetSchema", ctx, "bad_template").Return("", errors.New("not found")).Once()

		output, err := handlers.GetVMAlertConfig(ctx, &struct{}{})

		// Even though individual rule fails, GenerateVMAlertConfig should not error (it skips bad rules)
		assert.NoError(t, err)
		assert.NotNil(t, output)
		mockStore.AssertExpectations(t)
		mockTP.AssertExpectations(t)
	})
}
