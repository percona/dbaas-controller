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
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/AlekSi/pointer"
	pxc "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/percona-platform/dbaas-controller/logger"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// NewClusterCreate returns new object of ClusterCreate.
func NewClusterCreate(l logger.Logger, name string, size int32) *ClusterCreate {
	return &ClusterCreate{
		kubectl:  kubectl.NewKubeCtl(l),
		l:        l,
		name:     name,
		size:     size,
		pxcImage: "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
		kind:     "PerconaXtraDBCluster",
	}
}

// ClusterCreate creates new kubernetes cluster.
type ClusterCreate struct {
	kubectl *kubectl.KubeCtl
	l       logger.Logger

	name     string
	size     int32
	pxcImage string
	kind     string
}

// Start starts new cluster creating process.
func (c *ClusterCreate) Start(ctx context.Context) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: "pxc.percona.com/v1-4-0",
			Kind:       c.kind,
		},
		ObjectMeta: meta.ObjectMeta{
			Name: c.name,
		},
		Spec: pxc.PerconaXtraDBClusterSpec{
			AllowUnsafeConfig: true,
			SecretsName:       "my-cluster-secrets",

			PXC: &pxc.PodSpec{
				Size:  c.size,
				Image: c.pxcImage,
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
	return c.apply(ctx, res)
}

// Wait waits until cluster is ready.
func (c *ClusterCreate) Wait(ctx context.Context) error {
	_, err := c.kubectl.Wait(ctx, c.kind, c.name, "condition=Ready", time.Minute)
	if err != nil {
		c.l.Errorf(err.Error())
	}
	// TODO: fail after timeout.

	for {
		res, err := c.get(ctx, c.kind, c.name)
		if err != nil {
			c.l.Errorf("%v", err)
			continue
		}

		if res.Status.Status == pxc.AppStateReady {
			return nil
		}
		c.l.Infof("status.state != 'ready', will wait")
		time.Sleep(30 * time.Second)
	}
}

func (c *ClusterCreate) get(ctx context.Context, kind, name string) (*pxc.PerconaXtraDBCluster, error) {
	b, err := c.kubectl.Run(ctx, []string{"get", "-o=json", kind, name}, nil)
	if err != nil {
		return nil, err
	}
	var res pxc.PerconaXtraDBCluster
	c.l.Infof("%s", string(b))
	if err := json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *ClusterCreate) apply(ctx context.Context, res *pxc.PerconaXtraDBCluster) error {
	var buf bytes.Buffer
	e := json.NewEncoder(&buf)
	e.SetIndent("", "  ")
	if err := e.Encode(res); err != nil {
		log.Fatal(err)
	}
	log.Printf("apply:\n%s", buf.String())

	b, err := c.kubectl.Run(ctx, []string{"apply", "-f", "-"}, &buf)
	if err != nil {
		return err
	}
	log.Printf("%s", b)
	return nil
}
