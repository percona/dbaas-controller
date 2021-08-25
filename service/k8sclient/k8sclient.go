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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kubectl"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/monitoring"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/psmdb"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/pxc"
	"github.com/percona-platform/dbaas-controller/utils/convertors"
	"github.com/percona-platform/dbaas-controller/utils/logger"
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
	// ClusterStateFailed represents a failed cluster (error).
	ClusterStateFailed ClusterState = 2
	// ClusterStateReady represents a cluster without pending changes (ready).
	ClusterStateReady ClusterState = 3
	// ClusterStateDeleting represents a cluster which are in deleting state (deleting).
	ClusterStateDeleting ClusterState = 4
	// ClusterStatePaused represents a paused cluster state (status.state.ready and spec.pause.true).
	ClusterStatePaused ClusterState = 5
	// ClusterStateUpgrading represents a state of a cluster that is undergoing an upgrade.
	ClusterStateUpgrading ClusterState = 6
)

const (
	pmmClientImage = "perconalab/pmm-client:dev-latest"

	k8sAPIVersion     = "v1"
	k8sMetaKindSecret = "Secret"

	pxcCRVersion            = "1.8.0"
	pxcBackupImage          = "percona/percona-xtradb-cluster-operator:1.8.0-pxc8.0-backup"
	pxcDefaultImage         = "percona/percona-xtradb-cluster:8.0.20-11.1"
	pxcBackupStorageName    = "pxc-backup-storage-%s"
	pxcAPIVersion           = "pxc.percona.com/v1-8-0"
	pxcProxySQLDefaultImage = "percona/percona-xtradb-cluster-operator:1.8.0-proxysql"
	pxcHAProxyDefaultImage  = "percona/percona-xtradb-cluster-operator:1.8.0-haproxy"
	pxcSecretNameTmpl       = "dbaas-%s-pxc-secrets"
	pxcInternalSecretTmpl   = "internal-%s"

	psmdbCRVersion      = "1.9.0"
	psmdbBackupImage    = "percona/percona-server-mongodb-operator:1.9.0-backup"
	psmdbDefaultImage   = "percona/percona-server-mongodb:4.2.8-8"
	psmdbAPIVersion     = "psmdb.percona.com/v1-9-0"
	psmdbSecretNameTmpl = "dbaas-%s-psmdb-secrets"

	// Max size of volume for AWS Elastic Block Storage service is 16TiB.
	maxVolumeSizeEBS uint64 = 16 * 1024 * 1024 * 1024 * 1024
	pullPolicy              = common.PullIfNotPresent
)

// KubernetesClusterType represents kubernetes cluster type(eg: EKS, Minikube).
type KubernetesClusterType uint8

const (
	clusterTypeUnknown KubernetesClusterType = iota
	// AmazonEKSClusterType represents EKS cluster type.
	AmazonEKSClusterType
	// MinikubeClusterType represents minikube Kubernetes cluster.
	MinikubeClusterType
)

// ContainerState describes container's state - waiting, running, terminated.
type ContainerState string

const (
	// ContainerStateWaiting represents a state when container requires some
	// operations being done in order to complete start up.
	ContainerStateWaiting ContainerState = "waiting"
)

// OperatorStatus represents status of operator.
type OperatorStatus int32

const (
	// OperatorStatusOK represents that operators are installed and have supported API version.
	OperatorStatusOK OperatorStatus = 1
	// OperatorStatusUnsupported represents that operators are installed, but doesn't have supported API version.
	OperatorStatusUnsupported OperatorStatus = 2
	// OperatorStatusNotInstalled represents that operators are not installed.
	OperatorStatusNotInstalled OperatorStatus = 3
)

const (
	clusterWithSameNameExistsErrTemplate = "Cluster '%s' already exists"
	canNotGetCredentialsErrTemplate      = "cannot get %s cluster credentials"
)

// Operator represents kubernetes operator.
type Operator struct {
	Status  OperatorStatus
	Version string
}

// Operators contains statuses of operators.
type Operators struct {
	Xtradb Operator
	Psmdb  Operator
}

// ComputeResources represents container computer resources requests or limits.
type ComputeResources struct {
	CPUM        string
	MemoryBytes string
}

// PXC contains information related to PXC containers in Percona XtraDB cluster.
type PXC struct {
	Image            string
	ComputeResources *ComputeResources
	DiskSize         string
}

// ProxySQL contains information related to ProxySQL containers in Percona XtraDB cluster.
type ProxySQL struct {
	Image            string
	ComputeResources *ComputeResources
	DiskSize         string
}

// HAProxy contains information related to HAProxy containers in Percona XtraDB cluster.
type HAProxy struct {
	Image            string
	ComputeResources *ComputeResources
}

// Replicaset contains information related to Replicaset containers in PSMDB cluster.
type Replicaset struct {
	ComputeResources *ComputeResources
	DiskSize         string
}

// PMM contains information related to PMM.
type PMM struct {
	// PMM server public address.
	PublicAddress string
	// PMM server admin login.
	Login string
	// PMM server admin password.
	Password string
}

// XtraDBParams contains all parameters required to create or update Percona XtraDB cluster.
type XtraDBParams struct {
	Name              string
	Size              int32
	Suspend           bool
	Resume            bool
	PXC               *PXC
	ProxySQL          *ProxySQL
	PMM               *PMM
	HAProxy           *HAProxy
	Expose            bool
	VersionServiceURL string
}

// Cluster contains common information related to cluster.
type Cluster struct {
	Name string
}

// PSMDBParams contains all parameters required to create or update percona server for mongodb cluster.
type PSMDBParams struct {
	Name              string
	Image             string
	Size              int32
	Suspend           bool
	Resume            bool
	Replicaset        *Replicaset
	PMM               *PMM
	Expose            bool
	VersionServiceURL string
}

type appStatus struct {
	size  int32
	ready int32
}

// DetailedState contains pods' status.
type DetailedState []appStatus

// XtraDBCluster contains information related to xtradb cluster.
type XtraDBCluster struct {
	Name          string
	Size          int32
	State         ClusterState
	Message       string
	PXC           *PXC
	ProxySQL      *ProxySQL
	HAProxy       *HAProxy
	Pause         bool
	DetailedState DetailedState
	Exposed       bool
}

// PSMDBCluster contains information related to psmdb cluster.
type PSMDBCluster struct {
	Name          string
	Pause         bool
	Size          int32
	State         ClusterState
	Message       string
	Replicaset    *Replicaset
	DetailedState DetailedState
	Exposed       bool
	Image         string
}

// PSMDBCredentials represents PSMDB connection credentials.
type PSMDBCredentials struct {
	Username   string
	Password   string
	Host       string
	Port       int32
	Replicaset string
}

// XtraDBCredentials represents XtraDB connection credentials.
type XtraDBCredentials struct {
	Username string
	Password string
	Host     string
	Port     int32
}

// StorageClass represents a cluster storage class information.
// We use the Items.Provisioner to detect if we are running against minikube or AWS.
// Returned value examples:
// - AWS EKS: kubernetes.io/aws-ebs
// - Minukube: k8s.io/minikube-hostpath.
type StorageClass struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Annotations struct {
				KubectlKubernetesIoLastAppliedConfiguration string `json:"kubectl.kubernetes.io/last-applied-configuration"`
				StorageclassKubernetesIoIsDefaultClass      string `json:"storageclass.kubernetes.io/is-default-class"`
			} `json:"annotations"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			Labels            struct {
				AddonmanagerKubernetesIoMode string `json:"addonmanager.kubernetes.io/mode"`
			} `json:"labels"`
			Name            string `json:"name"`
			ResourceVersion string `json:"resourceVersion"`
			SelfLink        string `json:"selfLink"`
			UID             string `json:"uid"`
		} `json:"metadata"`
		Provisioner       string `json:"provisioner"`
		ReclaimPolicy     string `json:"reclaimPolicy"`
		VolumeBindingMode string `json:"volumeBindingMode"`
	} `json:"items"`
	Kind     string `json:"kind"`
	Metadata struct {
		ResourceVersion string `json:"resourceVersion"`
		SelfLink        string `json:"selfLink"`
	} `json:"metadata"`
}

// pxcStatesMap matches pxc app states to cluster states.
var pxcStatesMap = map[pxc.AppState]ClusterState{ //nolint:gochecknoglobals
	pxc.AppStateUnknown: ClusterStateInvalid,
	pxc.AppStateInit:    ClusterStateChanging,
	pxc.AppStateReady:   ClusterStateReady,
	pxc.AppStateError:   ClusterStateFailed,
}

// psmdbStatesMap matches psmdb app states to cluster states.
var psmdbStatesMap = map[psmdb.AppState]ClusterState{ //nolint:gochecknoglobals
	psmdb.AppStateUnknown: ClusterStateInvalid,
	psmdb.AppStatePending: ClusterStateChanging,
	psmdb.AppStateInit:    ClusterStateChanging,
	psmdb.AppStateReady:   ClusterStateReady,
	psmdb.AppStateError:   ClusterStateFailed,
}

var (
	// ErrXtraDBClusterNotReady The PXC cluster is not in ready state.
	ErrXtraDBClusterNotReady = errors.New("XtraDB cluster is not ready")
	// ErrPSMDBClusterNotReady The PSMDB cluster is not ready.
	ErrPSMDBClusterNotReady = errors.New("PSMDB cluster is not ready")
	// ErrNotFound should be returned when referenced resource does not exist
	// inside Kubernetes cluster.
	ErrNotFound error = errors.New("resource was not found in Kubernetes cluster")
)

// K8sClient is a client for Kubernetes.
type K8sClient struct {
	kubeCtl    *kubectl.KubeCtl
	l          logger.Logger
	kubeconfig string
}

// CountReadyPods returns number of pods that are ready and belong to the
// database cluster.
func (d DetailedState) CountReadyPods() (count int32) {
	for _, status := range d {
		count += status.ready
	}
	return
}

// CountAllPods returns number of all pods belonging to the database cluster.
func (d DetailedState) CountAllPods() (count int32) {
	for _, status := range d {
		count += status.size
	}
	return
}

// New returns new K8Client object.
func New(ctx context.Context, kubeconfig string) (*K8sClient, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "K8sClient")

	kubeCtl, err := kubectl.NewKubeCtl(ctx, kubeconfig)
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		kubeCtl:    kubeCtl,
		l:          l,
		kubeconfig: kubeconfig,
	}, nil
}

// Cleanup removes temporary files created by that object.
func (c *K8sClient) Cleanup() error {
	return c.kubeCtl.Cleanup()
}

// ListXtraDBClusters returns list of Percona XtraDB clusters and their statuses.
func (c *K8sClient) ListXtraDBClusters(ctx context.Context) ([]XtraDBCluster, error) {
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

// CreateSecret creates secret resource to use as credential source for clusters.
func (c *K8sClient) CreateSecret(ctx context.Context, secretName string, data map[string][]byte) error {
	secret := common.Secret{
		TypeMeta: common.TypeMeta{
			APIVersion: k8sAPIVersion,
			Kind:       k8sMetaKindSecret,
		},
		ObjectMeta: common.ObjectMeta{
			Name: secretName,
		},
		Type: common.SecretTypeOpaque,
		Data: data,
	}
	return c.kubeCtl.Apply(ctx, secret)
}

// CreateXtraDBCluster creates Percona XtraDB cluster with provided parameters.
func (c *K8sClient) CreateXtraDBCluster(ctx context.Context, params *XtraDBParams) error {
	if (params.ProxySQL != nil) == (params.HAProxy != nil) {
		return errors.New("xtradb cluster must have one and only one proxy type defined")
	}

	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), params.Name, &cluster)
	if err == nil {
		return fmt.Errorf(clusterWithSameNameExistsErrTemplate, params.Name)
	}

	secretName := fmt.Sprintf(pxcSecretNameTmpl, params.Name)
	secrets, err := generateXtraDBPasswords()
	if err != nil {
		return err
	}

	storageName := fmt.Sprintf(pxcBackupStorageName, params.Name)
	pxcImage := pxcDefaultImage
	if params.PXC.Image != "" {
		pxcImage = params.PXC.Image
	}

	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: common.TypeMeta{
			APIVersion: pxcAPIVersion,
			Kind:       string(perconaXtraDBClusterKind),
		},
		ObjectMeta: common.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-proxysql-pvc", "delete-pxc-pvc"},
		},
		Spec: &pxc.PerconaXtraDBClusterSpec{
			UpdateStrategy: updateStrategySmartUpdate,
			UpgradeOptions: &common.UpgradeOptions{
				VersionServiceEndpoint: params.VersionServiceURL,
			},
			CRVersion:         pxcCRVersion,
			AllowUnsafeConfig: true,
			SecretsName:       secretName,

			PXC: &pxc.PodSpec{
				Size:            &params.Size,
				Resources:       c.setComputeResources(params.PXC.ComputeResources),
				Image:           pxcImage,
				ImagePullPolicy: pullPolicy,
				VolumeSpec:      c.volumeSpec(params.PXC.DiskSize),
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
				PodDisruptionBudget: &common.PodDisruptionBudgetSpec{
					MaxUnavailable: pointer.ToInt(1),
				},
			},

			PMM: &pxc.PMMSpec{
				Enabled: false,
			},

			Backup: &pxc.PXCScheduledBackup{
				Image: pxcBackupImage,
				Schedule: []pxc.PXCScheduledBackupSchedule{{
					Name:        "test",
					Schedule:    "*/30 * * * *",
					Keep:        3,
					StorageName: storageName,
				}},
				Storages: map[string]*pxc.BackupStorageSpec{
					storageName: {
						Type:   pxc.BackupStorageFilesystem,
						Volume: c.volumeSpec(params.PXC.DiskSize),
					},
				},
				ServiceAccountName: "percona-xtradb-cluster-operator",
			},
		},
	}
	if params.PMM != nil {
		res.Spec.PMM = &pxc.PMMSpec{
			Enabled:         true,
			ServerHost:      params.PMM.PublicAddress,
			ServerUser:      params.PMM.Login,
			Image:           pmmClientImage,
			ImagePullPolicy: pullPolicy,
			Resources: &common.PodResources{
				Requests: &common.ResourcesList{
					Memory: "500M",
					CPU:    "500m",
				},
			},
		}
		secrets["pmmserver"] = []byte(params.PMM.Password)
	}

	var podSpec *pxc.PodSpec
	if params.ProxySQL != nil {
		res.Spec.ProxySQL = new(pxc.PodSpec)
		podSpec = res.Spec.ProxySQL
		podSpec.Image = pxcProxySQLDefaultImage
		if params.ProxySQL.Image != "" {
			podSpec.Image = params.ProxySQL.Image
		}
		podSpec.Resources = c.setComputeResources(params.ProxySQL.ComputeResources)
		podSpec.VolumeSpec = c.volumeSpec(params.ProxySQL.DiskSize)
	} else {
		res.Spec.HAProxy = new(pxc.PodSpec)
		podSpec = res.Spec.HAProxy
		podSpec.Image = pxcHAProxyDefaultImage
		if params.HAProxy.Image != "" {
			podSpec.Image = params.HAProxy.Image
		}
		podSpec.Resources = c.setComputeResources(params.HAProxy.ComputeResources)
	}

	// This enables ingress for the cluster and exposes the cluster to the world.
	// The cluster will have an internal IP and a world accessible hostname.
	// This feature cannot be tested with minikube. Please use EKS for testing.
	if clusterType := c.GetKubernetesClusterType(ctx); clusterType != MinikubeClusterType && params.Expose {
		podSpec.ServiceType = common.ServiceTypeLoadBalancer
	}

	podSpec.Enabled = true
	podSpec.ImagePullPolicy = pullPolicy
	podSpec.Size = &params.Size
	podSpec.Affinity = &pxc.PodAffinity{
		TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
	}

	err = c.CreateSecret(ctx, secretName, secrets)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}

	return c.kubeCtl.Apply(ctx, res)
}

// UpdateXtraDBCluster changes size of provided Percona XtraDB cluster.
func (c *K8sClient) UpdateXtraDBCluster(ctx context.Context, params *XtraDBParams) error {
	if (params.ProxySQL != nil) && (params.HAProxy != nil) {
		return errors.New("can't update both proxies, only one should be in use")
	}

	getCluster := func(ctx context.Context, k *kubectl.KubeCtl) (*pxc.PerconaXtraDBCluster, error) {
		var cluster pxc.PerconaXtraDBCluster
		err := k.Get(ctx, string(perconaXtraDBClusterKind), params.Name, &cluster)
		if err != nil {
			return nil, err
		}
		return &cluster, nil
	}
	getDatabaseCluster := func(ctx context.Context, k *kubectl.KubeCtl) (common.DatabaseCluster, error) {
		cluster, err := getCluster(ctx, k)
		return common.DatabaseCluster(cluster), err
	}

	cluster, err := getCluster(ctx, c.kubeCtl)
	if err != nil {
		return err
	}

	// This is to prevent concurrent updates
	if cluster.Status.PXC.Status != pxc.AppStateReady {
		return errors.Wrapf(ErrXtraDBClusterNotReady, "state is %v", cluster.Status.Status) //nolint:wrapcheck
	}

	if params.Resume {
		cluster.Spec.Pause = false
	}
	if params.Suspend {
		cluster.Spec.Pause = true
	}

	if params.Size > 0 {
		cluster.Spec.PXC.Size = &params.Size
		if cluster.Spec.ProxySQL != nil {
			cluster.Spec.ProxySQL.Size = &params.Size
		} else {
			cluster.Spec.HAProxy.Size = &params.Size
		}
	}

	if params.PXC != nil {
		cluster.Spec.PXC.Resources = c.updateComputeResources(params.PXC.ComputeResources, cluster.Spec.PXC.Resources)
	}

	if params.ProxySQL != nil {
		cluster.Spec.ProxySQL.Resources = c.updateComputeResources(params.ProxySQL.ComputeResources, cluster.Spec.ProxySQL.Resources)
	}

	if params.HAProxy != nil {
		cluster.Spec.HAProxy.Resources = c.updateComputeResources(params.HAProxy.ComputeResources, cluster.Spec.HAProxy.Resources)
	}

	if params.PXC.Image != "" {
		err = c.addUpgradeTriggers(ctx, getDatabaseCluster, cluster, params.PXC.Image)
		if err != nil {
			return err
		}
	}

	return c.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, common.DatabaseCluster(cluster).GetCRDName(), common.DatabaseCluster(cluster).GetName(), cluster)
}

// DeleteXtraDBCluster deletes Percona XtraDB cluster with provided name.
func (c *K8sClient) DeleteXtraDBCluster(ctx context.Context, name string) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: common.TypeMeta{
			APIVersion: pxcAPIVersion,
			Kind:       string(perconaXtraDBClusterKind),
		},
		ObjectMeta: common.ObjectMeta{
			Name: name,
		},
	}
	err := c.kubeCtl.Delete(ctx, res)
	if err != nil {
		return errors.Wrap(err, "cannot delete PXC")
	}

	err = c.deleteSecret(ctx, fmt.Sprintf(pxcSecretNameTmpl, name))
	if err != nil {
		c.l.Errorf("cannot delete secret for %s: %v", name, err)
	}

	err = c.deleteSecret(ctx, fmt.Sprintf(pxcInternalSecretTmpl, name))
	if err != nil {
		c.l.Errorf("cannot delete internal secret for %s: %v", name, err)
	}

	return nil
}

func (c *K8sClient) deleteSecret(ctx context.Context, secretName string) error {
	secret := &common.Secret{
		TypeMeta: common.TypeMeta{
			APIVersion: k8sAPIVersion,
			Kind:       k8sMetaKindSecret,
		},
		ObjectMeta: common.ObjectMeta{
			Name: secretName,
		},
	}

	return c.kubeCtl.Delete(ctx, secret)
}

// GetXtraDBClusterCredentials returns an XtraDB cluster credentials.
func (c *K8sClient) GetXtraDBClusterCredentials(ctx context.Context, name string) (*XtraDBCredentials, error) {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), name, &cluster)
	if err != nil {
		if errors.Is(err, kubectl.ErrNotFound) {
			return nil, errors.Wrap(ErrNotFound, fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"))
		}
		return nil, errors.Wrap(err, fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"))
	}
	if cluster.Status == nil || cluster.Status.Status != pxc.AppStateReady {
		return nil, errors.Wrap(ErrXtraDBClusterNotReady,
			fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"),
		)
	}

	password := ""
	var secret common.Secret
	// Retrieve secrets only for initializing or ready cluster.
	if cluster.Status.Status == pxc.AppStateReady || cluster.Status.Status == pxc.AppStateInit {
		err = c.kubeCtl.Get(ctx, k8sMetaKindSecret, fmt.Sprintf(pxcSecretNameTmpl, name), &secret)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get XtraDb cluster secrets")
		}
		password = string(secret.Data["root"])
	}

	credentials := &XtraDBCredentials{
		Host:     cluster.Status.Host,
		Port:     3306,
		Username: "root",
		Password: password,
	}

	return credentials, nil
}

func (c *K8sClient) getStorageClass(ctx context.Context) (*StorageClass, error) {
	var storageClass *StorageClass

	err := c.kubeCtl.Get(ctx, "storageclass", "", &storageClass)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get storageClass")
	}

	return storageClass, nil
}

// GetKubernetesClusterType returns k8s cluster type based on storage class.
func (c *K8sClient) GetKubernetesClusterType(ctx context.Context) KubernetesClusterType {
	sc, err := c.getStorageClass(ctx)
	if err != nil {
		c.l.Error(errors.Wrap(err, "failed to get k8s cluster type"))
		return clusterTypeUnknown
	}

	if len(sc.Items) == 0 {
		return clusterTypeUnknown
	}

	for _, class := range sc.Items {
		if strings.Contains(class.Provisioner, "aws") {
			return AmazonEKSClusterType
		}
		if strings.Contains(class.Provisioner, "minikube") {
			return MinikubeClusterType
		}
	}

	return clusterTypeUnknown
}

func (c *K8sClient) restartDBClusterCmd(name, kind string) []string {
	return []string{"rollout", "restart", "StatefulSets", fmt.Sprintf("%s-%s", name, kind)}
}

// RestartXtraDBCluster restarts Percona XtraDB cluster with provided name.
// FIXME: https://jira.percona.com/browse/PMM-6980
func (c *K8sClient) RestartXtraDBCluster(ctx context.Context, name string) error {
	_, err := c.kubeCtl.Run(ctx, c.restartDBClusterCmd(name, "pxc"), nil)
	if err != nil {
		return err
	}

	for _, proxy := range []string{"proxysql", "haproxy"} {
		if _, err := c.kubeCtl.Run(ctx, []string{"get", "statefulset", name + "-" + proxy}, nil); err == nil {
			_, err = c.kubeCtl.Run(ctx, c.restartDBClusterCmd(name, proxy), nil)
			return err
		}
	}

	return errors.New("failed to restart pxc cluster proxy statefulset")
}

// getPerconaXtraDBClusters returns Percona XtraDB clusters.
func (c *K8sClient) getPerconaXtraDBClusters(ctx context.Context) ([]XtraDBCluster, error) {
	var list pxc.PerconaXtraDBClusterList
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get Percona XtraDB clusters")
	}

	res := make([]XtraDBCluster, len(list.Items))
	for i, cluster := range list.Items {
		val := XtraDBCluster{
			Name:    cluster.Name,
			Size:    *cluster.Spec.PXC.Size,
			Message: strings.Join(cluster.Status.Messages, ";"),
			PXC: &PXC{
				Image:            cluster.Spec.PXC.Image,
				DiskSize:         c.getDiskSize(cluster.Spec.PXC.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.PXC.Resources),
			},
			Pause: cluster.Spec.Pause,
			DetailedState: []appStatus{
				{size: cluster.Status.PMM.Size, ready: cluster.Status.PMM.Ready},
				{size: cluster.Status.HAProxy.Size, ready: cluster.Status.HAProxy.Ready},
				{size: cluster.Status.ProxySQL.Size, ready: cluster.Status.ProxySQL.Ready},
				{size: cluster.Status.PXC.Size, ready: cluster.Status.PXC.Ready},
			},
		}

		if cluster.Status != nil {
			if cluster.Spec.UpgradeOptions != nil {
				if _, err := version.NewVersion(cluster.Spec.UpgradeOptions.Apply); err == nil {
					val.State = ClusterStateUpgrading
				} else {
					val.State = getPXCState(cluster.Status.Status)
				}
			} else {
				val.State = getPXCState(cluster.Status.Status)
			}
		} else {
			val.State = ClusterStateInvalid
		}

		if cluster.Spec.ProxySQL != nil {
			val.ProxySQL = &ProxySQL{
				DiskSize:         c.getDiskSize(cluster.Spec.ProxySQL.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.ProxySQL.Resources),
			}
			val.Exposed = cluster.Spec.ProxySQL.ServiceType != "" &&
				cluster.Spec.ProxySQL.ServiceType != common.ServiceTypeClusterIP
			res[i] = val
			continue
		}
		if cluster.Spec.HAProxy != nil {
			val.HAProxy = &HAProxy{
				ComputeResources: c.getComputeResources(cluster.Spec.HAProxy.Resources),
			}
			val.Exposed = cluster.Spec.HAProxy.ServiceType != "" &&
				cluster.Spec.HAProxy.ServiceType != common.ServiceTypeClusterIP
		}

		res[i] = val
	}
	return res, nil
}

func getPXCState(state pxc.AppState) ClusterState {
	clusterState, ok := pxcStatesMap[state]
	if !ok {
		l := logger.Get(context.Background())
		l = l.WithField("component", "K8sClient")
		l.Warn("Cannot get cluster state. Setting status to ClusterStateChanging")
		return ClusterStateChanging
	}
	return clusterState
}

// getDeletingClusters returns clusters which are not fully deleted yet.
func (c *K8sClient) getDeletingClusters(ctx context.Context, managedBy string, runningClusters map[string]struct{}) ([]Cluster, error) {
	var list common.PodList

	err := c.kubeCtl.Get(ctx, "pods", "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get kubernetes pods")
	}

	res := []Cluster{}
	for _, pod := range list.Items {
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
func (c *K8sClient) getDeletingXtraDBClusters(ctx context.Context, clusters []XtraDBCluster) ([]XtraDBCluster, error) {
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
			Name:          cluster.Name,
			Size:          0,
			State:         ClusterStateDeleting,
			PXC:           new(PXC),
			ProxySQL:      new(ProxySQL),
			HAProxy:       new(HAProxy),
			DetailedState: []appStatus{},
		}
	}
	return xtradbClusters, nil
}

// ListPSMDBClusters returns list of psmdb clusters and their statuses.
func (c *K8sClient) ListPSMDBClusters(ctx context.Context) ([]PSMDBCluster, error) {
	clusters, err := c.getPSMDBClusters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get PSMDB clusters")
	}

	deletingClusters, err := c.getDeletingPSMDBClusters(ctx, clusters)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get deleting PSMDB clusters")
	}
	res := append(clusters, deletingClusters...)

	return res, nil
}

// CreatePSMDBCluster creates percona server for mongodb cluster with provided parameters.
func (c *K8sClient) CreatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	var cluster psmdb.PerconaServerMongoDB
	err := c.kubeCtl.Get(ctx, string(perconaServerMongoDBKind), params.Name, &cluster)
	if err == nil {
		return fmt.Errorf(clusterWithSameNameExistsErrTemplate, params.Name)
	}

	secretName := fmt.Sprintf(psmdbSecretNameTmpl, params.Name)
	secrets, err := generatePSMDBPasswords()
	if err != nil {
		return err
	}

	affinity := new(psmdb.PodAffinity)
	var expose psmdb.Expose
	if clusterType := c.GetKubernetesClusterType(ctx); clusterType != MinikubeClusterType {
		affinity.TopologyKey = pointer.ToString("kubernetes.io/hostname")

		if params.Expose {
			// This enables ingress for the cluster and exposes the cluster to the world.
			// The cluster will have an internal IP and a world accessible hostname.
			// This feature cannot be tested with minikube. Please use EKS for testing.
			expose = psmdb.Expose{
				Enabled:    true,
				ExposeType: common.ServiceTypeLoadBalancer,
			}
		}
	} else {
		// https://www.percona.com/doc/kubernetes-operator-for-psmongodb/minikube.html
		// > Install Percona Server for MongoDB on Minikube
		// > ...
		// > set affinity.antiAffinityTopologyKey key to "none"
		// > (the Operator will be unable to spread the cluster on several nodes)
		affinity.TopologyKey = pointer.ToString(psmdb.AffinityOff)
	}
	psmdbImage := psmdbDefaultImage
	if params.Image != "" {
		psmdbImage = params.Image
	}
	res := &psmdb.PerconaServerMongoDB{
		TypeMeta: common.TypeMeta{
			APIVersion: psmdbAPIVersion,
			Kind:       string(perconaServerMongoDBKind),
		},
		ObjectMeta: common.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-psmdb-pvc"},
		},
		Spec: &psmdb.PerconaServerMongoDBSpec{
			UpdateStrategy: updateStrategySmartUpdate,
			UpgradeOptions: &common.UpgradeOptions{
				VersionServiceEndpoint: params.VersionServiceURL,
			},
			CRVersion: psmdbCRVersion,
			Image:     psmdbImage,
			Secrets: &psmdb.SecretsSpec{
				Users: secretName,
			},
			Mongod: &psmdb.MongodSpec{
				Net: &psmdb.MongodSpecNet{
					Port: 27017,
				},
				OperationProfiling: &psmdb.MongodSpecOperationProfiling{
					Mode: psmdb.OperationProfilingModeSlowOp,
				},
				Security: &psmdb.MongodSpecSecurity{
					RedactClientLogData:  false,
					EnableEncryption:     pointer.ToBool(true),
					EncryptionKeySecret:  fmt.Sprintf("%s-mongodb-encryption-key", params.Name),
					EncryptionCipherMode: psmdb.MongodChiperModeCBC,
				},
				SetParameter: &psmdb.MongodSpecSetParameter{
					TTLMonitorSleepSecs: 60,
				},
				Storage: &psmdb.MongodSpecStorage{
					Engine: psmdb.StorageEngineWiredTiger,
					MMAPv1: &psmdb.MongodSpecMMAPv1{
						NsSize:     16,
						Smallfiles: false,
					},
					WiredTiger: &psmdb.MongodSpecWiredTiger{
						CollectionConfig: &psmdb.MongodSpecWiredTigerCollectionConfig{
							BlockCompressor: &psmdb.WiredTigerCompressorSnappy,
						},
						EngineConfig: &psmdb.MongodSpecWiredTigerEngineConfig{
							DirectoryForIndexes: false,
							JournalCompressor:   &psmdb.WiredTigerCompressorSnappy,
						},
						IndexConfig: &psmdb.MongodSpecWiredTigerIndexConfig{
							PrefixCompression: true,
						},
					},
				},
			},
			Sharding: &psmdb.ShardingSpec{
				Enabled: true,
				ConfigsvrReplSet: &psmdb.ReplsetSpec{
					Size:       3,
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					Arbiter: psmdb.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdb.MultiAZ{
							Affinity: affinity,
						},
					},
					MultiAZ: psmdb.MultiAZ{
						Affinity: affinity,
					},
				},
				Mongos: &psmdb.ReplsetSpec{
					Arbiter: psmdb.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdb.MultiAZ{
							Affinity: affinity,
						},
					},
					Size: params.Size,
					MultiAZ: psmdb.MultiAZ{
						Affinity: affinity,
					},
					Expose: expose,
				},
				OperationProfiling: &psmdb.MongodSpecOperationProfiling{
					Mode: psmdb.OperationProfilingModeSlowOp,
				},
			},
			Replsets: []*psmdb.ReplsetSpec{
				{
					Name:      "rs0",
					Size:      params.Size,
					Resources: c.setComputeResources(params.Replicaset.ComputeResources),
					Arbiter: psmdb.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdb.MultiAZ{
							Affinity: affinity,
						},
					},
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					PodDisruptionBudget: &common.PodDisruptionBudgetSpec{
						MaxUnavailable: pointer.ToInt(1),
					},
					MultiAZ: psmdb.MultiAZ{
						Affinity: affinity,
					},
				},
			},

			PMM: &psmdb.PmmSpec{
				Enabled: false,
			},

			Backup: &psmdb.BackupSpec{
				Enabled:            true,
				Image:              psmdbBackupImage,
				ServiceAccountName: "percona-server-mongodb-operator",
			},
		},
	}
	if params.Replicaset != nil {
		res.Spec.Replsets[0].Resources = c.setComputeResources(params.Replicaset.ComputeResources)
		res.Spec.Sharding.Mongos.Resources = c.setComputeResources(params.Replicaset.ComputeResources)
	}
	if params.PMM != nil {
		res.Spec.PMM = &psmdb.PmmSpec{
			Enabled:    true,
			ServerHost: params.PMM.PublicAddress,
			Image:      pmmClientImage,
			Resources: &common.PodResources{
				Requests: &common.ResourcesList{
					Memory: "500M",
					CPU:    "500m",
				},
			},
		}
		secrets["PMM_SERVER_USER"] = []byte(params.PMM.Login)
		secrets["PMM_SERVER_PASSWORD"] = []byte(params.PMM.Password)
	}

	err = c.CreateSecret(ctx, secretName, secrets)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}

	return c.kubeCtl.Apply(ctx, res)
}

// UpdatePSMDBCluster changes size, stops, resumes or upgrades provided percona server for mongodb cluster.
func (c *K8sClient) UpdatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	getCluster := func(ctx context.Context, k *kubectl.KubeCtl) (*psmdb.PerconaServerMongoDB, error) {
		var cluster psmdb.PerconaServerMongoDB
		err := k.Get(ctx, string(perconaServerMongoDBKind), params.Name, &cluster)
		if err != nil {
			return nil, errors.Wrap(err, "UpdatePSMDBCluster get error")
		}
		return &cluster, nil
	}
	getDatabaseCluster := func(ctx context.Context, k *kubectl.KubeCtl) (common.DatabaseCluster, error) {
		cluster, err := getCluster(ctx, k)
		return common.DatabaseCluster(cluster), err
	}

	cluster, err := getCluster(ctx, c.kubeCtl)
	if err != nil {
		return err
	}

	// This is to prevent concurrent updates
	if cluster.Status == nil || cluster.Status.Status != psmdb.AppStateReady {
		return errors.Wrap(ErrPSMDBClusterNotReady, "cluster is not in ready state") //nolint:wrapcheck
	}
	if params.Size > 0 {
		cluster.Spec.Replsets[0].Size = params.Size
	}

	if params.Resume {
		cluster.Spec.Pause = false
	}
	if params.Suspend {
		cluster.Spec.Pause = true
	}

	if params.Replicaset != nil {
		cluster.Spec.Replsets[0].Resources = c.updateComputeResources(params.Replicaset.ComputeResources, cluster.Spec.Replsets[0].Resources)
	}
	if params.Image != "" {
		// We want to update the cluster.
		err = c.addUpgradeTriggers(ctx, getDatabaseCluster, cluster, params.Image)
		if err != nil {
			return err
		}
	}
	return c.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, common.DatabaseCluster(cluster).GetCRDName(), common.DatabaseCluster(cluster).GetName(), cluster)
}

const (
	updateStrategySmartUpdate = "SmartUpdate"
	// Operators can't upgrade database version without given schedule. This ensures the upgrade is ran.
	cronScheduleForSmartUpgrade = "* * * * *"
	SmartUpdateDisabled         = "Disabled"
)

// func removeUpgradeTriggers(ctx context.Context, k *kubectl.KubeCtl) {
//TODO
// }

type getClusterFunc func(ctx context.Context, k *kubectl.KubeCtl) (common.DatabaseCluster, error)

// addUpgradeTriggers stores triggers into cluster CRs that are needed for cluster upgrade execution.
// The cluster is upgraded to version supplied in image tag. It starts goroutine that waits for upgrade to be done.
// Then it removes stored triggers.
func (c *K8sClient) addUpgradeTriggers(ctx context.Context, getCluster getClusterFunc, cluster common.DatabaseCluster, newImage string) error {
	// Check the image is the same.
	newImageAndTag := strings.Split(newImage, ":")
	if len(newImageAndTag) != 2 {
		return errors.New("image has to have version tag")
	}
	currentImageAndTag := strings.Split(cluster.GetImage(), ":")
	if currentImageAndTag[0] != newImageAndTag[0] {
		return errors.Errorf("expected image is %q, %q was given", currentImageAndTag[0], newImageAndTag[0])
	}
	if currentImageAndTag[1] == newImageAndTag[1] {
		return errors.Errorf("version %q is already in use, can't upgrade to it", newImageAndTag[1])
	}

	// Change CR for upgrade to be triggered.
	cluster.SetUpgradeOptions(newImageAndTag[1], cronScheduleForSmartUpgrade)

	// We need to disable the upgrade trigger after the upgrade is done.
	// We need to store kubeconfig because K8sClient's kubectl is Cleanup-ed when a request is done.
	go func(kubeconfig string, cluster common.DatabaseCluster) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
		defer cancel()

		client, err := New(ctx, kubeconfig)
		if err != nil {
			c.l.Errorf("failed to disable SmartUpdate: %v", err)
			return
		}
		defer client.Cleanup()

		removeUpgradeTriggers := func() {
			// Disable SmartUpdate when cluster is ready -> cluster is upgraded.
			clusterpatch := cluster.NewEmptyCluster()
			clusterpatch.SetUpgradeOptions(SmartUpdateDisabled, "")
			c.l.Infof("disabling SmartUpdate for cluster %v/%v", cluster.GetCRDName(), cluster.GetName())
			err = client.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, cluster.GetCRDName(), cluster.GetName(), clusterpatch)
			if err != nil {
				c.l.Errorf("failed to disable SmartUpdate: %v", err)
			}
		}

		crVersionAndPodsVersionMatch := func(ctx context.Context, pods *common.PodList, cluster common.DatabaseCluster) bool {
			crImage := cluster.GetImage()
			images := make(map[string]struct{})
			conatinerNames := cluster.GetDatabaseContainersName()
			for _, p := range pods.Items {
				for _, containerName := range conatinerNames {
					image, err := p.GetContainerImage(containerName)
					if err != nil {
						c.l.Debugf("failed to check pods for container image: %v", err)
						continue
					}
					images[image] = struct{}{}
				}
			}
			_, ok := images[crImage]
			return len(images) == 1 && ok
		}
		allClusterPodsReady := func(pods *common.PodList) bool {
			for _, p := range pods.Items {
				if !p.IsReady() {
					return false
				}
			}
			return true
		}
		type upgradeConditionFunc func(bool, bool) bool
		waitForUpgradeCondition := func(ctx context.Context, onWaitingDoneMessage string, done upgradeConditionFunc) error {
			for {
				time.Sleep(2 * time.Second)
				c.l.Info("waitForUpgradeCondition looping")
				cluster, err := getCluster(ctx, client.kubeCtl)
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						removeUpgradeTriggers()
						c.l.Info("waitForUpgradeCondition dedline exceeded")
						return err
					}
					continue
				}
				labels := cluster.GetDatabasePodLabels()
				pods, err := client.GetPods(ctx, labels...)
				if err != nil {
					c.l.Errorf("failed to get pods: %v", err)
					continue
				}
				match := crVersionAndPodsVersionMatch(ctx, pods, cluster)
				c.l.Infof("------ match %v, all pods rady %v", match, allClusterPodsReady(pods))
				if done(match, allClusterPodsReady(pods)) {
					c.l.Infof(onWaitingDoneMessage, cluster.GetCRDName(), cluster.GetName())
					break
				}
			}
			return nil
		}

		// TODO SHOW TOTAL AND CURRENT STEP TO PODS THAT HAVE THE SAME VERSION AS THE CR.

		err = waitForUpgradeCondition(ctx, "upgrade of cluster %v/%v has started",
			func(versionsMatch bool, podsReady bool) bool {
				return !versionsMatch && !podsReady
			},
		)
		if err != nil {
			c.l.Errorf("failed to wait for cluster %v/%v upgrade to start", cluster.GetCRDName(), cluster.GetName())
			return
		}

		err = waitForUpgradeCondition(ctx, "upgrade of cluster %v/%v is done",
			func(versionsMatch bool, podsReady bool) bool {
				return versionsMatch && podsReady
			},
		)
		if err != nil {
			c.l.Errorf("failed to wait for cluster %v/%v upgrade to finish", cluster.GetCRDName(), cluster.GetName())
			return
		}

		removeUpgradeTriggers()
	}(c.kubeCtl.GetKubeconfig(), cluster)

	return nil
}

// waitForClusterCondition waits until given done returns true.
func waitForClusterCondition(ctx context.Context, k *kubectl.KubeCtl, get getClusterFunc, done func(cluster common.DatabaseCluster) bool) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(2 * time.Second)
		cluster, err := get(ctx, k)
		if err != nil {
			return errors.Wrap(err, "failed to get database cluster")
		}
		if done(cluster) {
			return nil
		}
	}
}

// DeletePSMDBCluster deletes percona server for mongodb cluster with provided name.
func (c *K8sClient) DeletePSMDBCluster(ctx context.Context, name string) error {
	res := &psmdb.PerconaServerMongoDB{
		TypeMeta: common.TypeMeta{
			APIVersion: psmdbAPIVersion,
			Kind:       string(perconaServerMongoDBKind),
		},
		ObjectMeta: common.ObjectMeta{
			Name: name,
		},
	}
	err := c.kubeCtl.Delete(ctx, res)
	if err != nil {
		return errors.Wrap(err, "cannot delete PSMDB")
	}

	err = c.deleteSecret(ctx, fmt.Sprintf(psmdbSecretNameTmpl, name))
	if err != nil {
		c.l.Errorf("cannot delete secret for %s: %v", name, err)
	}

	psmdbInternalSecrets := []string{"internal-%s-users", "%s-ssl", "%s-ssl-internal", "%-mongodb-keyfile", "%s-mongodb-encryption-key"}

	for _, secretTmpl := range psmdbInternalSecrets {
		err = c.deleteSecret(ctx, fmt.Sprintf(secretTmpl, name))
		if err != nil {
			c.l.Errorf("cannot delete internal secret for %s: %v", name, err)
		}
	}

	return nil
}

// RestartPSMDBCluster restarts Percona server for mongodb cluster with provided name.
// FIXME: https://jira.percona.com/browse/PMM-6980
func (c *K8sClient) RestartPSMDBCluster(ctx context.Context, name string) error {
	_, err := c.kubeCtl.Run(ctx, c.restartDBClusterCmd(name, "rs0"), nil)

	return err
}

// GetPSMDBClusterCredentials returns a PSMDB cluster.
func (c *K8sClient) GetPSMDBClusterCredentials(ctx context.Context, name string) (*PSMDBCredentials, error) {
	var cluster psmdb.PerconaServerMongoDB
	err := c.kubeCtl.Get(ctx, string(perconaServerMongoDBKind), name, &cluster)
	if err != nil {
		if errors.Is(err, kubectl.ErrNotFound) {
			return nil, errors.Wrap(ErrNotFound, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
		}
		return nil, errors.Wrap(err, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
	}
	if cluster.Status == nil || cluster.Status.Status != psmdb.AppStateReady {
		return nil, errors.Wrap(ErrPSMDBClusterNotReady, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
	}

	password := ""
	username := ""
	var secret common.Secret
	// Retrieve secrets only for initializing or ready cluster.
	if cluster.Status.Status == psmdb.AppStateReady || cluster.Status.Status == psmdb.AppStateInit {
		err = c.kubeCtl.Get(ctx, k8sMetaKindSecret, fmt.Sprintf(psmdbSecretNameTmpl, name), &secret)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get PSMDB cluster secrets")
		}
		username = string(secret.Data["MONGODB_USER_ADMIN_USER"])
		password = string(secret.Data["MONGODB_USER_ADMIN_PASSWORD"])
	}

	credentials := &PSMDBCredentials{
		Username:   username,
		Password:   password,
		Host:       cluster.Status.Host,
		Port:       27017,
		Replicaset: "rs0",
	}

	return credentials, nil
}

// getPSMDBClusters returns Percona Server for MongoDB clusters.
func (c *K8sClient) getPSMDBClusters(ctx context.Context) ([]PSMDBCluster, error) {
	var list psmdb.PerconaServerMongoDBList
	err := c.kubeCtl.Get(ctx, string(perconaServerMongoDBKind), "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get percona server MongoDB clusters")
	}

	res := make([]PSMDBCluster, len(list.Items))
	for i, cluster := range list.Items {
		var message string
		var status []appStatus
		if cluster.Status != nil {
			message = cluster.Status.Message
			conditions := cluster.Status.Conditions
			if message == "" && len(conditions) > 0 {
				message = conditions[len(conditions)-1].Message
			}

			status = make([]appStatus, 0, len(cluster.Status.Replsets)+1)
			for _, rs := range cluster.Status.Replsets {
				status = append(status, appStatus{rs.Size, rs.Ready})
			}
			status = append(status, appStatus{
				size:  cluster.Status.Mongos.Size,
				ready: cluster.Status.Mongos.Ready,
			})
		}

		val := PSMDBCluster{
			Name:    cluster.Name,
			Size:    cluster.Spec.Replsets[0].Size,
			Pause:   cluster.Spec.Pause,
			Message: message,
			Replicaset: &Replicaset{
				DiskSize:         c.getDiskSize(cluster.Spec.Replsets[0].VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.Replsets[0].Resources),
			},
			Image:         cluster.Spec.Image,
			DetailedState: status,
			Exposed:       cluster.Spec.Sharding.Mongos.Expose.Enabled,
		}
		if cluster.Status != nil {
			if cluster.Spec.UpgradeOptions != nil {
				if _, err := version.NewVersion(cluster.Spec.UpgradeOptions.Apply); err == nil {
					val.State = ClusterStateUpgrading
				} else {
					val.State = psmdbStatesMap[cluster.Status.Status]
				}
			} else {
				val.State = getReplicasetStatus(cluster)
			}
		} else {
			val.State = ClusterStateInvalid
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
func getReplicasetStatus(cluster psmdb.PerconaServerMongoDB) ClusterState {
	if cluster.Status == nil {
		return ClusterStateInvalid
	}
	if strings.ToLower(string(cluster.Status.Status)) != string(psmdb.AppStateError) {
		return psmdbStatesMap[cluster.Status.Status]
	}

	if len(cluster.Status.Replsets) == 0 {
		return ClusterStateInvalid
	}

	// We shouldn't return ready state.
	status := ClusterStateFailed

	// We need to extract the lowest value so the first time, that's the lowest value.
	// Its is not possible to get the initial value in other way since cluster.Status.Replsets is a map
	// not an array.
	for _, replset := range cluster.Status.Replsets {
		replStatus := psmdbStatesMap[replset.Status]
		if replStatus < status {
			status = replStatus
		}
	}

	return status
}

// getDeletingXtraDBClusters returns Percona XtraDB clusters which are not fully deleted yet.
func (c *K8sClient) getDeletingPSMDBClusters(ctx context.Context, clusters []PSMDBCluster) ([]PSMDBCluster, error) {
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
			Name:          cluster.Name,
			Size:          0,
			State:         ClusterStateDeleting,
			Replicaset:    new(Replicaset),
			DetailedState: []appStatus{},
		}
	}
	return xtradbClusters, nil
}

func (c *K8sClient) getComputeResources(resources *common.PodResources) *ComputeResources {
	if resources == nil || resources.Limits == nil {
		return nil
	}
	res := new(ComputeResources)
	if resources.Limits.CPU != "" {
		res.CPUM = resources.Limits.CPU
	}
	if resources.Limits.Memory != "" {
		res.MemoryBytes = resources.Limits.Memory
	}
	return res
}

func (c *K8sClient) setComputeResources(res *ComputeResources) *common.PodResources {
	if res == nil {
		return nil
	}
	r := &common.PodResources{
		Limits: new(common.ResourcesList),
	}
	r.Limits.CPU = res.CPUM
	r.Limits.Memory = res.MemoryBytes
	return r
}

func (c *K8sClient) updateComputeResources(res *ComputeResources, podResources *common.PodResources) *common.PodResources {
	if res == nil {
		return podResources
	}
	if podResources == nil || podResources.Limits == nil {
		podResources = &common.PodResources{
			Limits: new(common.ResourcesList),
		}
	}

	podResources.Limits.CPU = res.CPUM
	podResources.Limits.Memory = res.MemoryBytes
	return podResources
}

func (c *K8sClient) getDiskSize(volumeSpec *common.VolumeSpec) string {
	if volumeSpec == nil || volumeSpec.PersistentVolumeClaim == nil {
		return "0"
	}
	quantity, ok := volumeSpec.PersistentVolumeClaim.Resources.Requests[common.ResourceStorage]
	if !ok {
		return "0"
	}
	return quantity
}

func (c *K8sClient) volumeSpec(diskSize string) *common.VolumeSpec {
	return &common.VolumeSpec{
		PersistentVolumeClaim: &common.PersistentVolumeClaimSpec{
			Resources: common.ResourceRequirements{
				Requests: common.ResourceList{
					common.ResourceStorage: diskSize,
				},
			},
		},
	}
}

// CheckOperators checks if operator installed and have required API version.
func (c *K8sClient) CheckOperators(ctx context.Context) (*Operators, error) {
	output, err := c.kubeCtl.Run(ctx, []string{"api-versions"}, "")
	if err != nil {
		return nil, errors.Wrap(err, "can't get api versions list")
	}

	apiVersions := strings.Split(string(output), "\n")

	return &Operators{
		Xtradb: c.checkOperatorStatus(apiVersions, pxcAPIVersion),
		Psmdb:  c.checkOperatorStatus(apiVersions, psmdbAPIVersion),
	}, nil
}

// checkOperatorStatus returns if operator is installed and operators version.
// It checks for all API versions supported by the operator and based on the latest API version in the list
// figures out which version of operator is installed.
func (c *K8sClient) checkOperatorStatus(installedVersions []string, expectedAPIVersion string) (operator Operator) {
	apiNamespace := strings.Split(expectedAPIVersion, "/")[0]
	operator.Status = OperatorStatusNotInstalled
	lastVersion, _ := version.NewVersion("v0.0.0")
	for _, apiVersion := range installedVersions {
		if !strings.HasPrefix(apiVersion, apiNamespace) {
			continue
		}
		if apiVersion == expectedAPIVersion {
			operator.Status = OperatorStatusOK
		}
		if operator.Status == OperatorStatusNotInstalled {
			operator.Status = OperatorStatusUnsupported
		}
		v := strings.Split(apiVersion, "/")[1]

		versionParts := strings.Split(v, "-")
		if len(versionParts) != 3 {
			continue
		}
		v = strings.Join(versionParts, ".")
		newVersion, err := version.NewVersion(v)
		if err != nil {
			c.l.Warn("can't parse version %s: %s", v, err)
			continue
		}
		if newVersion.LessThanOrEqual(lastVersion) {
			continue
		}
		lastVersion = newVersion
		operator.Version = lastVersion.String()
	}
	return operator
}

// sumVolumesSize returns sum of persistent volumes storage size in bytes.
func sumVolumesSize(pvs *common.PersistentVolumeList) (sum uint64, err error) {
	for _, pv := range pvs.Items {
		bytes, err := convertors.StrToBytes(pv.Spec.Capacity.Storage)
		if err != nil {
			return 0, err
		}
		sum += bytes
	}
	return
}

// TODO USE CHECK.PERCONA.COM INSTEAD OF CHECK.PERCONA/VERSIONS/V1 FOR INITAL VALUE OF VERSION SERVICE ENDPOINT

// GetPersistentVolumes returns list of persistent volumes.
func (c *K8sClient) GetPersistentVolumes(ctx context.Context) (*common.PersistentVolumeList, error) {
	list := new(common.PersistentVolumeList)
	args := []string{"get", "pv", "-ojson"}
	out, err := c.kubeCtl.Run(ctx, args, nil)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get persistent volumes")
	}
	err = json.Unmarshal(out, list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get persistent volumes")
	}
	return list, nil
}

// GetPods returns list of pods based on given filters. Filters are args to
// kubectl command. For example "-lyour-label=value", "-ntest-namespace".
func (c *K8sClient) GetPods(ctx context.Context, filters ...string) (*common.PodList, error) {
	list := new(common.PodList)
	args := []string{"get", "pods"}
	args = append(args, filters...)
	args = append(args, "-ojson")
	out, err := c.kubeCtl.Run(ctx, args, nil)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get kubernetes pods")
	}

	err = json.Unmarshal(out, list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get kubernetes pods")
	}
	return list, nil
}

// GetLogs returns logs as slice of log lines - strings - for given pod's container.
func (c *K8sClient) GetLogs(
	ctx context.Context,
	containerStatuses []common.ContainerStatus,
	pod,
	container string,
) ([]string, error) {
	if common.IsContainerInState(containerStatuses, common.ContainerStateWaiting, container) {
		return []string{}, nil
	}
	stdout, err := c.kubeCtl.Run(ctx, []string{"logs", pod, container}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get logs")
	}
	if string(stdout) == "" {
		return []string{}, nil
	}
	return strings.Split(string(stdout), "\n"), nil
}

// GetEvents returns pod's events as a slice of strings.
func (c *K8sClient) GetEvents(ctx context.Context, pod string) ([]string, error) {
	stdout, err := c.kubeCtl.Run(ctx, []string{"describe", "pod", pod}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't describe pod")
	}
	lines := strings.Split(string(stdout), "\n")
	var line string
	var i int
	for i, line = range lines {
		if strings.Contains(line, "Events") {
			break
		}
	}
	// Add name of the pod to the Events line so it's clear what pod events we got.
	lines[i] = pod + " " + lines[i]
	return lines[i:], nil
}

// getWorkerNodes returns list of cluster workers nodes.
func (c *K8sClient) getWorkerNodes(ctx context.Context) ([]common.Node, error) {
	nodes := new(common.NodeList)
	out, err := c.kubeCtl.Run(ctx, []string{"get", "nodes", "-ojson"}, nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not get nodes of Kubernetes cluster")
	}
	err = json.Unmarshal(out, nodes)
	if err != nil {
		return nil, errors.Wrap(err, "could not get nodes of Kubernetes cluster")
	}
	forbidenTaints := map[string]string{
		"node.cloudprovider.kubernetes.io/uninitialized": "NoSchedule",
		"node.kubernetes.io/unschedulable":               "NoSchedule",
		"node-role.kubernetes.io/master":                 "NoSchedule",
	}
	workers := make([]common.Node, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		if len(node.Spec.Taints) == 0 {
			workers = append(workers, node)
			continue
		}
		for _, taint := range node.Spec.Taints {
			effect, keyFound := forbidenTaints[taint.Key]
			if !keyFound || effect != taint.Effect {
				workers = append(workers, node)
			}
		}
	}
	return workers, nil
}

// GetAllClusterResources goes through all cluster nodes and sums their allocatable resources.
func (c *K8sClient) GetAllClusterResources(ctx context.Context, clusterType KubernetesClusterType, volumes *common.PersistentVolumeList) (
	cpuMillis uint64, memoryBytes uint64, diskSizeBytes uint64, err error,
) {
	nodes, err := c.getWorkerNodes(ctx)
	if err != nil {
		return 0, 0, 0, errors.Wrap(err, "could not get a list of nodes")
	}
	var volumeCountEKS uint64
	for _, node := range nodes {
		cpu, memory, err := getResources(node.Status.Allocatable)
		if err != nil {
			return 0, 0, 0, errors.Wrap(err, "could not get allocatable resources of the node")
		}
		cpuMillis += cpu
		memoryBytes += memory

		switch clusterType {
		case MinikubeClusterType:
			storage, ok := node.Status.Allocatable[common.ResourceEphemeralStorage]
			if !ok {
				return 0, 0, 0, errors.Errorf("could not get storage size of the node")
			}
			bytes, err := convertors.StrToBytes(storage)
			if err != nil {
				return 0, 0, 0, errors.Wrapf(err, "could not convert storage size '%s' to bytes", storage)
			}
			diskSizeBytes += bytes
		case AmazonEKSClusterType:
			// See https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/#scheduler.
			if common.IsNodeInCondition(node, common.NodeConditionDiskPressure) {
				continue
			}

			// Get nodes's type.
			nodeType, ok := node.Labels["beta.kubernetes.io/instance-type"]
			if !ok {
				return 0, 0, 0, errors.New("dealing with AWS EKS cluster but the node does not have label 'beta.kubernetes.io/instance-type'")
			}
			// 39 is a default limit for EKS cluster nodes ...
			var volumeLimitPerNode uint64 = 39
			typeAndSize := strings.Split(strings.ToLower(nodeType), ".")
			if len(typeAndSize) < 2 {
				return 0, 0, 0, errors.Errorf("failed to parse EKS node type '%s', it's not in expected format 'type.size'", nodeType)
			}
			// ... however, if the node type is one of M5, C5, R5, T3, Z1D it's 25.
			limitedVolumesSet := map[string]struct{}{
				"m5": {}, "c5": {}, "r5": {}, "t3": {}, "t1d": {},
			}
			if _, ok := limitedVolumesSet[typeAndSize[0]]; ok {
				volumeLimitPerNode = 25
			}
			volumeCountEKS += volumeLimitPerNode
		}
	}
	if clusterType == AmazonEKSClusterType {
		volumeCountEKSBackup := volumeCountEKS
		volumeCountEKS -= uint64(len(volumes.Items))
		if volumeCountEKS > volumeCountEKSBackup {
			// handle uint underflow
			volumeCountEKS = 0
		}

		consumedBytes, err := sumVolumesSize(volumes)
		if err != nil {
			return 0, 0, 0, errors.Wrap(err, "failed to sum persistent volumes storage sizes")
		}
		diskSizeBytes = (volumeCountEKS * maxVolumeSizeEBS) + consumedBytes
	}
	return cpuMillis, memoryBytes, diskSizeBytes, nil
}

// getResources extracts resources out of common.ResourceList and converts them to int64 values.
// Millicpus are used for CPU values and bytes for memory.
func getResources(resources common.ResourceList) (cpuMillis uint64, memoryBytes uint64, err error) {
	cpu, ok := resources[common.ResourceCPU]
	if ok {
		cpuMillis, err = convertors.StrToMilliCPU(cpu)
		if err != nil {
			return 0, 0, errors.Wrapf(err, "failed to convert '%s' to millicpus", cpu)
		}
	}
	memory, ok := resources[common.ResourceMemory]
	if ok {
		memoryBytes, err = convertors.StrToBytes(memory)
		if err != nil {
			return 0, 0, errors.Wrapf(err, "failed to convert '%s' to bytes", memory)
		}
	}
	return cpuMillis, memoryBytes, nil
}

// GetConsumedCPUAndMemory returns consumed CPU and Memory in given namespace. If namespaces
// is empty slice, it tries to get them from all namespaces.
func (c *K8sClient) GetConsumedCPUAndMemory(ctx context.Context, namespaces ...string) (
	cpuMillis uint64, memoryBytes uint64, err error,
) {
	// Get CPU and Memory Requests of Pods' containers.
	if len(namespaces) == 0 {
		namespaces = []string{"--all-namespaces"}
	} else {
		for i := 0; i < len(namespaces); i++ {
			namespaces[i] = "-n" + namespaces[i]
		}
	}

	pods, err := c.GetPods(ctx, namespaces...)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get consumed resources")
	}
	for _, ppod := range pods.Items {
		if ppod.Status.Phase != common.PodPhaseRunning {
			continue
		}
		nonTerminatedInitContainers := make([]common.ContainerSpec, 0, len(ppod.Spec.InitContainers))
		for _, container := range ppod.Spec.InitContainers {
			if !common.IsContainerInState(
				ppod.Status.InitContainerStatuses, common.ContainerStateTerminated, container.Name,
			) {
				nonTerminatedInitContainers = append(nonTerminatedInitContainers, container)
			}
		}
		for _, container := range append(ppod.Spec.Containers, nonTerminatedInitContainers...) {
			cpu, memory, err := getResources(container.Resources.Requests)
			if err != nil {
				return 0, 0, errors.Wrap(err, "failed to sum all consumed resources")
			}
			cpuMillis += cpu
			memoryBytes += memory
		}
	}

	return cpuMillis, memoryBytes, nil
}

// GetConsumedDiskBytes returns consumed bytes. The strategy differs based on k8s cluster type.
func (c *K8sClient) GetConsumedDiskBytes(ctx context.Context, clusterType KubernetesClusterType, volumes *common.PersistentVolumeList) (consumedBytes uint64, err error) {
	switch clusterType {
	case MinikubeClusterType:
		nodes, err := c.getWorkerNodes(ctx)
		if err != nil {
			return 0, errors.Wrap(err, "can't compute consumed disk size: failed to get worker nodes")
		}
		clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(c.kubeconfig))
		if err != nil {
			return 0, errors.Wrap(err, "failed to build kubeconfig out of given path")
		}
		config, err := clientConfig.ClientConfig()
		if err != nil {
			return 0, errors.Wrap(err, "failed to build kubeconfig out of given path")
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return 0, errors.Wrap(err, "failed to build client out of submited kubeconfig")
		}
		for _, node := range nodes {
			var summary common.NodeSummary
			request := clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(node.Name).SubResource("proxy").Suffix("stats/summary")
			responseRawArrayOfBytes, err := request.DoRaw(context.Background())
			if err != nil {
				return 0, errors.Wrap(err, "failed to get stats from node")
			}
			if err := json.Unmarshal(responseRawArrayOfBytes, &summary); err != nil {
				return 0, errors.Wrap(err, "failed to unmarshal response from kubernetes API")
			}
			consumedBytes += summary.Node.FileSystem.UsedBytes
		}
		return consumedBytes, nil
	case AmazonEKSClusterType:
		consumedBytes, err := sumVolumesSize(volumes)
		if err != nil {
			return 0, errors.Wrap(err, "failed to sum persistent volumes storage sizes")
		}
		return consumedBytes, nil
	}

	return 0, nil
}

func (c *K8sClient) InstallXtraDBOperator(ctx context.Context) error {
	file, err := dbaascontroller.DeployDir.ReadFile("deploy/pxc-operator.yaml")
	if err != nil {
		return err
	}
	return c.kubeCtl.Apply(ctx, file)
}

func (c *K8sClient) InstallPSMDBOperator(ctx context.Context) error {
	file, err := dbaascontroller.DeployDir.ReadFile("deploy/psmdb-operator.yaml")
	if err != nil {
		return err
	}
	return c.kubeCtl.Apply(ctx, file)
}

func (c *K8sClient) CreateVMOperator(ctx context.Context, params *PMM) error {
	files := []string{
		"deploy/victoriametrics/crds/crd.yaml",
		"deploy/victoriametrics/operator/manager.yaml",
		"deploy/victoriametrics/operator/rbac.yaml",
		"deploy/victoriametrics/crs/vmagent_rbac.yaml",
		"deploy/victoriametrics/crs/vmnodescrape.yaml",
		"deploy/victoriametrics/crs/vmpodscrape.yaml",
	}
	for _, path := range files {
		file, err := dbaascontroller.DeployDir.ReadFile(path)
		if err != nil {
			return err
		}
		err = c.kubeCtl.Apply(ctx, file)
		if err != nil {
			return err
		}
	}

	randomCrypto, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		return err
	}

	secretName := fmt.Sprintf("victoria-metrics-operator-%d", randomCrypto)
	err = c.CreateSecret(ctx, secretName, map[string][]byte{
		"username": []byte(params.Login),
		"password": []byte(params.Password),
	})
	if err != nil {
		return err
	}

	vmagent := vmAgentSpec(params, secretName)
	return c.kubeCtl.Apply(ctx, vmagent)
}

func vmAgentSpec(params *PMM, secretName string) monitoring.VMAgent {
	return monitoring.VMAgent{
		TypeMeta: common.TypeMeta{
			Kind:       "VMAgent",
			APIVersion: "operator.victoriametrics.com/v1beta1",
		},
		ObjectMeta: common.ObjectMeta{
			Name: "pmm-vmagent-" + secretName,
		},
		Spec: monitoring.VMAgentSpec{
			ServiceScrapeNamespaceSelector: new(common.LabelSelector),
			ServiceScrapeSelector:          new(common.LabelSelector),
			PodScrapeNamespaceSelector:     new(common.LabelSelector),
			PodScrapeSelector:              new(common.LabelSelector),
			ProbeSelector:                  new(common.LabelSelector),
			ProbeNamespaceSelector:         new(common.LabelSelector),
			StaticScrapeSelector:           new(common.LabelSelector),
			StaticScrapeNamespaceSelector:  new(common.LabelSelector),
			ReplicaCount:                   1,
			Resources: &common.PodResources{
				Requests: &common.ResourcesList{
					CPU:    "250m",
					Memory: "350Mi",
				},
				Limits: &common.ResourcesList{
					CPU:    "500m",
					Memory: "850Mi",
				},
			},
			AdditionalArgs: map[string]string{
				"memory.allowedPercent": "40",
			},
			RemoteWrite: []monitoring.VMAgentRemoteWriteSpec{
				{
					URL:       fmt.Sprintf("%s/victoriametrics/api/v1/write", params.PublicAddress),
					TLSConfig: &monitoring.TLSConfig{InsecureSkipVerify: true},
					BasicAuth: &monitoring.BasicAuth{
						Username: common.SecretKeySelector{
							LocalObjectReference: common.LocalObjectReference{
								Name: secretName,
							},
							Key: "username",
						},
						Password: common.SecretKeySelector{
							LocalObjectReference: common.LocalObjectReference{
								Name: secretName,
							},
							Key: "password",
						},
					},
				},
			},
		},
	}
}
