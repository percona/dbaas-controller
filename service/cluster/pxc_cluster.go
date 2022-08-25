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
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/percona-platform/dbaas-controller/utils/convertors"
)

//nolint:gochecknoglobals
// dbClusterStatesMap matches pxc app states to cluster states.
var dbClusterStatesMap = map[k8sclient.ClusterState]controllerv1beta1.DBClusterState{
	k8sclient.ClusterStateInvalid:   controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_INVALID,
	k8sclient.ClusterStateChanging:  controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_CHANGING,
	k8sclient.ClusterStateReady:     controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_READY,
	k8sclient.ClusterStateFailed:    controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_FAILED,
	k8sclient.ClusterStateDeleting:  controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_DELETING,
	k8sclient.ClusterStatePaused:    controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_PAUSED,
	k8sclient.ClusterStateUpgrading: controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_UPGRADING,
}

// PXCClusterService implements methods of gRPC server and other business logic related to PXC clusters.
type PXCClusterService struct { // p *message.Printer
}

// NewPXCClusterService returns new PXCClusterService instance.
func NewPXCClusterService() *PXCClusterService {
	return new(PXCClusterService)
}

// setComputeResources converts input resources and sets them to output compute resources.
func setComputeResources(inputResources *k8sclient.ComputeResources, outputResources *controllerv1beta1.ComputeResources) error {
	if inputResources == nil || outputResources == nil {
		return nil
	}

	cpuMillis, err := convertors.StrToMilliCPU(inputResources.CPUM)
	if err != nil {
		return err
	}
	memoryBytes, err := convertors.StrToBytes(inputResources.MemoryBytes)
	if err != nil {
		return err
	}

	outputResources.CpuM = int32(cpuMillis)
	outputResources.MemoryBytes = int64(memoryBytes)
	return nil
}

// ListPXCClusters returns a list of PXC clusters.
func (s *PXCClusterService) ListPXCClusters(ctx context.Context, req *controllerv1beta1.ListPXCClustersRequest) (*controllerv1beta1.ListPXCClustersResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, "Cannot initialize K8s client: "+err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	xtradbClusters, err := client.ListPXCClusters(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	res := &controllerv1beta1.ListPXCClustersResponse{
		Clusters: make([]*controllerv1beta1.ListPXCClustersResponse_Cluster, len(xtradbClusters)),
	}

	for i, cluster := range xtradbClusters {
		pxcDiskSize, err := convertors.StrToBytes(cluster.PXC.DiskSize)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		params := &controllerv1beta1.PXCClusterParams{
			ClusterSize: cluster.Size,
			Pxc: &controllerv1beta1.PXCClusterParams_PXC{
				DiskSize: int64(pxcDiskSize),
				Image:    cluster.PXC.Image,
			},
		}

		if cluster.ProxySQL != nil {
			proxySQLDiskSize, err := convertors.StrToBytes(cluster.ProxySQL.DiskSize)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			params.Proxysql = &controllerv1beta1.PXCClusterParams_ProxySQL{
				DiskSize: int64(proxySQLDiskSize),
			}
			params.Proxysql.ComputeResources = new(controllerv1beta1.ComputeResources)
			err = setComputeResources(cluster.ProxySQL.ComputeResources, params.Proxysql.ComputeResources)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		if cluster.HAProxy != nil {
			params.Haproxy = new(controllerv1beta1.PXCClusterParams_HAProxy)
			params.Haproxy.ComputeResources = new(controllerv1beta1.ComputeResources)
			err = setComputeResources(cluster.HAProxy.ComputeResources, params.Haproxy.ComputeResources)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		if cluster.PXC.ComputeResources != nil {
			params.Pxc.ComputeResources = new(controllerv1beta1.ComputeResources)
			err = setComputeResources(cluster.PXC.ComputeResources, params.Pxc.ComputeResources)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		res.Clusters[i] = &controllerv1beta1.ListPXCClustersResponse_Cluster{
			Name:  cluster.Name,
			State: dbClusterStatesMap[cluster.State],
			Operation: &controllerv1beta1.RunningOperation{
				FinishedSteps: cluster.DetailedState.CountReadyPods(),
				TotalSteps:    cluster.DetailedState.CountAllPods(),
				Message:       cluster.Message,
			},
			Params:  params,
			Exposed: cluster.Exposed,
		}
	}

	return res, nil
}

// CreatePXCCluster creates a new PXC cluster.
func (s *PXCClusterService) CreatePXCCluster(ctx context.Context, req *controllerv1beta1.CreatePXCClusterRequest) (*controllerv1beta1.CreatePXCClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	params := &k8sclient.PXCParams{
		Name: req.Name,
		Size: req.Params.ClusterSize,
		PXC: &k8sclient.PXC{
			Image:            req.Params.Pxc.Image,
			ComputeResources: computeResources(req.Params.Pxc.ComputeResources),
			DiskSize:         convertors.BytesToStr(req.Params.Pxc.DiskSize),
		},
		Expose:            req.Expose,
		VersionServiceURL: req.Params.VersionServiceUrl,
	}
	if req.Params.Proxysql != nil {
		params.ProxySQL = &k8sclient.ProxySQL{
			Image:            req.Params.Proxysql.Image,
			ComputeResources: computeResources(req.Params.Proxysql.ComputeResources),
			DiskSize:         convertors.BytesToStr(req.Params.Proxysql.DiskSize),
		}
	} else {
		params.HAProxy = &k8sclient.HAProxy{
			Image:            req.Params.Haproxy.Image,
			ComputeResources: computeResources(req.Params.Haproxy.ComputeResources),
		}
	}

	if req.Pmm != nil {
		params.PMM = &k8sclient.PMM{
			PublicAddress: req.Pmm.PublicAddress,
			Login:         req.Pmm.Login,
			Password:      req.Pmm.Password,
		}
	}
	err = client.CreatePXCCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.CreatePXCClusterResponse), nil
}

func computeResources(pxcRes *controllerv1beta1.ComputeResources) *k8sclient.ComputeResources {
	if pxcRes == nil {
		return nil
	}
	return &k8sclient.ComputeResources{
		CPUM:        convertors.MilliCPUToStr(pxcRes.CpuM),
		MemoryBytes: convertors.BytesToStr(pxcRes.MemoryBytes),
	}
}

// UpdatePXCCluster updates existing PXC cluster.
func (s *PXCClusterService) UpdatePXCCluster(ctx context.Context, req *controllerv1beta1.UpdatePXCClusterRequest) (*controllerv1beta1.UpdatePXCClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	if req.Params.Suspend && req.Params.Resume {
		return nil, status.Error(codes.InvalidArgument, "resume and suspend cannot be set together")
	}

	params := &k8sclient.PXCParams{
		Name:    req.Name,
		Size:    req.Params.ClusterSize,
		Suspend: req.Params.Suspend,
		Resume:  req.Params.Resume,
	}

	if req.Params.Pxc != nil {
		if req.Params.Pxc.ComputeResources != nil {
			if req.Params.Pxc.ComputeResources.CpuM > 0 || req.Params.Pxc.ComputeResources.MemoryBytes > 0 {
				params.PXC = &k8sclient.PXC{
					ComputeResources: computeResources(req.Params.Pxc.ComputeResources),
				}
			}
		}
		params.PXC.Image = req.Params.Pxc.Image
	}

	if req.Params.Proxysql != nil && req.Params.Proxysql.ComputeResources != nil {
		if req.Params.Proxysql.ComputeResources.CpuM > 0 || req.Params.Proxysql.ComputeResources.MemoryBytes > 0 {
			params.ProxySQL = &k8sclient.ProxySQL{
				ComputeResources: computeResources(req.Params.Proxysql.ComputeResources),
			}
		}
	}

	err = client.UpdatePXCCluster(ctx, params)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return new(controllerv1beta1.UpdatePXCClusterResponse), nil
}

// DeletePXCCluster deletes PXC cluster.
func (s *PXCClusterService) DeletePXCCluster(ctx context.Context, req *controllerv1beta1.DeletePXCClusterRequest) (*controllerv1beta1.DeletePXCClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.DeletePXCCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.DeletePXCClusterResponse), nil
}

// RestartPXCCluster restarts PXC cluster.
func (s *PXCClusterService) RestartPXCCluster(ctx context.Context, req *controllerv1beta1.RestartPXCClusterRequest) (*controllerv1beta1.RestartPXCClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.RestartPXCCluster(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return new(controllerv1beta1.RestartPXCClusterResponse), nil
}

// GetPXCClusterCredentials returns an PXC cluster connection credentials.
func (s PXCClusterService) GetPXCClusterCredentials(ctx context.Context, req *controllerv1beta1.GetPXCClusterCredentialsRequest) (*controllerv1beta1.GetPXCClusterCredentialsResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	cluster, err := client.GetPXCClusterCredentials(ctx, req.Name)
	if err != nil {
		if errors.Is(err, k8sclient.ErrPXCClusterStateUnexpected) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		} else if errors.Is(err, k8sclient.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &controllerv1beta1.GetPXCClusterCredentialsResponse{
		Credentials: &controllerv1beta1.PXCCredentials{
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
	_ controllerv1beta1.PXCClusterAPIServer = (*PXCClusterService)(nil)
)
