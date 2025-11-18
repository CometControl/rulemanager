package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
)

func TestTemplateHandlers(t *testing.T) {
	// Setup
	router := chi.NewMux()
	config := huma.DefaultConfig("Test API", "1.0.0")
	humaAPI := humachi.New(router, config)

	mockStore := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator()
	// For these tests we don't need the real service, but we need to pass something.
	// We can pass nil if we don't call ValidateTemplate, or a real service with mocks.
	// Let's pass a service with the mock store and validator.
	service := rules.NewService(mockStore, validator)

	NewTemplateHandlers(humaAPI, mockStore, validator, service)
	t.Run("CreateSchema", func(t *testing.T) {
		name := "test-schema"
		content := `{"type": "object"}`
		payload := map[string]interface{}{
			"name":    name,
			"content": json.RawMessage(content),
		}
		body, _ := json.Marshal(payload)

		mockStore.On("CreateSchema", mock.Anything, name, mock.Anything).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/templates/schemas", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockStore.AssertExpectations(t)
	})

	// Test Create Template
	t.Run("CreateTemplate", func(t *testing.T) {
		name := "test-template"
		content := `alert: test`
		payload := map[string]string{
			"name":    name,
			"content": content,
		}
		body, _ := json.Marshal(payload)

		mockStore.On("CreateTemplate", mock.Anything, name, content).Return(nil)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/templates/go-templates", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockStore.AssertExpectations(t)
	})

	// Test Validate Template
	t.Run("ValidateTemplate", func(t *testing.T) {
		tmplContent := `alert: {{ .name }}`
		params := map[string]string{"name": "test"}
		paramsJSON, _ := json.Marshal(params)
		
		payload := map[string]interface{}{
			"templateContent": tmplContent,
			"parameters":      json.RawMessage(paramsJSON),
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/templates/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var resp struct {
			Result string `json:"result"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Equal(t, "alert: test", resp.Result)
	})
}
