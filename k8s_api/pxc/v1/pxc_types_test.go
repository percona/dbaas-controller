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

// Package v1 contains tests for specification to works with Percona XtraDB Cluster Operator
package v1

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/require"

	metav1 "github.com/percona-platform/dbaas-controller/k8s_api/apimachinery/pkg/apis/meta/v1"
	"github.com/percona-platform/dbaas-controller/k8s_api/common"
)

const expected = `
{
	"kind": "PerconaXtraDBCluster",
    "apiVersion": "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
    "metadata": {
        "name": "test-pxc"
    },
    "spec": {
        "secretsName": "my-cluster-secrets",
        "pxc": {
            "size": 3,
            "image": "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
            "resources": {
                "limits": {
                    "memory": "600000000",
                    "cpu": "500m"
                }
            },
            "volumeSpec": {
                "persistentVolumeClaim": {
                    "resources": {
                        "requests": {
                            "storage": "1000000000"
                        }
                    }
                }
            },
            "affinity": {
                "antiAffinityTopologyKey": "none"
            }
        },
        "proxysql": {
            "size": 3,
            "image": "percona/percona-xtradb-cluster-operator:1.4.0-proxysql",
            "resources": {
                "limits": {
                    "memory": "600000000",
                    "cpu": "500m"
                }
            },
            "volumeSpec": {
                "persistentVolumeClaim": {
                    "resources": {
                        "requests": {
                            "storage": "1000000000"
                        }
                    }
                }
            },
            "affinity": {
                "antiAffinityTopologyKey": "none"
            }
        },
        "pmm": {},
        "backup": {
            "image": "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0-backup",
            "schedule": [
                {
                    "name": "test",
                    "schedule": "*/1 * * * *",
                    "keep": 3,
                    "storageName": "test-backup-storage"
                }
            ],
            "storages": {
                "test-backup-storage": {
                    "type": "filesystem",
                    "s3": {
                        "bucket": "",
                        "credentialsSecret": ""
                    },
                    "volume": {
                        "persistentVolumeClaim": {
                            "resources": {
                                "requests": {
                                    "storage": "1000000000"
                                }
                            }
                        }
                    }
                }
            },
            "serviceAccountName": "percona-xtradb-cluster-operator"
        },
        "upgradeOptions": {},
        "allowUnsafeConfigurations": true
    },
    "status": {
        "pxc": {},
        "proxysql": {},
        "haproxy": {},
        "backup": {},
        "pmm": {}
    }
}
`

func TestPSMDBTypesMarshal(t *testing.T) {
	t.Run("check inline marshal", func(t *testing.T) {
		res := &PerconaXtraDBCluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
				Kind:       "PerconaXtraDBCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pxc",
			},
			Spec: PerconaXtraDBClusterSpec{
				AllowUnsafeConfig: true,
				SecretsName:       "my-cluster-secrets",
				PXC: &PodSpec{
					Size: 3,
					Resources: &common.PodResources{
						Limits: &common.ResourcesList{
							CPU:    "500m",
							Memory: "600000000",
						},
					},
					Image: "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
					VolumeSpec: &common.VolumeSpec{
						PersistentVolumeClaim: &common.PersistentVolumeClaimSpec{
							Resources: common.ResourceRequirements{
								Requests: common.ResourceList{
									common.ResourceStorage: "1000000000",
								},
							},
						},
					},
					Affinity: &PodAffinity{
						TopologyKey: pointer.ToString(AffinityTopologyKeyOff),
					},
				},
				ProxySQL: &PodSpec{
					Size: 3,
					Resources: &common.PodResources{
						Limits: &common.ResourcesList{
							CPU:    "500m",
							Memory: "600000000",
						},
					},
					Image: "percona/percona-xtradb-cluster-operator:1.4.0-proxysql",
					VolumeSpec: &common.VolumeSpec{
						PersistentVolumeClaim: &common.PersistentVolumeClaimSpec{
							Resources: common.ResourceRequirements{
								Requests: common.ResourceList{
									common.ResourceStorage: "1000000000",
								},
							},
						},
					},
					Affinity: &PodAffinity{
						TopologyKey: pointer.ToString(AffinityTopologyKeyOff),
					},
				},
				PMM: &PMMSpec{
					Enabled: false,
				},
				Backup: &PXCScheduledBackup{
					Image: "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0-backup",
					Schedule: []PXCScheduledBackupSchedule{{
						Name:        "test",
						Schedule:    "*/1 * * * *",
						Keep:        3,
						StorageName: "test-backup-storage",
					}},
					Storages: map[string]*BackupStorageSpec{
						"test-backup-storage": {
							Type: "filesystem",
							Volume: &common.VolumeSpec{
								PersistentVolumeClaim: &common.PersistentVolumeClaimSpec{
									Resources: common.ResourceRequirements{
										Requests: common.ResourceList{
											common.ResourceStorage: "1000000000",
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

		actual, e := json.MarshalIndent(res, "", "    ")
		fmt.Printf("%s", actual)
		require.NoError(t, e)
		require.JSONEq(t, expected, string(actual))
	})
}
