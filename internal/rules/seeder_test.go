package rules

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeedTemplates(t *testing.T) {
	// Setup temporary directory for templates
	tmpDir, err := os.MkdirTemp("", "rulemanager-templates")
	assert.NoError(t, err)

	// Create structure
	baseDir := filepath.Join(tmpDir, "_base")
	goTemplatesDir := filepath.Join(tmpDir, "go_templates")
	assert.NoError(t, os.MkdirAll(baseDir, 0o755))
	assert.NoError(t, os.MkdirAll(goTemplatesDir, 0o755))

	// Create dummy files
	schemaContent := `{"type":"object"}`
	templateContent := `{{ .foo }}`
	assert.NoError(t, os.WriteFile(filepath.Join(baseDir, "test_schema.json"), []byte(schemaContent), 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(goTemplatesDir, "test_template.tmpl"), []byte(templateContent), 0o644))

	t.Run("Seeds new templates", func(t *testing.T) {
		mockProvider := new(MockTemplateProvider)
		ctx := context.Background()

		// Expect GetSchema -> Not Found
		mockProvider.On("GetSchema", ctx, "test_schema").Return("", errors.New("not found"))
		// Expect CreateSchema
		mockProvider.On("CreateSchema", ctx, "test_schema", schemaContent).Return(nil)

		// Expect GetTemplate -> Not Found
		mockProvider.On("GetTemplate", ctx, "test_template").Return("", errors.New("not found"))
		// Expect CreateTemplate
		mockProvider.On("CreateTemplate", ctx, "test_template", templateContent).Return(nil)

		err := SeedTemplates(ctx, mockProvider, tmpDir)
		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Skips existing templates", func(t *testing.T) {
		mockProvider := new(MockTemplateProvider)
		ctx := context.Background()

		// Expect GetSchema -> Found
		mockProvider.On("GetSchema", ctx, "test_schema").Return("existing content", nil)
		// Expect NO CreateSchema call

		// Expect GetTemplate -> Found
		mockProvider.On("GetTemplate", ctx, "test_template").Return("existing content", nil)
		// Expect NO CreateTemplate call

		err := SeedTemplates(ctx, mockProvider, tmpDir)
		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})
}
