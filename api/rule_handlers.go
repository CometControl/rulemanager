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

	"dario.cat/mergo"
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

	h.RegisterVMAlertEndpoint(api)
}

// RuleCreationParams defines the expected structure for rule creation parameters.
type RuleCreationParams struct {
	Target json.RawMessage   `json:"target"`
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
		IDs   []string `json:"ids" doc:"The IDs of the created rules"`
		Count int      `json:"count" doc:"The number of rules created"`
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
		// Construct parameters for this single rule: {target, rule}
		singleRuleParams := struct {
			Target json.RawMessage `json:"target"`
			Rule   json.RawMessage `json:"rule"`
		}{
			Target: params.Target,
			Rule:   ruleItem,
		}

		singleRuleJSON, err := json.Marshal(singleRuleParams)
		if err != nil {
			slog.Error("CreateRule: Failed to marshal parameters", "rule_index", i, "error", err)
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to marshal parameters for rule %d", i))
		}

		// Validate parameters and pipelines
		if err := h.ruleService.ValidateRule(ctx, input.Body.TemplateName, singleRuleJSON); err != nil {
			slog.Warn("CreateRule: Validation failed", "rule_index", i, "template", input.Body.TemplateName, "error", err)
			return nil, huma.Error400BadRequest(fmt.Sprintf("Validation failed for rule %d: %s", i, err.Error()))
		}

		// Validate template syntax by attempting generation
		if _, err := h.ruleService.GenerateRule(ctx, input.Body.TemplateName, singleRuleJSON); err != nil {
			slog.Warn("CreateRule: Generation failed", "rule_index", i, "template", input.Body.TemplateName, "error", err)
			return nil, huma.Error400BadRequest(fmt.Sprintf("Generation failed for rule %d: %s", i, err.Error()))
		}

		// Create the rule in the database
		rule := &database.Rule{
			ID:           primitive.NewObjectID().Hex(),
			TemplateName: input.Body.TemplateName,
			Parameters:   singleRuleJSON,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := h.ruleStore.CreateRule(ctx, rule); err != nil {
			slog.Error("CreateRule: Failed to persist rule", "rule_index", i, "error", err)
			return nil, huma.Error500InternalServerError(fmt.Sprintf("Failed to create rule %d: %s", i, err.Error()))
		}

		createdIDs = append(createdIDs, rule.ID)
	}

	resp := &CreateRuleOutput{}
	resp.Body.IDs = createdIDs
	resp.Body.Count = len(createdIDs)
	slog.Info("CreateRule: Successfully created rules", "count", len(createdIDs), "template", input.Body.TemplateName)
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
	// 1. Fetch existing rule
	existingRule, err := h.ruleStore.GetRule(ctx, input.ID)
	if err != nil {
		slog.Warn("UpdateRule: Rule not found", "id", input.ID, "error", err)
		return nil, huma.Error404NotFound("Rule not found: " + err.Error())
	}

	// 2. Determine template name (use existing if not provided)
	templateName := input.Body.TemplateName
	if templateName == "" {
		templateName = existingRule.TemplateName
	}

	// 3. Merge parameters (Partial Update)
	var finalParamsJSON json.RawMessage

	if len(input.Body.Parameters) > 0 {
		// Unmarshal existing parameters
		var existingParams map[string]interface{}
		if err := json.Unmarshal(existingRule.Parameters, &existingParams); err != nil {
			return nil, huma.Error500InternalServerError("Failed to parse existing rule parameters: " + err.Error())
		}

		// Unmarshal new parameters
		var newParams map[string]interface{}
		if err := json.Unmarshal(input.Body.Parameters, &newParams); err != nil {
			return nil, huma.Error400BadRequest("Invalid parameters JSON: " + err.Error())
		}

		// Deep merge: existing <- new (new values override existing)
		if err := mergo.Merge(&existingParams, newParams, mergo.WithOverride); err != nil {
			slog.Error("UpdateRule: Failed to merge parameters", "id", input.ID, "error", err)
			return nil, huma.Error500InternalServerError("Failed to merge parameters: " + err.Error())
		}

		// Marshal back to JSON
		mergedJSON, err := json.Marshal(existingParams)
		if err != nil {
			return nil, huma.Error500InternalServerError("Failed to marshal merged parameters: " + err.Error())
		}
		finalParamsJSON = mergedJSON
	} else {
		// No new parameters, keep existing
		finalParamsJSON = existingRule.Parameters
	}

	// 4. Validate merged parameters
	if err := h.ruleService.ValidateRule(ctx, templateName, finalParamsJSON); err != nil {
		slog.Warn("UpdateRule: Validation failed", "id", input.ID, "error", err)
		return nil, huma.Error400BadRequest(err.Error())
	}

	// 5. Validate template syntax
	_, err = h.ruleService.GenerateRule(ctx, templateName, finalParamsJSON)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}

	// 6. Update the rule
	rule := &database.Rule{
		TemplateName: templateName,
		Parameters:   finalParamsJSON,
	}

	if err := h.ruleStore.UpdateRule(ctx, input.ID, rule); err != nil {
		slog.Error("UpdateRule: Failed to update rule", "id", input.ID, "error", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := &UpdateRuleOutput{}
	resp.Body.ID = input.ID
	return resp, nil
}

// DeleteRule deletes a rule by ID.
func (h *RuleHandlers) DeleteRule(ctx context.Context, input *DeleteRuleInput) (*DeleteRuleOutput, error) {
	if err := h.ruleStore.DeleteRule(ctx, input.ID); err != nil {
		slog.Error("DeleteRule: Failed to delete rule", "id", input.ID, "error", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &DeleteRuleOutput{Status: http.StatusNoContent}, nil
}
