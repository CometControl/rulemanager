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

func (s *Service) GenerateVMAlertConfig(ctx context.Context, rules []*database.Rule) (string, error) {
	// Group rules by template (or some other logic)
	// For simplicity, we'll put all rules in one group for now, or group by template name.
	// Let's group by template name.
	
	groups := make(map[string][]string)
	
	for _, rule := range rules {
		ruleContent, err := s.GenerateRule(ctx, rule.TemplateName, rule.Parameters)
		if err != nil {
			// Log error and continue? Or fail?
			// For now, let's return error.
			return "", err
		}
		groups[rule.TemplateName] = append(groups[rule.TemplateName], ruleContent)
	}

	// Construct YAML
	// We need to parse the generated rule content (which is YAML) and restructure it into groups.
	// However, the generated rule content is a list of alert rules (e.g. "- alert: ...").
	// So we can just concatenate them under a group.
	
	var buf bytes.Buffer
	buf.WriteString("groups:\n")
	
	for groupName, ruleContents := range groups {
		buf.WriteString(fmt.Sprintf("  - name: %s\n", groupName))
		buf.WriteString("    rules:\n")
		for _, content := range ruleContents {
			// Indent the content
			// The content is already valid YAML for a rule list item.
			// We just need to indent it by 4 spaces (under "rules:")
			// But wait, the content might be multiple lines.
			// We need to indent each line.
			
			// Simple indentation
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
