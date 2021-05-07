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

package logs

import (
	"os"
	"testing"
	"time"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/tests"
)

const (
	clusterSize = int32(1)
	cpuM        = int32(200)
	memoryBytes = int64(1024 * 1024 * 1024)
	diskSize    = int64(1024 * 1024 * 1024)
	name        = "get-logs-cluster"
)

func TestGetLogs(t *testing.T) {
	// PERCONA_TEST_DBAAS_KUBECONFIG=$(minikube kubectl -- config view --flatten --minify --output json)
	kubeconfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
	}

	clusterSize := int32(1)
	cpuM := int32(200)
	memoryBytes := int64(1024 * 1024 * 1024)
	diskSize := int64(1024 * 1024 * 1024)

	createXtraDBClusterResponse, err := tests.XtraDBClusterAPIClient.CreateXtraDBCluster(tests.Context, &controllerv1beta1.CreateXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
		Params: &controllerv1beta1.XtraDBClusterParams{
			ClusterSize: clusterSize,
			Pxc: &controllerv1beta1.XtraDBClusterParams_PXC{
				ComputeResources: &controllerv1beta1.ComputeResources{
					CpuM:        cpuM,
					MemoryBytes: memoryBytes,
				},
				DiskSize: diskSize,
			},
			Proxysql: &controllerv1beta1.XtraDBClusterParams_ProxySQL{
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
	require.NotNil(t, createXtraDBClusterResponse)

	request := &controllerv1beta1.GetLogsRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		ClusterName: name,
	}

	t.Run("Get logs of initializing cluster", func(t *testing.T) {
		t.Parallel()
		err = tests.WaitForClusterState(tests.Context, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_CHANGING)
		require.NoError(t, err)
		// Wait 5 seconds for pods to be scheduled.
		time.Sleep(time.Second * 5)
		response, err := tests.LogsAPIClient.GetLogs(tests.Context, request)
		require.NoError(t, err)
		assert.Equal(t, 0, len(response.Logs))
	})

	err = tests.WaitForClusterState(tests.Context, kubeconfig, name, controllerv1beta1.XtraDBClusterState_XTRA_DB_CLUSTER_STATE_READY)
	require.NoError(t, err)

	response, err := tests.LogsAPIClient.GetLogs(tests.Context, request)
	require.NoError(t, err)

	assert.Equalf(t, 7, len(response.Logs), "got %v", response.Logs)
	sum := func(logsEntries []*controllerv1beta1.Logs) int {
		sum := 0
		for _, entry := range logsEntries {
			sum += len(entry.Logs)
		}
		return sum
	}
	assert.LessOrEqual(t, sum(response.Logs), 1000)

	_, err = tests.XtraDBClusterAPIClient.DeleteXtraDBCluster(tests.Context, &controllerv1beta1.DeleteXtraDBClusterRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: kubeconfig,
		},
		Name: name,
	})
	require.NoError(t, err)
}
