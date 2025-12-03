package rules

import (
	"context"
	"os"
	"path/filepath"
	"rulemanager/internal/database"
	"testing"
	"time"

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
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(testConnectionString))
	if err != nil {
		t.Skipf("Skipping integration test: failed to create mongo client: %v", err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Logf("Failed to disconnect: %v", err)
		}
	}()

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		t.Skipf("Skipping integration test: mongodb not available: %v", err)
	}

	err = client.Database(testDBName).Drop(ctx)
	if err != nil {
		t.Skipf("Skipping integration test: failed to drop db: %v", err)
	}
}

func TestSeedTemplates_Integration(t *testing.T) {
	// 1. Setup
	cleanDB(t)
	defer cleanDB(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := database.NewMongoStore(ctx, testConnectionString, testDBName)
	if err != nil {
		t.Skipf("Skipping integration test: %v", err)
	}
	defer store.Close(ctx)

	// Create temporary directory for templates
	tmpDir, err := os.MkdirTemp("", "rulemanager-integration-templates")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	baseDir := filepath.Join(tmpDir, "_base")
	goTemplatesDir := filepath.Join(tmpDir, "go_templates")
	require.NoError(t, os.MkdirAll(baseDir, 0o755))
	require.NoError(t, os.MkdirAll(goTemplatesDir, 0o755))

	// Create test files
	schemaContent := `{"type":"object", "description":"integration test schema"}`
	templateContent := `{{ .foo }}`
	schemaName := "integration_schema"
	templateName := "integration_template"

	require.NoError(t, os.WriteFile(filepath.Join(baseDir, schemaName+".json"), []byte(schemaContent), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(goTemplatesDir, templateName+".tmpl"), []byte(templateContent), 0o644))

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
