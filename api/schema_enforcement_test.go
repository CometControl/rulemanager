package api

import (
	"net/http"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateSchema_Enforcement(t *testing.T) {
	_, api := humatest.New(t)
	mockStore := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	svc := &rules.Service{} // Not used in this test

	NewTemplateHandlers(api, mockStore, validator, svc)

	t.Run("Reject unsupported version", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "bad-schema",
			"content": map[string]interface{}{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type":    "object",
			},
		}

		resp := api.Post("/api/v1/templates/schemas", body)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Contains(t, resp.Body.String(), "Unsupported schema version")
	})

	t.Run("Default to draft-07 if missing", func(t *testing.T) {
		mockStore.On("CreateSchema", mock.Anything, "no-schema", mock.MatchedBy(func(content string) bool {
			return assert.Contains(t, content, "http://json-schema.org/draft-07/schema")
		})).Return(nil)

		body := map[string]interface{}{
			"name": "no-schema",
			"content": map[string]interface{}{
				"type": "object",
			},
		}

		resp := api.Post("/api/v1/templates/schemas", body)
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})

	t.Run("Accept draft-07", func(t *testing.T) {
		mockStore.On("CreateSchema", mock.Anything, "good-schema", mock.MatchedBy(func(content string) bool {
			return assert.Contains(t, content, "http://json-schema.org/draft-07/schema")
		})).Return(nil)

		body := map[string]interface{}{
			"name": "good-schema",
			"content": map[string]interface{}{
				"$schema": "http://json-schema.org/draft-07/schema",
				"type":    "object",
			},
		}

		resp := api.Post("/api/v1/templates/schemas", body)
		assert.Equal(t, http.StatusNoContent, resp.Code)
	})
}
