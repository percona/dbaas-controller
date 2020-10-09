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

package tests

import (
	"context"
	"flag"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"

	dbaasClient "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

//nolint:gochecknoglobals
var (
	// Context is canceled on SIGTERM or SIGINT. Tests should cleanup and exit.
	Context context.Context

	// True if -debug or -trace flag is passed.
	Debug bool

	// XtraDBClusterAPIClient contains client for dbaas-controller API.
	XtraDBClusterAPIClient dbaasClient.XtraDBClusterAPIClient

	KubernetesClusterAPIClient dbaasClient.KubernetesClusterAPIClient
)

//nolint:gochecknoinits
func init() {
	rand.Seed(time.Now().UnixNano())

	debugF := flag.Bool("dbaas.debug", false, "Enable debug output [DBAAS_DEBUG].")
	traceF := flag.Bool("dbaas.trace", false, "Enable trace output [DBAAS_TRACE].")
	serverURLF := flag.String("dbaas.server-url", "127.0.0.1:20201", "DBaas Controller URL [DBAAS_SERVER_URL].")

	testing.Init()
	flag.Parse()

	for envVar, f := range map[string]*flag.Flag{
		"DBAAS_DEBUG":      flag.Lookup("dbaas.debug"),
		"DBAAS_TRACE":      flag.Lookup("dbaas.trace"),
		"DBAAS_SERVER_URL": flag.Lookup("dbaas.server-url"),
	} {
		env, ok := os.LookupEnv(envVar)
		if ok {
			err := f.Value.Set(env)
			if err != nil {
				logrus.Fatalf("Invalid ENV variable %s: %s", envVar, env)
			}
		}
	}

	if *debugF {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if *traceF {
		logrus.SetLevel(logrus.TraceLevel)
		logrus.SetReportCaller(true)
	}
	Debug = *debugF || *traceF

	var cancel context.CancelFunc
	Context, cancel = context.WithCancel(context.Background())

	// handle termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		s := <-signals
		signal.Stop(signals)
		logrus.Warnf("Got %s, shutting down...", unix.SignalName(s.(syscall.Signal)))
		cancel()
	}()

	var err error
	logrus.Debugf("DBaaS Controller URL: %s.", *serverURLF)

	// make client and channel
	opts := []grpc.DialOption{
		// grpc.WithBlock(),
		grpc.WithInsecure(),
	}
	cc, err := grpc.DialContext(Context, *serverURLF, opts...)
	if err != nil {
		logrus.Fatalf("failed to dial server: %s", err)
	}
	XtraDBClusterAPIClient = dbaasClient.NewXtraDBClusterAPIClient(cc)
	KubernetesClusterAPIClient = dbaasClient.NewKubernetesClusterAPIClient(cc)
}
