package api

import (
	"context"
	"encoding/json"
	"net/http"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"text/template"

	"github.com/danielgtaylor/huma/v2"
)

// TemplateHandlers handles template-related API requests.
type TemplateHandlers struct {
	store       database.TemplateProvider
	validator   validation.SchemaValidator
	ruleService *rules.Service
}

// NewTemplateHandlers registers template handlers with the API.
func NewTemplateHandlers(api huma.API, store database.TemplateProvider, validator validation.SchemaValidator, svc *rules.Service) {
	h := &TemplateHandlers{
		store:       store,
		validator:   validator,
		ruleService: svc,
	}

	// Schema Endpoints
	huma.Register(api, huma.Operation{
		OperationID: "create-schema",
		Method:      http.MethodPost,
		Path:        "/api/v1/templates/schemas",
		Summary:     "Create or update a schema",
		Tags:        []string{"Templates"},
	}, h.CreateSchema)

	huma.Register(api, huma.Operation{
		OperationID: "get-schema",
		Method:      http.MethodGet,
		Path:        "/api/v1/templates/schemas/{name}",
		Summary:     "Get a schema",
		Tags:        []string{"Templates"},
	}, h.GetSchema)

	huma.Register(api, huma.Operation{
		OperationID: "delete-schema",
		Method:      http.MethodDelete,
		Path:        "/api/v1/templates/schemas/{name}",
		Summary:     "Delete a schema",
		Tags:        []string{"Templates"},
	}, h.DeleteSchema)

	// Template Endpoints
	huma.Register(api, huma.Operation{
		OperationID: "create-template",
		Method:      http.MethodPost,
		Path:        "/api/v1/templates/go-templates",
		Summary:     "Create or update a Go template",
		Tags:        []string{"Templates"},
	}, h.CreateTemplate)

	huma.Register(api, huma.Operation{
		OperationID: "get-template",
		Method:      http.MethodGet,
		Path:        "/api/v1/templates/go-templates/{name}",
		Summary:     "Get a Go template",
		Tags:        []string{"Templates"},
	}, h.GetTemplate)

	huma.Register(api, huma.Operation{
		OperationID: "delete-template",
		Method:      http.MethodDelete,
		Path:        "/api/v1/templates/go-templates/{name}",
		Summary:     "Delete a Go template",
		Tags:        []string{"Templates"},
	}, h.DeleteTemplate)

	huma.Register(api, huma.Operation{
		OperationID: "validate-template",
		Method:      http.MethodPost,
		Path:        "/api/v1/templates/validate",
		Summary:     "Validate a template",
		Description: "Dry-run validation of a template with parameters.",
		Tags:        []string{"Templates"},
	}, h.ValidateTemplate)
}

// Inputs/Outputs

type CreateSchemaInput struct {
	Body struct {
		Name    string          `json:"name"`
		Content json.RawMessage `json:"content"`
	}
}

type CreateTemplateInput struct {
	Body struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
}

type GetTemplateInput struct {
	Name string `path:"name"`
}

type GetSchemaOutput struct {
	Body struct {
		Content json.RawMessage `json:"content"`
	}
}

type GetTemplateOutput struct {
	Body struct {
		Content string `json:"content"`
	}
}

type ValidateTemplateInput struct {
	Body struct {
		TemplateContent string          `json:"templateContent"`
		Parameters      json.RawMessage `json:"parameters"`
	}
}

type ValidateTemplateOutput struct {
	Body struct {
		Result string `json:"result"`
	}
}

// Handlers

// CreateSchema creates or updates a schema.
func (h *TemplateHandlers) CreateSchema(ctx context.Context, input *CreateSchemaInput) (*struct{}, error) {
	// Parse content to check/set $schema
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(input.Body.Content, &schemaMap); err != nil {
		return nil, huma.Error400BadRequest("Invalid JSON content: " + err.Error())
	}

	const supportedSchema = "http://json-schema.org/draft-07/schema"

	if val, ok := schemaMap["$schema"]; ok {
		version, ok := val.(string)
		if !ok {
			return nil, huma.Error400BadRequest("$schema must be a string")
		}
		if version != supportedSchema {
			return nil, huma.Error400BadRequest("Unsupported schema version. Only " + supportedSchema + " is supported.")
		}
	} else {
		// Default to supported schema
		schemaMap["$schema"] = supportedSchema
	}

	// Re-marshal to ensure we store the updated version
	updatedContent, err := json.MarshalIndent(schemaMap, "", "  ")
	if err != nil {
		return nil, huma.Error500InternalServerError("Failed to process schema: " + err.Error())
	}

	if err := h.store.CreateSchema(ctx, input.Body.Name, string(updatedContent)); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

// GetSchema retrieves a schema by name.
func (h *TemplateHandlers) GetSchema(ctx context.Context, input *GetTemplateInput) (*GetSchemaOutput, error) {
	content, err := h.store.GetSchema(ctx, input.Name)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &GetSchemaOutput{Body: struct {
		Content json.RawMessage `json:"content"`
	}{Content: json.RawMessage(content)}}, nil
}

// DeleteSchema deletes a schema by name.
func (h *TemplateHandlers) DeleteSchema(ctx context.Context, input *GetTemplateInput) (*struct{}, error) {
	if err := h.store.DeleteSchema(ctx, input.Name); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

// CreateTemplate creates or updates a Go template.
func (h *TemplateHandlers) CreateTemplate(ctx context.Context, input *CreateTemplateInput) (*struct{}, error) {
	// Validate PromQL
	// We need to validate that the template produces valid PromQL.
	// However, we don't have parameters here.
	// We can try to validate with empty parameters or dummy data if possible,
	// but often templates need specific data to render valid PromQL.
	// For now, let's at least check if it parses as a Go template.
	// The ruleService.ValidateTemplate does both render and PromQL check.
	// If we want to enforce PromQL validity, we might need example data.
	// The DEVELOPMENT.md mentions a "dry-run" validation endpoint, but for creation it says:
	// "On any POST request ... the service will first attempt to parse it."
	// It doesn't explicitly say it must validate PromQL on creation without data.
	// But it's good practice.
	// Let's just check Go template syntax for now as per minimum requirement,
	// since we can't easily generate valid PromQL without data.

	// Actually, we can try to parse the template itself.
	// The service doesn't expose a raw "ParseTemplate" but we can add one or just do it here.
	// But wait, `ruleService.ValidateTemplate` is for the `validate` endpoint.

	// Let's just ensure it's a valid Go template.
	if _, err := template.New("check").Parse(input.Body.Content); err != nil {
		return nil, huma.Error400BadRequest("Invalid Go template: " + err.Error())
	}

	if err := h.store.CreateTemplate(ctx, input.Body.Name, input.Body.Content); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

// GetTemplate retrieves a Go template by name.
func (h *TemplateHandlers) GetTemplate(ctx context.Context, input *GetTemplateInput) (*GetTemplateOutput, error) {
	content, err := h.store.GetTemplate(ctx, input.Name)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &GetTemplateOutput{Body: struct {
		Content string `json:"content"`
	}{Content: content}}, nil
}

// DeleteTemplate deletes a Go template by name.
func (h *TemplateHandlers) DeleteTemplate(ctx context.Context, input *GetTemplateInput) (*struct{}, error) {
	if err := h.store.DeleteTemplate(ctx, input.Name); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

// ValidateTemplate validates a template with parameters.
func (h *TemplateHandlers) ValidateTemplate(ctx context.Context, input *ValidateTemplateInput) (*ValidateTemplateOutput, error) {
	result, err := h.ruleService.ValidateTemplate(ctx, input.Body.TemplateContent, input.Body.Parameters)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &ValidateTemplateOutput{Body: struct {
		Result string `json:"result"`
	}{Result: result}}, nil
}
