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
	"google.golang.org/grpc/grpclog"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/percona-platform/dbaas-controller/app"
	"github.com/percona-platform/dbaas-controller/logger"
	"github.com/percona-platform/dbaas-controller/servers"
)

func main() {
	// TODO
	logger.SetupGlobal()

	ctx := app.Context()
	l := logger.Get(ctx).WithField("component", "main")
	defer l.Sync() //nolint:errcheck

	l.Infof("Starting...")

	flags, err := app.Setup(&app.SetupOpts{
		Name: "dbaas-controller",
	})
	if err != nil {
		l.Fatalf("%s", err)
	}

	kingpin.Parse()

	// TODO: create instance of your gRPC server implementation
	// s := dbaas-controller.New()

	// Setup grpc server
	grpclog.SetLoggerV2(l.GRPCLogger())

	gRPCServer := servers.NewGRPCServer(ctx, &servers.NewGRPCServerOpts{
		Addr: flags.GRPCAddr,
	})
	if err != nil {
		l.Fatalf("Failed to create gRPC server: %s.", err)
	}

	// TODO: register your gRPC server implementation
	// example.RegisterExampleAPIServer(gRPCServer.GetUnderlyingServer(), s)

	go servers.RunDebugServer(ctx, &servers.RunDebugServerOpts{
		Addr: flags.DebugAddr,
		Readyz: func() error {
			// TODO: add your services checks here
			return nil
		},
	})

	gRPCServer.Run(ctx)
}
