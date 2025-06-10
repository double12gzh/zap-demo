package demo_config

import (
	"path/filepath"

	"go.uber.org/zap"

	"github.com/double12gzh/zap-demo/logger"
)

func DemoConfig() {
	// Initialize logger from YAML config
	configPath := filepath.Join("config", "log.yaml")
	if err := logger.InitLoggerFromYaml(configPath); err != nil {
		panic(err)
	}

	// Get logger instance
	log := logger.GetLogger()

	// Use logger
	log.Info("Logger initialized from YAML config")
	log.Debug("This is a debug message")
	log.Warn("This is a warning message")
	log.Error("This is an error message")

	// Use logger with fields
	log.WithFields(zap.String("user", "john"), zap.Int("age", 30)).
		Info("User logged in")

	// Use sugared logger
	sugar := log.GetSugaredLogger()
	sugar.Infof("Hello, %s!", "World")

	// Close logger
	if err := log.Close(); err != nil {
		panic(err)
	}
}
