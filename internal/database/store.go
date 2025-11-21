package database

import (
	"context"
	"encoding/json"
	"time"
)

// Rule represents a user-defined alert rule instance.
type Rule struct {
	ID           string          `json:"id" bson:"_id,omitempty"`
	TemplateName string          `json:"templateName" bson:"templateName"`
	Parameters   json.RawMessage `json:"parameters" bson:"parameters"`
	For          string          `json:"for,omitempty" bson:"for,omitempty"`
	CreatedAt    time.Time       `json:"createdAt" bson:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt" bson:"updatedAt"`
}

// RuleStore defines the interface for database operations on rules.
type RuleStore interface {
	CreateRule(ctx context.Context, rule *Rule) error
	GetRule(ctx context.Context, id string) (*Rule, error)
	ListRules(ctx context.Context, offset, limit int) ([]*Rule, error)
	UpdateRule(ctx context.Context, id string, rule *Rule) error
	DeleteRule(ctx context.Context, id string) error
}

// TemplateProvider defines the interface for retrieving rule templates.
type TemplateProvider interface {
	GetSchema(ctx context.Context, name string) (string, error)
	GetTemplate(ctx context.Context, name string) (string, error)
	CreateSchema(ctx context.Context, name, content string) error
	CreateTemplate(ctx context.Context, name, content string) error
	DeleteSchema(ctx context.Context, name string) error
	DeleteTemplate(ctx context.Context, name string) error
}
