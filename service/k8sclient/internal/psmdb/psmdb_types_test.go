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

package psmdb

import (
	"encoding/json"
	"testing"

	"github.com/AlekSi/pointer"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
)

const expected = `
{
    "kind": "PerconaServerMongoDB",
    "apiVersion": "psmdb.percona.com/v1-4-0",
    "metadata": {
        "name": "test-psmdb"
    },
    "spec": {
        "crVersion": "1.8.0",
        "allowUnsafeConfigurations": false,
        "image": "percona/percona-server-mongodb-operator:1.4.0-mongod4.2",
        "mongod": {
            "net": {
                "port": 27017
            },
            "operationProfiling": {
                "mode": "slowOp"
            },
            "security": {
                "enableEncryption": true,
                "encryptionKeySecret": "my-cluster-name-mongodb-encryption-key",
                "encryptionCipherMode": "AES256-CBC"
            },
            "storage": {
                "engine": "wiredTiger",
                "mmapv1": {
                    "nsSize": 16
                },
                "wiredTiger": {
                    "collectionConfig": {
                        "blockCompressor": "snappy"
                    },
                    "engineConfig": {
                        "journalCompressor": "snappy"
                    },
                    "indexConfig": {
                        "prefixCompression": true
                    }
                }
            }
        },
        "replsets": [
            {
                "expose": {
                    "enabled": false,
                    "exposeType": ""
                },
                "size": 3,
                "arbiter": {
                    "enabled": false,
                    "size": 1,
                    "affinity": {
                        "antiAffinityTopologyKey": "kubernetes.io/hostname"
                    }
                },
                "resources": {
                    "limits": {
                        "memory": "800000000",
                        "cpu": "500m"
                    }
                },
                "name": "rs0",
                "volumeSpec": {
                    "persistentVolumeClaim": {
                        "resources": {
                            "requests": {
                                "storage": "1000000000"
                            }
                        }
                    }
                },
                "podDisruptionBudget": {
                    "maxUnavailable": 1
                },
                "affinity": {
                    "antiAffinityTopologyKey": "none"
                }
            }
        ],
        "secrets": {
            "users": "my-cluster-name-secrets"
        },
        "backup": {
            "enabled": true,
            "image": "percona/percona-server-mongodb-operator:1.4.0-backup",
            "serviceAccountName": "percona-server-mongodb-operator"
        },
        "pmm": {}
    }
}
`

func TestPSMDBTypesMarshal(t *testing.T) {
	t.Parallel()
	t.Run("check inline marshal", func(t *testing.T) {
		t.Parallel()
		res := &PerconaServerMongoDB{
			TypeMeta: common.TypeMeta{
				APIVersion: "psmdb.percona.com/v1-4-0",
				Kind:       "PerconaServerMongoDB",
			},
			ObjectMeta: common.ObjectMeta{
				Name: "test-psmdb",
			},
			Spec: &PerconaServerMongoDBSpec{
				CRVersion: "1.8.0",
				Image:     "percona/percona-server-mongodb-operator:1.4.0-mongod4.2",
				Secrets: &SecretsSpec{
					Users: "my-cluster-name-secrets",
				},
				Mongod: &MongodSpec{
					Net: &MongodSpecNet{
						Port: 27017,
					},
					OperationProfiling: &MongodSpecOperationProfiling{
						Mode: OperationProfilingModeSlowOp,
					},
					Security: &MongodSpecSecurity{
						RedactClientLogData:  false,
						EnableEncryption:     pointer.ToBool(true),
						EncryptionKeySecret:  "my-cluster-name-mongodb-encryption-key",
						EncryptionCipherMode: MongodChiperModeCBC,
					},
					Storage: &MongodSpecStorage{
						Engine: StorageEngineWiredTiger,
						MMAPv1: &MongodSpecMMAPv1{
							NsSize:     16,
							Smallfiles: false,
						},
						WiredTiger: &MongodSpecWiredTiger{
							CollectionConfig: &MongodSpecWiredTigerCollectionConfig{
								BlockCompressor: &WiredTigerCompressorSnappy,
							},
							EngineConfig: &MongodSpecWiredTigerEngineConfig{
								DirectoryForIndexes: false,
								JournalCompressor:   &WiredTigerCompressorSnappy,
							},
							IndexConfig: &MongodSpecWiredTigerIndexConfig{
								PrefixCompression: true,
							},
						},
					},
				},
				Replsets: []*ReplsetSpec{
					{
						Name: "rs0",
						Size: 3,
						Resources: &common.PodResources{
							Limits: &common.ResourcesList{
								CPU:    "500m",
								Memory: "800000000",
							},
						},
						PodDisruptionBudget: &common.PodDisruptionBudgetSpec{
							MaxUnavailable: pointer.ToInt(1),
						},
						Arbiter: Arbiter{
							Enabled: false,
							Size:    1,
							MultiAZ: MultiAZ{
								Affinity: &PodAffinity{
									TopologyKey: pointer.ToString("kubernetes.io/hostname"),
								},
							},
						},
						VolumeSpec: &common.VolumeSpec{
							PersistentVolumeClaim: &common.PersistentVolumeClaimSpec{
								Resources: common.ResourceRequirements{
									Requests: common.ResourceList{
										common.ResourceStorage: "1000000000",
									},
								},
							},
						},
						MultiAZ: MultiAZ{
							Affinity: &PodAffinity{
								TopologyKey: pointer.ToString(AffinityOff),
							},
						},
					},
				},

				PMM: &PmmSpec{
					Enabled: false,
				},

				Backup: &BackupSpec{
					Enabled:            true,
					Image:              "percona/percona-server-mongodb-operator:1.4.0-backup",
					ServiceAccountName: "percona-server-mongodb-operator",
				},
			},
		}

		actual, e := json.MarshalIndent(res, "", "    ")
		require.NoError(t, e)
		require.JSONEq(t, expected, string(actual))
	})
}
