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

// Package k8sclient provides client for kubernetes.
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

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// ClusterKind is a kind of a cluster.
type ClusterKind string

const perconaXtraDBClusterKind = ClusterKind("PerconaXtraDBCluster")

// ClusterState represents XtraDB cluster CR state.
type ClusterState int32

const (
	// ClusterStateInvalid represents unknown state.
	ClusterStateInvalid ClusterState = 0
	// ClusterStateChanging represents a cluster being changed (initializing).
	ClusterStateChanging ClusterState = 1
	// ClusterStateReady represents a cluster without pending changes (ready).
	ClusterStateReady ClusterState = 2
	// ClusterStateFailed represents a failed cluster (error).
	ClusterStateFailed ClusterState = 3
	// ClusterStateDeleting represents a cluster which are in deleting state (deleting).
	ClusterStateDeleting ClusterState = 4
)

const (
	pxcBackupImage       = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0-backup"
	pxcImage             = "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0"
	pxcBackupStorageName = "test-backup-storage"
	pxcAPIVersion        = "pxc.percona.com/v1-4-0"
	pxcProxySQLImage     = "percona/percona-xtradb-cluster-operator:1.4.0-proxysql"
)

// XtraDBParams contains all parameters required to create or update Percona XtraDB cluster.
type XtraDBParams struct {
	Name string
	Size int32
}

// Cluster contains common information related to cluster.
type Cluster struct {
	Name string
}

// XtraDBCluster contains information related to xtradb cluster.
type XtraDBCluster struct {
	Name  string
	Size  int32
	State ClusterState
}

// pxcStatesMap matches pxc app states to cluster states.
var pxcStatesMap = map[pxc.AppState]ClusterState{
	pxc.AppStateUnknown: ClusterStateInvalid,
	pxc.AppStateInit:    ClusterStateChanging,
	pxc.AppStateReady:   ClusterStateReady,
	pxc.AppStateError:   ClusterStateFailed,
}

// K8Client is a client for Kubernetes.
type K8Client struct {
	kubeCtl *kubectl.KubeCtl
}

// NewK8Client returns new K8Client object.
func NewK8Client(logger logger.Logger, kubeconfig string) *K8Client {
	return &K8Client{
		kubeCtl: kubectl.NewKubeCtl(logger, kubeconfig),
	}
}

// Cleanup removes temporary files created by that object.
func (c *K8Client) Cleanup() {
	c.kubeCtl.Cleanup()
}

// ListXtraDBClusters returns list of Percona XtraDB clusters and their statuses.
func (c *K8Client) ListXtraDBClusters(ctx context.Context) ([]XtraDBCluster, error) {
	perconaXtraDBClusters, err := c.getPerconaXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}

	deletingClusters, err := c.getDeletingXtraDBClusters(ctx, perconaXtraDBClusters)
	if err != nil {
		return nil, err
	}
	res := append(perconaXtraDBClusters, deletingClusters...)

	return res, nil
}

// CreateXtraDBCluster creates Percona XtraDB cluster with provided parameters.
func (c *K8Client) CreateXtraDBCluster(ctx context.Context, params *XtraDBParams) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: pxcAPIVersion,
			Kind:       string(perconaXtraDBClusterKind),
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
				Image:   pxcProxySQLImage,
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

			Backup: &pxc.PXCScheduledBackup{
				Image: pxcBackupImage,
				Schedule: []pxc.PXCScheduledBackupSchedule{{
					Name:        "test",
					Schedule:    "*/1 * * * *",
					Keep:        3,
					StorageName: pxcBackupStorageName,
				}},
				Storages: map[string]*pxc.BackupStorageSpec{
					pxcBackupStorageName: {
						Type: pxc.BackupStorageFilesystem,
						Volume: &pxc.VolumeSpec{
							PersistentVolumeClaim: &core.PersistentVolumeClaimSpec{
								Resources: core.ResourceRequirements{
									Requests: core.ResourceList{
										core.ResourceStorage: resource.MustParse("1Gi"),
									},
								},
							},
						},
					},
				},
				ServiceAccountName: "percona-xtradb-cluster-operator",
			},
		},
	}
	return c.kubeCtl.Apply(ctx, res)
}

// UpdateXtraDBCluster changes size of provided Percona XtraDB cluster.
func (c *K8Client) UpdateXtraDBCluster(ctx context.Context, params *XtraDBParams) error {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), params.Name, &cluster)
	if err != nil {
		return err
	}

	cluster.Spec.PXC.Size = params.Size
	cluster.Spec.ProxySQL.Size = params.Size

	return c.kubeCtl.Apply(ctx, &cluster)
}

// DeleteXtraDBCluster deletes Percona XtraDB cluster with provided name.
func (c *K8Client) DeleteXtraDBCluster(ctx context.Context, name string) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       string(perconaXtraDBClusterKind),
		},
		ObjectMeta: meta.ObjectMeta{
			Name: name,
		},
	}
	return c.kubeCtl.Delete(ctx, res)
}

// getPerconaXtraDBClusters returns Percona XtraDB clusters.
func (c *K8Client) getPerconaXtraDBClusters(ctx context.Context) ([]XtraDBCluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get Percona XtraDB clusters")
	}

	res := make([]XtraDBCluster, len(list.Items))
	for i, item := range list.Items {
		var cluster pxc.PerconaXtraDBCluster
		if err := json.Unmarshal(item.Raw, &cluster); err != nil {
			return nil, err
		}
		val := XtraDBCluster{
			Name:  cluster.Name,
			Size:  cluster.Spec.ProxySQL.Size,
			State: pxcStatesMap[cluster.Status.Status],
		}
		res[i] = val
	}
	return res, nil
}

// getDeletingClusters returns clusters which are not fully deleted yet.
func (c *K8Client) getDeletingClusters(ctx context.Context, managedBy string, runningClusters map[string]struct{}) ([]Cluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, "pods", "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get kubernetes pods")
	}

	res := make([]Cluster, 0)
	for _, item := range list.Items {
		var pod core.Pod
		if err := json.Unmarshal(item.Raw, &pod); err != nil {
			return nil, err
		}

		clusterName := pod.Labels["app.kubernetes.io/instance"]
		if _, ok := runningClusters[clusterName]; ok {
			continue
		}

		if pod.Labels["app.kubernetes.io/managed-by"] != managedBy {
			continue
		}

		cluster := Cluster{
			Name: clusterName,
		}
		res = append(res, cluster)

		runningClusters[clusterName] = struct{}{}
	}
	return res, nil
}

// getDeletingXtraDBClusters returns Percona XtraDB clusters which are not fully deleted yet.
func (c *K8Client) getDeletingXtraDBClusters(ctx context.Context, clusters []XtraDBCluster) ([]XtraDBCluster, error) {
	runningClusters := make(map[string]struct{}, len(clusters))
	for _, cluster := range clusters {
		runningClusters[cluster.Name] = struct{}{}
	}

	deletingClusters, err := c.getDeletingClusters(ctx, "percona-xtradb-cluster-operator", runningClusters)
	if err != nil {
		return nil, err
	}

	xtradbClusters := make([]XtraDBCluster, len(deletingClusters))
	for i, cluster := range deletingClusters {
		xtradbClusters[i] = XtraDBCluster{
			Name:  cluster.Name,
			State: ClusterStateDeleting,
		}
	}
	return xtradbClusters, nil
}
