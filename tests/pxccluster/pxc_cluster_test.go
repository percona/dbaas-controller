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

package pxccluster

import (
	"context"
	"os"
	"testing"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/tests"
)

func TestPXCClusterAPI(t *testing.T) {
	// PERCONA_TEST_DBAAS_KUBECONFIG=$(minikube kubectl -- config view --flatten --minify --output json)
	kubeconfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
	}

	name := "pxdb-api-test-cluster"

	ctx := context.TODO()

	clusters, err := tests.PXCClusterAPIClient.ListPXCClusters(tests.Context, &controllerv1beta1.ListPXCClustersRequest{
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

	clusterSize := int32(1)
	cpuM := int32(200)
	memoryBytes := int64(1024 * 1024 * 1024)
	diskSize := int64(1024 * 1024 * 1024)

	createPXCClusterResponse, err := tests.PXCClusterAPIClient.CreatePXCCluster(tests.Context, &controllerv1beta1.CreatePXCClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.PXCClusterParams{
			ClusterSize: clusterSize,
			Pxc: &controllerv1beta1.PXCClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        cpuM,
					MemoryBytes: memoryBytes,
				},
				DiskSize: diskSize,
			},
			Proxysql: &controllerv1beta1.PXCClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        cpuM,
					MemoryBytes: memoryBytes,
				},
				DiskSize: diskSize,
			},
		},
		Pmm: tests.PMMServerParams,
	})
	require.NoError(t, err)
	require.NotNil(t, createPXCClusterResponse)

	// This gets the list of cluster immediately after after the creation.
	// At this point, k8 doesn't return a valid status (returns empty status) but our controller should
	// return CLUSTER_STATE_CHANGING. That's what we check here.
	clusters, err = tests.PXCClusterAPIClient.ListPXCClusters(tests.Context, &controllerv1beta1.ListPXCClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_CHANGING, cluster.State)
			break
		}
	}

	err = tests.WaitForClusterState(ctx, kubeconfig, name, controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_READY)
	assert.NoError(t, err)

	clusters, err = tests.PXCClusterAPIClient.ListPXCClusters(tests.Context, &controllerv1beta1.ListPXCClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, clusterSize, cluster.Params.ClusterSize)
			assert.Equal(t, memoryBytes, cluster.Params.Proxysql.ComputeResources.MemoryBytes)
			assert.Equal(t, cpuM, cluster.Params.Proxysql.ComputeResources.CpuM)
			clusterFound = true
		}
	}
	require.True(t, clusterFound)

	// There is no Ingress in minikube
	if os.Getenv("IN_EKS") != "" {
		cluster, err := tests.PXCClusterAPIClient.GetPXCClusterCredentials(tests.Context, &controllerv1beta1.GetPXCClusterCredentialsRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{
				Kubeconfig: kubeconfig,
			},
			Name: name,
		})
		assert.NoError(t, err)
		assert.NotEmpty(t, cluster.Credentials.Host)
	}

	clusterSize = 3
	memoryBytes = 512 * 1024 * 1024 * 2

	updateClusterReq := &controllerv1beta1.UpdatePXCClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdatePXCClusterRequest_UpdatePXCClusterParams{
			ClusterSize: clusterSize,
			Pxc: &controllerv1beta1.UpdatePXCClusterRequest_UpdatePXCClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					MemoryBytes: memoryBytes,
				},
			},
			Proxysql: &controllerv1beta1.UpdatePXCClusterRequest_UpdatePXCClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					MemoryBytes: memoryBytes,
				},
			},
		},
	}

	t.Log("Before first update")

	updatePXCClusterResponse, err := tests.PXCClusterAPIClient.UpdatePXCCluster(tests.Context, updateClusterReq)
	assert.NoError(t, err)
	assert.NotNil(t, updatePXCClusterResponse)

	t.Log("Waiting for cluster into changing state")
	err = tests.WaitForClusterState(ctx, kubeconfig, name, controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_CHANGING)
	assert.NoError(t, err)

	t.Log("Before second update")

	// Cannot run a second update while the first haven't finish yet
	updatePXCClusterResponse, err = tests.PXCClusterAPIClient.UpdatePXCCluster(tests.Context, updateClusterReq)
	assert.Error(t, err)
	assert.Nil(t, updatePXCClusterResponse)

	err = tests.WaitForClusterState(ctx, kubeconfig, name, controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_CHANGING)
	require.NoError(t, err)

	clusters, err = tests.PXCClusterAPIClient.ListPXCClusters(tests.Context, &controllerv1beta1.ListPXCClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, clusterSize, clusters.Clusters[0].Params.ClusterSize)
	assert.Equal(t, memoryBytes, clusters.Clusters[0].Params.Pxc.ComputeResources.MemoryBytes)
	assert.Equal(t, memoryBytes, clusters.Clusters[0].Params.Proxysql.ComputeResources.MemoryBytes)

	suspendClusterReq := &controllerv1beta1.UpdatePXCClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdatePXCClusterRequest_UpdatePXCClusterParams{
			Suspend: true,
		},
	}
	t.Log("Before suspend")

	// Cannot run a second update while the first haven't finish yet
	suspendPXCClusterResponse, err := tests.PXCClusterAPIClient.UpdatePXCCluster(tests.Context, suspendClusterReq)
	assert.Error(t, err)
	assert.Nil(t, suspendPXCClusterResponse)

	err = tests.WaitForClusterState(ctx, kubeconfig, name, controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_PAUSED)
	require.NoError(t, err)

	resumeClusterReq := &controllerv1beta1.UpdatePXCClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.UpdatePXCClusterRequest_UpdatePXCClusterParams{
			Resume: true,
		},
	}
	t.Log("Before resume")

	resumePXCClusterResponse, err := tests.PXCClusterAPIClient.UpdatePXCCluster(tests.Context, resumeClusterReq)
	assert.Error(t, err)
	assert.Nil(t, resumePXCClusterResponse)

	err = tests.WaitForClusterState(ctx, kubeconfig, name, controllerv1beta1.DBClusterState_DB_CLUSTER_STATE_READY)
	require.NoError(t, err)

	restartPXCClusterResponse, err := tests.PXCClusterAPIClient.RestartPXCCluster(tests.Context, &controllerv1beta1.RestartPXCClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, restartPXCClusterResponse)

	deletePXCClusterResponse, err := tests.PXCClusterAPIClient.DeletePXCCluster(tests.Context, &controllerv1beta1.DeletePXCClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, deletePXCClusterResponse)
}
