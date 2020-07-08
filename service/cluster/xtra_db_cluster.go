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

// Package cluster TODO
package cluster

import (
	"context"

	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
)

// Service implements methods of gRPC server and other business logic.
type Service struct {
	p message.Printer
}

// New returns new Service instance.
func New(p message.Printer) *Service {
	return &Service{p: p}
}

// ListXtraDBClusters returns a list of XtraDB clusters.
func (s *Service) ListXtraDBClusters(ctx context.Context, req *controllerv1beta1.ListXtraDBClustersRequest) (*controllerv1beta1.ListXtraDBClustersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, s.p.Sprintf("not implemented"))
}

// CreateXtraDBCluster creates a new XtraDB cluster.
func (s *Service) CreateXtraDBCluster(ctx context.Context, req *controllerv1beta1.CreateXtraDBClusterRequest) (*controllerv1beta1.CreateXtraDBClusterResponse, error) {
	methodName := "CreateXtraDBCluster"
	return nil, status.Errorf(codes.Unimplemented, s.p.Sprintf("%s is not implemented", methodName))
}

// UpdateXtraDBCluster updates existing XtraDB cluster.
func (s *Service) UpdateXtraDBCluster(ctx context.Context, req *controllerv1beta1.UpdateXtraDBClusterRequest) (*controllerv1beta1.UpdateXtraDBClusterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, s.p.Sprintf("This method is not implemented yet."))
}

// DeleteXtraDBCluster deletes XtraDB cluster.
func (s *Service) DeleteXtraDBCluster(ctx context.Context, req *controllerv1beta1.DeleteXtraDBClusterRequest) (*controllerv1beta1.DeleteXtraDBClusterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, s.p.Sprintf("This method is not implemented yet."))
}

// Check interface.
var (
	_ controllerv1beta1.XtraDBClusterAPIServer = (*Service)(nil)
)
