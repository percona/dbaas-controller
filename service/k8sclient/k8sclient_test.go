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

package k8sclient

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
	"github.com/percona-platform/dbaas-controller/utils/app"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

const (
	consumedResourcesTestNamespace        string = "consumed-resources-test"
	consumedResourcesTestPodsManifestPath string = "../../deploy/test-pods.yaml"
)

// pod is struct just for testing purposes. It contains expected pod and
// container names.
type pod struct {
	name       string
	containers []string
}

func TestK8sClient(t *testing.T) {
	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	client, err := New(ctx, string(kubeconfig))
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
	})

	l := logger.Get(ctx)

	t.Run("Get non-existing clusters", func(t *testing.T) {
		t.Parallel()
		_, err := client.GetPSMDBClusterCredentials(ctx, "d0ca1166b638c-psmdb")
		assert.EqualError(t, errors.Cause(err), ErrNotFound.Error())
		_, err = client.GetXtraDBClusterCredentials(ctx, "871f766d43f8e-xtradb")
		assert.EqualError(t, errors.Cause(err), ErrNotFound.Error())
	})

	pmmPublicAddress := ""
	t.Run("XtraDB", func(t *testing.T) {
		name := "test-cluster-xtradb"
		_ = client.DeleteXtraDBCluster(ctx, name)

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster == nil
		})

		l.Info("No XtraDB Clusters running")

		err = client.CreateXtraDBCluster(ctx, &XtraDBParams{
			Name:             name,
			Size:             1,
			PXC:              &PXC{DiskSize: "1000000000"},
			ProxySQL:         &ProxySQL{DiskSize: "1000000000"},
			PMMPublicAddress: pmmPublicAddress,
		})
		require.NoError(t, err)

		l.Info("XtraDB Cluster is created")

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil
		})
		t.Run("Get credentials of cluster that is not Ready", func(t *testing.T) {
			_, err := client.GetXtraDBClusterCredentials(ctx, name)
			assert.EqualError(t, errors.Cause(err), ErrXtraDBClusterNotReady.Error())
		})

		t.Run("Create cluster with the same name", func(t *testing.T) {
			err = client.CreateXtraDBCluster(ctx, &XtraDBParams{
				Name:             name,
				Size:             1,
				PXC:              &PXC{DiskSize: "1000000000"},
				ProxySQL:         &ProxySQL{DiskSize: "1000000000"},
				PMMPublicAddress: pmmPublicAddress,
			})
			require.Error(t, err)
			assert.Equal(t, err.Error(), fmt.Sprintf(clusterWithSameNameExistsErrTemplate, name))
		})

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		t.Run("All pods are ready", func(t *testing.T) {
			cluster, err := getXtraDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, int32(2), cluster.DetailedState.CountReadyPods())
			assert.Equal(t, int32(2), cluster.DetailedState.CountAllPods())
		})

		t.Run("Get logs", func(t *testing.T) {
			pods, err := client.GetPods(ctx, "-lapp.kubernetes.io/instance="+name)
			require.NoError(t, err)

			expectedPods := []pod{
				{
					name:       name + "-proxysql-0",
					containers: []string{"pmm-client", "proxysql", "pxc-monit", "proxysql-monit"},
				},
				{
					name:       name + "-pxc-0",
					containers: []string{"pxc", "pmm-client", "pxc-init"},
				},
			}
			for _, ppod := range pods.Items {
				var foundPod pod
				assert.Conditionf(t,
					func(ppod common.Pod) assert.Comparison {
						return func() bool {
							for _, expectedPod := range expectedPods {
								if ppod.Name == expectedPod.name {
									foundPod = expectedPod
									return true
								}
							}
							return false
						}
					}(ppod),
					"pod name '%s' was not expected",
					ppod.Name,
				)

				for _, container := range ppod.Spec.Containers {
					assert.Conditionf(
						t,
						func(container common.ContainerSpec) assert.Comparison {
							return func() bool {
								for _, expectedContainerName := range foundPod.containers {
									if expectedContainerName == container.Name {
										return true
									}
								}
								return false
							}
						}(container),
						"container name '%s' was not expected",
						container.Name,
					)

					logs, err := client.GetLogs(ctx, ppod.Status.ContainerStatuses, ppod.Name, container.Name)
					require.NoError(t, err, "failed to get logs")
					assert.Greater(t, len(logs), 0)
					for _, l := range logs {
						assert.False(t, strings.Contains(l, "\n"), "new lines should have been removed")
					}
				}
			}
		})

		err = client.RestartXtraDBCluster(ctx, name)
		require.NoError(t, err)
		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateChanging
		})

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})
		l.Info("XtraDB Cluster is restarted")

		err = client.UpdateXtraDBCluster(ctx, &XtraDBParams{
			Name: name,
			Size: 3,
		})
		require.NoError(t, err)
		l.Info("XtraDB Cluster is updated")

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(3), cluster.Size)
				return true
			}
			return false
		})

		err = client.DeleteXtraDBCluster(ctx, name)
		require.NoError(t, err)

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster == nil
		})
		l.Info("XtraDB Cluster is deleted")
	})

	t.Run("PSMDB", func(t *testing.T) {
		name := "test-cluster-psmdb"
		_ = client.DeletePSMDBCluster(ctx, name)

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster == nil
		})

		l.Info("No PSMDB Clusters running")

		err = client.CreatePSMDBCluster(ctx, &PSMDBParams{
			Name:             name,
			Size:             3,
			Replicaset:       &Replicaset{DiskSize: "1000000000"},
			PMMPublicAddress: pmmPublicAddress,
		})
		require.NoError(t, err)

		l.Info("PSMDB Cluster is created")

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil
		})

		t.Run("Get credentials of cluster that is not Ready", func(t *testing.T) {
			_, err := client.GetPSMDBClusterCredentials(ctx, name)
			assert.EqualError(t, errors.Cause(err), ErrPSMDBClusterNotReady.Error())
		})

		t.Run("Create cluster with the same name", func(t *testing.T) {
			err = client.CreatePSMDBCluster(ctx, &PSMDBParams{
				Name:             name,
				Size:             1,
				Replicaset:       &Replicaset{DiskSize: "1000000000"},
				PMMPublicAddress: pmmPublicAddress,
			})
			require.Error(t, err)
			assert.Equal(t, err.Error(), fmt.Sprintf(clusterWithSameNameExistsErrTemplate, name))
		})

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		t.Run("All pods are ready", func(t *testing.T) {
			cluster, err := getPSMDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, int32(9), cluster.DetailedState.CountReadyPods())
			assert.Equal(t, int32(9), cluster.DetailedState.CountAllPods())
		})

		err = client.RestartPSMDBCluster(ctx, name)
		require.NoError(t, err)

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateChanging
		})

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})
		l.Info("PSMDB Cluster is restarted")

		err = client.UpdatePSMDBCluster(ctx, &PSMDBParams{
			Name: name,
			Size: 5,
		})
		require.NoError(t, err)
		l.Info("PSMDB Cluster is updated")

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(5), cluster.Size)
				return true
			}
			return false
		})

		err = client.DeletePSMDBCluster(ctx, name)
		require.NoError(t, err)

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster == nil
		})
		l.Info("PSMDB Cluster is deleted")
	})

	t.Run("CheckOperators", func(t *testing.T) {
		operators, err := client.CheckOperators(ctx)
		require.NoError(t, err)
		assert.Equal(t, operators.Xtradb, OperatorStatusOK)
		assert.Equal(t, operators.Psmdb, OperatorStatusOK)
	})
}

// ErrNoSuchCluster indicates that no cluster with given name was found.
var ErrNoSuchCluster error = errors.New("no cluster found with given name")

func getPSMDBCluster(ctx context.Context, client *K8sClient, name string) (*PSMDBCluster, error) {
	l := logger.Get(ctx)
	clusters, err := client.ListPSMDBClusters(ctx)
	if err != nil {
		return nil, err
	}
	l.Debug(clusters)
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, ErrNoSuchCluster
}

func getXtraDBCluster(ctx context.Context, client *K8sClient, name string) (*XtraDBCluster, error) {
	l := logger.Get(ctx)
	clusters, err := client.ListXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}
	l.Debug(clusters)
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, ErrNoSuchCluster
}

func assertListXtraDBCluster(ctx context.Context, t *testing.T, client *K8sClient, name string, conditionFunc func(cluster *XtraDBCluster) bool) {
	t.Helper()
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	for {
		time.Sleep(5 * time.Second)
		cluster, err := getXtraDBCluster(timeoutCtx, client, name)
		if !errors.Is(err, ErrNoSuchCluster) {
			require.NoError(t, err)
		}

		if conditionFunc(cluster) {
			break
		}
	}
}

func assertListPSMDBCluster(ctx context.Context, t *testing.T, client *K8sClient, name string, conditionFunc func(cluster *PSMDBCluster) bool) {
	t.Helper()
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	for {
		time.Sleep(1 * time.Second)
		cluster, err := getPSMDBCluster(timeoutCtx, client, name)
		if !errors.Is(err, ErrNoSuchCluster) {
			require.NoError(t, err)
		}

		if conditionFunc(cluster) {
			break
		}
	}
}

func TestConvertToCPUMilis(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		in             string
		expectedOut    int64
		errShouldBeNil bool
	}{
		{in: "100m", expectedOut: 100, errShouldBeNil: true},
		{in: "1", expectedOut: 1000, errShouldBeNil: true},
		{in: "1.252", expectedOut: 1252, errShouldBeNil: true},
		{in: "0.252", expectedOut: 252, errShouldBeNil: true},
		{in: "0.0", expectedOut: 0, errShouldBeNil: true},
		{in: "0.", expectedOut: 0, errShouldBeNil: false},
		{in: ".0", expectedOut: 0, errShouldBeNil: false},
		{in: ".", expectedOut: 0, errShouldBeNil: false},
		{in: "", expectedOut: 0, errShouldBeNil: false},
		{in: "adf", expectedOut: 0, errShouldBeNil: false},
	}

	for _, test := range testCases {
		out, err := convertToCPUMilis(test.in)
		assert.Equal(t, test.expectedOut, out, "in=%v, out=%v, err=%v", test.in, out, err)
		assert.Equal(
			t, test.errShouldBeNil, err == nil,
			"in=%v, out=%v, errShouldBeNil=%v: actually err == nil is %v\nerr=%v",
			test.in, out, test.errShouldBeNil, err == nil, err,
		)
	}
}

func TestConvertToBytes(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		in             string
		expectedOut    int64
		errShouldBeNil bool
	}{
		{in: "100M", expectedOut: 100 * 1000 * 1000, errShouldBeNil: true},
		{in: "100Mi", expectedOut: 100 * 1024 * 1024, errShouldBeNil: true},
		{in: "100", expectedOut: 100, errShouldBeNil: true},
		{in: "1G", expectedOut: 1000 * 1000 * 1000, errShouldBeNil: true},
		{in: "1Gi", expectedOut: 1024 * 1024 * 1024, errShouldBeNil: true},
		{in: "0.5Gi", expectedOut: 1024 * 1024 * 1024 / 2, errShouldBeNil: true},
		{in: "0.3Gi", expectedOut: 322122548, errShouldBeNil: true},
		{in: "3000m", expectedOut: 3, errShouldBeNil: true},
		{in: "Gi", expectedOut: 0, errShouldBeNil: false},
		{in: "", expectedOut: 0, errShouldBeNil: false},
		{in: "1Z", expectedOut: 0, errShouldBeNil: false},
		{in: "1Ki", expectedOut: 1024, errShouldBeNil: true},
		{in: "1K", expectedOut: 1000, errShouldBeNil: true},
		{in: "1T", expectedOut: 1000 * 1000 * 1000 * 1000, errShouldBeNil: true},
		{in: "1Ti", expectedOut: 1024 * 1024 * 1024 * 1024, errShouldBeNil: true},
	}

	for _, test := range testCases {
		out, err := convertToBytes(test.in)
		assert.Equal(t, test.expectedOut, out, "in=%v, out=%v, err=%v", test.in, out, err)
		assert.Equal(
			t, test.errShouldBeNil, err == nil,
			"in=%v, out=%v, errShouldBeNil=%v: actually err == nil is %v\nerr=%v",
			test.in, out, test.errShouldBeNil, err == nil, err,
		)
	}
}

func TestGetConsumedResources(t *testing.T) {
	t.Parallel()
	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	client, err := New(ctx, string(kubeconfig))
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
		_, _ = client.kubeCtl.Run(ctx, []string{"delete", "ns", consumedResourcesTestNamespace}, nil) // ignore err
	})

	_, _ = client.kubeCtl.Run(ctx, []string{"delete", "ns", consumedResourcesTestNamespace}, nil) // ignore err
	_, err = client.kubeCtl.Run(ctx, []string{"create", "ns", consumedResourcesTestNamespace}, nil)
	require.NoError(t, err)

	args := []string{
		"apply", "-f", consumedResourcesTestPodsManifestPath,
		"-n" + consumedResourcesTestNamespace,
	}
	_, err = client.kubeCtl.Run(ctx, args, nil)
	require.NoError(t, err)
	args = []string{
		"wait", "--for=condition=ready", "--timeout=20s",
		"pods", "hello1", "hello2", "-n" + consumedResourcesTestNamespace,
	}
	_, err = client.kubeCtl.Run(ctx, args, nil)
	require.NoError(t, err)

	cpuMilis, memoryBytes, _, err := client.GetConsumedResources(ctx, consumedResourcesTestNamespace)
	require.NoError(t, err)
	assert.Equal(t, int64(420), cpuMilis)
	assert.Equal(t, int64(288928615), memoryBytes)
}

func TestGetAllClusterResources(t *testing.T) {
	t.Parallel()
	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	client, err := New(ctx, string(kubeconfig))
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
	})

	// test getWorkerNodes
	nodes, err := client.getWorkerNodes(ctx)
	require.NoError(t, err)
	require.NotNil(t, nodes)
	assert.Greater(t, len(nodes), 0)
	for _, node := range nodes {
		cpu, ok := node.Status.Allocatable[common.ResourceCPU]
		assert.Truef(t, ok, "no value in node.Status.Allocatable under key %s", common.ResourceCPU)
		assert.NotEmpty(t, cpu)
		memory, ok := node.Status.Allocatable[common.ResourceMemory]
		assert.Truef(t, ok, "no value in node.Status.Allocatable under key %s", common.ResourceMemory)
		assert.NotEmpty(t, memory)
	}

	cpuMilis, memoryBytes, _, err := client.GetAllClusterResources(ctx)
	require.NoError(t, err)
	// We check 1 CPU because it is hard to imagine somebody runnig cluster with less CPU allocatable.
	t.Log("nodes is", len(nodes))
	assert.GreaterOrEqual(
		t, cpuMilis, int64(len(nodes)*1000),
		"expected to have at lease 1 CPU per node available to be allocated by pods",
	)

	// The same for memory, hard to imagine having less than 1 GB allocatable per node.
	assert.GreaterOrEqual(
		t, memoryBytes, int64(len(nodes))*gigaByte,
		"expected to have at lease 1GB available to be allocated by pods",
	)
}
