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
		mongoStore, err := database.NewMongoStore(ctx, cfg.Database.ConnectionString, cfg.Database.DatabaseName)
		if err != nil {
			log.Fatalf("Failed to connect to MongoDB: %v", err)
		}
		defer mongoStore.Close(ctx)
		ruleStore = mongoStore
		// Wrap with caching
		templateProvider = database.NewCachingTemplateProvider(mongoStore)
	}

	// 3. Initialize Services
	validator := validation.NewJSONSchemaValidator()
	// Use the initialized store and provider
	ruleService := rules.NewService(templateProvider, validator)

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
