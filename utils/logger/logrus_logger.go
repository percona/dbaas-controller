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

//go:build !saas
// +build !saas

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

// SetupGlobal sets up global logrus logger.
func SetupGlobal() {
	logrus.SetFormatter(&logrus.TextFormatter{
		// Enable multiline-friendly formatter in both development (with terminal) and production (without terminal):
		// https://github.com/sirupsen/logrus/blob/839c75faf7f98a33d445d181f3018b5c3409a45e/text_formatter.go#L176-L178
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000-07:00",

		CallerPrettyfier: func(f *runtime.Frame) (function string, file string) {
			_, function = filepath.Split(f.Function)

			// keep a single directory name as a compromise between brevity and unambiguity
			var dir string
			dir, file = filepath.Split(f.File)
			dir = filepath.Base(dir)
			file = fmt.Sprintf("%s/%s:%d", dir, file, f.Line)

			return
		},
	})
}

type logrusGrpcLoggerV2 struct {
	*logrus.Entry
}

func (l *logrusGrpcLoggerV2) V(level int) bool {
	return int(l.Level) >= level
}

// NewLogger returns new LogrusLogger.
func NewLogger() Logger {
	// l := logrus.NewEntry(logrus.New())
	formater := new(logrus.TextFormatter)
	formater.TimestampFormat = time.StampNano
	l := logrus.NewEntry(&logrus.Logger{
		Out:          os.Stderr,
		Formatter:    formater,
		Hooks:        make(logrus.LevelHooks),
		Level:        logrus.InfoLevel,
		ExitFunc:     os.Exit,
		ReportCaller: false,
	})

	return &LogrusLogger{
		l: l,
	}
}

// LogrusLogger iz a wrapper for logrus .Logger.
type LogrusLogger struct {
	l *logrus.Entry
}

// Sync calls the underlying Core's Sync method, flushing any buffered log
// entries. Applications should take care to call Sync before exiting.
func (z *LogrusLogger) Sync() error {
	return nil
}

// GRPCLogger wraps zap.Logger in grpc compatible wrapper and returns it.
func (z *LogrusLogger) GRPCLogger() grpclog.LoggerV2 {
	return &logrusGrpcLoggerV2{Entry: z.l}
}

// WithField Add a single field to the Logger.
func (z *LogrusLogger) WithField(key string, value interface{}) Logger {
	return &LogrusLogger{l: z.l.WithField(key, value)}
}

// SetLevel sets log level.
func (z *LogrusLogger) SetLevel(level Level) {
	z.l.Level = logrus.Level(level)
}

func (z *LogrusLogger) Debug(args ...interface{}) { z.l.Debug(args...) } //nolint:golint
func (z *LogrusLogger) Info(args ...interface{})  { z.l.Info(args...) }  //nolint:golint
func (z *LogrusLogger) Warn(args ...interface{})  { z.l.Warn(args...) }  //nolint:golint
func (z *LogrusLogger) Error(args ...interface{}) { z.l.Error(args...) }
func (z *LogrusLogger) Fatal(args ...interface{}) { z.l.Fatal(args...) } //nolint:golint
func (z *LogrusLogger) Panic(args ...interface{}) { z.l.Panic(args...) } //nolint:golint

func (z *LogrusLogger) Debugf(format string, args ...interface{}) { z.l.Debugf(format, args...) } //nolint:golint
func (z *LogrusLogger) Infof(format string, args ...interface{})  { z.l.Infof(format, args...) }  //nolint:golint
func (z *LogrusLogger) Warnf(format string, args ...interface{})  { z.l.Warnf(format, args...) }  //nolint:golint
func (z *LogrusLogger) Errorf(format string, args ...interface{}) { z.l.Errorf(format, args...) } //nolint:golint
func (z *LogrusLogger) Fatalf(format string, args ...interface{}) { z.l.Fatalf(format, args...) } //nolint:golint
func (z *LogrusLogger) Panicf(format string, args ...interface{}) { z.l.Panicf(format, args...) } //nolint:golint

// check interfaces.
var (
	_ Logger = (*LogrusLogger)(nil)
)
