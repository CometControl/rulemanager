package api

import (
	"context"
	"log/slog"
	"rulemanager/internal/database"

	"github.com/danielgtaylor/huma/v2"
)

type SearchRulesInput struct {
	QueryParams map[string]string // Populated by Resolve method with all query parameters
}

// Resolve implements huma.Resolver to capture all query parameters dynamically
func (i *SearchRulesInput) Resolve(ctx huma.Context) []error {
	i.QueryParams = make(map[string]string)

	// Get the URL from the context and extract all query parameters
	url := ctx.URL()
	for key, values := range url.Query() {
		if len(values) > 0 {
			i.QueryParams[key] = values[0]
		}
	}

	return nil
}

type SearchRulesOutput struct {
	Body []*database.Rule
}

// SearchRules searches for rules using explicit MongoDB field names.
// Query parameters map directly to MongoDB document fields (no magic conversions).
// Examples:
//
//	?templateName=demo                              → Search by template name
//	?parameters.target.service=api                  → Search by nested parameter
//	?templateName=demo&parameters.target.env=prod   → Combine multiple filters
func (h *RuleHandlers) SearchRules(ctx context.Context, input *SearchRulesInput) (*SearchRulesOutput, error) {
	filter := database.RuleFilter{
		Parameters: make(map[string]string),
	}

	// Pass all query parameters directly to MongoDB without conversion
	// Special handling for templateName to populate the dedicated filter field
	for key, value := range input.QueryParams {
		if key == "templateName" {
			filter.TemplateName = value
		} else {
			// All other params (including parameters.* fields) are passed as-is
			filter.Parameters[key] = value
		}
	}

	rules, err := h.ruleStore.SearchRules(ctx, filter)
	if err != nil {
		slog.Error("SearchRules: Failed to search rules", "error", err)
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &SearchRulesOutput{Body: rules}, nil
}
