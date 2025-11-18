package rules

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rulemanager/internal/database"
)

// SeedTemplates populates the TemplateProvider with default templates from the filesystem.
func SeedTemplates(ctx context.Context, provider database.TemplateProvider, templatesDir string) error {
	// 1. Seed Schemas from templates/_base
	schemasDir := filepath.Join(templatesDir, "_base")
	entries, err := os.ReadDir(schemasDir)
	if err != nil {
		// It's okay if the directory doesn't exist, just log/return
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read schemas directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		content, err := os.ReadFile(filepath.Join(schemasDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", entry.Name(), err)
		}

		// Check if exists
		if _, err := provider.GetSchema(ctx, name); err == nil {
			fmt.Printf("Schema %s already exists, skipping seed.\n", name)
			continue
		}

		if err := provider.CreateSchema(ctx, name, string(content)); err != nil {
			return fmt.Errorf("failed to create schema %s: %w", name, err)
		}
		fmt.Printf("Seeded schema: %s\n", name)
	}

	// 2. Seed Templates from templates/go_templates
	tmplsDir := filepath.Join(templatesDir, "go_templates")
	entries, err = os.ReadDir(tmplsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tmpl" {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".tmpl")
		content, err := os.ReadFile(filepath.Join(tmplsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed to read template file %s: %w", entry.Name(), err)
		}

		// Check if exists
		if _, err := provider.GetTemplate(ctx, name); err == nil {
			fmt.Printf("Template %s already exists, skipping seed.\n", name)
			continue
		}

		if err := provider.CreateTemplate(ctx, name, string(content)); err != nil {
			return fmt.Errorf("failed to create template %s: %w", name, err)
		}
		fmt.Printf("Seeded template: %s\n", name)
	}

	return nil
}
