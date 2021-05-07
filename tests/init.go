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
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	dbaasClient "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

//nolint:gochecknoglobals
var (
	// Context is canceled on SIGTERM or SIGINT. Tests should cleanup and exit.
	Context context.Context

	// True if -debug or -trace flag is passed.
	Debug bool

	// PMM Server Params.
	PMMServerParams *dbaasClient.PMMParams

	// XtraDBClusterAPIClient contains client for dbaas-controller API related to XtraDB clusters.
	XtraDBClusterAPIClient dbaasClient.XtraDBClusterAPIClient

	// PSMDBClusterAPIClient contains client for dbaas-controller API related to PSMDB clusters.
	PSMDBClusterAPIClient dbaasClient.PSMDBClusterAPIClient

	// KubernetesClusterAPIClient contains client for dbaas-controller API related to Kubernetes clusters.
	KubernetesClusterAPIClient dbaasClient.KubernetesClusterAPIClient

	// LogsAPIClient contails client for dbaas-controller API related to database cluster's logs
	LogsAPIClient dbaasClient.LogsAPIClient
)

//nolint:gochecknoinits
func init() {
	rand.Seed(time.Now().UnixNano())

	debugF := flag.Bool("dbaas.debug", false, "Enable debug output [DBAAS_DEBUG].")
	traceF := flag.Bool("dbaas.trace", false, "Enable trace output [DBAAS_TRACE].")
	serverURLF := flag.String("dbaas.server-url", "127.0.0.1:20201", "DBaas Controller URL [DBAAS_SERVER_URL].")
	pmmServerURLF := flag.String("pmm.server-url", "", "PMM Server URL in `https://username:password@pmm-server-host/` format") // FIXME: fix this once we start using CI.

	testing.Init()
	flag.Parse()

	for envVar, f := range map[string]*flag.Flag{
		"DBAAS_DEBUG":      flag.Lookup("dbaas.debug"),
		"DBAAS_TRACE":      flag.Lookup("dbaas.trace"),
		"DBAAS_SERVER_URL": flag.Lookup("dbaas.server-url"),
		"PMM_SERVER_URL":   flag.Lookup("pmm.server-url"),
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

	if *pmmServerURLF != "" {
		pmmServerURL, err := url.Parse(*pmmServerURLF)
		if err != nil {
			logrus.Fatalf("failed to parse PMM server URL: %s", err)
		}
		password, _ := pmmServerURL.User.Password()
		PMMServerParams = &dbaasClient.PMMParams{
			PublicAddress: pmmServerURL.Hostname(),
			Login:         pmmServerURL.User.Username(),
			Password:      password,
		}
	}

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
	PSMDBClusterAPIClient = dbaasClient.NewPSMDBClusterAPIClient(cc)
	KubernetesClusterAPIClient = dbaasClient.NewKubernetesClusterAPIClient(cc)
	LogsAPIClient = dbaasClient.NewLogsAPIClient(cc)
}
