package api

import (
	"context"
	"encoding/json"
	"net/http"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"

	"github.com/danielgtaylor/huma/v2"
)

type RuleHandlers struct {
	ruleStore   database.RuleStore
	ruleService *rules.Service
}

func NewRuleHandlers(api huma.API, rs database.RuleStore, svc *rules.Service) {
	h := &RuleHandlers{
		ruleStore:   rs,
		ruleService: svc,
	}

	huma.Register(api, huma.Operation{
		OperationID: "create-rule",
		Method:      http.MethodPost,
		Path:        "/api/v1/rules",
		Summary:     "Create a new rule",
		Description: "Creates a new rule based on a template and parameters.",
		Tags:        []string{"Rules"},
	}, h.CreateRule)

	huma.Register(api, huma.Operation{
		OperationID: "get-rule",
		Method:      http.MethodGet,
		Path:        "/api/v1/rules/{id}",
		Summary:     "Get a rule",
		Description: "Retrieves a rule by its ID.",
		Tags:        []string{"Rules"},
	}, h.GetRule)

	huma.Register(api, huma.Operation{
		OperationID: "list-rules",
		Method:      http.MethodGet,
		Path:        "/api/v1/rules",
		Summary:     "List rules",
		Description: "Lists all rules with pagination.",
		Tags:        []string{"Rules"},
	}, h.ListRules)

	huma.Register(api, huma.Operation{
		OperationID: "update-rule",
		Method:      http.MethodPut,
		Path:        "/api/v1/rules/{id}",
		Summary:     "Update a rule",
		Description: "Updates an existing rule.",
		Tags:        []string{"Rules"},
	}, h.UpdateRule)

	huma.Register(api, huma.Operation{
		OperationID: "delete-rule",
		Method:      http.MethodDelete,
		Path:        "/api/v1/rules/{id}",
		Summary:     "Delete a rule",
		Description: "Deletes a rule by its ID.",
		Tags:        []string{"Rules"},
	}, h.DeleteRule)

	h.RegisterVMAlertEndpoint(api)
}

type CreateRuleInput struct {
	Body struct {
		TemplateName string          `json:"templateName" doc:"The name of the template to use"`
		Parameters   json.RawMessage `json:"parameters" doc:"The parameters for the rule template"`
	}
}

type CreateRuleOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

type GetRuleInput struct {
	ID string `path:"id" doc:"The ID of the rule to retrieve"`
}

type GetRuleOutput struct {
	Body *database.Rule
}

type ListRulesInput struct {
	Offset int `query:"offset" doc:"The offset for pagination" default:"0"`
	Limit  int `query:"limit" doc:"The limit for pagination" default:"10"`
}

type ListRulesOutput struct {
	Body []*database.Rule
}

type UpdateRuleInput struct {
	ID   string `path:"id" doc:"The ID of the rule to update"`
	Body struct {
		TemplateName string          `json:"templateName" doc:"The name of the template to use"`
		Parameters   json.RawMessage `json:"parameters" doc:"The parameters for the rule template"`
	}
}

type UpdateRuleOutput struct {
	Body struct {
		ID string `json:"id"`
	}
}

type DeleteRuleInput struct {
	ID string `path:"id" doc:"The ID of the rule to delete"`
}

type DeleteRuleOutput struct {
	Status int
}

func (h *RuleHandlers) CreateRule(ctx context.Context, input *CreateRuleInput) (*CreateRuleOutput, error) {
	// 1. Generate the rule content (validates parameters against schema)
	// Note: We are not storing the generated content yet, just validating that it CAN be generated.
	// In a real scenario, we might want to store the generated rule or just the parameters.
	// The design says we store the parameters and generate on demand (or cache).
	_, err := h.ruleService.GenerateRule(ctx, input.Body.TemplateName, input.Body.Parameters)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	// 2. Create the rule in the database
	rule := &database.Rule{
		TemplateName: input.Body.TemplateName,
		Parameters:   input.Body.Parameters,
	}

	if err := h.ruleStore.CreateRule(ctx, rule); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := &CreateRuleOutput{}
	resp.Body.ID = rule.ID
	return resp, nil
}

func (h *RuleHandlers) GetRule(ctx context.Context, input *GetRuleInput) (*GetRuleOutput, error) {
	rule, err := h.ruleStore.GetRule(ctx, input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}

	return &GetRuleOutput{Body: rule}, nil
}

func (h *RuleHandlers) ListRules(ctx context.Context, input *ListRulesInput) (*ListRulesOutput, error) {
	rules, err := h.ruleStore.ListRules(ctx, input.Offset, input.Limit)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &ListRulesOutput{Body: rules}, nil
}

func (h *RuleHandlers) UpdateRule(ctx context.Context, input *UpdateRuleInput) (*UpdateRuleOutput, error) {
	// 1. Generate/Validate
	_, err := h.ruleService.GenerateRule(ctx, input.Body.TemplateName, input.Body.Parameters)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	// 2. Update
	rule := &database.Rule{
		TemplateName: input.Body.TemplateName,
		Parameters:   input.Body.Parameters,
	}

	if err := h.ruleStore.UpdateRule(ctx, input.ID, rule); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := &UpdateRuleOutput{}
	resp.Body.ID = input.ID
	return resp, nil
}

func (h *RuleHandlers) DeleteRule(ctx context.Context, input *DeleteRuleInput) (*DeleteRuleOutput, error) {
	if err := h.ruleStore.DeleteRule(ctx, input.ID); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &DeleteRuleOutput{Status: http.StatusNoContent}, nil
}
