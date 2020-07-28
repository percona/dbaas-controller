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

package servers

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// RunHTTPServerOpts configure HTTP server.
type RunHTTPServerOpts struct { //nolint:unused
	Addr            string
	Handler         http.Handler
	ShutdownTimeout time.Duration
}

// RunHTTPServer runs HTTP server with given options until ctx is canceled.
// All errors cause panic.
func RunHTTPServer(ctx context.Context, opts *RunHTTPServerOpts) { //nolint:unused
	if opts == nil {
		opts = new(RunHTTPServerOpts)
	}

	l := logger.Get(ctx).WithField("component", "servers.http")

	if opts.Addr == "" {
		l.Panicf("No Addr set.")
	}
	if opts.Handler == nil {
		opts.Handler = http.NotFoundHandler()
	}
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 3 * time.Second
	}

	l.Infof("Starting HTTP server on http://%s/", opts.Addr)

	server := &http.Server{
		Addr: opts.Addr,
		ErrorLog: log.New(
			os.Stderr,
			"dbaas-controller.servers.http.Server",
			log.Ldate|log.Lmicroseconds|log.Lshortfile|log.Lmsgprefix,
		),

		// propagate ctx cancellation signals to handlers
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},

		// propagate ctx cancellation signals and pass logger to handlers
		ConnContext: func(connCtx context.Context, _ net.Conn) context.Context {
			c, _ := getCtxForRequest(connCtx)
			return c
		},

		Handler: opts.Handler,
	}

	stopped := make(chan error)
	go func() {
		stopped <- server.ListenAndServe()
	}()

	// any ListenAndServe error before ctx is canceled is fatal
	select {
	case <-ctx.Done():
	case err := <-stopped:
		l.Panicf("Unexpected server stop: %v.", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), opts.ShutdownTimeout)
	if err := server.Shutdown(shutdownCtx); err != nil {
		l.Errorf("Failed to shutdown gracefully: %s", err)
	}
	shutdownCancel()

	<-stopped
	l.Info("Server stopped.")
}
