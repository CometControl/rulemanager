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

	// 2. Initialize Database
	ctx := context.Background()
	mongoStore, err := database.NewMongoStore(ctx, cfg.Database.ConnectionString, cfg.Database.DatabaseName)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoStore.Close(ctx)

	// 3. Initialize Services
	validator := validation.NewJSONSchemaValidator()
	// MongoStore implements both RuleStore and TemplateProvider
	ruleService := rules.NewService(mongoStore, validator)

	// 4. Initialize API
	apiInstance := api.NewAPI()
	api.NewRuleHandlers(apiInstance.Huma, mongoStore, ruleService)

	// 5. Start Server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	fmt.Printf("Server listening on %s\n", addr)
	if err := apiInstance.Start(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
