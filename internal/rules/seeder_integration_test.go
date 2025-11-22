package rules

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rulemanager/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	testConnectionString = "mongodb://localhost:27017"
	testDBName           = "rulemanager_test_seeder"
)

func cleanDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(testConnectionString))
	require.NoError(t, err)
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Failed to disconnect: %v", err)
		}
	}()

	err = client.Database(testDBName).Drop(ctx)
	require.NoError(t, err)
}

func TestSeedTemplates_Integration(t *testing.T) {
	// 1. Setup
	cleanDB(t)
	defer cleanDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := database.NewMongoStore(ctx, testConnectionString, testDBName)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create temporary directory for templates
	tmpDir, err := os.MkdirTemp("", "rulemanager-integration-templates")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	baseDir := filepath.Join(tmpDir, "_base")
	goTemplatesDir := filepath.Join(tmpDir, "go_templates")
	require.NoError(t, os.MkdirAll(baseDir, 0755))
	require.NoError(t, os.MkdirAll(goTemplatesDir, 0755))

	// Create test files
	schemaContent := `{"type":"object", "description":"integration test schema"}`
	templateContent := `{{ .foo }}`
	schemaName := "integration_schema"
	templateName := "integration_template"

	require.NoError(t, os.WriteFile(filepath.Join(baseDir, schemaName+".json"), []byte(schemaContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(goTemplatesDir, templateName+".tmpl"), []byte(templateContent), 0644))

	// 2. Run Seeding
	err = SeedTemplates(ctx, store, tmpDir)
	require.NoError(t, err)

	// 3. Verify in MongoDB
	fetchedSchema, err := store.GetSchema(ctx, schemaName)
	require.NoError(t, err)
	assert.Equal(t, schemaContent, fetchedSchema)

	fetchedTemplate, err := store.GetTemplate(ctx, templateName)
	require.NoError(t, err)
	assert.Equal(t, templateContent, fetchedTemplate)

	// 4. Run Seeding Again (Idempotency)
	// It should not error and should not change anything (since we don't update existing)
	err = SeedTemplates(ctx, store, tmpDir)
	require.NoError(t, err)

	fetchedSchema2, err := store.GetSchema(ctx, schemaName)
	require.NoError(t, err)
	assert.Equal(t, schemaContent, fetchedSchema2)
}
