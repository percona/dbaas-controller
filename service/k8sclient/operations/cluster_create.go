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

	"github.com/AlekSi/pointer"
	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// NewClusterCreate returns new object of ClusterCreate.
func NewClusterCreate(kubeCtl *kubectl.KubeCtl, name string, size int32) *ClusterCreate {
	return &ClusterCreate{
		kubeCtl: kubeCtl,
		name:    name,
		size:    size,
	}
}

// ClusterCreate creates new kubernetes cluster.
type ClusterCreate struct {
	kubeCtl *kubectl.KubeCtl

	name string
	size int32
}

// Start starts new cluster creating process.
func (c *ClusterCreate) Start(ctx context.Context) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       clusterKind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: c.name,
		},
		Spec: pxc.PerconaXtraDBClusterSpec{
			AllowUnsafeConfig: true,
			SecretsName:       "my-cluster-secrets",

			PXC: &pxc.PodSpec{
				Size:  c.size,
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
				Size:    c.size,
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
