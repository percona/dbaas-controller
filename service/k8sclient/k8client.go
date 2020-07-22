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
	"encoding/json"

	"github.com/AlekSi/pointer"
	_ "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1" // It'll be implemented later.
	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/logger"
	kubectl2 "github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

const backupImage = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0-backup"
const pxcImage = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0"
const backupStorageName = "test-backup-storage"

const perconaXtradbClusterKind = "PerconaXtraDBCluster"
const perconaServerMongoDBKind = "PerconaServerMongoDB"

// CreateXtraDBParams contains all parameters required to create percona xtradb cluster.
type CreateXtraDBParams struct {
	Name string
	Size int32
}

// UpdateXtraDBParams contains all parameters required to update percona xtradb cluster.
type UpdateXtraDBParams struct {
	Name string
	Size int32
}

// DeleteParams contains all parameters required to delete cluster.
type DeleteParams struct {
	Name string
}

// Cluster contains information related to cluster.
type Cluster struct {
	Name   string
	Kind   string
	Size   int32
	Status string
}

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

// CreateXtraDBCluster creates percona xtradb cluster with provided parameters.
func (c *K8Client) CreateXtraDBCluster(ctx context.Context, params CreateXtraDBParams) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       perconaXtradbClusterKind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: params.Name,
		},
		Spec: pxc.PerconaXtraDBClusterSpec{
			AllowUnsafeConfig: true,
			SecretsName:       "my-cluster-secrets",

			PXC: &pxc.PodSpec{
				Size:  params.Size,
				Image: pxcImage,
				VolumeSpec: &pxc.VolumeSpec{
					PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
			},

			ProxySQL: &pxc.PodSpec{
				Enabled: true,
				Size:    params.Size,
				Image:   "percona/percona-xtradb-cluster-operator:1.4.0-proxysql",
				VolumeSpec: &pxc.VolumeSpec{
					PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
			},

			PMM: &pxc.PMMSpec{
				Enabled: false,
			},

			//Backup: &pxc.PXCScheduledBackup{
			//	Image: backupImage,
			//	Schedule: []pxc.PXCScheduledBackupSchedule{{
			//		Name:        "test",
			//		Schedule:    "*/1 * * * *",
			//		Keep:        3,
			//		StorageName: backupStorageName,
			//	}},
			//	Storages: map[string]*pxc.BackupStorageSpec{
			//		backupStorageName: {
			//			Type: pxc.BackupStorageFilesystem,
			//			Volume: &pxc.VolumeSpec{
			//				PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
			//					Resources: core.ResourceRequirements{
			//						Requests: core.ResourceList{
			//							core.ResourceStorage: resource.MustParse("1Gi"),
			//						},
			//					},
			//				},
			//			},
			//		},
			//	},
			//	ServiceAccountName: "percona-xtradb-cluster-operator",
			//},
		},
	}
	return c.kubeCtl.Apply(ctx, res)
}

// UpdateXtraDBCluster changes size of provided percona xtradb cluster.
func (c *K8Client) UpdateXtraDBCluster(ctx context.Context, params UpdateXtraDBParams) error {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, perconaXtradbClusterKind, params.Name, &cluster)
	if err != nil {
		return err
	}

	cluster.Spec.PXC.Size = params.Size
	cluster.Spec.ProxySQL.Size = params.Size

	return c.kubeCtl.Apply(ctx, &cluster)
}

// DeleteXtraDBCluster deletes percona xtradb cluster with provided name.
func (c *K8Client) DeleteXtraDBCluster(ctx context.Context, params DeleteParams) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       perconaXtradbClusterKind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: params.Name,
		},
	}
	return c.kubeCtl.Delete(ctx, res)
}

// ListClusters returns list of clusters and their statuses.
func (c *K8Client) ListClusters(ctx context.Context) ([]Cluster, error) {
	perconaXtraDBClusters, err := c.getPerconaXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: get PSMDB clusters.

	deletingClusters, err := c.getDeletingClusters(ctx, perconaXtraDBClusters)
	if err != nil {
		return nil, err
	}
	res := append(perconaXtraDBClusters, deletingClusters...)

	return res, nil
}

// getPerconaXtraDBClusters returns percona xtradb clusters.
func (c *K8Client) getPerconaXtraDBClusters(ctx context.Context) ([]Cluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, perconaXtradbClusterKind, "", &list)
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
			Kind:   perconaXtradbClusterKind,
			Size:   cluster.Spec.ProxySQL.Size,
		}
		res[i] = val
	}
	return res, nil
}

// getDeletingClusters returns percona xtradb clusters which are not fully deleted yet.
func (c *K8Client) getDeletingClusters(ctx context.Context, runningClusters []Cluster) ([]Cluster, error) {
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
		var pod core.Pod
		if err := json.Unmarshal(item.Raw, &pod); err != nil {
			return nil, err
		}

		clusterName := pod.Labels["app.kubernetes.io/instance"]
		if _, ok := exists[clusterName]; ok {
			continue
		}

		var kind string
		deploymentName := pod.Labels["app.kubernetes.io/name"]
		switch deploymentName {
		case "percona-xtradb-cluster":
			kind = perconaXtradbClusterKind
		case "psmdb-cluster":
			kind = perconaServerMongoDBKind
		default:
			continue
		}

		cluster := Cluster{
			Status: "deleting",
			Kind:   kind,
			Name:   clusterName,
		}
		res = append(res, cluster)

		exists[clusterName] = struct{}{}
	}
	return res, nil
}
