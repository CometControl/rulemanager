package database

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileStore_SearchRules(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "filestore_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	store, err := NewFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test rules
	rule1 := &Rule{
		ID:           "1",
		TemplateName: "openshift",
		Parameters:   json.RawMessage(`{"target": {"namespace": "ns1", "env": "prod"}}`),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	rule2 := &Rule{
		ID:           "2",
		TemplateName: "openshift",
		Parameters:   json.RawMessage(`{"target": {"namespace": "ns2", "env": "prod"}}`),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	rule3 := &Rule{
		ID:           "3",
		TemplateName: "other",
		Parameters:   json.RawMessage(`{"target": {"namespace": "ns1"}}`),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	require.NoError(t, store.CreateRule(ctx, rule1))
	require.NoError(t, store.CreateRule(ctx, rule2))
	require.NoError(t, store.CreateRule(ctx, rule3))

	t.Run("FilterByTemplate", func(t *testing.T) {
		filter := RuleFilter{TemplateName: "openshift"}
		rules, err := store.SearchRules(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, rules, 2)
		ids := []string{rules[0].ID, rules[1].ID}
		assert.Contains(t, ids, "1")
		assert.Contains(t, ids, "2")
	})

	t.Run("FilterByParameter", func(t *testing.T) {
		filter := RuleFilter{
			Parameters: map[string]string{"target.namespace": "ns1"},
		}
		rules, err := store.SearchRules(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, rules, 2)
		ids := []string{rules[0].ID, rules[1].ID}
		assert.Contains(t, ids, "1")
		assert.Contains(t, ids, "3")
	})

	t.Run("FilterByTemplateAndParameter", func(t *testing.T) {
		filter := RuleFilter{
			TemplateName: "openshift",
			Parameters:   map[string]string{"target.namespace": "ns1"},
		}
		rules, err := store.SearchRules(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, rules, 1)
		assert.Equal(t, "1", rules[0].ID)
	})

	t.Run("NoMatches", func(t *testing.T) {
		filter := RuleFilter{
			Parameters: map[string]string{"target.env": "dev"},
		}
		rules, err := store.SearchRules(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, rules, 0)
	})
}
