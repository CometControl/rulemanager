package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testConnectionString = "mongodb://localhost:27017"
	testDBName           = "rulemanager_test"
)

func setupTestStore(t *testing.T) *MongoStore {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewMongoStore(ctx, testConnectionString, testDBName)
	if err != nil {
		t.Skipf("Skipping MongoDB integration test: %v", err)
	}

	// Clean up database before test
	err = store.database.Drop(ctx)
	require.NoError(t, err)

	return store
}

func teardownTestStore(t *testing.T, store *MongoStore) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Clean up database after test
	err := store.database.Drop(ctx)
	assert.NoError(t, err)

	err = store.Close(ctx)
	assert.NoError(t, err)
}

func TestMongoStore_Rules(t *testing.T) {
	store := setupTestStore(t)
	defer teardownTestStore(t, store)

	ctx := context.Background()

	t.Run("CreateRule", func(t *testing.T) {
		rule := &Rule{
			TemplateName: "test-template",
			Parameters:   json.RawMessage(`{"key": "value"}`),
			For:          "5m",
		}

		err := store.CreateRule(ctx, rule)
		require.NoError(t, err)
		assert.NotEmpty(t, rule.ID)
		assert.False(t, rule.CreatedAt.IsZero())
		assert.False(t, rule.UpdatedAt.IsZero())
	})

	t.Run("GetRule", func(t *testing.T) {
		// Create a rule first
		rule := &Rule{
			TemplateName: "get-test",
			Parameters:   json.RawMessage(`{"foo": "bar"}`),
		}
		err := store.CreateRule(ctx, rule)
		require.NoError(t, err)

		// Get it back
		fetchedRule, err := store.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		assert.Equal(t, rule.ID, fetchedRule.ID)
		assert.Equal(t, rule.TemplateName, fetchedRule.TemplateName)
		assert.JSONEq(t, string(rule.Parameters), string(fetchedRule.Parameters))
	})

	t.Run("ListRules", func(t *testing.T) {
		// Clear existing rules for this test or just count them
		// Better to just add some known ones
		rules := []*Rule{
			{TemplateName: "list-1", Parameters: json.RawMessage(`{}`)},
			{TemplateName: "list-2", Parameters: json.RawMessage(`{}`)},
			{TemplateName: "list-3", Parameters: json.RawMessage(`{}`)},
		}

		for _, r := range rules {
			require.NoError(t, store.CreateRule(ctx, r))
		}

		// List all
		fetchedRules, err := store.ListRules(ctx, 0, 100)
		require.NoError(t, err)
		// We might have rules from previous tests if we didn't drop DB in between,
		// but setupTestStore drops it at the start of the suite.
		// However, subtests run sequentially here, so we accumulate rules.
		// We have 1 from CreateRule, 1 from GetRule, 3 from ListRules = 5 total.
		assert.GreaterOrEqual(t, len(fetchedRules), 3)

		// Test pagination
		pagedRules, err := store.ListRules(ctx, 0, 2)
		require.NoError(t, err)
		assert.Len(t, pagedRules, 2)
	})

	t.Run("UpdateRule", func(t *testing.T) {
		rule := &Rule{
			TemplateName: "update-test",
			Parameters:   json.RawMessage(`{"v": 1}`),
		}
		require.NoError(t, store.CreateRule(ctx, rule))

		rule.Parameters = json.RawMessage(`{"v": 2}`)
		err := store.UpdateRule(ctx, rule.ID, rule)
		require.NoError(t, err)

		updatedRule, err := store.GetRule(ctx, rule.ID)
		require.NoError(t, err)
		assert.JSONEq(t, `{"v": 2}`, string(updatedRule.Parameters))
	})

	t.Run("DeleteRule", func(t *testing.T) {
		rule := &Rule{
			TemplateName: "delete-test",
			Parameters:   json.RawMessage(`{}`),
		}
		require.NoError(t, store.CreateRule(ctx, rule))

		err := store.DeleteRule(ctx, rule.ID)
		require.NoError(t, err)

		_, err = store.GetRule(ctx, rule.ID)
		assert.Error(t, err)
		assert.Equal(t, "rule not found", err.Error())
	})

	t.Run("SearchRules", func(t *testing.T) {
		// Create some rules for searching
		r1 := &Rule{TemplateName: "search-1", Parameters: json.RawMessage(`{"target": {"ns": "a"}}`)}
		r2 := &Rule{TemplateName: "search-1", Parameters: json.RawMessage(`{"target": {"ns": "b"}}`)}
		r3 := &Rule{TemplateName: "search-2", Parameters: json.RawMessage(`{"target": {"ns": "a"}}`)}

		require.NoError(t, store.CreateRule(ctx, r1))
		require.NoError(t, store.CreateRule(ctx, r2))
		require.NoError(t, store.CreateRule(ctx, r3))

		// Search by template
		filter := RuleFilter{TemplateName: "search-1"}
		rules, err := store.SearchRules(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, rules, 2)

		// Search by parameter (using explicit MongoDB field name)
		filter = RuleFilter{Parameters: map[string]string{"parameters.target.ns": "a"}}
		rules, err = store.SearchRules(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, rules, 2)

		// Search by both
		filter = RuleFilter{
			TemplateName: "search-1",
			Parameters:   map[string]string{"parameters.target.ns": "a"},
		}
		rules, err = store.SearchRules(ctx, filter)
		require.NoError(t, err)
		assert.Len(t, rules, 1)
		assert.Equal(t, r1.ID, rules[0].ID)
	})
}

func TestMongoStore_Templates(t *testing.T) {
	store := setupTestStore(t)
	defer teardownTestStore(t, store)

	ctx := context.Background()

	t.Run("SchemaOperations", func(t *testing.T) {
		name := "test-schema"
		content := `{"type": "object"}`

		// Create
		err := store.CreateSchema(ctx, name, content)
		require.NoError(t, err)

		// Get
		fetchedContent, err := store.GetSchema(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, content, fetchedContent)

		// Update (Upsert)
		newContent := `{"type": "string"}`
		err = store.CreateSchema(ctx, name, newContent)
		require.NoError(t, err)

		fetchedContent, err = store.GetSchema(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, newContent, fetchedContent)

		// Delete
		err = store.DeleteSchema(ctx, name)
		require.NoError(t, err)

		_, err = store.GetSchema(ctx, name)
		assert.Error(t, err)
		assert.Equal(t, "schema not found", err.Error())
	})

	t.Run("TemplateOperations", func(t *testing.T) {
		name := "test-template"
		content := `{{ .Values }}`

		// Create
		err := store.CreateTemplate(ctx, name, content)
		require.NoError(t, err)

		// Get
		fetchedContent, err := store.GetTemplate(ctx, name)
		require.NoError(t, err)
		assert.Equal(t, content, fetchedContent)

		// Delete
		err = store.DeleteTemplate(ctx, name)
		require.NoError(t, err)

		_, err = store.GetTemplate(ctx, name)
		assert.Error(t, err)
		assert.Equal(t, "template not found", err.Error())
	})
}
