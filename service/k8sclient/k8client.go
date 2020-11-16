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
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	pxc "github.com/percona-platform/dbaas-controller/k8_api/pxc/v1"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// ClusterKind is a kind of a cluster.
type ClusterKind string

const (
	perconaXtraDBClusterKind = ClusterKind("PerconaXtraDBCluster")
	perconaServerMongoDBKind = ClusterKind("PerconaServerMongoDB")
)

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

	psmdbBackupImage = "percona/percona-server-mongodb-operator:1.4.0-backup"
	psmdbImage       = "percona/percona-server-mongodb-operator:1.4.0-mongod4.2"
	psmdbAPIVersion  = "psmdb.percona.com/v1-4-0"
)

// ComputeResources represents container computer resources requests or limits.
type ComputeResources struct {
	CPUM        int32
	MemoryBytes int64
}

// PXC contains information related to PXC containers in Percona XtraDB cluster.
type PXC struct {
	ComputeResources *ComputeResources
}

// ProxySQL contains information related to ProxySQL containers in Percona XtraDB cluster.
type ProxySQL struct {
	ComputeResources *ComputeResources
}

// Replicaset contains information related to Replicaset containers in PSMDB cluster.
type Replicaset struct {
	ComputeResources *ComputeResources
}

// XtraDBParams contains all parameters required to create or update Percona XtraDB cluster.
type XtraDBParams struct {
	Name     string
	Size     int32
	PXC      *PXC
	ProxySQL *ProxySQL
}

// Cluster contains common information related to cluster.
type Cluster struct {
	Name string
}

// PSMDBParams contains all parameters required to create or update percona server for mongodb cluster.
type PSMDBParams struct {
	Name       string
	Size       int32
	Replicaset *Replicaset
}

// XtraDBCluster contains information related to xtradb cluster.
type XtraDBCluster struct {
	Name     string
	Size     int32
	State    ClusterState
	PXC      *PXC
	ProxySQL *ProxySQL
}

// PSMDBCluster contains information related to psmdb cluster.
type PSMDBCluster struct {
	Name       string
	Size       int32
	State      ClusterState
	Replicaset *Replicaset
}

// pxcStatesMap matches pxc app states to cluster states.
var pxcStatesMap = map[pxc.AppState]ClusterState{
	pxc.AppStateUnknown: ClusterStateInvalid,
	pxc.AppStateInit:    ClusterStateChanging,
	pxc.AppStateReady:   ClusterStateReady,
	pxc.AppStateError:   ClusterStateFailed,
}

// psmdbStatesMap matches psmdb app states to cluster states.
var psmdbStatesMap = map[appState]ClusterState{
	appStateUnknown: ClusterStateInvalid,
	appStatePending: ClusterStateChanging,
	appStateInit:    ClusterStateChanging,
	appStateReady:   ClusterStateReady,
	appStateError:   ClusterStateFailed,
}

var (
	// ErrXtraDBClusterNotReady The PXC cluster is not in ready state.
	ErrXtraDBClusterNotReady = errors.New("XtraDB cluster is not ready")
	// ErrPSMDBClusterNotReady The PSMDB cluster is not ready.
	ErrPSMDBClusterNotReady = errors.New("PSMDB cluster is not ready")
)

// K8Client is a client for Kubernetes.
type K8Client struct {
	kubeCtl *kubectl.KubeCtl
}

// New returns new K8Client object.
func New(ctx context.Context, kubeconfig string) (*K8Client, error) {
	kubeCtl, err := kubectl.NewKubeCtl(ctx, kubeconfig)
	if err != nil {
		return nil, err
	}
	return &K8Client{
		kubeCtl: kubeCtl,
	}, nil
}

// Cleanup removes temporary files created by that object.
func (c *K8Client) Cleanup() error {
	return c.kubeCtl.Cleanup()
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
	if params.PXC != nil {
		c.setXtraDBComputeResources(res.Spec.PXC, params.PXC.ComputeResources)
	}
	if params.ProxySQL != nil {
		c.setXtraDBComputeResources(res.Spec.ProxySQL, params.ProxySQL.ComputeResources)
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

	// This is to prevent concurrent updates
	if cluster.Status.PXC.Status != pxc.AppStateReady {
		return ErrXtraDBClusterNotReady //nolint:wrapcheck
	}

	cluster.Spec.PXC.Size = params.Size
	cluster.Spec.ProxySQL.Size = params.Size

	return c.kubeCtl.Apply(ctx, &cluster)
}

// DeleteXtraDBCluster deletes Percona XtraDB cluster with provided name.
func (c *K8Client) DeleteXtraDBCluster(ctx context.Context, name string) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: meta.TypeMeta{
			APIVersion: pxcAPIVersion,
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
		if cluster.Spec.PXC.Resources != nil {
			val.PXC = &PXC{
				ComputeResources: c.getXtraDBComputeResources(*cluster.Spec.PXC.Resources),
			}
		}
		if cluster.Spec.ProxySQL.Resources != nil {
			val.ProxySQL = &ProxySQL{
				ComputeResources: c.getXtraDBComputeResources(*cluster.Spec.ProxySQL.Resources),
			}
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

func (c *K8Client) getXtraDBComputeResources(resources pxc.PodResources) *ComputeResources {
	if resources.Limits == nil {
		return nil
	}
	cpum := resource.MustParse(resources.Limits.CPU)
	memory := resource.MustParse(resources.Limits.Memory)
	return &ComputeResources{
		CPUM:        int32(cpum.MilliValue()),
		MemoryBytes: memory.Value(),
	}
}

func (c *K8Client) setXtraDBComputeResources(podResources *pxc.PodSpec, computeResources *ComputeResources) {
	if computeResources == nil {
		return
	}

	if computeResources.MemoryBytes > 0 || computeResources.CPUM > 0 {
		podResources.Resources = &pxc.PodResources{Limits: new(pxc.ResourcesList)}
		if computeResources.MemoryBytes > 0 {
			podResources.Resources.Limits.Memory = resource.NewQuantity(computeResources.MemoryBytes, resource.DecimalSI).String()
		}

		if computeResources.CPUM > 0 {
			podResources.Resources.Limits.CPU = resource.NewMilliQuantity(int64(computeResources.CPUM), resource.DecimalSI).String()
		}
	}
}

// ListPSMDBClusters returns list of psmdb clusters and their statuses.
func (c *K8Client) ListPSMDBClusters(ctx context.Context) ([]PSMDBCluster, error) {
	clusters, err := c.getPSMDBClusters(ctx)
	if err != nil {
		return nil, err
	}

	deletingClusters, err := c.getDeletingPSMDBClusters(ctx, clusters)
	if err != nil {
		return nil, err
	}
	res := append(clusters, deletingClusters...)

	return res, nil
}

// CreatePSMDBCluster creates percona server for mongodb cluster with provided parameters.
func (c *K8Client) CreatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	res := &perconaServerMongoDB{
		TypeMeta: TypeMeta{
			APIVersion: psmdbAPIVersion,
			Kind:       string(perconaServerMongoDBKind),
		},
		ObjectMeta: ObjectMeta{
			Name: params.Name,
		},
		Spec: perconaServerMongoDBSpec{
			Image: psmdbImage,
			Secrets: &secretsSpec{
				Users: "my-cluster-name-secrets",
			},
			Mongod: &mongodSpec{
				Net: &mongodSpecNet{
					Port: 27017,
				},
				OperationProfiling: &mongodSpecOperationProfiling{
					Mode: operationProfilingModeSlowOp,
				},
				Security: &mongodSpecSecurity{
					RedactClientLogData:  false,
					EnableEncryption:     pointer.ToBool(true),
					EncryptionKeySecret:  "my-cluster-name-mongodb-encryption-key",
					EncryptionCipherMode: mongodChiperModeCBC,
				},
				Storage: &mongodSpecStorage{
					Engine: storageEngineWiredTiger,
					MMAPv1: &mongodSpecMMAPv1{
						NsSize:     16,
						Smallfiles: false,
					},
					WiredTiger: &mongodSpecWiredTiger{
						CollectionConfig: &mongodSpecWiredTigerCollectionConfig{
							BlockCompressor: &wiredTigerCompressorSnappy,
						},
						EngineConfig: &mongodSpecWiredTigerEngineConfig{
							DirectoryForIndexes: false,
							JournalCompressor:   &wiredTigerCompressorSnappy,
						},
						IndexConfig: &mongodSpecWiredTigerIndexConfig{
							PrefixCompression: true,
						},
					},
				},
			},
			Replsets: []*replsetSpec{
				{
					Name: "rs0",
					Size: params.Size,
					Arbiter: arbiter{
						Enabled: false,
						Size:    1,
						multiAZ: multiAZ{
							Affinity: &podAffinity{
								TopologyKey: pointer.ToString("kubernetes.io/hostname"),
							},
						},
					},
					VolumeSpec: &volumeSpec{
						PersistentVolumeClaim: &persistentVolumeClaimSpec{
							Resources: resourceRequirements{
								Requests: resourceList{
									ResourceStorage: resource.MustParse("1Gi"),
								},
							},
						},
					},
					multiAZ: multiAZ{
						Affinity: &podAffinity{
							TopologyKey: pointer.ToString(affinityOff),
						},
					},
				},
			},

			PMM: pmmSpec{
				Enabled: false,
			},

			Backup: backupSpec{
				Enabled:            true,
				Image:              psmdbBackupImage,
				ServiceAccountName: "percona-server-mongodb-operator",
			},
		},
	}
	if params.Replicaset != nil {
		res.Spec.Replsets[0].Resources = c.setComputeResources(params.Replicaset.ComputeResources)
	}
	return c.kubeCtl.Apply(ctx, res)
}

// UpdatePSMDBCluster changes size of provided percona server for mongodb cluster.
func (c *K8Client) UpdatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	var cluster perconaServerMongoDB
	err := c.kubeCtl.Get(ctx, string(perconaServerMongoDBKind), params.Name, &cluster)
	if err != nil {
		return errors.Wrap(err, "UpdatePSMDBCluster get error")
	}

	// This is to prevent concurrent updates
	if cluster.Status.Status != appStateReady {
		return ErrPSMDBClusterNotReady //nolint:wrapcheck
	}

	cluster.Spec.Replsets[0].Size = params.Size

	if params.Replicaset != nil {
		cluster.Spec.Replsets[0].Resources = c.setComputeResources(params.Replicaset.ComputeResources)
	}

	return c.kubeCtl.Apply(ctx, cluster)
}

// DeletePSMDBCluster deletes percona server for mongodb cluster with provided name.
func (c *K8Client) DeletePSMDBCluster(ctx context.Context, name string) error {
	res := &perconaServerMongoDB{
		TypeMeta: TypeMeta{
			APIVersion: psmdbAPIVersion,
			Kind:       string(perconaServerMongoDBKind),
		},
		ObjectMeta: ObjectMeta{
			Name: name,
		},
	}
	return c.kubeCtl.Delete(ctx, res)
}

// getPSMDBClusters returns Percona Server for MongoDB clusters.
func (c *K8Client) getPSMDBClusters(ctx context.Context) ([]PSMDBCluster, error) {
	var list meta.List
	err := c.kubeCtl.Get(ctx, string(perconaServerMongoDBKind), "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get percona server MongoDB clusters")
	}

	res := make([]PSMDBCluster, len(list.Items))
	for i, item := range list.Items {
		var cluster perconaServerMongoDB
		if err := json.Unmarshal(item.Raw, &cluster); err != nil {
			return nil, err
		}

		val := PSMDBCluster{
			Name:  cluster.Name,
			Size:  cluster.Spec.Replsets[0].Size,
			State: getReplicasetStatus(cluster),
		}

		if cluster.Spec.Replsets[0].Resources != nil {
			val.Replicaset = &Replicaset{
				ComputeResources: c.getComputeResources(*cluster.Spec.Replsets[0].Resources),
			}
		}
		res[i] = val
	}
	return res, nil
}

/*
  When a cluster is being initialized but there are not enough nodes to form a cluster (less than 3)
  the operator returns State=Error but that's not the real cluster state.
  While the cluster is being initialized, we need to return the lowest state value found in the
  replicaset list of members.
*/
func getReplicasetStatus(cluster perconaServerMongoDB) ClusterState {
	if strings.ToLower(string(cluster.Status.Status)) != string(appStateError) {
		return psmdbStatesMap[cluster.Status.Status]
	}

	if len(cluster.Status.Replsets) == 0 {
		return ClusterStateInvalid
	}

	var status ClusterState
	var i int

	// We need to extract the lowest value so the first time, that's the lowest value.
	// Its is not possible to get the initial value in other way since cluster.Status.Replsets is a map
	// not an array.
	for _, replset := range cluster.Status.Replsets {
		replStatus := psmdbStatesMap[replset.Status]
		if replStatus < status || i == 0 {
			status = replStatus
		}
		i++
	}

	return status
}

// getDeletingXtraDBClusters returns Percona XtraDB clusters which are not fully deleted yet.
func (c *K8Client) getDeletingPSMDBClusters(ctx context.Context, clusters []PSMDBCluster) ([]PSMDBCluster, error) {
	runningClusters := make(map[string]struct{}, len(clusters))
	for _, cluster := range clusters {
		runningClusters[cluster.Name] = struct{}{}
	}

	deletingClusters, err := c.getDeletingClusters(ctx, "percona-server-mongodb-operator", runningClusters)
	if err != nil {
		return nil, err
	}

	xtradbClusters := make([]PSMDBCluster, len(deletingClusters))
	for i, cluster := range deletingClusters {
		xtradbClusters[i] = PSMDBCluster{
			Name:  cluster.Name,
			State: ClusterStateDeleting,
		}
	}
	return xtradbClusters, nil
}

func (c *K8Client) getComputeResources(resources resourcesSpec) *ComputeResources {
	if resources.Limits == nil {
		return nil
	}
	res := new(ComputeResources)
	if resources.Limits.CPU != "" {
		cpum := resource.MustParse(resources.Limits.CPU)
		res.CPUM = int32(cpum.MilliValue())
	}
	if resources.Limits.Memory != "" {
		memory := resource.MustParse(resources.Limits.Memory)
		res.MemoryBytes = memory.Value()
	}
	return res
}

func (c *K8Client) setComputeResources(res *ComputeResources) *resourcesSpec {
	if res == nil || (res.CPUM <= 0 && res.MemoryBytes <= 0) {
		return nil
	}
	r := &resourcesSpec{
		Limits: new(resourceSpecRequirements),
	}
	if res.CPUM > 0 {
		r.Limits.CPU = resource.NewMilliQuantity(int64(res.CPUM), resource.DecimalSI).String()
	}
	if res.MemoryBytes > 0 {
		r.Limits.Memory = resource.NewQuantity(res.MemoryBytes, resource.DecimalSI).String()
	}
	return r
}
