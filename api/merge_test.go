package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeepMergeJSON(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]interface{}
		updates  map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "Simple field update",
			existing: map[string]interface{}{
				"threshold": 0.7,
				"severity":  "warning",
			},
			updates: map[string]interface{}{
				"threshold": 0.8,
			},
			expected: map[string]interface{}{
				"threshold": 0.8,
				"severity":  "warning",
			},
		},
		{
			name: "Nested object merge",
			existing: map[string]interface{}{
				"target": map[string]interface{}{
					"environment": "prod",
					"namespace":   "api",
					"workload":    "service",
				},
				"rule": map[string]interface{}{
					"threshold": 0.7,
					"severity":  "warning",
				},
			},
			updates: map[string]interface{}{
				"rule": map[string]interface{}{
					"threshold": 0.9,
				},
			},
			expected: map[string]interface{}{
				"target": map[string]interface{}{
					"environment": "prod",
					"namespace":   "api",
					"workload":    "service",
				},
				"rule": map[string]interface{}{
					"threshold": 0.9,
					"severity":  "warning",
				},
			},
		},
		{
			name: "Add new field",
			existing: map[string]interface{}{
				"threshold": 0.7,
			},
			updates: map[string]interface{}{
				"severity": "critical",
			},
			expected: map[string]interface{}{
				"threshold": 0.7,
				"severity":  "critical",
			},
		},
		{
			name: "Deep nested merge",
			existing: map[string]interface{}{
				"rule": map[string]interface{}{
					"threshold": 0.7,
					"labels": map[string]interface{}{
						"team": "platform",
						"env":  "prod",
					},
				},
			},
			updates: map[string]interface{}{
				"rule": map[string]interface{}{
					"labels": map[string]interface{}{
						"team": "backend",
					},
				},
			},
			expected: map[string]interface{}{
				"rule": map[string]interface{}{
					"threshold": 0.7,
					"labels": map[string]interface{}{
						"team": "backend",
						"env":  "prod",
					},
				},
			},
		},
		{
			name: "Array replacement",
			existing: map[string]interface{}{
				"items": []interface{}{1, 2, 3},
			},
			updates: map[string]interface{}{
				"items": []interface{}{4, 5},
			},
			expected: map[string]interface{}{
				"items": []interface{}{4, 5},
			},
		},
		{
			name:     "Empty existing",
			existing: map[string]interface{}{},
			updates: map[string]interface{}{
				"new": "value",
			},
			expected: map[string]interface{}{
				"new": "value",
			},
		},
		{
			name: "Empty updates",
			existing: map[string]interface{}{
				"existing": "value",
			},
			updates: map[string]interface{}{},
			expected: map[string]interface{}{
				"existing": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deepMergeJSON(tt.existing, tt.updates)

			// Compare as JSON to handle deep equality
			expectedJSON, _ := json.Marshal(tt.expected)
			resultJSON, _ := json.Marshal(result)

			assert.JSONEq(t, string(expectedJSON), string(resultJSON))
		})
	}
}
