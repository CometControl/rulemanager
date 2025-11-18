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

	"github.com/VictoriaMetrics/metricsql"
)

type Service struct {
	templateProvider  database.TemplateProvider
	validator         validation.SchemaValidator
	pipelineProcessor *PipelineProcessor
}

func NewService(tp database.TemplateProvider, v validation.SchemaValidator) *Service {
	return &Service{
		templateProvider:  tp,
		validator:         v,
		pipelineProcessor: NewPipelineProcessor(),
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

func (s *Service) GenerateVMAlertConfig(ctx context.Context, rules []*database.Rule) (string, error) {
	groups := make(map[string][]string)
	
	for _, rule := range rules {
		// We use the rule ID or template name for grouping? 
		// Let's group by template name as before.
		ruleContent, err := s.GenerateRule(ctx, rule.TemplateName, rule.Parameters)
		if err != nil {
			// Log error and continue to ensure valid rules are still generated
			// In a real app we should use a logger
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

func (s *Service) ValidateTemplate(ctx context.Context, templateContent string, parameters json.RawMessage) (string, error) {
	// 1. Render Template
	rendered, err := s.renderTemplate("validate", templateContent, parameters)
	if err != nil {
		return "", err
	}

	// 2. Validate PromQL/MetricQL
	if err := s.ValidateQuery(rendered); err != nil {
		return "", fmt.Errorf("invalid query: %w", err)
	}

	return rendered, nil
}

// ValidateQuery parses the generated rule to ensure it contains valid PromQL/MetricQL expressions.
func (s *Service) ValidateQuery(ruleYaml string) error {
	lines := strings.Split(ruleYaml, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "expr:") {
			expr := strings.TrimPrefix(trimmed, "expr:")
			expr = strings.TrimSpace(expr)
			// Handle multi-line strings (basic support)
			if expr == "|" || expr == ">" {
				continue 
			}
			// Remove quotes if present
			if strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"") {
				expr = strings.Trim(expr, "\"")
			}
			
			if _, err := metricsql.Parse(expr); err != nil {
				return err
			}
		}
	}
	return nil
}
