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

	"github.com/percona-platform/dbaas-controller/logger"
	kubectl2 "github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/operations"
)

// NewK8Client returns new K8Client object.
func NewK8Client(logger logger.Logger) *K8Client {
	return &K8Client{
		kubeCtl: kubectl2.NewKubeCtl(logger),
	}
}

// K8Client is a client for Kubernetes.
type K8Client struct {
	kubeCtl *kubectl2.KubeCtl
}

// CreateCluster creates cluster with provided name and size.
func (c *K8Client) CreateCluster(ctx context.Context, name string, size int32) error {
	clusterCreate := operations.NewClusterCreate(c.kubeCtl, name, size)
	return clusterCreate.Start(ctx)
}

// UpdateCluster changes size of provided cluster.
func (c *K8Client) UpdateCluster(ctx context.Context, name string, size int32) error {
	clusterUpdate := operations.NewClusterUpdate(c.kubeCtl, name, size)
	return clusterUpdate.Start(ctx)
}

// DeleteCluster deletes cluster with provided name.
func (c *K8Client) DeleteCluster(ctx context.Context, name string) error {
	clusterDelete := operations.NewClusterDelete(c.kubeCtl, name)
	return clusterDelete.Start(ctx)
}

// ListClusters returns list of clusters and their statuses.
func (c *K8Client) ListClusters(ctx context.Context) ([]operations.Cluster, error) {
	clusterList := operations.NewClusterList(c.kubeCtl)
	return clusterList.GetClusters(ctx)
}
