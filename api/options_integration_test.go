package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTemplateProviderLocal to avoid conflict if run in same package
type MockTemplateProviderLocal struct {
	mock.Mock
}

func (m *MockTemplateProviderLocal) GetSchema(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateProviderLocal) GetTemplate(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateProviderLocal) CreateSchema(ctx context.Context, name, content string) error {
	return m.Called(ctx, name, content).Error(0)
}

func (m *MockTemplateProviderLocal) DeleteSchema(ctx context.Context, name string) error {
	return m.Called(ctx, name).Error(0)
}

func (m *MockTemplateProviderLocal) CreateTemplate(ctx context.Context, name, content string) error {
	return m.Called(ctx, name, content).Error(0)
}

func (m *MockTemplateProviderLocal) DeleteTemplate(ctx context.Context, name string) error {
	return m.Called(ctx, name).Error(0)
}

func (m *MockTemplateProviderLocal) ListSchemas(ctx context.Context) ([]*database.Schema, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*database.Schema), args.Error(1)
}

// PrometheusLabelValuesResponse represents Prometheus /api/v1/label/<label>/values response
type PrometheusLabelValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

func TestGetOptions(t *testing.T) {
	// 1. Mock Datasource (Prometheus)
	promServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle /api/v1/label/environment/values
		if r.URL.Path == "/api/v1/label/environment/values" {
			match := r.URL.Query().Get("match[]")
			if match == "up" {
				resp := PrometheusLabelValuesResponse{
					Status: "success",
					Data:   []string{"prod", "dev"},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}

		// Handle /api/v1/label/deployment/values
		if r.URL.Path == "/api/v1/label/deployment/values" {
			match := r.URL.Query().Get("match[]")
			if match == "kube_deployment_created{namespace=\"prod\"}" {
				resp := PrometheusLabelValuesResponse{
					Status: "success",
					Data:   []string{"app-1", "app-2"},
				}
				_ = json.NewEncoder(w).Encode(resp)
				return
			}
		}

		// Unknown endpoint or mismatch
		resp := PrometheusLabelValuesResponse{
			Status: "success",
			Data:   []string{},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer promServer.Close()

	// 2. Mock Template Provider
	mockTP := new(MockTemplateProviderLocal)
	schema := `{
		"properties": {
			"target": {
				"properties": {
					"environment": {
						"x-dynamic-options": {
							"type": "prometheus_query",
							"label": "environment",
							"match": "up",
							"dependencies": []
						}
					},
					"workload": {
						"x-dynamic-options": {
							"type": "prometheus_query",
							"label": "deployment",
							"match": "kube_deployment_created{namespace=\"{{.target.namespace}}\"}",
							"dependencies": ["target.namespace"]
						}
					}
				}
			}
		},
		"datasource": {
			"type": "prometheus",
			"url": "` + promServer.URL + `"
		}
	}`
	mockTP.On("GetSchema", mock.Anything, "k8s").Return(schema, nil)

	// 3. Setup Service and API
	mockRS := new(MockRuleStore)
	svc := rules.NewService(mockTP, mockRS, validation.NewJSONSchemaValidator())

	apiInstance := NewAPI()
	NewRuleHandlers(apiInstance.Huma, mockRS, svc)

	// 4. Test Case 1: Simple label_values
	t.Run("Simple label_values", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"template_name":  "k8s",
			"field_path":     "target.environment",
			"current_values": map[string]interface{}{},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/options", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		apiInstance.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Options []string `json:"options"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Contains(t, resp.Options, "prod")
		assert.Contains(t, resp.Options, "dev")
	})

	// 5. Test Case 2: Dependent label_values
	t.Run("Dependent label_values", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"template_name": "k8s",
			"field_path":    "target.workload",
			"current_values": map[string]interface{}{
				"target": map[string]interface{}{
					"namespace": "prod",
				},
			},
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/rules/options", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		apiInstance.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp struct {
			Options []string `json:"options"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NoError(t, err)
		assert.Contains(t, resp.Options, "app-1")
		assert.Contains(t, resp.Options, "app-2")
	})
}
