package database

import (
	"context"
	"sync"
)

// CachingTemplateProvider wraps a TemplateProvider and caches the results.
type CachingTemplateProvider struct {
	provider  TemplateProvider
	schemas   sync.Map
	templates sync.Map
}

// NewCachingTemplateProvider creates a new CachingTemplateProvider.
func NewCachingTemplateProvider(provider TemplateProvider) *CachingTemplateProvider {
	return &CachingTemplateProvider{
		provider: provider,
	}
}

// GetSchema retrieves a schema by name, checking the cache first.
func (c *CachingTemplateProvider) GetSchema(ctx context.Context, name string) (string, error) {
	if val, ok := c.schemas.Load(name); ok {
		return val.(string), nil
	}

	schema, err := c.provider.GetSchema(ctx, name)
	if err != nil {
		return "", err
	}

	c.schemas.Store(name, schema)
	return schema, nil
}

// GetTemplate retrieves a template by name, checking the cache first.
func (c *CachingTemplateProvider) GetTemplate(ctx context.Context, name string) (string, error) {
	if val, ok := c.templates.Load(name); ok {
		return val.(string), nil
	}

	tmpl, err := c.provider.GetTemplate(ctx, name)
	if err != nil {
		return "", err
	}

	c.templates.Store(name, tmpl)
	return tmpl, nil
}

// InvalidateSchema removes a schema from the cache.
func (c *CachingTemplateProvider) InvalidateSchema(name string) {
	c.schemas.Delete(name)
}

// InvalidateTemplate removes a template from the cache.
func (c *CachingTemplateProvider) InvalidateTemplate(name string) {
	c.templates.Delete(name)
}

// Pass-through methods for creation/deletion to ensure cache invalidation

// CreateSchema creates a new schema and invalidates the cache.
func (c *CachingTemplateProvider) CreateSchema(ctx context.Context, name, content string) error {
	// Invalidate cache to ensure fresh data on next read
	c.InvalidateSchema(name)
	return c.provider.(interface {
		CreateSchema(ctx context.Context, name, content string) error
	}).CreateSchema(ctx, name, content)
}

// CreateTemplate creates a new template and invalidates the cache.
func (c *CachingTemplateProvider) CreateTemplate(ctx context.Context, name, content string) error {
	c.InvalidateTemplate(name)
	return c.provider.(interface {
		CreateTemplate(ctx context.Context, name, content string) error
	}).CreateTemplate(ctx, name, content)
}

// DeleteSchema deletes a schema and invalidates the cache.
func (c *CachingTemplateProvider) DeleteSchema(ctx context.Context, name string) error {
	c.InvalidateSchema(name)
	return c.provider.(interface {
		DeleteSchema(ctx context.Context, name string) error
	}).DeleteSchema(ctx, name)
}

// DeleteTemplate deletes a template and invalidates the cache.
func (c *CachingTemplateProvider) DeleteTemplate(ctx context.Context, name string) error {
	c.InvalidateTemplate(name)
	return c.provider.(interface {
		DeleteTemplate(ctx context.Context, name string) error
	}).DeleteTemplate(ctx, name)
}
