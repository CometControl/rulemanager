package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileStore implements RuleStore and TemplateProvider using the local filesystem.
type FileStore struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStore creates a new FileStore with the given base path.
func NewFileStore(basePath string) (*FileStore, error) {
	// Ensure base directories exist
	if err := os.MkdirAll(filepath.Join(basePath, "rules"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create rules directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(basePath, "templates"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create templates directory: %w", err)
	}

	return &FileStore{
		basePath: basePath,
	}, nil
}

// Close closes the FileStore (no-op).
func (s *FileStore) Close(ctx context.Context) error {
	return nil
}

// --- RuleStore Implementation ---

// CreateRule saves a new rule to the file store.
func (s *FileStore) CreateRule(ctx context.Context, rule *Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.ID == "" {
		return errors.New("rule ID is required")
	}

	path := filepath.Join(s.basePath, "rules", rule.ID+".json")

	// Check if exists
	if _, err := os.Stat(path); err == nil {
		return errors.New("rule already exists")
	}

	data, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetRule retrieves a rule by ID from the file store.
func (s *FileStore) GetRule(ctx context.Context, id string) (*Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.basePath, "rules", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("rule not found")
		}
		return nil, err
	}

	var rule Rule
	if err := json.Unmarshal(data, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// UpdateRule updates an existing rule in the file store.
func (s *FileStore) UpdateRule(ctx context.Context, id string, rule *Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.basePath, "rules", id+".json")

	// Check if exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errors.New("rule not found")
	}

	// Ensure ID in rule matches
	rule.ID = id

	data, err := json.MarshalIndent(rule, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// DeleteRule removes a rule from the file store.
func (s *FileStore) DeleteRule(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.basePath, "rules", id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return errors.New("rule not found")
		}
		return err
	}
	return nil
}

// ListRules retrieves a paginated list of rules from the file store.
func (s *FileStore) ListRules(ctx context.Context, limit, offset int) ([]*Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rules []*Rule
	dir := filepath.Join(s.basePath, "rules")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Read all rules first (inefficient but simple for file store)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // Skip unreadable files
		}

		var rule Rule
		if err := json.Unmarshal(data, &rule); err != nil {
			continue // Skip invalid JSON
		}

		rules = append(rules, &rule)
	}

	// Apply pagination
	total := len(rules)
	if offset >= total {
		return []*Rule{}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return rules[offset:end], nil
}

// --- TemplateProvider Implementation ---

// Templates are stored as JSON files: templates/{name}_{type}.json

type fileTemplateDoc struct {
	ID      string `json:"id"`
	Type    string `json:"type"` // "schema" or "template"
	Content string `json:"content"`
}

// GetTemplate retrieves a template by name from the file store.
func (s *FileStore) GetTemplate(ctx context.Context, name string) (string, error) {
	return s.readTemplateFile(name, "template")
}

// GetSchema retrieves a schema by name from the file store.
func (s *FileStore) GetSchema(ctx context.Context, name string) (string, error) {
	return s.readTemplateFile(name, "schema")
}

// CreateTemplate saves a new template to the file store.
func (s *FileStore) CreateTemplate(ctx context.Context, name string, content string) error {
	return s.writeTemplateFile(name, "template", content)
}

// CreateSchema saves a new schema to the file store.
func (s *FileStore) CreateSchema(ctx context.Context, name string, content string) error {
	return s.writeTemplateFile(name, "schema", content)
}

// DeleteTemplate removes a template from the file store.
func (s *FileStore) DeleteTemplate(ctx context.Context, name string) error {
	return s.deleteTemplateFile(name, "template")
}

// DeleteSchema removes a schema from the file store.
func (s *FileStore) DeleteSchema(ctx context.Context, name string) error {
	return s.deleteTemplateFile(name, "schema")
}

// Helper functions

func (s *FileStore) readTemplateFile(name, typeStr string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filename: name_type.json
	filename := fmt.Sprintf("%s_%s.json", name, typeStr)
	path := filepath.Join(s.basePath, "templates", filename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if typeStr == "schema" {
				return "", errors.New("schema not found")
			}
			return "", errors.New("template not found")
		}
		return "", err
	}

	var doc fileTemplateDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return "", err
	}
	return doc.Content, nil
}

func (s *FileStore) writeTemplateFile(name, typeStr, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s_%s.json", name, typeStr)
	path := filepath.Join(s.basePath, "templates", filename)

	doc := fileTemplateDoc{
		ID:      name,
		Type:    typeStr,
		Content: content,
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (s *FileStore) deleteTemplateFile(name, typeStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s_%s.json", name, typeStr)
	path := filepath.Join(s.basePath, "templates", filename)

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already gone
		}
		return err
	}
	return nil
}
