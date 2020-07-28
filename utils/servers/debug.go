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
	"bytes"
	"context"
	_ "expvar" // register /debug/vars
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec // register /debug/pprof
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// RunDebugServerOpts configure debug server.
type RunDebugServerOpts struct {
	Addr            string
	ShutdownTimeout time.Duration
	Healthz         func() error
	Readyz          func() error
}

// RunDebugServer runs debug server with given options until ctx is canceled.
// All errors cause panic.
//nolint:funlen
func RunDebugServer(ctx context.Context, opts *RunDebugServerOpts) {
	if opts == nil {
		opts = new(RunDebugServerOpts)
	}

	l := logger.Get(ctx).WithField("component", "servers.debug")

	if opts.Addr == "" {
		l.Panicf("No Addr set.")
	}
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 3 * time.Second
	}
	if opts.Healthz == nil {
		opts.Healthz = func() error { return nil }
	}
	if opts.Readyz == nil {
		opts.Readyz = func() error { return nil }
	}

	healthzHandler := func(rw http.ResponseWriter, req *http.Request) {
		err := opts.Healthz()
		if err != nil {
			l.Errorf("Healthz: %+v.", err)
			rw.WriteHeader(500)
			fmt.Fprint(rw, err)
			return
		}

		l.Debugf("Healthz: ok.")
		rw.WriteHeader(200)
	}
	http.Handle("/debug/healthz", http.HandlerFunc(healthzHandler))

	readyzHandler := func(rw http.ResponseWriter, req *http.Request) {
		err := opts.Readyz()
		if err != nil {
			l.Warnf("Readyz: %+v.", err)
			rw.WriteHeader(500)
			fmt.Fprint(rw, err)
			return
		}

		l.Debugf("Readyz: ok.")
		rw.WriteHeader(200)
	}
	http.Handle("/debug/readyz", http.HandlerFunc(readyzHandler))

	metricsHandler := promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		ErrorLog:      &logger.PromHTTP{L: l},
		ErrorHandling: promhttp.ContinueOnError,
	})
	http.Handle("/debug/metrics", promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, metricsHandler))

	handlers := []string{
		"/debug/healthz",  // by healthzHandler above
		"/debug/readyz",   // by readyzHandler above
		"/debug/metrics",  // by metricsHandler above
		"/debug/vars",     // by expvar
		"/debug/requests", // by golang.org/x/net/trace imported by google.golang.org/grpc
		"/debug/events",   // by golang.org/x/net/trace imported by google.golang.org/grpc
		"/debug/pprof",    // by net/http/pprof
	}
	for i, h := range handlers {
		handlers[i] = "http://" + opts.Addr + h
	}

	var buf bytes.Buffer
	err := template.Must(template.New("debug").Parse(`
	<html>
	<body>
	<ul>
	{{ range . }}
		<li><a href="{{ . }}">{{ . }}</a></li>
	{{ end }}
	</ul>
	</body>
	</html>
	`)).Execute(&buf, handlers)
	if err != nil {
		l.Panicf(err.Error())
	}
	http.HandleFunc("/debug", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write(buf.Bytes()) //nolint:errcheck,gosec
	})

	l.Infof("Starting server on http://%s/debug\nRegistered handlers:\n\t%s", opts.Addr, strings.Join(handlers, "\n\t"))

	server := &http.Server{
		Addr: opts.Addr,
		ErrorLog: log.New(
			os.Stderr,
			"dbaas-controller.servers.debug.Server",
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

		// Handler defaults to http.DefaultServeMux
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
