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
)

func TestCreateCluster(t *testing.T) {
	l := logger.NewLogger()
	ctx := context.TODO()

	deleteCluster := NewClusterDelete(l, "test-cluster", 2)
	deleteCluster.Start(ctx) // nolint:errcheck
	deleteCluster.Wait(ctx)  // nolint:errcheck

	createCluster := NewClusterCreate(l, "test-cluster", 2) // FIXME cluster is not start when we create it second time.
	err := createCluster.Start(ctx)
	require.NoError(t, err)

	err = createCluster.Wait(ctx)
	require.NoError(t, err)

	err = deleteCluster.Start(ctx)
	require.NoError(t, err)
	err = deleteCluster.Wait(ctx)
	require.NoError(t, err)
}
