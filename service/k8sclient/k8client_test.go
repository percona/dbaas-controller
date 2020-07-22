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
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/logger"
)

func TestK8Client(t *testing.T) {
	l := logger.NewLogger()
	ctx := context.TODO()

	client := NewK8Client(l)

	name := fmt.Sprintf("test-cluster-%d", rand.Int31())
	deleteParams := DeleteClusterParams{
		Name: name,
	}
	_ = client.DeleteXtraDBCluster(ctx, deleteParams)

	for {
		clusters, err := client.ListClusters(ctx)
		require.NoError(t, err)

		if findCluster(clusters, name) == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	err := client.CreateXtraDBCluster(ctx, CreateXtraDBParams{
		Name: name,
		Size: 2,
	})
	require.NoError(t, err)
	for {
		clusters, err := client.ListClusters(ctx)
		require.NoError(t, err)

		if cluster := findCluster(clusters, name); cluster != nil && cluster.Status == "ready" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	err = client.UpdateXtraDBCluster(ctx, UpdateXtraDBParams{
		Name: name,
		Size: 3,
	})
	require.NoError(t, err)
	for {
		clusters, err := client.ListClusters(ctx)
		require.NoError(t, err)

		cluster := findCluster(clusters, name)
		if cluster != nil && cluster.Status == "ready" {
			assert.Equal(t, int32(3), cluster.Size)
			break
		}
		time.Sleep(1 * time.Second)
	}

	err = client.DeleteXtraDBCluster(ctx, deleteParams)
	require.NoError(t, err)
	for {
		clusters, err := client.ListClusters(ctx)
		require.NoError(t, err)

		if findCluster(clusters, name) == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	clusters, err := client.ListClusters(ctx)
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
