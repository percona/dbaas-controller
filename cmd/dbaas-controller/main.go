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

package main

import (
	"log"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/percona/pmm/version"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"google.golang.org/grpc/grpclog"
	"gopkg.in/alecthomas/kingpin.v2"

	_ "github.com/percona-platform/dbaas-controller/catalog" // load messages.
	"github.com/percona-platform/dbaas-controller/service/cluster"
	"github.com/percona-platform/dbaas-controller/service/logs"
	"github.com/percona-platform/dbaas-controller/service/operator"
	"github.com/percona-platform/dbaas-controller/utils/app"
	"github.com/percona-platform/dbaas-controller/utils/logger"
	"github.com/percona-platform/dbaas-controller/utils/servers"
)

func main() {
	if version.Version == "" {
		panic("dbaas-controller version is not set during build.")
	}

	flags, err := app.Setup(&app.SetupOpts{
		Name: "dbaas-controller",
	})
	if err != nil {
		log.Fatal(err)
	}

	kingpin.Parse()

	logger.SetupGlobal()
	ctx := app.Context()
	l := logger.Get(ctx).WithField("component", "main")
	defer l.Sync() //nolint:errcheck
	if flags.LogDebug {
		l.SetLevel(logger.DebugLevel)
	}

	l.Infof("Starting...")

	// Setup grpc server
	grpclog.SetLoggerV2(l.GRPCLogger())

	gRPCServer := servers.NewGRPCServer(ctx, &servers.NewGRPCServerOpts{
		Addr: flags.GRPCAddr,
	})
	if err != nil {
		l.Fatalf("Failed to create gRPC server: %s.", err)
	}

	i18nPrinter := message.NewPrinter(language.English)
	controllerv1beta1.RegisterXtraDBClusterAPIServer(gRPCServer.GetUnderlyingServer(), cluster.NewXtraDBClusterService(i18nPrinter))
	controllerv1beta1.RegisterPSMDBClusterAPIServer(gRPCServer.GetUnderlyingServer(), cluster.NewPSMDBClusterService(i18nPrinter))
	controllerv1beta1.RegisterKubernetesClusterAPIServer(gRPCServer.GetUnderlyingServer(), cluster.NewKubernetesClusterService(i18nPrinter))
	controllerv1beta1.RegisterLogsAPIServer(gRPCServer.GetUnderlyingServer(), logs.NewService(i18nPrinter))
	controllerv1beta1.RegisterXtraDBOperatorAPIServer(gRPCServer.GetUnderlyingServer(), operator.NewXtraDBOperatorService(i18nPrinter, flags.PXCOperatorURLTemplate))
	controllerv1beta1.RegisterPSMDBOperatorAPIServer(gRPCServer.GetUnderlyingServer(), operator.NewPSMDBOperatorService(i18nPrinter, flags.PSMDBOperatorURLTemplate))

	go servers.RunDebugServer(ctx, &servers.RunDebugServerOpts{
		Addr: flags.DebugAddr,
		Readyz: func() error {
			// TODO: add your services checks here
			return nil
		},
	})

	gRPCServer.Run(ctx)
}
