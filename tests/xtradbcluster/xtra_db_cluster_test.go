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

package xtradbcluster

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/tests"
)

func TestXtraDBClusterAPI(t *testing.T) {
	// PERCONA_TEST_DBAAS_KUBECONFIG=$(minikube kubectl -- config view --flatten --minify --output json)
	kubeconfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
	}

	name := "pxdb-api-test-cluster"

	ctx := context.TODO()

	clusters, err := tests.XtraDBClusterAPIClient.ListXtraDBClusters(tests.Context, &controllerv1beta1.ListXtraDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	require.NoError(t, err)

	var clusterFound bool
	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			clusterFound = true
		}
	}
	require.Falsef(t, clusterFound, "There should not be cluster with name %s", name)

	createXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.CreateXtraDBCluster(tests.Context, &controllerv1beta1.CreateXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.XtraDBClusterParams{
			ClusterSize: 2,
			Pxc: &controllerv1beta1.XtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        600,
					MemoryBytes: 1024 * 1024 * 1024,
				},
				DiskSize: 1024 * 1024 * 1024,
			},
			Proxysql: &controllerv1beta1.XtraDBClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        600,
					MemoryBytes: 1024 * 1024 * 1024,
				},
				DiskSize: 1024 * 1024 * 1024,
			},
		},
		PmmPublicAddress: tests.PMMServerAddress,
	})
	require.NoError(t, err)
	require.NotNil(t, createXtraDBClusterResponse)

	// This gets the list of cluster immediately after after the creation.
	// At this point, k8 doesn't return a valid status (returns empty status) but our controller should
	// return CLUSTER_STATE_CHANGING. That's what we check here.
	clusters, err = tests.XtraDBClusterAPIClient.ListXtraDBClusters(tests.Context, &controllerv1beta1.ListXtraDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING, cluster.State)
			break
		}
	}

	err = waitForClusterState(ctx, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_READY)
	assert.NoError(t, err)

	clusters, err = tests.XtraDBClusterAPIClient.ListXtraDBClusters(tests.Context, &controllerv1beta1.ListXtraDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, int32(2), cluster.Params.ClusterSize)
			assert.Equal(t, int64(1024*1024*1024), cluster.Params.Proxysql.ComputeResources.MemoryBytes)
			assert.Equal(t, int32(600), cluster.Params.Proxysql.ComputeResources.CpuM)
			clusterFound = true
		}
	}
	require.True(t, clusterFound)

	// There is no Ingress in minikube
	if os.Getenv("IN_EKS") != "" {
		cluster, err := tests.XtraDBClusterAPIClient.GetXtraDBCluster(tests.Context, &controllerv1beta1.GetXtraDBClusterRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{
				Kubeconfig: kubeconfig,
			},
			Name: name,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, cluster.Credentials.Host)
	}

	updateClusterReq := &controllerv1beta1.UpdateXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams{
			ClusterSize: 3,
			Pxc: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					MemoryBytes: 512 * 1024 * 1024 * 2,
				},
			},
			Proxysql: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					MemoryBytes: 512 * 1024 * 1024 * 2,
				},
			},
		},
	}

	t.Log("Before first update")

	updateXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.UpdateXtraDBCluster(tests.Context, updateClusterReq)
	assert.NoError(t, err)
	assert.NotNil(t, updateXtraDBClusterResponse)

	t.Log("Waiting for cluster into changing state")
	err = waitForClusterState(ctx, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING)
	assert.NoError(t, err)

	t.Log("Before second update")

	// Cannot run a second update while the first haven't finish yet
	updateXtraDBClusterResponse, err = tests.XtraDBClusterAPIClient.UpdateXtraDBCluster(tests.Context, updateClusterReq)
	assert.Error(t, err)
	assert.Nil(t, updateXtraDBClusterResponse)

	err = waitForClusterState(ctx, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING)
	require.NoError(t, err)

	clusters, err = tests.XtraDBClusterAPIClient.ListXtraDBClusters(tests.Context, &controllerv1beta1.ListXtraDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, int32(3), clusters.Clusters[0].Params.ClusterSize)
	assert.Equal(t, int64(512*1024*1024*2), clusters.Clusters[0].Params.Pxc.ComputeResources.MemoryBytes)
	assert.Equal(t, int64(512*1024*1024*2), clusters.Clusters[0].Params.Proxysql.ComputeResources.MemoryBytes)

	suspendClusterReq := &controllerv1beta1.UpdateXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams{
			Suspend: true,
		},
	}
	t.Log("Before suspend")

	// Cannot run a second update while the first haven't finish yet
	suspendXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.UpdateXtraDBCluster(tests.Context, suspendClusterReq)
	assert.Error(t, err)
	assert.Nil(t, suspendXtraDBClusterResponse)

	err = waitForClusterState(ctx, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_PAUSED)
	require.NoError(t, err)

	resumeClusterReq := &controllerv1beta1.UpdateXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams{
			Resume: true,
		},
	}
	t.Log("Before resume")

	resumeXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.UpdateXtraDBCluster(tests.Context, resumeClusterReq)
	assert.Error(t, err)
	assert.Nil(t, resumeXtraDBClusterResponse)

	err = waitForClusterState(ctx, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_READY)
	require.NoError(t, err)

	restartXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.RestartXtraDBCluster(tests.Context, &controllerv1beta1.RestartXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, restartXtraDBClusterResponse)

	deleteXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.DeleteXtraDBCluster(tests.Context, &controllerv1beta1.DeleteXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, deleteXtraDBClusterResponse)
}

func waitForClusterState(ctx context.Context, kubeconfig string, name string, state controllerv1beta1.XtraDBClusterState) error {
	for {
		clusters, err := tests.XtraDBClusterAPIClient.ListXtraDBClusters(tests.Context, &controllerv1beta1.ListXtraDBClustersRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{
				Kubeconfig: kubeconfig,
			},
		})
		if err != nil {
			return errors.Wrap(err, "cannot get clusters list")
		}

		if len(clusters.Clusters) > 0 && clusters.Clusters[0].State == state && clusters.Clusters[0].Name == name {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for the cluster to be ready")
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}
