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
	"github.com/pkg/errors"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/percona-platform/dbaas-controller/utils/convertors"
)

//nolint:gochecknoglobals
var (
	// psmdbStatesMap matches psmdb app states to cluster states.
	psmdbStatesMap = map[k8sclient.ClusterState]controllerv1beta1.PSMDBClusterState{
		k8sclient.ClusterStateInvalid:   controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_INVALID,
		k8sclient.ClusterStateChanging:  controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_CHANGING,
		k8sclient.ClusterStateReady:     controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_READY,
		k8sclient.ClusterStateFailed:    controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_FAILED,
		k8sclient.ClusterStateDeleting:  controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_DELETING,
		k8sclient.ClusterStatePaused:    controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_PAUSED,
		k8sclient.ClusterStateUpgrading: controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_UPGRADING,
	}
)

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
		diskSizeBytes, err := convertors.StrToBytes(cluster.Replicaset.DiskSize)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		params := &controllerv1beta1.PSMDBClusterParams{
			Image:       cluster.Image,
			ClusterSize: cluster.Size,
			Replicaset: &controllerv1beta1.PSMDBClusterParams_ReplicaSet{
				DiskSize: int64(diskSizeBytes),
			},
		}
		if cluster.Replicaset.ComputeResources != nil {
			cpuMillis, err := convertors.StrToMilliCPU(cluster.Replicaset.ComputeResources.CPUM)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			memoryBytes, err := convertors.StrToBytes(cluster.Replicaset.ComputeResources.MemoryBytes)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			params.Replicaset.ComputeResources = &controllerv1beta1.ComputeResources{
				CpuM:        int32(cpuMillis),
				MemoryBytes: int64(memoryBytes),
			}
		}
		res.Clusters[i] = &controllerv1beta1.ListPSMDBClustersResponse_Cluster{
			Name:  cluster.Name,
			State: psmdbStatesMap[cluster.State],
			Operation: &controllerv1beta1.RunningOperation{
				FinishedSteps: cluster.DetailedState.CountReadyPods(),
				TotalSteps:    cluster.DetailedState.CountAllPods(),
				Message:       cluster.Message,
			},
			Params:  params,
			Exposed: cluster.Exposed,
		}

		if cluster.State == k8sclient.ClusterStateReady && cluster.Pause {
			res.Clusters[i].State = controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_PAUSED
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
		Name:  req.Name,
		Image: req.Params.Image,
		Size:  req.Params.ClusterSize,
		Replicaset: &k8sclient.Replicaset{
			DiskSize: convertors.BytesToStr(req.Params.Replicaset.DiskSize),
		},
		Expose:            req.Expose,
		VersionServiceURL: req.Params.VersionServiceUrl,
	}

	if req.Pmm != nil {
		params.PMM = &k8sclient.PMM{
			PublicAddress: req.Pmm.PublicAddress,
			Login:         req.Pmm.Login,
			Password:      req.Pmm.Password,
		}
	}

	if req.Params.Replicaset.ComputeResources != nil {
		params.Replicaset.ComputeResources = computeResources(req.Params.Replicaset.ComputeResources)
	}

	err = client.CreatePSMDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return new(controllerv1beta1.CreatePSMDBClusterResponse), nil
}

// UpdatePSMDBCluster updates existing PSMDB cluster.
func (s *PSMDBClusterService) UpdatePSMDBCluster(ctx context.Context, req *controllerv1beta1.UpdatePSMDBClusterRequest) (*controllerv1beta1.UpdatePSMDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	params := &k8sclient.PSMDBParams{
		Name: req.Name,
	}

	if req.Params != nil {
		if req.Params.Suspend && req.Params.Resume {
			return nil, status.Error(codes.InvalidArgument, "field suspend and resume cannot be true simultaneously")
		}

		params.Suspend = req.Params.Suspend
		params.Resume = req.Params.Resume
		params.Size = req.Params.ClusterSize

		if req.Params.Replicaset != nil {
			params.Replicaset = new(k8sclient.Replicaset)
			if req.Params.Replicaset.ComputeResources != nil {
				if req.Params.Replicaset.ComputeResources.CpuM > 0 || req.Params.Replicaset.ComputeResources.MemoryBytes > 0 {
					params.Replicaset.ComputeResources = computeResources(req.Params.Replicaset.ComputeResources)
				}
			}
		}

		params.Image = req.Params.Image
	}

	err = client.UpdatePSMDBCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return new(controllerv1beta1.UpdatePSMDBClusterResponse), nil
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

// GetPSMDBClusterCredentials returns a PSMDB cluster connection credentials.
func (s *PSMDBClusterService) GetPSMDBClusterCredentials(ctx context.Context, req *controllerv1beta1.GetPSMDBClusterCredentialsRequest) (*controllerv1beta1.GetPSMDBClusterCredentialsResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	cluster, err := client.GetPSMDBClusterCredentials(ctx, req.Name)
	if err != nil {
		if errors.Is(err, k8sclient.ErrPSMDBClusterNotReady) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		} else if errors.Is(err, k8sclient.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &controllerv1beta1.GetPSMDBClusterCredentialsResponse{
		Credentials: &controllerv1beta1.PSMDBCredentials{
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
	_ controllerv1beta1.PSMDBClusterAPIServer = (*PSMDBClusterService)(nil)
)
