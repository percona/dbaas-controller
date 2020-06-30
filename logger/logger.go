// Package logger contains interface and implementations for logging.
package logger

import "google.golang.org/grpc/grpclog"

// Logger contains all methods related to zap and logrus loggers.
type Logger interface {
	Sync() error
	GRPCLogger() grpclog.LoggerV2

	WithField(key string, value interface{}) Logger

	Info(args ...interface{})

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
}
