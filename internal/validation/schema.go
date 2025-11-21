package validation

import (
	"errors"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaValidator defines the interface for validating JSON schemas.
type SchemaValidator interface {
	Validate(schema string, data []byte) error
}

// JSONSchemaValidator implements SchemaValidator using gojsonschema.
type JSONSchemaValidator struct{}

// NewJSONSchemaValidator creates a new JSONSchemaValidator.
func NewJSONSchemaValidator() *JSONSchemaValidator {
	return &JSONSchemaValidator{}
}

// Validate validates a JSON document against a JSON schema.
func (v *JSONSchemaValidator) Validate(schema string, data []byte) error {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	documentLoader := gojsonschema.NewBytesLoader(data)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return err
	}

	if result.Valid() {
		return nil
	}

	var errMsgs string
	for _, desc := range result.Errors() {
		errMsgs += fmt.Sprintf("- %s\n", desc)
	}
	return errors.New(errMsgs)
}
