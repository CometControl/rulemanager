package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"rulemanager/internal/database"
	"rulemanager/internal/validation"
	"strings"
	"text/template"

	"dario.cat/mergo"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/metricsql"
	"github.com/stretchr/testify/assert/yaml"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Service provides methods for managing rules and templates.
type Service struct {
	templateProvider  database.TemplateProvider
	ruleStore         database.RuleStore
	validator         validation.SchemaValidator
	pipelineProcessor *PipelineProcessor
}

// NewService creates a new Service with the given dependencies.
func NewService(tp database.TemplateProvider, rs database.RuleStore, v validation.SchemaValidator) *Service {
	return &Service{
		templateProvider:  tp,
		ruleStore:         rs,
		validator:         v,
		pipelineProcessor: NewPipelineProcessor(),
	}
}

// GenerateRule generates a rule configuration from a template and parameters.
func (s *Service) GenerateRule(ctx context.Context, templateName string, parameters json.RawMessage) (string, error) {
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return "", err
	}

	if err := s.validator.Validate(schemaStr, parameters); err != nil {
		return "", err
	}

	tmplStr, err := s.templateProvider.GetTemplate(ctx, templateName)
	if err != nil {
		return "", err
	}

	return s.renderTemplate(templateName, tmplStr, parameters)
}

func (s *Service) renderTemplate(name, tmplStr string, parameters json.RawMessage) (string, error) {
	funcMap := template.FuncMap{
		"title": cases.Title(language.English).String,
	}
	tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var data interface{}
	if err := json.Unmarshal(parameters, &data); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ValidateRule validates parameters against the schema and executes any defined pipelines.
func (s *Service) ValidateRule(ctx context.Context, templateName string, parameters json.RawMessage) error {
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return err
	}

	if err := s.validator.Validate(schemaStr, parameters); err != nil {
		return err
	}

	// 1. Execute Global Pipelines
	var schemaObj struct {
		Datasource *DatasourceConfig `json:"datasource"`
		Pipelines  []PipelineStep    `json:"pipelines"`
		Properties struct {
			Rules struct {
				Items struct {
					OneOf []struct {
						Properties struct {
							RuleType struct {
								Const string `json:"const"`
							} `json:"rule_type"`
						} `json:"properties"`
						Pipelines []PipelineStep `json:"pipelines"`
					} `json:"oneOf"`
				} `json:"items"`
			} `json:"rules"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(schemaStr), &schemaObj); err != nil {
		return fmt.Errorf("failed to parse schema for pipelines: %w", err)
	}

	// Execute global pipelines
	if len(schemaObj.Pipelines) > 0 {
		if err := s.pipelineProcessor.Execute(ctx, schemaObj.Pipelines, schemaObj.Datasource, parameters); err != nil {
			return err
		}
	}

	// 2. Execute Per-Rule Pipelines
	var paramsObj struct {
		Rules []map[string]interface{} `json:"rules"`
	}
	if err := json.Unmarshal(parameters, &paramsObj); err != nil {
		return fmt.Errorf("failed to parse parameters for rules: %w", err)
	}

	// Map rule types to their schema definitions (containing pipelines)
	rulePipelines := make(map[string][]PipelineStep)
	for _, option := range schemaObj.Properties.Rules.Items.OneOf {
		if option.Properties.RuleType.Const != "" && len(option.Pipelines) > 0 {
			rulePipelines[option.Properties.RuleType.Const] = option.Pipelines
		}
	}

	// Iterate over user rules and execute corresponding pipelines
	for i, rule := range paramsObj.Rules {
		ruleType, ok := rule["rule_type"].(string)
		if !ok {
			continue // Should be caught by schema validation, but safe to skip
		}

		if pipelines, exists := rulePipelines[ruleType]; exists {
			// Create a merged context for the pipeline: Root Params + Rule Params
			// We re-marshal the root parameters to a map to merge
			var rootParams map[string]interface{}
			if err := json.Unmarshal(parameters, &rootParams); err != nil {
				return err
			}

			// Merge rule properties into root params (overwriting if collision, though structure usually differs)
			// Actually, better to keep them separate or just rely on the fact that templates access what they need.
			// If we merge `rule` into root, `threshold` becomes top-level.
			// Let's merge rule properties into the root map so {{ .threshold }} works if the pipeline expects it.
			for k, v := range rule {
				rootParams[k] = v
			}

			mergedParams, err := json.Marshal(rootParams)
			if err != nil {
				return fmt.Errorf("failed to marshal merged parameters for rule %d: %w", i, err)
			}

			if err := s.pipelineProcessor.Execute(ctx, pipelines, schemaObj.Datasource, mergedParams); err != nil {
				return fmt.Errorf("pipeline failed for rule %d (%s): %w", i, ruleType, err)
			}
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
			slog.Warn("Failed to generate rule", "id", rule.ID, "error", err)
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
	rendered, err := s.renderTemplate("validate", templateContent, parameters)
	if err != nil {
		return "", err
	}

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

// RulePlan represents the result of a rule planning operation.
type RulePlan struct {
	Action       string         `json:"action"` // "create", "update", "no_change"
	Reason       string         `json:"reason"`
	ExistingRule *database.Rule `json:"existing_rule,omitempty"`
	NewRule      *database.Rule `json:"new_rule"`
}

// PlanRuleCreation simulates rule creation and checks for conflicts.
func (s *Service) PlanRuleCreation(ctx context.Context, templateName string, parameters json.RawMessage) (*RulePlan, error) {
	// 1. Validate parameters against schema
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	if err := s.validator.Validate(schemaStr, parameters); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 2. Parse parameters
	var paramsMap map[string]interface{}
	if err := json.Unmarshal(parameters, &paramsMap); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	// 3. Determine Uniqueness Keys
	var schemaObj struct {
		UniquenessKeys []string `json:"uniqueness_keys"`
	}
	if err := json.Unmarshal([]byte(schemaStr), &schemaObj); err != nil {
		return nil, fmt.Errorf("failed to parse schema for uniqueness keys: %w", err)
	}

	uniquenessKeys := schemaObj.UniquenessKeys
	if len(uniquenessKeys) == 0 {
		// Fallback to default: target + rule_type
		uniquenessKeys = []string{"target", "rules.rule_type"}
	}

	// 4. Construct Search Filter
	filter := database.RuleFilter{
		TemplateName: templateName,
		Parameters:   make(map[string]string),
	}

	for _, key := range uniquenessKeys {
		if key == "target" {
			// Special handling for target: expand all leaf fields
			if target, ok := paramsMap["target"].(map[string]interface{}); ok {
				for k, v := range target {
					if strVal, ok := v.(string); ok {
						filter.Parameters["target."+k] = strVal
					}
				}
			}
			continue
		}

		// Handle dot notation (e.g., "rules.rule_type", "common.severity")
		val, found := getValueByPath(paramsMap, key)
		if found && val != "" {
			filter.Parameters[key] = val
		}
	}

	// 5. Search for existing rules
	existingRules, err := s.ruleStore.SearchRules(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search for existing rules: %w", err)
	}

	// 6. Determine Action
	newRule := &database.Rule{
		TemplateName: templateName,
		Parameters:   parameters,
	}

	if len(existingRules) > 0 {
		existing := existingRules[0]
		return &RulePlan{
			Action:       "update",
			Reason:       fmt.Sprintf("Rule with same uniqueness constraints (%v) already exists", uniquenessKeys),
			ExistingRule: existing,
			NewRule:      newRule,
		}, nil
	}

	return &RulePlan{
		Action:  "create",
		Reason:  "No existing rule found with these constraints",
		NewRule: newRule,
	}, nil
}

// PlanRuleUpdate simulates rule update and checks for conflicts.
func (s *Service) PlanRuleUpdate(ctx context.Context, id string, templateName string, parameters json.RawMessage) (*RulePlan, error) {
	// 1. Fetch existing rule
	existingRule, err := s.ruleStore.GetRule(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing rule: %w", err)
	}

	// 2. Merge parameters (Partial Update)
	var finalParamsJSON json.RawMessage
	if len(parameters) > 0 {
		var existingParams map[string]interface{}
		if err := json.Unmarshal(existingRule.Parameters, &existingParams); err != nil {
			return nil, fmt.Errorf("failed to parse existing rule parameters: %w", err)
		}

		var newParams map[string]interface{}
		if err := json.Unmarshal(parameters, &newParams); err != nil {
			return nil, fmt.Errorf("invalid parameters JSON: %w", err)
		}

		if err := mergo.Merge(&existingParams, newParams, mergo.WithOverride); err != nil {
			return nil, fmt.Errorf("failed to merge parameters: %w", err)
		}

		mergedJSON, err := json.Marshal(existingParams)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal merged parameters: %w", err)
		}
		finalParamsJSON = mergedJSON
	} else {
		finalParamsJSON = existingRule.Parameters
	}

	// 3. Validate merged parameters against schema
	schemaStr, err := s.templateProvider.GetSchema(ctx, templateName)
	if err != nil {
		return nil, fmt.Errorf("failed to get schema: %w", err)
	}

	if err := s.validator.Validate(schemaStr, finalParamsJSON); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// 4. Determine Uniqueness Keys
	var schemaObj struct {
		UniquenessKeys []string `json:"uniqueness_keys"`
	}
	if err := json.Unmarshal([]byte(schemaStr), &schemaObj); err != nil {
		return nil, fmt.Errorf("failed to parse schema for uniqueness keys: %w", err)
	}

	uniquenessKeys := schemaObj.UniquenessKeys
	if len(uniquenessKeys) == 0 {
		uniquenessKeys = []string{"target", "rules.rule_type"}
	}

	// 5. Construct Search Filter
	var paramsMap map[string]interface{}
	if err := json.Unmarshal(finalParamsJSON, &paramsMap); err != nil {
		return nil, fmt.Errorf("failed to parse final parameters: %w", err)
	}

	filter := database.RuleFilter{
		TemplateName: templateName,
		Parameters:   make(map[string]string),
	}

	for _, key := range uniquenessKeys {
		if key == "target" {
			if target, ok := paramsMap["target"].(map[string]interface{}); ok {
				for k, v := range target {
					if strVal, ok := v.(string); ok {
						filter.Parameters["target."+k] = strVal
					}
				}
			}
			continue
		}

		val, found := getValueByPath(paramsMap, key)
		if found && val != "" {
			filter.Parameters[key] = val
		}
	}

	// 6. Search for existing rules
	existingRules, err := s.ruleStore.SearchRules(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to search for existing rules: %w", err)
	}

	// 7. Check for conflicts (exclude current ID)
	for _, rule := range existingRules {
		if rule.ID != id {
			return &RulePlan{
				Action:       "conflict",
				Reason:       fmt.Sprintf("Rule with same uniqueness constraints (%v) already exists (ID: %s)", uniquenessKeys, rule.ID),
				ExistingRule: rule,
				NewRule: &database.Rule{
					ID:           id,
					TemplateName: templateName,
					Parameters:   finalParamsJSON,
				},
			}, nil
		}
	}

	// No conflict -> Update
	return &RulePlan{
		Action: "update",
		Reason: "No conflict found",
		NewRule: &database.Rule{
			ID:           id,
			TemplateName: templateName,
			Parameters:   finalParamsJSON,
		},
	}, nil
}

// getValueByPath extracts a string value from a map using dot notation.
// When encountering an array (e.g., "rules.rule_type"), it accesses the first element.
func getValueByPath(data map[string]interface{}, path string) (string, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case []interface{}:
			// For arrays, use the first element and continue navigation
			if len(v) == 0 {
				return "", false
			}
			// Navigate into the first array element
			if m, ok := v[0].(map[string]interface{}); ok {
				current = m[part]
			} else {
				return "", false
			}
		default:
			return "", false
		}
	}

	if str, ok := current.(string); ok {
		return str, true
	}
	return "", false
}
