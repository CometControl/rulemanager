package api

import (
	"context"
	"encoding/json"
	"errors"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRuleHandlers_CreateRule(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	ruleService := rules.NewService(mockTP, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	schema := `{"type": "object", "properties": {"target": {"type": "object"}}}`
	tmpl := `alert: test`

	t.Run("Success", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		mockTP.On("GetSchema", ctx, "openshift").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "openshift").Return(tmpl, nil).Once()
		mockStore.On("CreateRule", ctx, mock.AnythingOfType("*database.Rule")).Return(nil).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotEmpty(t, output.Body.ID)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("ValidationError", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"invalid": "data"}`)

		badSchema := `{"type": "object", "properties": {"required_field": {"type": "string"}}, "required": ["required_field"]}`
		mockTP.On("GetSchema", ctx, "openshift").Return(badSchema, nil).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
	})

	t.Run("GenerateError", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		badTmpl := `{{ .invalid_syntax`
		mockTP.On("GetSchema", ctx, "openshift").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "openshift").Return(badTmpl, nil).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		mockTP.On("GetSchema", ctx, "openshift").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "openshift").Return(tmpl, nil).Once()
		mockStore.On("CreateRule", ctx, mock.AnythingOfType("*database.Rule")).Return(errors.New("database error")).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})
}

func TestRuleHandlers_GetRule(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	ruleService := rules.NewService(mockTP, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ruleID := "123"
		expectedRule := &database.Rule{
			ID:           ruleID,
			TemplateName: "openshift",
			Parameters:   json.RawMessage(`{}`),
		}

		mockStore.On("GetRule", ctx, ruleID).Return(expectedRule, nil).Once()

		output, err := handlers.GetRule(ctx, &GetRuleInput{ID: ruleID})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, expectedRule, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("NotFound", func(t *testing.T) {
		ruleID := "nonexistent"
		mockStore.On("GetRule", ctx, ruleID).Return((*database.Rule)(nil), errors.New("not found")).Once()

		output, err := handlers.GetRule(ctx, &GetRuleInput{ID: ruleID})

		assert.Error(t, err)
		assert.Nil(t, output)
		mockStore.AssertExpectations(t)
	})
}

func TestRuleHandlers_ListRules(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	ruleService := rules.NewService(mockTP, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		expectedRules := []*database.Rule{
			{ID: "1", TemplateName: "openshift"},
			{ID: "2", TemplateName: "openshift"},
		}

		mockStore.On("ListRules", ctx, 0, 10).Return(expectedRules, nil).Once()

		output, err := handlers.ListRules(ctx, &ListRulesInput{Offset: 0, Limit: 10})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, expectedRules, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		mockStore.On("ListRules", ctx, 0, 10).Return(([]*database.Rule)(nil), errors.New("database error")).Once()

		output, err := handlers.ListRules(ctx, &ListRulesInput{Offset: 0, Limit: 10})

		assert.Error(t, err)
		assert.Nil(t, output)
		mockStore.AssertExpectations(t)
	})
}

func TestRuleHandlers_UpdateRule(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	ruleService := rules.NewService(mockTP, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	schema := `{"type": "object", "properties": {"target": {"type": "object"}}}`
	tmpl := `alert: test`

	t.Run("Success", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		mockTP.On("GetSchema", ctx, "openshift").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "openshift").Return(tmpl, nil).Once()
		mockStore.On("UpdateRule", ctx, "123", mock.AnythingOfType("*database.Rule")).Return(nil).Once()

		output, err := handlers.UpdateRule(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "123", output.Body.ID)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("ValidationError", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"invalid": "data"}`)

		badSchema := `{"type": "object", "properties": {"required_field": {"type": "string"}}, "required": ["required_field"]}`
		mockTP.On("GetSchema", ctx, "openshift").Return(badSchema, nil).Once()

		output, err := handlers.UpdateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		input.Body.TemplateName = "openshift"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		mockTP.On("GetSchema", ctx, "openshift").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "openshift").Return(tmpl, nil).Once()
		mockStore.On("UpdateRule", ctx, "123", mock.AnythingOfType("*database.Rule")).Return(errors.New("database error")).Once()

		output, err := handlers.UpdateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})
}

func TestRuleHandlers_DeleteRule(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	ruleService := rules.NewService(mockTP, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ruleID := "123"
		mockStore.On("DeleteRule", ctx, ruleID).Return(nil).Once()

		output, err := handlers.DeleteRule(ctx, &DeleteRuleInput{ID: ruleID})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, 204, output.Status)
		mockStore.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		ruleID := "123"
		mockStore.On("DeleteRule", ctx, ruleID).Return(errors.New("database error")).Once()

		output, err := handlers.DeleteRule(ctx, &DeleteRuleInput{ID: ruleID})

		assert.Error(t, err)
		assert.Nil(t, output)
		mockStore.AssertExpectations(t)
	})
}
