// Package logging configures the process-wide structured logger.
package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a production JSON logger suitable for backend services.
func New(level zapcore.Level) (*zap.Logger, error) {
	configuration := zap.NewProductionConfig()
	configuration.Level = zap.NewAtomicLevelAt(level)
	configuration.OutputPaths = []string{"stdout"}
	configuration.ErrorOutputPaths = []string{"stderr"}
	configuration.EncoderConfig.TimeKey = "timestamp"
	configuration.EncoderConfig.MessageKey = "message"
	configuration.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	configuration.InitialFields = map[string]any{"service": "controlled-api"}
	return configuration.Build()
}
