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

	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/logger"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

func TestCreateCluster(t *testing.T) {
	l := logger.NewLogger()
	ctx := context.TODO()

	kubeCtl := kubectl.NewKubeCtl(l)

	deleteCluster := NewClusterDelete(kubeCtl, "test-cluster")
	_ = deleteCluster.Start(ctx)

	// TODO: wait until cluster is actually deleted and all pods are terminated.
	// List all clusters and check that there no "test-cluster"
	createCluster := NewClusterCreate(kubeCtl, "test-cluster", 2) // FIXME cluster is not start when we create it second time.
	err := createCluster.Start(ctx)
	require.NoError(t, err)
	// TODO: wait until cluster is created and ready.

	err = deleteCluster.Start(ctx)
	require.NoError(t, err)
	// TODO: wait until cluster is actually deleted.
}
