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
	ruleService := rules.NewService(mockTP, mockStore, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	schema := `{"type": "object", "properties": {"target": {"type": "object"}}}`
	tmpl := `alert: test`

	t.Run("Success", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}, "rules": [{"rule_type": "cpu"}]}`)

		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "k8s").Return(tmpl, nil).Once()
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Once()
		mockStore.On("CreateRule", ctx, mock.AnythingOfType("*database.Rule")).Return(nil).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Len(t, output.Body.IDs, 1)
		assert.NotEmpty(t, output.Body.IDs[0])
		assert.Equal(t, 1, output.Body.Count)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("ValidationError", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}, "rules": [{"invalid": "data"}]}`)

		badSchema := `{"type": "object", "properties": {"required_field": {"type": "string"}}, "required": ["required_field"]}`
		mockTP.On("GetSchema", ctx, "k8s").Return(badSchema, nil).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
	})

	t.Run("GenerateError", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}, "rules": [{"rule_type": "cpu"}]}`)

		badTmpl := `{{ .invalid_syntax`
		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "k8s").Return(badTmpl, nil).Once()
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}, "rules": [{"rule_type": "cpu"}]}`)

		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Twice() // ValidateRule + GenerateRule
		mockTP.On("GetTemplate", ctx, "k8s").Return(tmpl, nil).Once()
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Once()
		mockStore.On("CreateRule", ctx, mock.AnythingOfType("*database.Rule")).Return(errors.New("database error")).Once()

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("BatchCreation", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{
			"target": {"environment": "prod", "namespace": "test", "workload": "my-app"},
			"rules": [
				{"rule_type": "cpu", "severity": "warning", "operator": ">", "threshold": 0.7},
				{"rule_type": "cpu", "severity": "critical", "operator": ">", "threshold": 0.9},
				{"rule_type": "ram", "severity": "critical", "operator": ">", "threshold": 2000000000}
			]
		}`)

		// Each rule needs validation + generation, and store creation
		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Times(6) // 3 rules * 2 (ValidateRule + GenerateRule)
		mockTP.On("GetTemplate", ctx, "k8s").Return(tmpl, nil).Times(3) // 3 rules
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Times(3)
		mockStore.On("CreateRule", ctx, mock.AnythingOfType("*database.Rule")).Return(nil).Times(3) // 3 rules

		output, err := handlers.CreateRule(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Len(t, output.Body.IDs, 3)
		assert.Equal(t, 3, output.Body.Count)
		for _, id := range output.Body.IDs {
			assert.NotEmpty(t, id)
		}
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("MissingRulesArray", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}, "rule": {"rule_type": "cpu"}}`)

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "'rules' array is required")
	})

	t.Run("EmptyRulesArray", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}, "rules": []}`)

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "'rules' array cannot be empty")
	})

	t.Run("MissingTarget", func(t *testing.T) {
		input := &CreateRuleInput{}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"rules": [{"rule_type": "cpu"}]}`)

		output, err := handlers.CreateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		assert.Contains(t, err.Error(), "'target' is required")
	})
}

func TestRuleHandlers_GetRule(t *testing.T) {
	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	ruleService := rules.NewService(mockTP, mockStore, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		ruleID := "123"
		expectedRule := &database.Rule{
			ID:           ruleID,
			TemplateName: "k8s",
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
	ruleService := rules.NewService(mockTP, mockStore, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		expectedRules := []*database.Rule{
			{ID: "1", TemplateName: "k8s"},
			{ID: "2", TemplateName: "k8s"},
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
	ruleService := rules.NewService(mockTP, mockStore, validator)

	handlers := &RuleHandlers{
		ruleStore:   mockStore,
		ruleService: ruleService,
	}
	ctx := context.Background()

	schema := `{"type": "object", "properties": {"target": {"type": "object"}}}`
	tmpl := `alert: test`

	t.Run("Success_FullUpdate", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		existingRule := &database.Rule{
			ID:           "123",
			TemplateName: "k8s",
			Parameters:   json.RawMessage(`{"target": {"namespace": "old"}}`),
		}

		mockStore.On("GetRule", ctx, "123").Return(existingRule, nil).Once()
		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Twice()                                                      // ValidateRule + PlanRuleUpdate
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Once() // PlanRuleUpdate check
		mockTP.On("GetTemplate", ctx, "k8s").Return(tmpl, nil).Once()
		mockStore.On("UpdateRule", ctx, "123", mock.AnythingOfType("*database.Rule")).Return(nil).Once()

		output, err := handlers.UpdateRule(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "123", output.Body.ID)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("Success_PartialUpdate", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		// No TemplateName provided, should use existing
		// Partial update: only updating namespace, keeping environment
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "new-ns"}}`)

		existingRule := &database.Rule{
			ID:           "123",
			TemplateName: "k8s",
			Parameters:   json.RawMessage(`{"target": {"environment": "prod", "namespace": "old-ns"}}`),
		}

		mockStore.On("GetRule", ctx, "123").Return(existingRule, nil).Twice()

		// Expect merged parameters: environment=prod (kept), namespace=new-ns (updated)
		// We can't easily match the exact JSON string in mock expectation due to key ordering,
		// but we can verify the behavior by what is passed to ValidateRule
		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Twice()
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Once() // PlanRuleUpdate check
		mockTP.On("GetTemplate", ctx, "k8s").Return(tmpl, nil).Once()

		// Verify UpdateRule is called with merged parameters
		mockStore.On("UpdateRule", ctx, "123", mock.MatchedBy(func(r *database.Rule) bool {
			var params map[string]interface{}
			if err := json.Unmarshal(r.Parameters, &params); err != nil {
				return false
			}
			target := params["target"].(map[string]interface{})
			return r.TemplateName == "k8s" &&
				target["environment"] == "prod" &&
				target["namespace"] == "new-ns"
		})).Return(nil).Once()

		output, err := handlers.UpdateRule(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		mockTP.AssertExpectations(t)
		mockStore.AssertExpectations(t)
	})

	t.Run("ValidationError", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"invalid": "data"}`)

		existingRule := &database.Rule{
			ID:           "123",
			TemplateName: "k8s",
			Parameters:   json.RawMessage(`{}`),
		}

		mockStore.On("GetRule", ctx, "123").Return(existingRule, nil).Once()

		badSchema := `{"type": "object", "properties": {"required_field": {"type": "string"}}, "required": ["required_field"]}`
		mockTP.On("GetSchema", ctx, "k8s").Return(badSchema, nil).Once()

		output, err := handlers.UpdateRule(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockTP.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		input := &UpdateRuleInput{ID: "123"}
		input.Body.TemplateName = "k8s"
		input.Body.Parameters = json.RawMessage(`{"target": {"namespace": "test"}}`)

		existingRule := &database.Rule{
			ID:           "123",
			TemplateName: "k8s",
			Parameters:   json.RawMessage(`{}`),
		}

		mockStore.On("GetRule", ctx, "123").Return(existingRule, nil).Once()
		mockTP.On("GetSchema", ctx, "k8s").Return(schema, nil).Twice()                                                      // ValidateRule + PlanRuleUpdate
		mockStore.On("SearchRules", ctx, mock.AnythingOfType("database.RuleFilter")).Return([]*database.Rule{}, nil).Once() // PlanRuleUpdate check
		mockTP.On("GetTemplate", ctx, "k8s").Return(tmpl, nil).Once()
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
	ruleService := rules.NewService(mockTP, mockStore, validator)

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

func TestRuleHandlers_SearchRules(t *testing.T) {
	mockStore := new(MockRuleStore)
	handlers := &RuleHandlers{
		ruleStore: mockStore,
	}
	ctx := context.Background()

	t.Run("SearchByTemplateName", func(t *testing.T) {
		expectedRules := []*database.Rule{
			{ID: "1", TemplateName: "demo"},
			{ID: "2", TemplateName: "demo"},
		}

		expectedFilter := database.RuleFilter{
			TemplateName: "demo",
			Parameters:   map[string]string{},
		}
		mockStore.On("SearchRules", ctx, expectedFilter).Return(expectedRules, nil).Once()

		input := &SearchRulesInput{
			QueryParams: map[string]string{"templateName": "demo"},
		}
		output, err := handlers.SearchRules(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, expectedRules, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("SearchByNestedParameter", func(t *testing.T) {
		expectedRules := []*database.Rule{
			{ID: "1", TemplateName: "demo"},
		}

		expectedFilter := database.RuleFilter{
			TemplateName: "",
			Parameters: map[string]string{
				"parameters.target.service": "payment-service",
			},
		}
		mockStore.On("SearchRules", ctx, expectedFilter).Return(expectedRules, nil).Once()

		input := &SearchRulesInput{
			QueryParams: map[string]string{
				"parameters.target.service": "payment-service",
			},
		}
		output, err := handlers.SearchRules(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, expectedRules, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("SearchByCombinedFilters", func(t *testing.T) {
		expectedRules := []*database.Rule{
			{ID: "1", TemplateName: "demo"},
		}

		expectedFilter := database.RuleFilter{
			TemplateName: "demo",
			Parameters: map[string]string{
				"parameters.target.service":     "api",
				"parameters.target.environment": "production",
			},
		}
		mockStore.On("SearchRules", ctx, expectedFilter).Return(expectedRules, nil).Once()

		input := &SearchRulesInput{
			QueryParams: map[string]string{
				"templateName":                  "demo",
				"parameters.target.service":     "api",
				"parameters.target.environment": "production",
			},
		}
		output, err := handlers.SearchRules(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, expectedRules, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("SearchNoResults", func(t *testing.T) {
		expectedFilter := database.RuleFilter{
			TemplateName: "",
			Parameters: map[string]string{
				"parameters.target.service": "non-existent",
			},
		}
		mockStore.On("SearchRules", ctx, expectedFilter).Return([]*database.Rule{}, nil).Once()

		input := &SearchRulesInput{
			QueryParams: map[string]string{
				"parameters.target.service": "non-existent",
			},
		}
		output, err := handlers.SearchRules(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Empty(t, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("SearchEmptyQuery", func(t *testing.T) {
		expectedFilter := database.RuleFilter{
			TemplateName: "",
			Parameters:   map[string]string{},
		}
		allRules := []*database.Rule{
			{ID: "1", TemplateName: "demo"},
			{ID: "2", TemplateName: "k8s"},
		}
		mockStore.On("SearchRules", ctx, expectedFilter).Return(allRules, nil).Once()

		input := &SearchRulesInput{
			QueryParams: map[string]string{},
		}
		output, err := handlers.SearchRules(ctx, input)

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, allRules, output.Body)
		mockStore.AssertExpectations(t)
	})

	t.Run("StoreError", func(t *testing.T) {
		expectedFilter := database.RuleFilter{
			TemplateName: "demo",
			Parameters:   map[string]string{},
		}
		mockStore.On("SearchRules", ctx, expectedFilter).Return(([]*database.Rule)(nil), errors.New("database error")).Once()

		input := &SearchRulesInput{
			QueryParams: map[string]string{"templateName": "demo"},
		}
		output, err := handlers.SearchRules(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, output)
		mockStore.AssertExpectations(t)
	})
}
