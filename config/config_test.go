package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	// Setup
	os.Setenv("RULEMANAGER_SERVER_PORT", "9090")
	os.Setenv("RULEMANAGER_DATABASE_CONNECTION_STRING", "mongodb://test:27017")
	os.Setenv("RULEMANAGER_DATABASE_DATABASE_NAME", "testdb")
	defer os.Unsetenv("RULEMANAGER_SERVER_PORT")
	defer os.Unsetenv("RULEMANAGER_DATABASE_CONNECTION_STRING")
	defer os.Unsetenv("RULEMANAGER_DATABASE_DATABASE_NAME")

	// Create a dummy config file to satisfy viper.ReadInConfig if it looks for one
	// However, our LoadConfig looks for "config.yaml" in "." or "./config".
	// We can rely on env vars overriding, but ReadInConfig might fail if no file is found.
	// Let's check the implementation.
	// It returns error if ReadInConfig fails.
	// So we should create a dummy config file.

	f, err := os.Create("config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString("server:\n  port: 8080\ndatabase:\n  connection_string: \"default\"\n  database_name: \"default\"\n")
	assert.NoError(t, err)
	f.Close()
	defer os.Remove("config.yaml")

	// Reset viper
	viper.Reset()

	// Execute
	cfg, err := LoadConfig()

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 9090, cfg.Server.Port) // Env var should override file
	assert.Equal(t, "mongodb://test:27017", cfg.Database.ConnectionString)
	assert.Equal(t, "testdb", cfg.Database.DatabaseName)
}
