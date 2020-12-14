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
	"github.com/percona-platform/dbaas-controller/utils/app"
)

// func TestGetXtraDBClusterAPI(t *testing.T) {
// 	// PERCONA_TEST_DBAAS_KUBECONFIG=$(minikube kubectl -- config view --flatten --minify --output json)
// 	kubeconfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
// 	if kubeconfig == "" {
// 		t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
// 	}
//
// 	if os.Getenv("IN_EKS") == "" {
// 		t.Skip("This tests needs to run in an EKS cluster")
// 	}
//
// 	name := "pxdb-api-test-cluster"
//
// 	cluster, err := tests.XtraDBClusterAPIClient.GetXtraDBCluster(tests.Context, &controllerv1beta1.GetXtraDBClusterRequest{
// 		KubeAuth: &controllerv1beta1.KubeAuth{
// 			Kubeconfig: kubeconfig,
// 		},
// 		Name: name,
// 	})
// 	assert.NoError(t, err)
// 	assert.NotEmpty(t, cluster.Credentials)
// }

func TestXtraDBClusterAPI(t *testing.T) {
	// PERCONA_TEST_DBAAS_KUBECONFIG=$(minikube kubectl -- config view --flatten --minify --output json)
	kubeconfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
	}

	name := "pxdb-api-test-cluster"
	ctx := app.Context()

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
			ClusterSize: 1,
			Pxc: &controllerv1beta1.XtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        1000,
					MemoryBytes: 1024 * 1024 * 1024,
				},
				DiskSize: 1024 * 1024 * 1024,
			},
			Proxysql: &controllerv1beta1.XtraDBClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        1000,
					MemoryBytes: 1024 * 1024 * 1024,
				},
				DiskSize: 1024 * 1024 * 1024,
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, createXtraDBClusterResponse)

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
			assert.Equal(t, int32(1), cluster.Params.ClusterSize)
			assert.Equal(t, int64(1024*1024*1024), cluster.Params.Proxysql.ComputeResources.MemoryBytes)
			assert.Equal(t, int32(1000), cluster.Params.Proxysql.ComputeResources.CpuM)
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
			ClusterSize: 2,
			Pxc: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					MemoryBytes: 1024 * 1024 * 1024 * 2,
				},
			},
			Proxysql: &controllerv1beta1.UpdateXtraDBClusterRequest_UpdateXtraDBClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					MemoryBytes: 1024 * 1024 * 1024 * 2,
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
	assert.Equal(t, int32(2), clusters.Clusters[0].Params.ClusterSize)
	assert.Equal(t, int64(1024*1024*1024*2), clusters.Clusters[0].Params.Pxc.ComputeResources.MemoryBytes)
	assert.Equal(t, int64(1024*1024*1024*2), clusters.Clusters[0].Params.Proxysql.ComputeResources.MemoryBytes)

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
