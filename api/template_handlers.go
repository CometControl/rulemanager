package api

import (
	"context"
	"encoding/json"
	"net/http"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"

	"github.com/danielgtaylor/huma/v2"
)

type TemplateHandlers struct {
	store       database.TemplateProvider
	validator   validation.SchemaValidator
	ruleService *rules.Service
}

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

// Handlers

func (h *TemplateHandlers) CreateSchema(ctx context.Context, input *CreateSchemaInput) (*struct{}, error) {
	// Validate that content is valid JSON schema?
	// For now just store it.
	contentStr := string(input.Body.Content)
	if err := h.store.CreateSchema(ctx, input.Body.Name, contentStr); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

func (h *TemplateHandlers) GetSchema(ctx context.Context, input *GetTemplateInput) (*GetSchemaOutput, error) {
	content, err := h.store.GetSchema(ctx, input.Name)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &GetSchemaOutput{Body: struct {
		Content json.RawMessage `json:"content"`
	}{Content: json.RawMessage(content)}}, nil
}

func (h *TemplateHandlers) DeleteSchema(ctx context.Context, input *GetTemplateInput) (*struct{}, error) {
	if err := h.store.DeleteSchema(ctx, input.Name); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

func (h *TemplateHandlers) CreateTemplate(ctx context.Context, input *CreateTemplateInput) (*struct{}, error) {
	if err := h.store.CreateTemplate(ctx, input.Body.Name, input.Body.Content); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
}

func (h *TemplateHandlers) GetTemplate(ctx context.Context, input *GetTemplateInput) (*GetTemplateOutput, error) {
	content, err := h.store.GetTemplate(ctx, input.Name)
	if err != nil {
		return nil, huma.Error404NotFound(err.Error())
	}
	return &GetTemplateOutput{Body: struct {
		Content string `json:"content"`
	}{Content: content}}, nil
}

func (h *TemplateHandlers) DeleteTemplate(ctx context.Context, input *GetTemplateInput) (*struct{}, error) {
	if err := h.store.DeleteTemplate(ctx, input.Name); err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return nil, nil
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

// We need to access the rule service for validation.
// The TemplateHandlers struct currently only has store and validator.
// We should probably inject the rule service into TemplateHandlers as well,
// or move the validation logic to the service.
// The service already has ValidateTemplate method.
// Let's update NewTemplateHandlers to accept the service.

func (h *TemplateHandlers) ValidateTemplate(ctx context.Context, input *ValidateTemplateInput) (*ValidateTemplateOutput, error) {
	result, err := h.ruleService.ValidateTemplate(ctx, input.Body.TemplateContent, input.Body.Parameters)
	if err != nil {
		return nil, huma.Error400BadRequest(err.Error())
	}
	return &ValidateTemplateOutput{Body: struct {
		Result string `json:"result"`
	}{Result: result}}, nil
}
