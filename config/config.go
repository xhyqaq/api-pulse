package config

import (
	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Apifox   ApifoxConfig   `mapstructure:"apifox"`
	Dingtalk DingtalkConfig `mapstructure:"dingtalk"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int `mapstructure:"port"`
}

// ApifoxConfig Apifox API 配置
type ApifoxConfig struct {
	ProjectID     string `mapstructure:"project_id"`
	BranchID      string `mapstructure:"branch_id"`
	Authorization string `mapstructure:"authorization"`
	BaseURL       string `mapstructure:"base_url"`
}

// DingtalkConfig 钉钉配置
type DingtalkConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

// LoadConfig 从配置文件加载配置
func LoadConfig(path string) (*Config, error) {
	viper.SetConfigFile(path)
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
