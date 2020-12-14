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
)

//nolint:gochecknoglobals
// pxcStatesMap matches pxc app states to cluster states.
var pxcStatesMap = map[k8sclient.ClusterState]controllerv1beta1.XtraDBClusterState{
	k8sclient.ClusterStateInvalid:  controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_INVALID,
	k8sclient.ClusterStateChanging: controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING,
	k8sclient.ClusterStateReady:    controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_READY,
	k8sclient.ClusterStateFailed:   controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_FAILED,
	k8sclient.ClusterStateDeleting: controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_DELETING,
}

// XtraDBClusterService implements methods of gRPC server and other business logic related to XtraDB clusters.
type XtraDBClusterService struct {
	p *message.Printer
}

// NewXtraDBClusterService returns new XtraDBClusterService instance.
func NewXtraDBClusterService(p *message.Printer) *XtraDBClusterService {
	return &XtraDBClusterService{p: p}
}

// ListXtraDBClusters returns a list of XtraDB clusters.
func (s *XtraDBClusterService) ListXtraDBClusters(ctx context.Context, req *controllerv1beta1.ListXtraDBClustersRequest) (*controllerv1beta1.ListXtraDBClustersResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, s.p.Sprintf("Cannot initialize K8s client: %s", err))
	}
	defer client.Cleanup() //nolint:errcheck

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
			Pxc: &controllerv1beta1.XtraDBClusterParams_PXC{
				DiskSize: cluster.PXC.DiskSize,
			},
			Proxysql: &controllerv1beta1.XtraDBClusterParams_ProxySQL{
				DiskSize: cluster.ProxySQL.DiskSize,
			},
		}
		if cluster.PXC.ComputeResources != nil {
			params.Pxc.ComputeResources = &controllerv1beta1.ComputeResources{
				CpuM:        cluster.PXC.ComputeResources.CPUM,
				MemoryBytes: cluster.PXC.ComputeResources.MemoryBytes,
			}
		}
		if cluster.ProxySQL.ComputeResources != nil {
			params.Proxysql.ComputeResources = &controllerv1beta1.ComputeResources{
				CpuM:        cluster.ProxySQL.ComputeResources.CPUM,
				MemoryBytes: cluster.ProxySQL.ComputeResources.MemoryBytes,
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
func (s *XtraDBClusterService) CreateXtraDBCluster(ctx context.Context, req *controllerv1beta1.CreateXtraDBClusterRequest) (*controllerv1beta1.CreateXtraDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	params := &k8sclient.XtraDBParams{
		Name: req.Name,
		Size: req.Params.ClusterSize,
		PXC: &k8sclient.PXC{
			DiskSize: req.Params.Pxc.DiskSize,
		},
		ProxySQL: &k8sclient.ProxySQL{
			DiskSize: req.Params.Proxysql.DiskSize,
		},
	}
	params.PXC.ComputeResources = computeResources(req.Params.Pxc.ComputeResources)
	params.ProxySQL.ComputeResources = computeResources(req.Params.Proxysql.ComputeResources)

	err = client.CreateXtraDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.CreateXtraDBClusterResponse), nil
}

func computeResources(pxcRes *controllerv1beta1.ComputeResources) *k8sclient.ComputeResources {
	if pxcRes == nil {
		return nil
	}
	return &k8sclient.ComputeResources{
		CPUM:        pxcRes.CpuM,
		MemoryBytes: pxcRes.MemoryBytes,
	}
}

// UpdateXtraDBCluster updates existing XtraDB cluster.
func (s *XtraDBClusterService) UpdateXtraDBCluster(ctx context.Context, req *controllerv1beta1.UpdateXtraDBClusterRequest) (*controllerv1beta1.UpdateXtraDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	params := &k8sclient.XtraDBParams{
		Name: req.Name,
		Size: req.Params.ClusterSize,
	}

	if req.Params.Pxc.ComputeResources.CpuM > 0 || req.Params.Pxc.ComputeResources.MemoryBytes > 0 {
		params.PXC = &k8sclient.PXC{
			ComputeResources: &k8sclient.ComputeResources{
				CPUM:        req.Params.Pxc.ComputeResources.CpuM,
				MemoryBytes: req.Params.Pxc.ComputeResources.MemoryBytes,
			},
		}
	}

	if req.Params.Proxysql.ComputeResources.CpuM > 0 || req.Params.Proxysql.ComputeResources.MemoryBytes > 0 {
		params.ProxySQL = &k8sclient.ProxySQL{
			ComputeResources: &k8sclient.ComputeResources{
				CPUM:        req.Params.Proxysql.ComputeResources.CpuM,
				MemoryBytes: req.Params.Proxysql.ComputeResources.MemoryBytes,
			},
		}
	}

	err = client.UpdateXtraDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return new(controllerv1beta1.UpdateXtraDBClusterResponse), nil
}

// DeleteXtraDBCluster deletes XtraDB cluster.
func (s *XtraDBClusterService) DeleteXtraDBCluster(ctx context.Context, req *controllerv1beta1.DeleteXtraDBClusterRequest) (*controllerv1beta1.DeleteXtraDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.DeleteXtraDBCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.DeleteXtraDBClusterResponse), nil
}

// RestartXtraDBCluster restarts XtraDB cluster.
func (s *XtraDBClusterService) RestartXtraDBCluster(ctx context.Context, req *controllerv1beta1.RestartXtraDBClusterRequest) (*controllerv1beta1.RestartXtraDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.RestartXtraDBCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.RestartXtraDBClusterResponse), nil
}

// GetXtraDBCluster returns an XtraDB cluster connection credentials.
func (s XtraDBClusterService) GetXtraDBCluster(ctx context.Context, req *controllerv1beta1.GetXtraDBClusterRequest) (*controllerv1beta1.GetXtraDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	cluster, err := client.GetXtraDBCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &controllerv1beta1.GetXtraDBClusterResponse{
		Credentials: &controllerv1beta1.XtraDBCredentials{
			Username: cluster.Username,
			Password: cluster.Password,
			Host:     cluster.Host,
			Port:     cluster.Port,
		},
	}

	return resp, nil
}

// Check interface.
var (
	_ controllerv1beta1.XtraDBClusterAPIServer = (*XtraDBClusterService)(nil)
)
