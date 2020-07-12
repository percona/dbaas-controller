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
	"encoding/json"

	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// NewClusterUpdate returns new object of ClusterUpdate.
func NewClusterUpdate(kubeCtl *kubectl.KubeCtl, name string, size int32) *ClusterUpdate {
	return &ClusterUpdate{
		kubeCtl: kubeCtl,
		name:    name,
		size:    size,
	}
}

// ClusterUpdate contains all logic related to updating cluster.
type ClusterUpdate struct {
	kubeCtl *kubectl.KubeCtl

	size int32
	name string
}

// Start updates existing cluster or returns error if cluster is not exists.
func (c *ClusterUpdate) Start(ctx context.Context) error {
	cluster, err := c.getPerconaXtraDBCluster(ctx, c.name)
	if err != nil {
		return err
	}

	cluster.Spec.PXC.Size = c.size
	cluster.Spec.ProxySQL.Size = c.size

	return c.kubeCtl.Apply(ctx, cluster)
}

func (c *ClusterUpdate) getPerconaXtraDBCluster(ctx context.Context, name string) (*pxc.PerconaXtraDBCluster, error) {
	stdout, err := c.kubeCtl.Get(ctx, clusterKind, name)
	if err != nil {
		return nil, err
	}
	var res pxc.PerconaXtraDBCluster
	if err := json.Unmarshal(stdout, &res); err != nil {
		return nil, err
	}
	return &res, nil
}
