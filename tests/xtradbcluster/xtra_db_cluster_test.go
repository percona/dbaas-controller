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
	"testing"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/tests"
)

func TestXtraDBClusterAPI(t *testing.T) {
	kubeconfig := `{"kind": "Config", "apiVersion": "v1"}`
	name := "api-test-cluster"

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
			ClusterSize: 3,
			Pxc: &controllerv1beta1.XtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        1000,
					MemoryBytes: 1024 * 1024 * 1024,
				},
			},
			Proxysql: &controllerv1beta1.XtraDBClusterParams_ProxySQL{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        1000,
					MemoryBytes: 1024 * 1024 * 1024,
				},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, createXtraDBClusterResponse)

	clusters, err = tests.XtraDBClusterAPIClient.ListXtraDBClusters(tests.Context, &controllerv1beta1.ListXtraDBClustersRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
	})
	assert.NoError(t, err)

	for _, cluster := range clusters.Clusters {
		if cluster.Name == name {
			assert.Equal(t, int32(3), cluster.Params.ClusterSize)
			assert.Equal(t, int64(1024*1024*1024), cluster.Params.Proxysql.ComputeResources.MemoryBytes)
			assert.Equal(t, int32(1000), cluster.Params.Proxysql.ComputeResources.CpuM)
			clusterFound = true
		}
	}
	assert.True(t, clusterFound)

	deleteXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.DeleteXtraDBCluster(tests.Context, &controllerv1beta1.DeleteXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
	require.NotNil(t, deleteXtraDBClusterResponse)
}
