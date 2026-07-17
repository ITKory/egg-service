package logging

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ErrorLevel string

const (
	ErrorLevelExpected    ErrorLevel = "expected"
	ErrorLevelRecoverable ErrorLevel = "recoverable"
	ErrorLevelCritical    ErrorLevel = "critical"
	ErrorLevelFatal       ErrorLevel = "fatal"
)

type Config struct {
	Environment string
	Format      string
	Level       string
}

func FromEnv() Config {
	return Config{
		Environment: envOrDefault("APP_ENV", "development"),
		Format:      strings.TrimSpace(os.Getenv("LOG_FORMAT")),
		Level:       envOrDefault("LOG_LEVEL", "info"),
	}
}

func New(config Config) (*zap.Logger, error) {
	level, err := parseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	production := strings.EqualFold(config.Environment, "production") || strings.EqualFold(config.Environment, "prod")
	zapConfig := zap.NewDevelopmentConfig()
	if production || strings.EqualFold(config.Format, "json") {
		zapConfig = zap.NewProductionConfig()
	}

	zapConfig.Level = zap.NewAtomicLevelAt(level)
	zapConfig.Encoding = normalizeFormat(config.Format, production)
	zapConfig.EncoderConfig.TimeKey = "ts"
	zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	return zapConfig.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
}

func Error(err error) zap.Field {
	return zap.Error(err)
}

func ErrorLevelField(level ErrorLevel) zap.Field {
	return zap.String("error_level", string(level))
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func normalizeFormat(format string, production bool) string {
	if format == "" && production {
		return "json"
	}
	if strings.EqualFold(format, "json") {
		return "json"
	}
	return "console"
}

func parseLevel(value string) (zapcore.Level, error) {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(strings.TrimSpace(value)))); err != nil {
		return level, fmt.Errorf("parse log level %q: %w", value, err)
	}
	return level, nil
}
