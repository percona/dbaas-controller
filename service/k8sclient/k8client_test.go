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

	client, err := NewK8Client(ctx, string(validKubeconfig))
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
	})

	l := logger.Get(ctx)

	t.Run("XtraDB", func(t *testing.T) {
		name := "test-cluster"
		_ = client.DeleteXtraDBCluster(ctx, name)

		for {
			clusters, err := client.ListXtraDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			if findXtraDBCluster(clusters, name) == nil {
				break
			}
			time.Sleep(5 * time.Second)
		}

		err = client.CreateXtraDBCluster(ctx, &XtraDBParams{
			Name: name,
			Size: 2,
		})
		require.NoError(t, err)
		for {
			clusters, err := client.ListXtraDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			if cluster := findXtraDBCluster(clusters, name); cluster != nil && cluster.State == ClusterStateReady {
				break
			}
			time.Sleep(5 * time.Second)
		}

		err = client.UpdateXtraDBCluster(ctx, &XtraDBParams{
			Name: name,
			Size: 3,
		})
		require.NoError(t, err)
		for {
			clusters, err := client.ListXtraDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			cluster := findXtraDBCluster(clusters, name)
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(3), cluster.Size)
				break
			}
			time.Sleep(5 * time.Second)
		}

		err = client.DeleteXtraDBCluster(ctx, name)
		require.NoError(t, err)
		for {
			clusters, err := client.ListXtraDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			if findXtraDBCluster(clusters, name) == nil {
				break
			}
			time.Sleep(5 * time.Second)
		}
		clusters, err := client.ListXtraDBClusters(ctx)
		require.NoError(t, err)
		assert.Nil(t, findXtraDBCluster(clusters, name))
	})

	t.Run("PSMDB", func(t *testing.T) {
		name := "test-cluster-psmdb"
		_ = client.DeletePSMDBCluster(ctx, name)

		for {
			clusters, err := client.ListPSMDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			if findPSMDBCluster(clusters, name) == nil {
				break
			}
			time.Sleep(5 * time.Second)
		}

		l.Info("No PSMDB Clusters running")

		err = client.CreatePSMDBCluster(ctx, &PSMDBParams{
			Name: name,
			Size: 3,
		})
		require.NoError(t, err)
		for {
			clusters, err := client.ListPSMDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			if cluster := findPSMDBCluster(clusters, name); cluster != nil && cluster.State == ClusterStateReady {
				break
			}
			time.Sleep(5 * time.Second)
		}

		l.Info("PSMDB Cluster is created")

		err = client.UpdatePSMDBCluster(ctx, &PSMDBParams{
			Name: name,
			Size: 5,
		})
		require.NoError(t, err)
		for {
			clusters, err := client.ListPSMDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			cluster := findPSMDBCluster(clusters, name)
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(5), cluster.Size)
				break
			}
			time.Sleep(5 * time.Second)
		}

		l.Info("PSMDB Cluster is updated")

		err = client.DeletePSMDBCluster(ctx, name)
		require.NoError(t, err)
		for {
			clusters, err := client.ListPSMDBClusters(ctx)
			require.NoError(t, err)

			l.Info(clusters)

			if findPSMDBCluster(clusters, name) == nil {
				break
			}
			time.Sleep(5 * time.Second)
		}
		clusters, err := client.ListPSMDBClusters(ctx)
		require.NoError(t, err)
		assert.Nil(t, findPSMDBCluster(clusters, name))

		l.Info("PSMDB Cluster is deleted")
	})
}

func findXtraDBCluster(clusters []XtraDBCluster, name string) *XtraDBCluster {
	for _, cluster := range clusters {
		if cluster.Name == name {
			return &cluster
		}
	}
	return nil
}

func findPSMDBCluster(clusters []PSMDBCluster, name string) *PSMDBCluster {
	for _, cluster := range clusters {
		if cluster.Name == name {
			return &cluster
		}
	}
	return nil
}
