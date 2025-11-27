package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RuleHandlers handles rule-related API requests.
type RuleHandlers struct {
	ruleStore   database.RuleStore
	ruleService *rules.Service
}

// NewRuleHandlers registers rule handlers with the API.
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

	huma.Register(api, huma.Operation{
		OperationID: "search-rules",
		Method:      http.MethodGet,
		Path:        "/api/v1/rules/search",
		Summary:     "Search rules",
		Description: "Search rules by template and parameters (e.g., ?template=demo&target.service=api&target.environment=prod).",
		Tags:        []string{"Rules"},
	}, h.SearchRules)

	huma.Register(api, huma.Operation{
		OperationID: "plan-rule",
		Method:      http.MethodPost,
		Path:        "/api/v1/rules/plan",
		Summary:     "Plan rule creation",
		Description: "Simulates rule creation and checks for conflicts/overrides.",
		Tags:        []string{"Rules"},
	}, h.PlanRule)

	huma.Register(api, huma.Operation{
		OperationID: "plan-update-rule",
		Method:      http.MethodPost,
		Path:        "/api/v1/rules/{id}/plan",
		Summary:     "Plan rule update",
		Description: "Simulates rule update and checks for conflicts.",
		Tags:        []string{"Rules"},
	}, h.PlanUpdateRule)

	h.RegisterVMAlertEndpoint(api)
}

// RuleCreationParams defines the expected structure for rule creation parameters.
type RuleCreationParams struct {
	Target json.RawMessage   `json:"target"`
	Common json.RawMessage   `json:"common"`
	Rules  []json.RawMessage `json:"rules"`
}

type CreateRuleInput struct {
	Body struct {
		TemplateName string          `json:"templateName" doc:"The name of the template to use"`
		Parameters   json.RawMessage `json:"parameters" doc:"The parameters for the rule template"`
	}
}

type CreateRuleOutput struct {
	Body struct {
		IDs   []string `json:"ids" doc:"The IDs of the created or updated rules"`
		Count int      `json:"count" doc:"The number of rules processed"`
	}
}

type PlanRuleOutput struct {
	Body struct {
		Plans []*rules.RulePlan `json:"plans"`
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

// CreateRule creates one or more rules from a template using a 'rules' array parameter.
func (h *RuleHandlers) CreateRule(ctx context.Context, input *CreateRuleInput) (*CreateRuleOutput, error) {
	// Parse parameters into the expected structure
	var params RuleCreationParams
	if err := json.Unmarshal(input.Body.Parameters, &params); err != nil {
		slog.Warn("CreateRule: Invalid parameters", "error", err)
		return nil, huma.Error400BadRequest("Invalid parameters: " + err.Error())
	}

	// Validate required fields
	if params.Target == nil {
		return nil, huma.Error400BadRequest("'target' is required")
	}

	if params.Rules == nil {
		return nil, huma.Error400BadRequest("'rules' array is required. For a single rule, send an array with one element.")
	}

	if len(params.Rules) == 0 {
		return nil, huma.Error400BadRequest("'rules' array cannot be empty")
	}

	var createdIDs []string

	// Create a separate rule for each item in the rules array
	for i, ruleItem := range params.Rules {
		// Construct parameters for this single rule: {target, common, rules: [rule]}
		singleRuleParams := struct {
			Target json.RawMessage   `json:"target"`
			Common json.RawMessage   `json:"common,omitempty"`
			Rules  []json.RawMessage `json:"rules"`
		}{
			Target: params.Target,
			Common: params.Common,
			Rules:  []json.RawMessage{ruleItem},
		}

		singleRuleJSON, err := json.Marshal(singleRuleParams)
		if err != nil {
			slog.Error("CreateRule: Failed to marshal parameters", "rule_index", i, "error", err)
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to marshal parameters for rule %d", i))
		}

		// Plan the creation (check for existence/validity)
		plan, err := h.ruleService.PlanRuleCreation(ctx, input.Body.TemplateName, singleRuleJSON)
		if err != nil {
			slog.Warn("CreateRule: Planning failed", "rule_index", i, "template", input.Body.TemplateName, "error", err)
			return nil, huma.Error400BadRequest(fmt.Sprintf("Validation/Planning failed for rule %d: %s", i, err.Error()))
		}

		// Validate template syntax by attempting generation (PlanRuleCreation only validates schema)
		if _, err := h.ruleService.GenerateRule(ctx, input.Body.TemplateName, singleRuleJSON); err != nil {
			slog.Warn("CreateRule: Generation failed", "rule_index", i, "template", input.Body.TemplateName, "error", err)
			return nil, huma.Error400BadRequest(fmt.Sprintf("Generation failed for rule %d: %s", i, err.Error()))
		}

		if plan.Action == "update" {
			// Update existing rule
			rule := plan.ExistingRule
			rule.Parameters = singleRuleJSON
			rule.TemplateName = input.Body.TemplateName // Ensure template name is updated if changed (though plan checks template name)

			if err := h.ruleStore.UpdateRule(ctx, rule.ID, rule); err != nil {
				slog.Error("CreateRule: Failed to update rule", "id", rule.ID, "error", err)
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to update rule %d: %s", i, err.Error()))
			}
			createdIDs = append(createdIDs, rule.ID)
			slog.Info("CreateRule: Updated existing rule", "id", rule.ID)
		} else {
			// Create new rule
			rule := plan.NewRule
			rule.ID = primitive.NewObjectID().Hex()
			rule.CreatedAt = time.Now()
			rule.UpdatedAt = time.Now()

			if err := h.ruleStore.CreateRule(ctx, rule); err != nil {
				slog.Error("CreateRule: Failed to persist rule", "rule_index", i, "error", err)
				return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to create rule %d: %s", i, err.Error()))
			}
			createdIDs = append(createdIDs, rule.ID)
			slog.Info("CreateRule: Created new rule", "id", rule.ID)
		}
	}

	resp := &CreateRuleOutput{}
	resp.Body.IDs = createdIDs
	resp.Body.Count = len(createdIDs)
	slog.Info("CreateRule: Successfully processed rules", "count", len(createdIDs), "template", input.Body.TemplateName)
	return resp, nil
}

// PlanRule simulates rule creation and returns the plan.
func (h *RuleHandlers) PlanRule(ctx context.Context, input *CreateRuleInput) (*PlanRuleOutput, error) {
	// Parse parameters into the expected structure
	var params RuleCreationParams
	if err := json.Unmarshal(input.Body.Parameters, &params); err != nil {
		return nil, huma.Error400BadRequest("Invalid parameters: " + err.Error())
	}

	if params.Target == nil {
		return nil, huma.Error400BadRequest("'target' is required")
	}
	if len(params.Rules) == 0 {
		return nil, huma.Error400BadRequest("'rules' array is required and cannot be empty")
	}

	var plans []*rules.RulePlan

	for i, ruleItem := range params.Rules {
		singleRuleParams := struct {
			Target json.RawMessage   `json:"target"`
			Common json.RawMessage   `json:"common,omitempty"`
			Rules  []json.RawMessage `json:"rules"`
		}{
			Target: params.Target,
			Common: params.Common,
			Rules:  []json.RawMessage{ruleItem},
		}

		singleRuleJSON, err := json.Marshal(singleRuleParams)
		if err != nil {
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to marshal parameters for rule %d", i))
		}

		plan, err := h.ruleService.PlanRuleCreation(ctx, input.Body.TemplateName, singleRuleJSON)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("Planning failed for rule %d: %s", i, err.Error()))
		}
		plans = append(plans, plan)
	}

	resp := &PlanRuleOutput{}
	resp.Body.Plans = plans
	return resp, nil
}

// GetRule retrieves a rule by ID.
func (h *RuleHandlers) GetRule(ctx context.Context, input *GetRuleInput) (*GetRuleOutput, error) {
	rule, err := h.ruleStore.GetRule(ctx, input.ID)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}

	return &GetRuleOutput{Body: rule}, nil
}

// ListRules lists all rules with pagination.
func (h *RuleHandlers) ListRules(ctx context.Context, input *ListRulesInput) (*ListRulesOutput, error) {
	rules, err := h.ruleStore.ListRules(ctx, input.Offset, input.Limit)
	if err != nil {
		slog.Error("ListRules: Failed to list rules", "error", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &ListRulesOutput{Body: rules}, nil
}

// UpdateRule updates an existing rule.
// Supports partial updates for parameters.
func (h *RuleHandlers) UpdateRule(ctx context.Context, input *UpdateRuleInput) (*UpdateRuleOutput, error) {
	// 1. Fetch existing rule to get template name if not provided
	// (PlanRuleUpdate fetches it again, but we need template name for the call if input is empty)
	// Actually, PlanRuleUpdate needs template name.
	templateName := input.Body.TemplateName
	if templateName == "" {
		existingRule, err := h.ruleStore.GetRule(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("Rule not found: " + err.Error())
		}
		templateName = existingRule.TemplateName
	}

	// 2. Plan the update (checks for conflicts)
	plan, err := h.ruleService.PlanRuleUpdate(ctx, input.ID, templateName, input.Body.Parameters)
	if err != nil {
		slog.Warn("UpdateRule: Planning failed", "id", input.ID, "error", err)
		return nil, huma.Error400BadRequest(err.Error())
	}

	// 3. Check for conflict
	if plan.Action == "conflict" {
		return nil, huma.Error409Conflict(plan.Reason)
	}

	// 4. Validate template syntax (PlanRuleUpdate only validates schema)
	// We use the NewRule from the plan which has the merged parameters
	if _, err := h.ruleService.GenerateRule(ctx, templateName, plan.NewRule.Parameters); err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	// 5. Update the rule
	// We use the NewRule from the plan which has the merged parameters
	if err := h.ruleStore.UpdateRule(ctx, input.ID, plan.NewRule); err != nil {
		slog.Error("UpdateRule: Failed to update rule", "id", input.ID, "error", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := &UpdateRuleOutput{}
	resp.Body.ID = input.ID
	return resp, nil
}

// PlanUpdateRule simulates rule update and returns the plan.
func (h *RuleHandlers) PlanUpdateRule(ctx context.Context, input *UpdateRuleInput) (*rules.RulePlan, error) {
	// 1. Fetch existing rule to get template name if not provided
	templateName := input.Body.TemplateName
	if templateName == "" {
		existingRule, err := h.ruleStore.GetRule(ctx, input.ID)
		if err != nil {
			return nil, huma.Error404NotFound("Rule not found: " + err.Error())
		}
		templateName = existingRule.TemplateName
	}

	// 2. Plan the update
	plan, err := h.ruleService.PlanRuleUpdate(ctx, input.ID, templateName, input.Body.Parameters)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	return plan, nil
}

// DeleteRule deletes a rule by ID.
func (h *RuleHandlers) DeleteRule(ctx context.Context, input *DeleteRuleInput) (*DeleteRuleOutput, error) {
	if err := h.ruleStore.DeleteRule(ctx, input.ID); err != nil {
		slog.Error("DeleteRule: Failed to delete rule", "id", input.ID, "error", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &DeleteRuleOutput{Status: http.StatusNoContent}, nil
}
