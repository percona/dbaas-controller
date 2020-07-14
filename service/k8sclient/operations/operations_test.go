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

package operations

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/logger"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

func TestOperations(t *testing.T) {
	l := logger.NewLogger()
	ctx := context.TODO()

	kubeCtl := kubectl.NewKubeCtl(l)

	name := "test-cluster"
	deleteCluster := NewClusterDelete(kubeCtl, name)
	_ = deleteCluster.Start(ctx)

	list := NewClusterList(kubeCtl)
	for {
		clusters, err := list.GetClusters(ctx)
		require.NoError(t, err)

		if findCluster(clusters, name) == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	createCluster := NewClusterCreate(kubeCtl, name, 2)
	err := createCluster.Start(ctx)
	require.NoError(t, err)
	for {
		clusters, err := list.GetClusters(ctx)
		require.NoError(t, err)

		if cluster := findCluster(clusters, name); cluster != nil && cluster.Status == "ready" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	createUpdate := NewClusterUpdate(kubeCtl, name, 3)
	err = createUpdate.Start(ctx)
	require.NoError(t, err)
	for {
		clusters, err := list.GetClusters(ctx)
		require.NoError(t, err)

		cluster := findCluster(clusters, name)
		if cluster != nil && cluster.Status == "ready" {
			assert.Equal(t, int32(3), cluster.Size)
			break
		}
		time.Sleep(1 * time.Second)
	}

	err = deleteCluster.Start(ctx)
	require.NoError(t, err)
	for {
		clusters, err := list.GetClusters(ctx)
		require.NoError(t, err)

		if findCluster(clusters, name) == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	clusters, err := list.GetClusters(ctx)
	require.NoError(t, err)
	assert.Nil(t, findCluster(clusters, name))
}

func findCluster(clusters []Cluster, name string) *Cluster {
	for _, cluster := range clusters {
		if cluster.Name == name {
			return &cluster
		}
	}
	return nil
}
