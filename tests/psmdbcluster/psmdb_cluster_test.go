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

package psmdbcluster

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/tests"
)

func TestPSMDBClusterAPI(t *testing.T) {
	kubeconfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
	}
	name := "api-psmdb-test-cluster"

	clusters, err := tests.PSMDBClusterAPIClient.ListPSMDBClusters(tests.Context, &controllerv1beta1.ListPSMDBClustersRequest{
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

	createPSMDBClusterResponse, err := tests.PSMDBClusterAPIClient.CreatePSMDBCluster(tests.Context, &controllerv1beta1.CreatePSMDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.PSMDBClusterParams{
			ClusterSize: 3,
			Replicaset: &controllerv1beta1.PSMDBClusterParams_ReplicaSet{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        1000,
					MemoryBytes: 1024 * 1024 * 1024,
				},
				DiskSize: 1024 * 1024 * 1024,
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, createPSMDBClusterResponse)

	clusters, err = tests.PSMDBClusterAPIClient.ListPSMDBClusters(tests.Context, &controllerv1beta1.ListPSMDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, int32(3), cluster.Params.ClusterSize)
			assert.Equal(t, int64(1024*1024*1024), cluster.Params.Replicaset.ComputeResources.MemoryBytes)
			assert.Equal(t, int32(1000), cluster.Params.Replicaset.ComputeResources.CpuM)
			clusterFound = true
		}
	}
	assert.True(t, clusterFound)

	t.Log("Wating for cluster to be ready")
	err = waitForPSMDBClusterState(tests.Context, kubeconfig, name, controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_READY)
	require.NoError(t, err)

	updateReq := &controllerv1beta1.UpdatePSMDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdatePSMDBClusterRequest_UpdatePSMDBClusterParams{
			ClusterSize: 3,
			Replicaset: &controllerv1beta1.UpdatePSMDBClusterRequest_UpdatePSMDBClusterParams_ReplicaSet{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        1000,
					MemoryBytes: 1024 * 1024 * 1024 * 2,
				},
			},
		},
	}

	t.Log("First update")
	upresp, err := tests.PSMDBClusterAPIClient.UpdatePSMDBCluster(tests.Context, updateReq)
	assert.NoError(t, err)
	assert.NotNil(t, upresp)

	// Second update should fail because running an update while the status is changing (there is a previous update running)
	// is not allowed.
	t.Log("Second update")
	upresp, err = tests.PSMDBClusterAPIClient.UpdatePSMDBCluster(tests.Context, updateReq)
	assert.Error(t, err)
	assert.Nil(t, upresp)

	t.Log("Wating for cluster to be ready after update")
	err = waitForPSMDBClusterState(tests.Context, kubeconfig, name, controllerv1beta1.PSMDBClusterState_PSMDB_CLUSTER_STATE_READY)
	require.NoError(t, err)

	clusterFound = false
	clusters, err = tests.PSMDBClusterAPIClient.ListPSMDBClusters(tests.Context, &controllerv1beta1.ListPSMDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, int32(3), cluster.Params.ClusterSize)
			assert.Equal(t, int64(1024*1024*1024*2), cluster.Params.Replicaset.ComputeResources.MemoryBytes)
			assert.Equal(t, int32(1000), cluster.Params.Replicaset.ComputeResources.CpuM)
			clusterFound = true
		}
	}
	assert.True(t, clusterFound)

	restartPSMDBClusterResponse, err := tests.PSMDBClusterAPIClient.RestartPSMDBCluster(tests.Context, &controllerv1beta1.RestartPSMDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, restartPSMDBClusterResponse)

	// Suspend  cluster
	suspendReq := &controllerv1beta1.UpdatePSMDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdatePSMDBClusterRequest_UpdatePSMDBClusterParams{
			Suspend: true,
		},
	}
	t.Log("Suspend cluster")
	suspendResp, err := tests.PSMDBClusterAPIClient.UpdatePSMDBCluster(tests.Context, suspendReq)
	assert.NoError(t, err)
	assert.NotNil(t, suspendResp)

	clusters, err = tests.PSMDBClusterAPIClient.ListPSMDBClusters(tests.Context, &controllerv1beta1.ListPSMDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.True(t, cluster.Params.Paused)
			break
		}
	}

	deletePSMDBClusterResponse, err := tests.PSMDBClusterAPIClient.DeletePSMDBCluster(tests.Context, &controllerv1beta1.DeletePSMDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, deletePSMDBClusterResponse)
}

func waitForPSMDBClusterState(ctx context.Context, kubeconfig string, name string, state controllerv1beta1.PSMDBClusterState) error {
	for {
		clusters, err := tests.PSMDBClusterAPIClient.ListPSMDBClusters(tests.Context, &controllerv1beta1.ListPSMDBClustersRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{
				Kubeconfig: kubeconfig,
			},
		})
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		for _, cluster := range clusters.Clusters {
			if cluster.Name == name && cluster.State == state {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for the cluster to be ready")
		case <-time.After(1000 * time.Millisecond):
			continue
		}
	}
}
