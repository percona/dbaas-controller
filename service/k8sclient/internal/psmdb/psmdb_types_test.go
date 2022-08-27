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
		"pause": false,
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

	t.Run("check marshal", func(t *testing.T) {
		input := `{
    "apiVersion": "v1",
    "items": [
        {
            "apiVersion": "psmdb.percona.com/v1",
            "kind": "PerconaServerMongoDB",
            "metadata": {
                "annotations": {
                    "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"psmdb.percona.com/v1-12-0\",\"kind\":\"PerconaServerMongoDB\",\"metadata\":{\"annotations\":{},\"finalizers\":[\"delete-psmdb-pvc\"],\"name\":\"mongodb-xcso1v\",\"namespace\":\"default\"},\"spec\":{\"backup\":{\"enabled\":true,\"image\":\"percona/percona-server-mongodb-operator:1.12.0-backup\",\"serviceAccountName\":\"percona-server-mongodb-operator\"},\"crVersion\":\"1.12.0\",\"image\":\"percona/percona-server-mongodb:5.0.7-6\",\"pause\":false,\"pmm\":{\"enabled\":true,\"image\":\"perconalab/pmm-client:dev-latest\",\"resources\":{\"requests\":{\"cpu\":\"500m\",\"memory\":\"300M\"}},\"serverHost\":\"localhost\"},\"replsets\":[{\"arbiter\":{\"affinity\":{\"antiAffinityTopologyKey\":\"kubernetes.io/hostname\"},\"enabled\":false,\"size\":1},\"configuration\":\"      operationProfiling:\\n        mode: slowOp\\n\",\"expose\":{\"enabled\":false,\"exposeType\":\"\"},\"name\":\"rs0\",\"podDisruptionBudget\":{\"maxUnavailable\":1},\"resources\":{\"limits\":{\"cpu\":\"1000m\",\"memory\":\"2000000000\"}},\"size\":3,\"volumeSpec\":{\"persistentVolumeClaim\":{\"resources\":{\"requests\":{\"storage\":\"25000000000\"}}}}}],\"secrets\":{\"users\":\"dbaas-mongodb-xcso1v-psmdb-secrets\"},\"sharding\":{\"configsvrReplSet\":{\"affinity\":{\"antiAffinityTopologyKey\":\"kubernetes.io/hostname\"},\"arbiter\":{\"affinity\":{\"antiAffinityTopologyKey\":\"kubernetes.io/hostname\"},\"enabled\":false,\"size\":1},\"expose\":{\"enabled\":false,\"exposeType\":\"\"},\"size\":3,\"volumeSpec\":{\"persistentVolumeClaim\":{\"resources\":{\"requests\":{\"storage\":\"25000000000\"}}}}},\"enabled\":true,\"expose\":null,\"mongos\":{\"affinity\":{\"antiAffinityTopologyKey\":\"kubernetes.io/hostname\"},\"expose\":{\"exposeType\":\"LoadBalancer\"},\"resources\":{\"limits\":{\"cpu\":\"1000m\",\"memory\":\"2000000000\"}},\"size\":3},\"operationProfiling\":null},\"updateStrategy\":\"RollingUpdate\"}}\n"
                },
                "creationTimestamp": "2022-08-27T15:16:37Z",
                "finalizers": [
                    "delete-psmdb-pvc"
                ],
                "generation": 1,
                "name": "mongodb-xcso1v",
                "namespace": "default",
                "resourceVersion": "29905208",
                "uid": "b0ee8194-085b-4f3e-bda2-c6e5a303f2da"
            },
            "spec": {
                "backup": {
                    "enabled": true,
                    "image": "percona/percona-server-mongodb-operator:1.12.0-backup",
                    "serviceAccountName": "percona-server-mongodb-operator"
                },
                "crVersion": "1.12.0",
                "image": "percona/percona-server-mongodb:5.0.7-6",
                "pause": false,
                "pmm": {
                    "enabled": true,
                    "image": "perconalab/pmm-client:dev-latest",
                    "resources": {
                        "requests": {
                            "cpu": "500m",
                            "memory": "300M"
                        }
                    },
                    "serverHost": "localhost"
                },
                "replsets": [
                    {
                        "arbiter": {
                            "affinity": {
                                "antiAffinityTopologyKey": "kubernetes.io/hostname"
                            },
                            "enabled": false,
                            "size": 1
                        },
                        "configuration": "      operationProfiling:\n        mode: slowOp\n",
                        "expose": {
                            "enabled": false,
                            "exposeType": ""
                        },
                        "name": "rs0",
                        "podDisruptionBudget": {
                            "maxUnavailable": 1
                        },
                        "resources": {
                            "limits": {
                                "cpu": "1000m",
                                "memory": "2000000000"
                            }
                        },
                        "size": 3,
                        "volumeSpec": {
                            "persistentVolumeClaim": {
                                "resources": {
                                    "requests": {
                                        "storage": "25000000000"
                                    }
                                }
                            }
                        }
                    }
                ],
                "secrets": {
                    "users": "dbaas-mongodb-xcso1v-psmdb-secrets"
                },
                "sharding": {
                    "configsvrReplSet": {
                        "affinity": {
                            "antiAffinityTopologyKey": "kubernetes.io/hostname"
                        },
                        "arbiter": {
                            "affinity": {
                                "antiAffinityTopologyKey": "kubernetes.io/hostname"
                            },
                            "enabled": false,
                            "size": 1
                        },
                        "expose": {
                            "enabled": false,
                            "exposeType": ""
                        },
                        "size": 3,
                        "volumeSpec": {
                            "persistentVolumeClaim": {
                                "resources": {
                                    "requests": {
                                        "storage": "25000000000"
                                    }
                                }
                            }
                        }
                    },
                    "enabled": true,
                    "mongos": {
                        "affinity": {
                            "antiAffinityTopologyKey": "kubernetes.io/hostname"
                        },
                        "expose": {
                            "exposeType": "LoadBalancer"
                        },
                        "resources": {
                            "limits": {
                                "cpu": "1000m",
                                "memory": "2000000000"
                            }
                        },
                        "size": 3
                    }
                },
                "updateStrategy": "RollingUpdate"
            },
            "status": {
                "conditions": [
                    {
                        "lastTransitionTime": "2022-08-27T15:16:41Z",
                        "status": "True",
                        "type": "initializing"
                    },
                    {
                        "lastTransitionTime": "2022-08-27T15:16:41Z",
                        "reason": "MongosReady",
                        "status": "True",
                        "type": "ready"
                    },
                    {
                        "lastTransitionTime": "2022-08-27T15:16:41Z",
                        "status": "True",
                        "type": "initializing"
                    }
                ],
                "mongos": {
                    "ready": 0,
                    "size": 0,
                    "status": "ready"
                },
                "observedGeneration": 1,
                "ready": 0,
                "replsets": {
                    "cfg": {
                        "message": "backup-agent: Back-off pulling image \"percona/percona-server-mongodb-operator:1.12.0-backup\"; mongod: back-off 5m0s restarting failed container=mongod pod=mongodb-xcso1v-cfg-0_default(913473ef-a7b4-4312-ba3f-84a3c05b30a9); ",
                        "ready": 0,
                        "size": 3,
                        "status": "initializing"
                    },
                    "rs0": {
                        "message": "backup-agent: Back-off pulling image \"percona/percona-server-mongodb-operator:1.12.0-backup\"; mongod: back-off 5m0s restarting failed container=mongod pod=mongodb-xcso1v-rs0-0_default(e196e010-3664-4ad6-8320-70bc78b75c1d); ",
                        "ready": 0,
                        "size": 3,
                        "status": "initializing"
                    }
                },
                "size": 6,
                "state": "initializing"
            }
        }
    ],
    "kind": "List",
    "metadata": {
        "resourceVersion": "",
        "selfLink": ""
    }
}`
		var list MinimumObjectListSpec
		err := json.Unmarshal([]byte(input), &list)
		require.NoError(t, err)
	})
}
