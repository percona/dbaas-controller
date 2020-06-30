// +build saas

package logger

import (
	"github.com/percona-platform/saas/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc/grpclog"
)

// NewLogger returns new ZapLogger.
func NewLogger() Logger {
	l := zap.L().Sugar()
	return &ZapLogger{
		l: l,
	}
}

// SetupGlobal sets up global zap logger.
func SetupGlobal() {
	logger.SetupGlobal()
}

// ZapLogger iz a wrapper for zap.SugaredLogger.
type ZapLogger struct {
	l *zap.SugaredLogger
}

// Sync calls the underlying Core's Sync method, flushing any buffered log
// entries. Applications should take care to call Sync before exiting.
func (z *ZapLogger) Sync() error {
	return zap.L().Sync()
}

// GRPCLogger wraps zap.Logger in grpc compatible wrapper and returns it.
func (z *ZapLogger) GRPCLogger() grpclog.LoggerV2 {
	return &logger.GRPC{SugaredLogger: z.l}
}

// WithField creates a child logger and adds structured context to it. Fields added
// to the child don't affect the parent, and vice versa.
func (z *ZapLogger) WithField(key string, value interface{}) Logger {
	return &ZapLogger{l: z.l.With(key, value)}
}

func (z *ZapLogger) Info(args ...interface{}) { z.l.Info(args...) } // nolint:golint

func (z *ZapLogger) Debugf(format string, args ...interface{}) { z.l.Debugf(format, args...) } // nolint:golint
func (z *ZapLogger) Infof(format string, args ...interface{})  { z.l.Infof(format, args...) }  // nolint:golint
func (z *ZapLogger) Warnf(format string, args ...interface{})  { z.l.Warnf(format, args...) }  // nolint:golint
func (z *ZapLogger) Errorf(format string, args ...interface{}) { z.l.Errorf(format, args...) } // nolint:golint
func (z *ZapLogger) Fatalf(format string, args ...interface{}) { z.l.Fatalf(format, args...) } // nolint:golint
func (z *ZapLogger) Panicf(format string, args ...interface{}) { z.l.Panicf(format, args...) } // nolint:golint

// check interfaces.
var (
	_ Logger = (*ZapLogger)(nil)
)
