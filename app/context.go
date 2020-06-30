// +build !windows

package app

import (
	"context"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"

	"github.com/percona-platform/dbaas-controller/logger"
)

// Context returns main application context with set logger
// that is canceled when SIGTERM or SIGINT is received.
func Context() context.Context {
	l := logger.NewLogger()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = logger.GetCtxWithLogger(ctx, l)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGTERM, unix.SIGINT)
	go func() {
		s := <-signals
		signal.Stop(signals)
		l.Warnf("Got %s, shutting down...", unix.SignalName(s.(unix.Signal)))
		cancel()
	}()

	return ctx
}
