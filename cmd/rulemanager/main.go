package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"rulemanager/api"
	"rulemanager/config"
	"rulemanager/internal/database"
	"rulemanager/internal/logger"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Logger
	logger.Setup(cfg.Logging)
	slog.Info("Rule Manager starting...")

	// 3. Initialize Database/Store
	ctx := context.Background()
	var ruleStore database.RuleStore
	var templateProvider database.TemplateProvider

	if cfg.TemplateStorage.Type == "file" {
		slog.Info("Using File Store (Local Mode)")
		path := cfg.TemplateStorage.File.Path
		if path == "" {
			path = "./data" // Default path
		}
		fileStore, err := database.NewFileStore(path)
		if err != nil {
			slog.Error("Failed to initialize file store", "error", err)
			os.Exit(1)
		}
		ruleStore = fileStore
		// Wrap with caching
		templateProvider = database.NewCachingTemplateProvider(fileStore)
	} else {
		slog.Info("Using MongoDB Store")

		// Initialize Rule Store
		ruleMongoStore, err := database.NewMongoStore(ctx, cfg.Database.ConnectionString, cfg.Database.DatabaseName)
		if err != nil {
			slog.Error("Failed to connect to Rules MongoDB", "error", err)
			os.Exit(1)
		}
		defer ruleMongoStore.Close(ctx)
		ruleStore = ruleMongoStore

		// Initialize Template Provider
		tmplConnStr := cfg.TemplateStorage.MongoDB.ConnectionString
		tmplDBName := cfg.TemplateStorage.MongoDB.DatabaseName

		if tmplConnStr == "" {
			tmplConnStr = cfg.Database.ConnectionString
		}
		if tmplDBName == "" {
			tmplDBName = cfg.Database.DatabaseName
		}

		if tmplConnStr == cfg.Database.ConnectionString && tmplDBName == cfg.Database.DatabaseName {
			templateProvider = database.NewCachingTemplateProvider(ruleMongoStore)
		} else {
			slog.Info("Using separate MongoDB for Templates", "database", tmplDBName)
			tmplMongoStore, err := database.NewMongoStore(ctx, tmplConnStr, tmplDBName)
			if err != nil {
				slog.Error("Failed to connect to Templates MongoDB", "error", err)
				os.Exit(1)
			}
			defer tmplMongoStore.Close(ctx)
			templateProvider = database.NewCachingTemplateProvider(tmplMongoStore)
		}
	}

	// 4. Initialize Services
	validator := validation.NewJSONSchemaValidator()
	// Use the initialized store and provider
	ruleService := rules.NewService(templateProvider, validator)

	// Seed default templates
	if err := rules.SeedTemplates(ctx, templateProvider, "./templates"); err != nil {
		slog.Warn("Failed to seed templates", "error", err)
	}

	// 5. Initialize API
	apiInstance := api.NewAPI()
	api.NewRuleHandlers(apiInstance.Huma, ruleStore, ruleService)
	api.NewTemplateHandlers(apiInstance.Huma, templateProvider, validator, ruleService)

	// 6. Start Server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("Server listening", "address", addr)
	if err := apiInstance.Start(addr); err != nil {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
