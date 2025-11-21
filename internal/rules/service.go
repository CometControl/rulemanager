package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"rulemanager/internal/database"
	"rulemanager/internal/validation"
	"strings"
	"text/template"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/metricsql"
	"gopkg.in/yaml.v2"
)

// Service provides methods for managing rules and templates.
type Service struct {
	templateProvider  database.TemplateProvider
	validator         validation.SchemaValidator
	pipelineProcessor *PipelineProcessor
}

// NewService creates a new Service with the given dependencies.
func NewService(tp database.TemplateProvider, v validation.SchemaValidator) *Service {
	return &Service{
		templateProvider:  tp,
		validator:         v,
		pipelineProcessor: NewPipelineProcessor(),
	}
}

// GenerateRule generates a rule configuration from a template and parameters.
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
	return s.renderTemplate(templateName, tmplStr, parameters)
}

func (s *Service) renderTemplate(name, tmplStr string, parameters json.RawMessage) (string, error) {
	tmpl, err := template.New(name).Parse(tmplStr)
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

// ValidateRule validates parameters against the schema and executes any defined pipelines.
func (s *Service) ValidateRule(ctx context.Context, templateName string, parameters json.RawMessage) error {
	// 1. Get Schema
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return err
	}

	// 2. Validate Parameters against Schema
	if err := s.validator.Validate(schemaStr, parameters); err != nil {
		return err
	}

	// 3. Parse Schema to get Pipelines and Datasource
	var schemaObj struct {
		Datasource *DatasourceConfig `json:"datasource"`
		Pipelines  []PipelineStep    `json:"pipelines"`
	}
	if err := json.Unmarshal([]byte(schemaStr), &schemaObj); err != nil {
		return fmt.Errorf("failed to parse schema for pipelines: %w", err)
	}

	// 4. Execute Pipelines
	if len(schemaObj.Pipelines) > 0 {
		if err := s.pipelineProcessor.Execute(ctx, schemaObj.Pipelines, schemaObj.Datasource, parameters); err != nil {
			return err
		}
	}

	return nil
}

// GenerateVMAlertConfig generates a vmalert configuration for a list of rules.
func (s *Service) GenerateVMAlertConfig(ctx context.Context, rules []*database.Rule) (string, error) {
	groups := make(map[string][]string)

	for _, rule := range rules {
		// Group rules by template name for organizational clarity
		ruleContent, err := s.GenerateRule(ctx, rule.TemplateName, rule.Parameters)
		if err != nil {
			// Skip rules that fail to generate and continue processing others
			fmt.Printf("Error generating rule %s: %v\n", rule.ID, err)
			continue
		}
		groups[rule.TemplateName] = append(groups[rule.TemplateName], ruleContent)
	}

	var buf bytes.Buffer
	buf.WriteString("groups:\n")

	for groupName, ruleContents := range groups {
		buf.WriteString(fmt.Sprintf("  - name: %s\n", groupName))
		buf.WriteString("    rules:\n")
		for _, content := range ruleContents {
			lines := strings.Split(content, "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					buf.WriteString("      " + line + "\n")
				}
			}
		}
	}

	return buf.String(), nil
}

// ValidateTemplate renders a template with parameters and validates the generated query.
func (s *Service) ValidateTemplate(ctx context.Context, templateContent string, parameters json.RawMessage) (string, error) {
	// 1. Render Template
	rendered, err := s.renderTemplate("validate", templateContent, parameters)
	if err != nil {
		return "", err
	}

	// 2. Validate Rule Content using vmalert config
	if err := s.ValidateRuleContent(rendered); err != nil {
		return "", fmt.Errorf("invalid rule content: %w", err)
	}

	return rendered, nil
}

// ValidateRuleContent parses the generated rule to ensure it is a valid vmalert rule.
func (s *Service) ValidateRuleContent(ruleYaml string) error {
	var rule config.Rule
	if err := yaml.Unmarshal([]byte(ruleYaml), &rule); err != nil {
		return fmt.Errorf("failed to parse rule: %w", err)
	}

	// First, validate rule structure using vmalert
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("rule validation failed: %w", err)
	}

	// Then, validate MetricsQL expression syntax
	if rule.Expr != "" {
		if _, err := metricsql.Parse(rule.Expr); err != nil {
			return fmt.Errorf("invalid MetricsQL expression: %w", err)
		}
	}

	return nil
}
