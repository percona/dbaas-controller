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
	"net"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	channelz "google.golang.org/grpc/channelz/service"

	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// GRPCServer is an interface wrapper for gRPC Server.
type GRPCServer interface {
	// Run runs the server until ctx is canceled.
	Run(ctx context.Context)

	// GetUnderlyingServer returns underlying grpc.Server, use it for your server
	// implementation registration. Don't use any control method of returned grpc.Server;
	// use GRPCServer.Run method only.
	GetUnderlyingServer() *grpc.Server
}

type grpcServer struct {
	grpc *grpc.Server

	addr            string
	shutdownTimeout time.Duration
	l               logger.Logger
}

func (s *grpcServer) GetUnderlyingServer() *grpc.Server {
	return s.grpc
}

// NewGRPCServerOpts configure gRPC server.
type NewGRPCServerOpts struct {
	Addr            string
	WarnDuration    time.Duration
	ShutdownTimeout time.Duration
}

// NewGRPCServer creates new gRPC server with given options.
func NewGRPCServer(ctx context.Context, opts *NewGRPCServerOpts) GRPCServer {
	l := logger.Get(ctx).WithField("component", "servers.grpc")

	grpc.EnableTracing = true

	if opts == nil {
		opts = new(NewGRPCServerOpts)
	}

	if opts.Addr == "" {
		l.Panicf("No Addr set.")
	}

	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 3 * time.Second
	}

	serverOpts := []grpc.ServerOption{
		grpc.ConnectionTimeout(5 * time.Second),
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), //nolint:gomnd

		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			unaryLoggingInterceptor(opts.WarnDuration),
			grpc_prometheus.UnaryServerInterceptor,
			grpc_validator.UnaryServerInterceptor(),
		)),

		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			streamLoggingInterceptor(opts.WarnDuration),
			grpc_prometheus.StreamServerInterceptor,
			grpc_validator.StreamServerInterceptor(),
		)),
	}

	return &grpcServer{
		grpc:            grpc.NewServer(serverOpts...),
		addr:            opts.Addr,
		shutdownTimeout: opts.ShutdownTimeout,
		l:               l,
	}
}

// Run runs the server until ctx is canceled.
// All errors cause panic.
func (s *grpcServer) Run(ctx context.Context) {
	// reflection should not be enabled because we don't want to expose our private APIs
	// reflection.Register(opts.Server)

	channelz.RegisterChannelzServiceToServer(s.grpc)

	grpc_prometheus.EnableHandlingTimeHistogram()
	grpc_prometheus.Register(s.grpc)

	s.l.Infof("Starting gRPC server on https://%s/ ...", s.addr)
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.l.Panicf(err.Error())
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		err := s.grpc.Serve(listener)
		s.l.Infof("Server stopped: %v.", err)
	}()

	<-ctx.Done()

	// try to stop server gracefully, then not
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	go func() {
		<-shutdownCtx.Done()
		s.grpc.Stop()
	}()
	s.grpc.GracefulStop()
	shutdownCancel()

	// listener is already closed there - Serve always closes it on exit,
	// and we can be there only if Serve already exited.
	// But we close it anyway in case gRPC breaks this contract.
	s.l.Infof("Listener closed: %v.", listener.Close())

	<-stopped
}
