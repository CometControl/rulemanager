package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"rulemanager/internal/database"
	"rulemanager/internal/validation"
	"text/template"
)

type Service struct {
	templateProvider database.TemplateProvider
	validator        validation.SchemaValidator
}

func NewService(tp database.TemplateProvider, v validation.SchemaValidator) *Service {
	return &Service{
		templateProvider: tp,
		validator:        v,
	}
}

func (s *Service) GenerateRule(ctx context.Context, templateName string, parameters json.RawMessage) (string, error) {
	// 1. Get Schema
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return "", err
	}

	// 2. Validate Parameters
	if err := s.validator.Validate(schemaStr, parameters); err != nil {
		return "", err
	}

	// 3. Get Template
	tmplStr, err := s.templateProvider.GetTemplate(ctx, templateName)
	if err != nil {
		return "", err
	}

	// 4. Render Template
	// We use html/template for safety, but text/template might be more appropriate for YAML if we want to avoid escaping.
	// However, Prometheus rules are YAML, so we need to be careful.
	// Usually text/template is used for config generation.
	// Let's switch to text/template.
	tmpl, err := template.New(templateName).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var paramMap map[string]interface{}
	if err := json.Unmarshal(parameters, &paramMap); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, paramMap); err != nil {
		return "", err
	}

	return buf.String(), nil
}
