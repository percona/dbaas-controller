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

	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// NewClusterDelete returns new object of ClusterDelete.
func NewClusterDelete(kubeCtl *kubectl.KubeCtl, name string) *ClusterDelete {
	return &ClusterDelete{
		kubeCtl: kubeCtl,
		name:    name,
		kind:    "PerconaXtraDBCluster",
	}
}

// ClusterDelete deletes kubernetes cluster.
type ClusterDelete struct {
	kubeCtl *kubectl.KubeCtl

	name string
	kind string
}

// Start starts cluster deleting process.
func (c *ClusterDelete) Start(ctx context.Context) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       c.kind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: c.name,
		},
	}
	return c.kubeCtl.Delete(ctx, res)
}
