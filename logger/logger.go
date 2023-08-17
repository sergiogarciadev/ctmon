package logger

import (
	"math"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func init() {
	cfg := zap.NewDevelopmentConfig()
	paths := make([]string, 1)
	paths[0] = "ctmon.log"
	cfg.OutputPaths = paths
	Logger, _ = cfg.Build()
}

func Close() {
	Logger.Sync()
}

func InitLogger(isDevelopment bool, samplingInitial int, samplingThereafter int) error {
	// Create and configure a Zap logger.  Log levels:
	//   debug = (unused).
	//   info = (default) information about each client request, and details of occasional operations.
	//   warn = a problem occurred that might correct itself.
	//   error = a problem occurred that requires investigation.
	//   fatal = application cannot continue.
	var cfg zap.Config
	if isDevelopment {
		cfg = zap.NewDevelopmentConfig() // "debug" and above; console-friendly output.
	} else {
		cfg = zap.NewProductionConfig() // "info" and above; JSON output.
		cfg.DisableCaller = true
	}
	if samplingInitial == math.MaxInt && samplingThereafter == math.MaxInt {
		cfg.Sampling = nil // Disable sampling.
	} else {
		cfg.Sampling = &zap.SamplingConfig{
			Initial:    samplingInitial,
			Thereafter: samplingThereafter,
		}
	}
	cfg.EncoderConfig.TimeKey = "@timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.EncodeDuration = zapcore.NanosDurationEncoder

	var err error
	Logger, err = cfg.Build()
	return err
}
