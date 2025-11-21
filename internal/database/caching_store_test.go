package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTemplateProvider is a mock for TemplateProvider interface
type MockTemplateProvider struct {
	mock.Mock
}

func (m *MockTemplateProvider) GetSchema(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateProvider) GetTemplate(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}

func (m *MockTemplateProvider) CreateSchema(ctx context.Context, name, content string) error {
	args := m.Called(ctx, name, content)
	return args.Error(0)
}

func (m *MockTemplateProvider) CreateTemplate(ctx context.Context, name, content string) error {
	args := m.Called(ctx, name, content)
	return args.Error(0)
}

func (m *MockTemplateProvider) DeleteSchema(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *MockTemplateProvider) DeleteTemplate(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func TestCachingTemplateProvider_GetSchema(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	t.Run("FirstCallFetchesFromProvider", func(t *testing.T) {
		mockProvider.On("GetSchema", ctx, "test").Return("schema content", nil).Once()

		result, err := cachingProvider.GetSchema(ctx, "test")

		assert.NoError(t, err)
		assert.Equal(t, "schema content", result)
		mockProvider.AssertExpectations(t)
	})

	t.Run("SecondCallUsesCache", func(t *testing.T) {
		// No new mock expectation - should use cache
		result, err := cachingProvider.GetSchema(ctx, "test")

		assert.NoError(t, err)
		assert.Equal(t, "schema content", result)
		mockProvider.AssertExpectations(t) // Should still pass, no new calls
	})

	t.Run("ErrorFromProvider", func(t *testing.T) {
		mockProvider.On("GetSchema", ctx, "error_schema").Return("", errors.New("not found")).Once()

		result, err := cachingProvider.GetSchema(ctx, "error_schema")

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Equal(t, "not found", err.Error())
		mockProvider.AssertExpectations(t)
	})
}

func TestCachingTemplateProvider_GetTemplate(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	t.Run("FirstCallFetchesFromProvider", func(t *testing.T) {
		mockProvider.On("GetTemplate", ctx, "test").Return("template content", nil).Once()

		result, err := cachingProvider.GetTemplate(ctx, "test")

		assert.NoError(t, err)
		assert.Equal(t, "template content", result)
		mockProvider.AssertExpectations(t)
	})

	t.Run("SecondCallUsesCache", func(t *testing.T) {
		// No new mock expectation - should use cache
		result, err := cachingProvider.GetTemplate(ctx, "test")

		assert.NoError(t, err)
		assert.Equal(t, "template content", result)
		mockProvider.AssertExpectations(t) // Should still pass, no new calls
	})

	t.Run("ErrorFromProvider", func(t *testing.T) {
		mockProvider.On("GetTemplate", ctx, "error_tmpl").Return("", errors.New("not found")).Once()

		result, err := cachingProvider.GetTemplate(ctx, "error_tmpl")

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Equal(t, "not found", err.Error())
		mockProvider.AssertExpectations(t)
	})
}

func TestCachingTemplateProvider_InvalidateSchema(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	// First, cache a schema
	mockProvider.On("GetSchema", ctx, "test").Return("schema content", nil).Once()
	_, _ = cachingProvider.GetSchema(ctx, "test")

	// Invalidate
	cachingProvider.InvalidateSchema("test")

	// Next call should fetch from provider again
	mockProvider.On("GetSchema", ctx, "test").Return("new schema content", nil).Once()
	result, err := cachingProvider.GetSchema(ctx, "test")

	assert.NoError(t, err)
	assert.Equal(t, "new schema content", result)
	mockProvider.AssertExpectations(t)
}

func TestCachingTemplateProvider_InvalidateTemplate(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	// First, cache a template
	mockProvider.On("GetTemplate", ctx, "test").Return("template content", nil).Once()
	_, _ = cachingProvider.GetTemplate(ctx, "test")

	// Invalidate
	cachingProvider.InvalidateTemplate("test")

	// Next call should fetch from provider again
	mockProvider.On("GetTemplate", ctx, "test").Return("new template content", nil).Once()
	result, err := cachingProvider.GetTemplate(ctx, "test")

	assert.NoError(t, err)
	assert.Equal(t, "new template content", result)
	mockProvider.AssertExpectations(t)
}

func TestCachingTemplateProvider_CreateSchema(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockProvider.On("CreateSchema", ctx, "new_schema", "content").Return(nil).Once()

		err := cachingProvider.CreateSchema(ctx, "new_schema", "content")

		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockProvider.On("CreateSchema", ctx, "bad_schema", "content").Return(errors.New("creation failed")).Once()

		err := cachingProvider.CreateSchema(ctx, "bad_schema", "content")

		assert.Error(t, err)
		assert.Equal(t, "creation failed", err.Error())
		mockProvider.AssertExpectations(t)
	})
}

func TestCachingTemplateProvider_CreateTemplate(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockProvider.On("CreateTemplate", ctx, "new_tmpl", "content").Return(nil).Once()

		err := cachingProvider.CreateTemplate(ctx, "new_tmpl", "content")

		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockProvider.On("CreateTemplate", ctx, "bad_tmpl", "content").Return(errors.New("creation failed")).Once()

		err := cachingProvider.CreateTemplate(ctx, "bad_tmpl", "content")

		assert.Error(t, err)
		assert.Equal(t, "creation failed", err.Error())
		mockProvider.AssertExpectations(t)
	})
}

func TestCachingTemplateProvider_DeleteSchema(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockProvider.On("DeleteSchema", ctx, "test").Return(nil).Once()

		err := cachingProvider.DeleteSchema(ctx, "test")

		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockProvider.On("DeleteSchema", ctx, "missing").Return(errors.New("not found")).Once()

		err := cachingProvider.DeleteSchema(ctx, "missing")

		assert.Error(t, err)
		mockProvider.AssertExpectations(t)
	})
}

func TestCachingTemplateProvider_DeleteTemplate(t *testing.T) {
	mockProvider := new(MockTemplateProvider)
	cachingProvider := NewCachingTemplateProvider(mockProvider)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockProvider.On("DeleteTemplate", ctx, "test").Return(nil).Once()

		err := cachingProvider.DeleteTemplate(ctx, "test")

		assert.NoError(t, err)
		mockProvider.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockProvider.On("DeleteTemplate", ctx, "missing").Return(errors.New("not found")).Once()

		err := cachingProvider.DeleteTemplate(ctx, "missing")

		assert.Error(t, err)
		mockProvider.AssertExpectations(t)
	})
}
