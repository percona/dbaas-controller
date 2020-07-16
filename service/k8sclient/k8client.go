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
	_ "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
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

// CreateParams contains all parameters required to create cluster.
type CreateParams struct {
	Name string
	Kind string
	Size int32
}

// UpdateParams contains all parameters required to update cluster.
type UpdateParams struct {
	Name string
	Kind string
	Size int32
}

// DeleteParams contains all parameters required to delete cluster.
type DeleteParams struct {
	Name string
	Kind string
}

// Cluster contains information related to cluster.
type Cluster struct {
	Name   string
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

// CreateCluster creates cluster with provided name and size.
func (c *K8Client) CreateCluster(ctx context.Context, params CreateParams) error {
	var res meta.Object
	switch params.Kind {
	case perconaXtradbClusterKind:
		res = c.xtradbSpecs(params)
	default:
		return errors.New("unexpected cluster kind")
	}
	return c.kubeCtl.Apply(ctx, res)
}

// xtradbSpecs returns specs for percona xtradb cluster.
func (c *K8Client) xtradbSpecs(params CreateParams) *pxc.PerconaXtraDBCluster {
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
	return res
}

// UpdateCluster changes size of provided cluster.
func (c *K8Client) UpdateCluster(ctx context.Context, params UpdateParams) error {
	switch params.Kind {
	case perconaXtradbClusterKind:
		return c.updatePerconaXtraDBCluster(ctx, params)
	default:
		return errors.New("unexpected object kind")
	}
}

// updatePerconaXtraDBCluster updates percona xtradb cluster.
func (c *K8Client) updatePerconaXtraDBCluster(ctx context.Context, params UpdateParams) error {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, perconaXtradbClusterKind, params.Name, &cluster)
	if err != nil {
		return err
	}

	cluster.Spec.PXC.Size = params.Size
	cluster.Spec.ProxySQL.Size = params.Size

	return c.kubeCtl.Apply(ctx, &cluster)
}

// DeleteCluster deletes cluster with provided name.
func (c *K8Client) DeleteCluster(ctx context.Context, params DeleteParams) error {
	var res meta.Object
	switch params.Kind {
	case perconaXtradbClusterKind:
		res = &pxc.PerconaXtraDBCluster{
			TypeMeta: meta.TypeMeta{
				APIVersion: "pxc.percona.com/v1-4-0",
				Kind:       perconaXtradbClusterKind,
			},
			ObjectMeta: meta.ObjectMeta{
				Name: params.Name,
			},
		}
	default:
		return errors.New("unexpected cluster kind")
	}
	return c.kubeCtl.Delete(ctx, res)
}

// ListClusters returns list of clusters and their statuses.
func (c *K8Client) ListClusters(ctx context.Context) ([]Cluster, error) {
	perconaXtraDBClusters, err := c.getPerconaXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}

	perconaServerMongoDBClusters, err := c.getPSMDBClusters(ctx)
	if err != nil {
		return nil, err
	}
	res := append(perconaXtraDBClusters, perconaServerMongoDBClusters...)

	deletingClusters, err := c.getDeletingClusters(ctx, perconaXtraDBClusters)
	if err != nil {
		return nil, err
	}
	res = append(res, deletingClusters...)

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
			Size:   cluster.Spec.ProxySQL.Size,
		}
		res[i] = val
	}
	return res, nil
}

// getPSMDBClusters returns Percona Server MongoDB clusters.
func (c *K8Client) getPSMDBClusters(ctx context.Context) ([]Cluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, perconaServerMongoDBKind, "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get percona server MongoDB clusters")
	}

	res := make([]Cluster, 0)
	//for i, item := range list.Items {
	//	var cluster psmdb.PerconaServerMongoDB
	//	if err := json.Unmarshal(item.Raw, &cluster); err != nil {
	//		return nil, err
	//	}
	//	val := Cluster{
	//		Name:   cluster.Name,
	//		Status: string(cluster.Status.Status),
	//		Size:   cluster.Spec.Replsets[0].Size,
	//	}
	//	res[i] = val
	//}
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

		deploymentName := pod.Labels["app.kubernetes.io/name"]
		switch deploymentName {
		case "percona-xtradb-cluster", "psmdb-cluster":
		default:
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
