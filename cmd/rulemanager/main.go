package main

import (
	"context"
	"fmt"
	"log"
	"rulemanager/api"
	"rulemanager/config"
	"rulemanager/internal/database"
	"rulemanager/internal/rules"
	"rulemanager/internal/validation"
)

func main() {
	fmt.Println("Rule Manager starting...")

	// 1. Load Configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Database/Store
	ctx := context.Background()
	var ruleStore database.RuleStore
	var templateProvider database.TemplateProvider

	if cfg.TemplateStorage.Type == "file" {
		fmt.Println("Using File Store (Local Mode)")
		path := cfg.TemplateStorage.File.Path
		if path == "" {
			path = "./data" // Default path
		}
		fileStore, err := database.NewFileStore(path)
		if err != nil {
			log.Fatalf("Failed to initialize file store: %v", err)
		}
		ruleStore = fileStore
		// Wrap with caching
		templateProvider = database.NewCachingTemplateProvider(fileStore)
	} else {
		fmt.Println("Using MongoDB Store")

		// Initialize Rule Store
		ruleMongoStore, err := database.NewMongoStore(ctx, cfg.Database.ConnectionString, cfg.Database.DatabaseName)
		if err != nil {
			log.Fatalf("Failed to connect to Rules MongoDB: %v", err)
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
			fmt.Printf("Using separate MongoDB for Templates: %s\n", tmplDBName)
			tmplMongoStore, err := database.NewMongoStore(ctx, tmplConnStr, tmplDBName)
			if err != nil {
				log.Fatalf("Failed to connect to Templates MongoDB: %v", err)
			}
			defer tmplMongoStore.Close(ctx)
			templateProvider = database.NewCachingTemplateProvider(tmplMongoStore)
		}
	}

	// 3. Initialize Services
	validator := validation.NewJSONSchemaValidator()
	// Use the initialized store and provider
	ruleService := rules.NewService(templateProvider, validator)

	// Seed default templates
	if err := rules.SeedTemplates(ctx, templateProvider, "./templates"); err != nil {
		log.Printf("Warning: Failed to seed templates: %v", err)
	}

	// 4. Initialize API
	apiInstance := api.NewAPI()
	api.NewRuleHandlers(apiInstance.Huma, ruleStore, ruleService)
	api.NewTemplateHandlers(apiInstance.Huma, templateProvider, validator, ruleService)

	// 5. Start Server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("Server listening on %s\n", addr)
	if err := apiInstance.Start(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
