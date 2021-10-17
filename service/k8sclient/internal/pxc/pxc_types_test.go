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

package pxc

import (
	"encoding/json"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
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
        "upgradeOptions": {
        	"versionServiceEndpoint": "https://check.percona.com"
        },
        "allowUnsafeConfigurations": true
    }
}
`

func TestPXCTypesMarshal(t *testing.T) {
	t.Parallel()
	t.Run("check inline marshal", func(t *testing.T) {
		t.Parallel()
		var size int32 = 3
		res := &PerconaXtraDBCluster{
			TypeMeta: common.TypeMeta{
				APIVersion: "percona/percona-xtradb-cluster-operator:1.4.0-pxc8.0",
				Kind:       "PerconaXtraDBCluster",
			},
			ObjectMeta: common.ObjectMeta{
				Name: "test-pxc",
			},
			Spec: &PerconaXtraDBClusterSpec{
				AllowUnsafeConfig: true,
				SecretsName:       "my-cluster-secrets",
				UpgradeOptions: &common.UpgradeOptions{
					VersionServiceEndpoint: "https://check.percona.com",
				},
				PXC: &PodSpec{
					Size: &size,
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
					Size: &size,
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
		require.NoError(t, e)
		require.JSONEq(t, expected, string(actual))
	})
}
