package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"rulemanager/internal/database"

	"github.com/danielgtaylor/huma/v2"
)

// EnhanceDocumentation adds dynamic schemas and markdown docs to the OpenAPI spec.
func EnhanceDocumentation(api huma.API, provider database.TemplateProvider, docsDir string) error {
	if err := registerSchemas(api, provider); err != nil {
		return fmt.Errorf("failed to register schemas: %w", err)
	}

	if err := embedMarkdownDocs(api, docsDir); err != nil {
		return fmt.Errorf("failed to embed markdown docs: %w", err)
	}

	return nil
}

func registerSchemas(api huma.API, provider database.TemplateProvider) error {
	ctx := context.Background()
	schemas, err := provider.ListSchemas(ctx)
	if err != nil {
		return fmt.Errorf("failed to list schemas: %w", err)
	}

	registry := api.OpenAPI().Components.Schemas
	if registry == nil {
		return fmt.Errorf("OpenAPI components schemas registry is nil")
	}

	loadedSchemas := make(map[string]bool)

	for _, schema := range schemas {
		var humaSchema huma.Schema
		if err := json.Unmarshal(schema.Schema, &humaSchema); err != nil {
			slog.Warn("Failed to parse schema content", "name", schema.Name, "error", err)
			continue
		}

		registry.Map()[schema.Name] = &humaSchema
		loadedSchemas[schema.Name] = true
		slog.Info("Registered OpenAPI schema", "name", schema.Name)
	}

	// Identify schemas to keep (templates) and remove (internal)
	keepSchemas := make(map[string]bool)
	for _, schema := range schemas {
		keepSchemas[schema.Name] = true
	}

	// Helper to resolve and inline schemas
	var resolve func(s *huma.Schema) *huma.Schema
	resolve = func(s *huma.Schema) *huma.Schema {
		if s == nil {
			return nil
		}

		if s.Ref != "" {
			refName := strings.TrimPrefix(s.Ref, "#/components/schemas/")
			if !keepSchemas[refName] {
				// It's an internal schema, inline it
				if target, ok := registry.Map()[refName]; ok {
					// Create a copy to avoid modifying the original registry item yet
					// We need to deep copy if we want to be safe, but for now shallow copy + recursive resolve
					inlined := *target
					inlined.Ref = "" // Clear ref
					return resolve(&inlined)
				}
			}
		}

		// Recursively resolve properties
		if len(s.Properties) > 0 {
			newProps := make(map[string]*huma.Schema)
			for k, v := range s.Properties {
				newProps[k] = resolve(v)
			}
			s.Properties = newProps
		}

		// Recursively resolve items
		if s.Items != nil {
			s.Items = resolve(s.Items)
		}

		// Recursively resolve additional properties
		if s.AdditionalProperties != nil {
			// AdditionalProperties can be bool or schema. Huma handles this via a custom type or pointer?
			// Huma v2 Schema.AdditionalProperties is interface{}.
			// We need to check if it's a schema.
			if schema, ok := s.AdditionalProperties.(*huma.Schema); ok {
				s.AdditionalProperties = resolve(schema)
			}
		}

		// Resolve OneOf/AnyOf/AllOf
		for i, sub := range s.OneOf {
			s.OneOf[i] = resolve(sub)
		}
		for i, sub := range s.AnyOf {
			s.AnyOf[i] = resolve(sub)
		}
		for i, sub := range s.AllOf {
			s.AllOf[i] = resolve(sub)
		}

		return s
	}

	// Walk through all Paths and Operations to inline schemas
	for _, pathItem := range api.OpenAPI().Paths {
		ops := []*huma.Operation{
			pathItem.Get, pathItem.Put, pathItem.Post, pathItem.Delete, pathItem.Options, pathItem.Head, pathItem.Patch,
		}
		for _, op := range ops {
			if op == nil {
				continue
			}

			// Inline Request Body
			if op.RequestBody != nil {
				for _, content := range op.RequestBody.Content {
					if content.Schema != nil {
						content.Schema = resolve(content.Schema)
					}
				}
			}

			// Inline Responses
			for _, resp := range op.Responses {
				for _, content := range resp.Content {
					if content.Schema != nil {
						content.Schema = resolve(content.Schema)
					}
				}
			}
		}
	}

	// Remove internal schemas from registry
	for name := range registry.Map() {
		if !keepSchemas[name] {
			delete(registry.Map(), name)
		}
	}

	return nil
}

func embedMarkdownDocs(api huma.API, docsDir string) error {
	// We only want to embed user_guide.md as the main description (Overview)
	userGuidePath := filepath.Join(docsDir, "user_guide.md")

	content, err := os.ReadFile(userGuidePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("User guide not found, skipping overview embedding", "path", userGuidePath)
			return nil
		}
		return err
	}

	// Set the API description to the user guide content
	api.OpenAPI().Info.Description = string(content)
	slog.Info("Embedded user_guide.md as API Overview")

	return nil
}
