package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks (reused or redefined if needed, but we can reuse the ones from rules package if exported,
// or define simple ones here since we are testing the API layer integration with services)

// We need to mock RuleStore and TemplateProvider (which is implemented by MongoStore in real app).
// For API integration test, we want to test the handler logic, routing, and interaction with service.
// We can mock the dependencies of the handler.

type MockRuleStore struct {
	mock.Mock
}

func (m *MockRuleStore) CreateRule(ctx context.Context, rule *database.Rule) error {
	args := m.Called(ctx, rule)
	rule.ID = "test-rule-id"
	return args.Error(0)
}
func (m *MockRuleStore) GetRule(ctx context.Context, id string) (*database.Rule, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*database.Rule), args.Error(1)
}
func (m *MockRuleStore) ListRules(ctx context.Context, offset, limit int) ([]*database.Rule, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*database.Rule), args.Error(1)
}
func (m *MockRuleStore) UpdateRule(ctx context.Context, id string, rule *database.Rule) error {
	args := m.Called(ctx, id, rule)
	return args.Error(0)
}
func (m *MockRuleStore) DeleteRule(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockRuleStore) SearchRules(ctx context.Context, filter database.RuleFilter) ([]*database.Rule, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*database.Rule), args.Error(1)
}

// We also need the TemplateProvider for the rules service.
type MockTemplateProvider struct {
	mock.Mock
}

func (m *MockTemplateProvider) GetSchema(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *MockTemplateProvider) GetTemplate(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *MockTemplateProvider) CreateSchema(ctx context.Context, name, content string) error {
	args := m.Called(ctx, name, content)
	return args.Error(0)
}
func (m *MockTemplateProvider) CreateTemplate(ctx context.Context, name, content string) error {
	args := m.Called(ctx, name, content)
	return args.Error(0)
}
func (m *MockTemplateProvider) DeleteSchema(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *MockTemplateProvider) DeleteTemplate(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func TestCreateRuleEndpoint(t *testing.T) {
	// Setup
	router := chi.NewMux()
	config := huma.DefaultConfig("Test API", "1.0.0")
	humaAPI := humachi.New(router, config)

	mockStore := new(MockRuleStore)
	mockTP := new(MockTemplateProvider)
	validator := validation.NewJSONSchemaValidator() // Use real validator
	ruleService := rules.NewService(mockTP, validator)

	NewRuleHandlers(humaAPI, mockStore, ruleService)

	// Test Data
	templateName := "test-template"
	schema := `{"type": "object", "properties": {"target": {"type": "object"}, "rule": {"type": "object", "properties": {"foo": {"type": "string"}}}}}`
	tmpl := `alert: {{.rule.foo}}`
	payload := map[string]interface{}{
		"templateName": templateName,
		"parameters": map[string]interface{}{
			"target": map[string]string{
				"namespace": "test",
			},
			"rules": []map[string]string{
				{"foo": "bar"},
			},
		},
	}
	body, _ := json.Marshal(payload)

	// Expectations
	mockTP.On("GetSchema", mock.Anything, templateName).Return(schema, nil)
	mockTP.On("GetTemplate", mock.Anything, templateName).Return(tmpl, nil)
	mockStore.On("CreateRule", mock.Anything, mock.AnythingOfType("*database.Rule")).Return(nil)

	// Request
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/rules", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var respBody struct {
		IDs   []string `json:"ids"`
		Count int      `json:"count"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &respBody)
	assert.NoError(t, err)
	assert.Len(t, respBody.IDs, 1)
	assert.Equal(t, "test-rule-id", respBody.IDs[0])
	assert.Equal(t, 1, respBody.Count)

	mockTP.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}
