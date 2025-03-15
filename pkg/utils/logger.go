package utils

import (
	"os"

	"github.com/sirupsen/logrus"
)

// SetupLogger 配置全局日志
func SetupLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 检查是否启用调试模式
	if os.Getenv("DEBUG") == "true" {
		logger.SetLevel(logrus.DebugLevel)
		logger.Debug("调试日志已启用")
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	return logger
}
