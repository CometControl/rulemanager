package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSONSchemaValidator_Validate(t *testing.T) {
	validator := NewJSONSchemaValidator()

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name"]
	}`

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "Valid data",
			data:    `{"name": "John", "age": 30}`,
			wantErr: false,
		},
		{
			name:    "Missing required field",
			data:    `{"age": 30}`,
			wantErr: true,
		},
		{
			name:    "Invalid type",
			data:    `{"name": "John", "age": "30"}`,
			wantErr: true,
		},
		{
			name:    "Constraint violation",
			data:    `{"name": "John", "age": -1}`,
			wantErr: true,
		},
		{
			name:    "Invalid JSON",
			data:    `{"name": "John", "age": }`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(schema, []byte(tt.data))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
