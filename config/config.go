package config

import (
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	Server          ServerConfig   `mapstructure:"server"`
	Database        DatabaseConfig `mapstructure:"database"`
	TemplateStorage StorageConfig  `mapstructure:"template_storage"`
}

// ServerConfig holds the HTTP server configuration.
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// DatabaseConfig holds the database connection configuration.
type DatabaseConfig struct {
	ConnectionString string `mapstructure:"connection_string"`
	DatabaseName     string `mapstructure:"database_name"`
}

// StorageConfig holds the template storage configuration.
type StorageConfig struct {
	Type    string          `mapstructure:"type"`
	MongoDB DatabaseConfig  `mapstructure:"mongodb"`
	File    FileStoreConfig `mapstructure:"file"`
}

// FileStoreConfig holds the file system storage configuration.
type FileStoreConfig struct {
	Path string `mapstructure:"path"`
}

// LoadConfig reads the configuration from config files and environment variables.
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")
	viper.AddConfigPath("../..") // Check project root if running from cmd/rulemanager

	viper.SetEnvPrefix("RULEMANAGER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
