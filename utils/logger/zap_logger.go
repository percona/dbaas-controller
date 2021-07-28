// dbaas-controller
// Copyright (C) 2020 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

//go:build saas
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
	logger.SetupGlobal(nil)
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

// SetLevel sets log level.
func (z *ZapLogger) SetLevel(level Level) {
	// TODO: implement it.
}

func (z *ZapLogger) Debug(args ...interface{}) { z.l.Debug(args...) } //nolint:golint
func (z *ZapLogger) Info(args ...interface{})  { z.l.Info(args...) }  //nolint:golint
func (z *ZapLogger) Warn(args ...interface{})  { z.l.Warn(args...) }  //nolint:golint
func (z *ZapLogger) Error(args ...interface{}) { z.l.Error(args...) } //nolint:golint
func (z *ZapLogger) Fatal(args ...interface{}) { z.l.Fatal(args...) } //nolint:golint
func (z *ZapLogger) Panic(args ...interface{}) { z.l.Panic(args...) } //nolint:golint

func (z *ZapLogger) Debugf(format string, args ...interface{}) { z.l.Debugf(format, args...) } //nolint:golint
func (z *ZapLogger) Infof(format string, args ...interface{})  { z.l.Infof(format, args...) }  //nolint:golint
func (z *ZapLogger) Warnf(format string, args ...interface{})  { z.l.Warnf(format, args...) }  //nolint:golint
func (z *ZapLogger) Errorf(format string, args ...interface{}) { z.l.Errorf(format, args...) } //nolint:golint
func (z *ZapLogger) Fatalf(format string, args ...interface{}) { z.l.Fatalf(format, args...) } //nolint:golint
func (z *ZapLogger) Panicf(format string, args ...interface{}) { z.l.Panicf(format, args...) } //nolint:golint

// check interfaces.
var (
	_ Logger = (*ZapLogger)(nil)
)
