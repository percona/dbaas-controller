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
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/percona-platform/dbaas-controller/utils/convertors"
)

//nolint:gochecknoglobals
// pxcStatesMap matches pxc app states to cluster states.
var pxcStatesMap = map[k8sclient.ClusterState]controllerv1beta1.XtraDBClusterState{
	k8sclient.ClusterStateInvalid:   controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_INVALID,
	k8sclient.ClusterStateChanging:  controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING,
	k8sclient.ClusterStateReady:     controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_READY,
	k8sclient.ClusterStateFailed:    controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_FAILED,
	k8sclient.ClusterStateDeleting:  controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_DELETING,
	k8sclient.ClusterStateUpgrading: controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_UPGRADING,
	k8sclient.ClusterStatePaused:    controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_PAUSED,
}

// XtraDBClusterService implements methods of gRPC server and other business logic related to XtraDB clusters.
type XtraDBClusterService struct {
	p *message.Printer
}

// NewXtraDBClusterService returns new XtraDBClusterService instance.
func NewXtraDBClusterService(p *message.Printer) *XtraDBClusterService {
	return &XtraDBClusterService{p: p}
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
		pxcDiskSize, err := convertors.StrToBytes(cluster.PXC.DiskSize)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		params := &controllerv1beta1.XtraDBClusterParams{
			ClusterSize: cluster.Size,
			Pxc: &controllerv1beta1.XtraDBClusterParams_PXC{
				DiskSize: int64(pxcDiskSize),
				Image:    cluster.PXC.Image,
			},
		}

		if cluster.ProxySQL != nil {
			proxySQLDiskSize, err := convertors.StrToBytes(cluster.ProxySQL.DiskSize)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			params.Proxysql = &controllerv1beta1.XtraDBClusterParams_ProxySQL{
				DiskSize: int64(proxySQLDiskSize),
			}
			params.Proxysql.ComputeResources = new(controllerv1beta1.ComputeResources)
			err = setComputeResources(cluster.ProxySQL.ComputeResources, params.Proxysql.ComputeResources)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		if cluster.HAProxy != nil {
			params.Haproxy = new(controllerv1beta1.XtraDBClusterParams_HAProxy)
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

		res.Clusters[i] = &controllerv1beta1.ListXtraDBClustersResponse_Cluster{
			Name:  cluster.Name,
			State: pxcStatesMap[cluster.State],
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
		CPUM:        convertors.MilliCPUToStr(pxcRes.CpuM),
		MemoryBytes: convertors.BytesToStr(pxcRes.MemoryBytes),
	}
}

// UpdateXtraDBCluster updates existing XtraDB cluster.
func (s *XtraDBClusterService) UpdateXtraDBCluster(ctx context.Context, req *controllerv1beta1.UpdateXtraDBClusterRequest) (*controllerv1beta1.UpdateXtraDBClusterResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	if req.Params.Suspend && req.Params.Resume {
		return nil, status.Error(codes.InvalidArgument, "resume and suspend cannot be set together")
	}

	params := &k8sclient.XtraDBParams{
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

// GetXtraDBClusterCredentials returns an XtraDB cluster connection credentials.
func (s XtraDBClusterService) GetXtraDBClusterCredentials(ctx context.Context, req *controllerv1beta1.GetXtraDBClusterCredentialsRequest) (*controllerv1beta1.GetXtraDBClusterCredentialsResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	cluster, err := client.GetXtraDBClusterCredentials(ctx, req.Name)
	if err != nil {
		if errors.Is(err, k8sclient.ErrClusterStateUnexpected) {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		} else if errors.Is(err, k8sclient.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	resp := &controllerv1beta1.GetXtraDBClusterCredentialsResponse{
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
