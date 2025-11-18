package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server          ServerConfig   `mapstructure:"server"`
	Database        DatabaseConfig `mapstructure:"database"`
	TemplateStorage StorageConfig  `mapstructure:"template_storage"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	ConnectionString string `mapstructure:"connection_string"`
	DatabaseName     string `mapstructure:"database_name"`
}

type StorageConfig struct {
	Type    string          `mapstructure:"type"`
	MongoDB DatabaseConfig  `mapstructure:"mongodb"`
	File    FileStoreConfig `mapstructure:"file"`
}

type FileStoreConfig struct {
	Path string `mapstructure:"path"`
}

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
