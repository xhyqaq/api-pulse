package config

import (
	"errors"
	"os"
	"strconv"
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
	ResponsibleId int    `mapstructure:"responsible_id"`
}

// DingtalkConfig 钉钉配置
type DingtalkConfig struct {
	WebhookURL string `mapstructure:"webhook_url"`
}

// LoadConfig 直接从环境变量加载配置
func LoadConfig(path string) (*Config, error) {
	// 创建配置实例
	cfg := &Config{}

	// 加载服务器配置
	port, err := strconv.Atoi(getEnvOrDefault("SERVER_PORT", "9501"))
	if err != nil {
		port = 9501 // 默认端口
	}
	cfg.Server = ServerConfig{
		Port: port,
	}

	// 加载Apifox配置
	projectID := getEnvOrDefault("APIFOX_PROJECT_ID", "") // 提供默认值
	branchID := getEnvOrDefault("APIFOX_BRANCH_ID", "")   // 提供默认值

	// 验证必要的配置项
	if projectID == "" {
		return nil, errors.New("APIFOX_PROJECT_ID 环境变量未设置")
	}
	if branchID == "" {
		return nil, errors.New("APIFOX_BRANCH_ID 环境变量未设置")
	}

	responsibleId, err := strconv.Atoi(getEnvOrDefault("APIFOX_RESPONSIBLE_ID", ""))

	cfg.Apifox = ApifoxConfig{
		ProjectID:     projectID,
		BranchID:      branchID,
		Authorization: getEnvOrDefault("APIFOX_AUTHORIZATION", ""),
		BaseURL:       getEnvOrDefault("APIFOX_BASE_URL", "https://api.apifox.com/api/v1"),
		ResponsibleId: responsibleId,
	}

	// 加载钉钉配置
	cfg.Dingtalk = DingtalkConfig{
		WebhookURL: getEnvOrDefault("DINGTALK_WEBHOOK_URL", ""),
	}

	return cfg, nil
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
