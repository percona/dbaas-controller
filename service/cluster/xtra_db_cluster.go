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

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// pxcStatesMap matches pxc app states to cluster states.
var pxcStatesMap = map[k8sclient.ClusterState]controllerv1beta1.XtraDBClusterState{
	k8sclient.ClusterStateInvalid:  controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_INVALID,
	k8sclient.ClusterStateChanging: controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING,
	k8sclient.ClusterStateReady:    controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_READY,
	k8sclient.ClusterStateFailed:   controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_FAILED,
	k8sclient.ClusterStateDeleting: 4, // TODO: fix it
}

// Service implements methods of gRPC server and other business logic.
type Service struct {
	p *message.Printer
}

// New returns new Service instance.
func New(p *message.Printer) *Service {
	return &Service{p: p}
}

// ListXtraDBClusters returns a list of XtraDB clusters.
func (s *Service) ListXtraDBClusters(ctx context.Context, req *controllerv1beta1.ListXtraDBClustersRequest) (*controllerv1beta1.ListXtraDBClustersResponse, error) {
	client := k8sclient.NewK8Client(logger.Get(ctx))
	defer client.Cleanup()

	xtradbClusters, err := client.ListXtraDBClusters(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	res := &controllerv1beta1.ListXtraDBClustersResponse{
		Clusters: make([]*controllerv1beta1.ListXtraDBClustersResponse_Cluster, len(xtradbClusters)),
	}

	for i, cluster := range xtradbClusters {
		params := &controllerv1beta1.XtraDBClusterParams{
			ClusterSize: cluster.Size,
		}
		if cluster.PXC != nil {
			params.Pxc = &controllerv1beta1.XtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        cluster.PXC.ComputeResources.CPUM,
					MemoryBytes: cluster.PXC.ComputeResources.MemoryBytes,
				},
			}
		}
		if cluster.ProxySQL != nil {
			params.Proxysql = &controllerv1beta1.XtraDBClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        cluster.ProxySQL.ComputeResources.CPUM,
					MemoryBytes: cluster.ProxySQL.ComputeResources.MemoryBytes,
				},
			}
		}
		res.Clusters[i] = &controllerv1beta1.ListXtraDBClustersResponse_Cluster{
			Name:      cluster.Name,
			State:     pxcStatesMap[cluster.State],
			Operation: nil,
			Params:    params,
		}
	}

	return res, nil
}

// CreateXtraDBCluster creates a new XtraDB cluster.
func (s *Service) CreateXtraDBCluster(ctx context.Context, req *controllerv1beta1.CreateXtraDBClusterRequest) (*controllerv1beta1.CreateXtraDBClusterResponse, error) {
	client := k8sclient.NewK8Client(logger.Get(ctx))
	defer client.Cleanup()

	params := &k8sclient.XtraDBParams{
		Name: req.Name,
		Size: req.Params.ClusterSize,
	}
	if req.Params.Pxc != nil && req.Params.Pxc.ComputeResources != nil {
		params.PXC = &k8sclient.PXC{
			ComputeResources: &k8sclient.ComputeResources{
				CPUM:        req.Params.Pxc.ComputeResources.CpuM,
				MemoryBytes: req.Params.Pxc.ComputeResources.MemoryBytes,
			},
		}
	}
	if req.Params.Proxysql != nil && req.Params.Proxysql.ComputeResources != nil {
		params.ProxySQL = &k8sclient.ProxySQL{
			ComputeResources: &k8sclient.ComputeResources{
				CPUM:        req.Params.Proxysql.ComputeResources.CpuM,
				MemoryBytes: req.Params.Proxysql.ComputeResources.MemoryBytes,
			},
		}
	}
	err := client.CreateXtraDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.CreateXtraDBClusterResponse), nil
}

// UpdateXtraDBCluster updates existing XtraDB cluster.
func (s *Service) UpdateXtraDBCluster(ctx context.Context, req *controllerv1beta1.UpdateXtraDBClusterRequest) (*controllerv1beta1.UpdateXtraDBClusterResponse, error) {
	client := k8sclient.NewK8Client(logger.Get(ctx))
	defer client.Cleanup()

	params := &k8sclient.XtraDBParams{
		Name: req.Name,
		Size: req.Params.ClusterSize,
	}
	if req.Params.Pxc != nil && req.Params.Pxc.ComputeResources != nil {
		params.PXC = &k8sclient.PXC{
			ComputeResources: &k8sclient.ComputeResources{
				CPUM:        req.Params.Pxc.ComputeResources.CpuM,
				MemoryBytes: req.Params.Pxc.ComputeResources.MemoryBytes,
			},
		}
	}
	if req.Params.Proxysql != nil && req.Params.Proxysql.ComputeResources != nil {
		params.ProxySQL = &k8sclient.ProxySQL{
			ComputeResources: &k8sclient.ComputeResources{
				CPUM:        req.Params.Proxysql.ComputeResources.CpuM,
				MemoryBytes: req.Params.Proxysql.ComputeResources.MemoryBytes,
			},
		}
	}
	err := client.UpdateXtraDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.UpdateXtraDBClusterResponse), nil
}

// DeleteXtraDBCluster deletes XtraDB cluster.
func (s *Service) DeleteXtraDBCluster(ctx context.Context, req *controllerv1beta1.DeleteXtraDBClusterRequest) (*controllerv1beta1.DeleteXtraDBClusterResponse, error) {
	client := k8sclient.NewK8Client(logger.Get(ctx))
	defer client.Cleanup()

	err := client.DeleteXtraDBCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.DeleteXtraDBClusterResponse), nil
}

// Check interface.
var (
	_ controllerv1beta1.XtraDBClusterAPIServer = (*Service)(nil)
)
