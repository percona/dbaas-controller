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
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kubectl"
	"github.com/percona-platform/dbaas-controller/utils/app"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

func TestK8sClient(t *testing.T) {
	ctx := app.Context()

	kubeCtl, err := kubectl.NewKubeCtl(ctx, "")
	require.NoError(t, err)

	validKubeconfig, err := kubeCtl.Run(ctx, []string{"config", "view", "-o", "json"}, nil)
	require.NoError(t, err)
	{
		all, err := kubeCtl.Run(ctx, []string{"version", "-o", "json"}, nil)
		require.NoError(t, err)
		logger.Get(ctx).Info("Running version using only kubectl pkg:", string(all))
	}
	client, err := New(ctx, string(validKubeconfig))
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
