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

package cluster

import (
	"context"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

// psmdbStatesMap matches psmdb app states to cluster states.
var psmdbStatesMap = map[k8sclient.ClusterState]controllerv1beta1.PSMDBClusterState{
	k8sclient.ClusterStateInvalid:  controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_INVALID,
	k8sclient.ClusterStateChanging: controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_CHANGING,
	k8sclient.ClusterStateReady:    controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_READY,
	k8sclient.ClusterStateFailed:   controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_FAILED,
	k8sclient.ClusterStateDeleting: controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_DELETING,
}

// PSMDBClusterService implements methods of gRPC server and other business logic related to PSMDB clusters.
type PSMDBClusterService struct {
	p *message.Printer
}

// NewPSMDBClusterService returns new PSMDBClusterService instance.
func NewPSMDBClusterService(p *message.Printer) *PSMDBClusterService {
	return &PSMDBClusterService{p: p}
}

// ListPSMDBClusters returns a list of PSMDB clusters.
func (s *PSMDBClusterService) ListPSMDBClusters(ctx context.Context, req *controllerv1beta1.ListPSMDBClustersRequest) (*controllerv1beta1.ListPSMDBClustersResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, s.p.Sprintf("Cannot initialize K8s client: %s", err))
	}
	defer client.Cleanup() //nolint:errcheck

	PSMDBClusters, err := client.ListPSMDBClusters(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	res := &controllerv1beta1.ListPSMDBClustersResponse{
		Clusters: make([]*controllerv1beta1.ListPSMDBClustersResponse_Cluster, len(PSMDBClusters)),
	}

	for i, cluster := range PSMDBClusters {
		params := &controllerv1beta1.PSMDBClusterParams{
			ClusterSize: cluster.Size,
		}
		if cluster.Replicaset != nil {
			params.Replicaset = &controllerv1beta1.PSMDBClusterParams_ReplicaSet{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        cluster.Replicaset.ComputeResources.CPUM,
					MemoryBytes: cluster.Replicaset.ComputeResources.MemoryBytes,
				},
			}
		}
		res.Clusters[i] = &controllerv1beta1.ListPSMDBClustersResponse_Cluster{
			Name:      cluster.Name,
			State:     psmdbStatesMap[cluster.State],
			Operation: nil,
			Params:    params,
		}
	}

	return res, nil
}

// CreatePSMDBCluster creates a new PSMDB cluster.
func (s *PSMDBClusterService) CreatePSMDBCluster(ctx context.Context, req *controllerv1beta1.CreatePSMDBClusterRequest) (*controllerv1beta1.CreatePSMDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	params := &k8sclient.PSMDBParams{
		Name: req.Name,
		Size: req.Params.ClusterSize,
	}
	params.Replicaset = &k8sclient.Replicaset{
		ComputeResources: &k8sclient.ComputeResources{
			CPUM:        req.Params.Replicaset.ComputeResources.CpuM,
			MemoryBytes: req.Params.Replicaset.ComputeResources.MemoryBytes,
		},
	}
	err = client.CreatePSMDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.CreatePSMDBClusterResponse), nil
}

// UpdatePSMDBCluster updates existing PSMDB cluster.
func (s *PSMDBClusterService) UpdatePSMDBCluster(ctx context.Context, req *controllerv1beta1.UpdatePSMDBClusterRequest) (*controllerv1beta1.UpdatePSMDBClusterResponse, error) {
	return nil, status.Error(codes.Unimplemented, s.p.Sprintf("This method is not implemented yet."))
}

// DeletePSMDBCluster deletes PSMDB cluster.
func (s *PSMDBClusterService) DeletePSMDBCluster(ctx context.Context, req *controllerv1beta1.DeletePSMDBClusterRequest) (*controllerv1beta1.DeletePSMDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.DeletePSMDBCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.DeletePSMDBClusterResponse), nil
}

// RestartPSMDBCluster restarts PSMDB cluster.
func (s *PSMDBClusterService) RestartPSMDBCluster(ctx context.Context, req *controllerv1beta1.RestartPSMDBClusterRequest) (*controllerv1beta1.RestartPSMDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.RestartPSMDBCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.RestartPSMDBClusterResponse), nil
}

// Check interface.
var (
	_ controllerv1beta1.PSMDBClusterAPIServer = (*PSMDBClusterService)(nil)
)
