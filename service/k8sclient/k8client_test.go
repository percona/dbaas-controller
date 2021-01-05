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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
	"github.com/percona-platform/dbaas-controller/utils/app"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

func TestK8Client(t *testing.T) {
	ctx := app.Context()

	kubeCtl, err := kubectl.NewKubeCtl(ctx, "")
	require.NoError(t, err)

	validKubeconfig, err := kubeCtl.Run(ctx, []string{"config", "view", "-o", "json"}, nil)
	require.NoError(t, err)

	client, err := New(ctx, string(validKubeconfig))
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
	})

	l := logger.Get(ctx)

	pmmPublicAddress := ""
	t.Run("XtraDB", func(t *testing.T) {
		name := "test-cluster-xtradb"
		_ = client.DeleteXtraDBCluster(ctx, name)

		assertListXtraDBCluster(t, ctx, client, name, func(cluster *XtraDBCluster) bool {
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

		assertListXtraDBCluster(t, ctx, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		err = client.RestartXtraDBCluster(ctx, name)
		require.NoError(t, err)
		assertListXtraDBCluster(t, ctx, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateChanging
		})

		assertListXtraDBCluster(t, ctx, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})
		l.Info("XtraDB Cluster is restarted")

		err = client.UpdateXtraDBCluster(ctx, &XtraDBParams{
			Name: name,
			Size: 3,
		})
		require.NoError(t, err)
		l.Info("XtraDB Cluster is updated")

		assertListXtraDBCluster(t, ctx, client, name, func(cluster *XtraDBCluster) bool {
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(3), cluster.Size)
				return true
			}
			return false
		})

		err = client.DeleteXtraDBCluster(ctx, name)
		require.NoError(t, err)

		assertListXtraDBCluster(t, ctx, client, name, func(cluster *XtraDBCluster) bool {
			return cluster == nil
		})
		l.Info("XtraDB Cluster is deleted")
	})

	t.Run("PSMDB", func(t *testing.T) {
		name := "test-cluster-psmdb"
		_ = client.DeletePSMDBCluster(ctx, name)

		assertListPSMDBCluster(t, ctx, client, name, func(cluster *PSMDBCluster) bool {
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

		assertListPSMDBCluster(t, ctx, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		err = client.RestartPSMDBCluster(ctx, name)
		require.NoError(t, err)

		assertListPSMDBCluster(t, ctx, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateChanging
		})

		assertListPSMDBCluster(t, ctx, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})
		l.Info("PSMDB Cluster is restarted")

		err = client.UpdatePSMDBCluster(ctx, &PSMDBParams{
			Name: name,
			Size: 5,
		})
		require.NoError(t, err)
		l.Info("PSMDB Cluster is updated")

		assertListPSMDBCluster(t, ctx, client, name, func(cluster *PSMDBCluster) bool {
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(5), cluster.Size)
				return true
			}
			return false
		})

		err = client.DeletePSMDBCluster(ctx, name)
		require.NoError(t, err)

		assertListPSMDBCluster(t, ctx, client, name, func(cluster *PSMDBCluster) bool {
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

func assertListXtraDBCluster(t *testing.T, ctx context.Context, client *K8Client, name string, conditionFunc func(cluster *XtraDBCluster) bool) {
	l := logger.Get(ctx)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	for {
		time.Sleep(5 * time.Second)
		clusters, err := client.ListXtraDBClusters(ctx)
		require.NoError(t, err)

		l.Debug(clusters)

		var cluster *XtraDBCluster
		for _, c := range clusters {
			c := c
			if c.Name == name {
				cluster = &c
				break
			}
		}
		if conditionFunc(cluster) {
			break
		}
	}
}

func assertListPSMDBCluster(t *testing.T, ctx context.Context, client *K8Client, name string, conditionFunc func(cluster *PSMDBCluster) bool) {
	l := logger.Get(ctx)
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	for {
		time.Sleep(1 * time.Second)
		clusters, err := client.ListPSMDBClusters(timeoutCtx)
		require.NoErrorf(t, err, "timeout for waiting condition: %v", clusters)

		l.Debug(clusters)

		var cluster *PSMDBCluster
		for _, c := range clusters {
			c := c
			if c.Name == name {
				cluster = &c
				break
			}
		}
		if conditionFunc(cluster) {
			break
		}
	}
}
