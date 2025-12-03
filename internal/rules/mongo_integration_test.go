package rules_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testConnectionString = "mongodb://localhost:27017"
	testDBName           = "rulemanager_integration_test"
)

func setupTestStore(t *testing.T) *database.MongoStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := database.NewMongoStore(ctx, testConnectionString, testDBName)
	if err != nil {
		t.Skipf("Skipping MongoDB integration test: %v", err)
	}

	return store
}

func teardownTestStore(t *testing.T, store *database.MongoStore) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := store.Close(ctx)
	assert.NoError(t, err)
}

func TestMongoIntegration_PerRulePipelines(t *testing.T) {
	store := setupTestStore(t)
	defer teardownTestStore(t, store)

	ctx := context.Background()

	validator := validation.NewJSONSchemaValidator()
	service := rules.NewService(store, store, validator)

	baseDir := "../../templates"
	schemaPath := filepath.Join(baseDir, "_base", "k8s.json")
	tmplPath := filepath.Join(baseDir, "go_templates", "k8s.tmpl")

	schemaContent, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	tmplContent, err := os.ReadFile(tmplPath)
	require.NoError(t, err)

	err = store.CreateSchema(ctx, "k8s", string(schemaContent))
	require.NoError(t, err)
	err = store.CreateTemplate(ctx, "k8s", string(tmplContent))
	require.NoError(t, err)

	t.Run("Success_GlobalAndPerRulePipelines", func(t *testing.T) {
		params := `{
			"target": {
				"environment": "production",
				"namespace": "backend",
				"workload": "api-server"
			},
			"common": {
				"severity": "critical"
			},
			"rules": [
				{
					"rule_type": "cpu",
					"operator": ">",
					"threshold": 0.8
				},
				{
					"rule_type": "service_up",
					"service_name": "auth-service"
				}
			]
		}`

		err := service.ValidateRule(ctx, "k8s", json.RawMessage(params))
		assert.NoError(t, err)
	})

	t.Run("Success_NoPerRulePipeline", func(t *testing.T) {
		params := `{
			"target": {
				"environment": "production",
				"namespace": "backend",
				"workload": "api-server"
			},
			"common": {
				"severity": "info"
			},
			"rules": [
				{
					"rule_type": "ram",
					"operator": ">",
					"threshold": 1024
				}
			]
		}`

		err := service.ValidateRule(ctx, "k8s", json.RawMessage(params))
		assert.NoError(t, err)
	})
}
