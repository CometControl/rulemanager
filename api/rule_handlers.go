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
