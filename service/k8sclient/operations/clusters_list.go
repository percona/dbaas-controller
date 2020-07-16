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
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// Cluster contains information related to cluster.
type Cluster struct {
	Name   string
	Size   int32
	Status string
}

// NewClusterList returns new object of ClusterList.
func NewClusterList(kubeCtl *kubectl.KubeCtl) *ClusterList {
	return &ClusterList{
		kubeCtl: kubeCtl,
	}
}

// ClusterList contains all logic related to getting cluster list.
type ClusterList struct {
	kubeCtl *kubectl.KubeCtl
}

// GetClusters returns clusters list.
func (c *ClusterList) GetClusters(ctx context.Context) ([]Cluster, error) {
	perconaXtraDBClusters, err := c.getPerconaXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}

	deletingClusters, err := c.getDeletingClusters(ctx, perconaXtraDBClusters)
	if err != nil {
		return nil, err
	}

	res := append(perconaXtraDBClusters, deletingClusters...)

	return res, nil
}

// getPerconaXtraDBClusters returns percona xtradb clusters.
func (c *ClusterList) getPerconaXtraDBClusters(ctx context.Context) ([]Cluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, clusterKind, "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get percona xtradb clusters")
	}

	res := make([]Cluster, len(list.Items))
	for i, item := range list.Items {
		var cluster pxc.PerconaXtraDBCluster
		if err := json.Unmarshal(item.Raw, &cluster); err != nil {
			return nil, err
		}
		val := Cluster{
			Name:   cluster.Name,
			Status: string(cluster.Status.Status),
			Size:   cluster.Spec.ProxySQL.Size,
		}
		res[i] = val
	}
	return res, nil
}

// getDeletingClusters returns percona xtradb clusters which are not fully deleted yet.
func (c *ClusterList) getDeletingClusters(ctx context.Context, runningClusters []Cluster) ([]Cluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, "pods", "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get kuberneters pods")
	}
	exists := make(map[string]struct{}, len(runningClusters))
	for _, cluster := range runningClusters {
		exists[cluster.Name] = struct{}{}
	}

	var res []Cluster
	for _, item := range list.Items {
		var pod v1.Pod
		if err := json.Unmarshal(item.Raw, &pod); err != nil {
			return nil, err
		}
		clusterName := pod.Labels["app.kubernetes.io/instance"]
		deploymentName := pod.Labels["app.kubernetes.io/name"]
		if _, ok := exists[clusterName]; ok || deploymentName != "percona-xtradb-cluster" {
			continue
		}
		cluster := Cluster{
			Status: "deleting",
			Name:   clusterName,
		}
		res = append(res, cluster)

		exists[clusterName] = struct{}{}
	}
	return res, nil
}
