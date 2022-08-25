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
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	goversion "github.com/hashicorp/go-version"
	pmmversion "github.com/percona/pmm/version"
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
	// ClusterStatePaused represents a paused cluster state.
	ClusterStatePaused ClusterState = 5
	// ClusterStateUpgrading represents a state of a cluster that is undergoing an upgrade.
	ClusterStateUpgrading ClusterState = 6
)

const (
	k8sAPIVersion     = "v1"
	k8sMetaKindSecret = "Secret"

	pxcBackupImageTemplate          = "percona/percona-xtradb-cluster-operator:%s-pxc8.0-backup"
	pxcDefaultImage                 = "percona/percona-xtradb-cluster:8.0.20-11.1"
	pxcBackupStorageName            = "pxc-backup-storage-%s"
	pxcAPINamespace                 = "pxc.percona.com"
	pxcAPIVersionTemplate           = pxcAPINamespace + "/v%s"
	pxcProxySQLDefaultImageTemplate = "percona/percona-xtradb-cluster-operator:%s-proxysql"
	pxcHAProxyDefaultImageTemplate  = "percona/percona-xtradb-cluster-operator:%s-haproxy"
	pxcSecretNameTmpl               = "dbaas-%s-pxc-secrets" //nolint:gosec
	pxcInternalSecretTmpl           = "internal-%s"

	psmdbBackupImageTemplate = "percona/percona-server-mongodb-operator:%s-backup"
	psmdbDefaultImage        = "percona/percona-server-mongodb:4.2.8-8"
	psmdbAPINamespace        = "psmdb.percona.com"
	psmdbAPIVersionTemplate  = psmdbAPINamespace + "/v%s"
	psmdbSecretNameTmpl      = "dbaas-%s-psmdb-secrets" //nolint:gosec
	stabePMMClientImage      = "percona/pmm-client:2"

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

const (
	clusterWithSameNameExistsErrTemplate = "Cluster '%s' already exists"
	canNotGetCredentialsErrTemplate      = "cannot get %s cluster credentials" //nolint:gosec
)

// Operator represents kubernetes operator.
type Operator struct {
	// If version is empty, operator is not installed.
	Version string
}

// Operators contains versions of installed operators.
// If version is empty, operator is not installed.
type Operators struct {
	PXCOperatorVersion   string
	PsmdbOperatorVersion string
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

// PXCParams contains all parameters required to create or update Percona XtraDB cluster.
type PXCParams struct {
	Name              string
	Size              int32
	Suspend           bool
	Resume            bool
	Expose            bool
	VersionServiceURL string
	PXC               *PXC
	ProxySQL          *ProxySQL
	PMM               *PMM
	HAProxy           *HAProxy
}

// Cluster contains common information related to cluster.
type Cluster struct {
	Name string
}

// PSMDBParams contains all parameters required to create or update percona server for mongodb cluster.
type PSMDBParams struct {
	Name              string
	Image             string
	BackupImage       string
	VersionServiceURL string
	Size              int32
	Suspend           bool
	Resume            bool
	Expose            bool
	Replicaset        *Replicaset
	PMM               *PMM
}

type appStatus struct {
	size  int32
	ready int32
}

// DetailedState contains pods' status.
type DetailedState []appStatus

// PXCCluster contains information related to pxc cluster.
type PXCCluster struct {
	Name          string
	Message       string
	Size          int32
	Pause         bool
	Exposed       bool
	State         ClusterState
	DetailedState DetailedState
	PXC           *PXC
	ProxySQL      *ProxySQL
	HAProxy       *HAProxy
}

// PSMDBCluster contains information related to psmdb cluster.
type PSMDBCluster struct {
	Name          string
	Image         string
	Message       string
	Size          int32
	Pause         bool
	Exposed       bool
	State         ClusterState
	DetailedState DetailedState
	Replicaset    *Replicaset
}

// PSMDBCredentials represents PSMDB connection credentials.
type PSMDBCredentials struct {
	Username   string
	Password   string
	Host       string
	Port       int32
	Replicaset string
}

// PXCCredentials represents PXC connection credentials.
type PXCCredentials struct {
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

type extraCRParams struct {
	secretName  string
	secrets     map[string][]byte
	psmdbImage  string
	backupImage string
	affinity    *psmdb.PodAffinity
	expose      psmdb.Expose
	operators   *Operators
}

// clustertatesMap matches pxc and psmdb app states to cluster states.
var clusterStatesMap = map[common.AppState]ClusterState{ //nolint:gochecknoglobals
	common.AppStateInit:     ClusterStateChanging,
	common.AppStateReady:    ClusterStateReady,
	common.AppStateError:    ClusterStateFailed,
	common.AppStatePaused:   ClusterStatePaused,
	common.AppStateStopping: ClusterStateChanging,
}

var (
	// ErrPXCClusterStateUnexpected The PXC cluster is not in desired state.
	ErrPXCClusterStateUnexpected = errors.New("PXC cluster state is not as expected")
	// ErrPSMDBClusterNotReady The PSMDB cluster is not ready.
	ErrPSMDBClusterNotReady = errors.New("PSMDB cluster is not ready")
	// ErrNotFound should be returned when referenced resource does not exist
	// inside Kubernetes cluster.
	ErrNotFound error = errors.New("resource was not found in Kubernetes cluster")
	// ErrEmptyResponse is a sentinel error to state it is not possible to get the CR version
	// since the response was empty.
	ErrEmptyResponse = errors.New("cannot get the CR version. Empty response")
	// v112 used to select the correct structure for different operator versions.
	v112, _ = goversion.NewVersion("1.12") //nolint:gochecknoglobals
)

var pmmClientImage string

// K8sClient is a client for Kubernetes.
type K8sClient struct {
	kubeCtl    *kubectl.KubeCtl
	l          logger.Logger
	kubeconfig string
	client     *http.Client
}

func init() {
	pmmClientImage = "perconalab/pmm-client:dev-latest"

	pmmClientImageEnv, ok := os.LookupEnv("PERCONA_TEST_DBAAS_PMM_CLIENT")
	if ok {
		pmmClientImage = pmmClientImageEnv
		return
	}

	if pmmversion.PMMVersion == "" { // No version set, use dev-latest.
		return
	}

	v, err := goversion.NewVersion(pmmversion.PMMVersion)
	if err != nil {
		logger.Get(context.Background()).Warnf("failed to decide what version of pmm-client to use: %s", err)
		logger.Get(context.Background()).Warnf("Using %q for pmm client image", pmmClientImage)
		return
	}
	// if version has a suffix like 1.2.0-dev or 3.4.1-HEAD-something it is an unreleased version.
	// Docker image won't exist in the repo so use latest stable.
	if v.Core().String() != v.String() {
		pmmClientImage = stabePMMClientImage
		return
	}

	pmmClientImage = "percona/pmm-client:" + v.Core().String()
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
		kubeCtl: kubeCtl,
		l:       l,
		client: &http.Client{
			Timeout: time.Second * 5,
			Transport: &http.Transport{
				MaxIdleConns:    1,
				IdleConnTimeout: 10 * time.Second,
			},
		},
		kubeconfig: kubeconfig,
	}, nil
}

// Cleanup removes temporary files created by that object.
func (c *K8sClient) Cleanup() error {
	return c.kubeCtl.Cleanup()
}

// ListPXCClusters returns list of Percona XtraDB clusters and their statuses.
func (c *K8sClient) ListPXCClusters(ctx context.Context) ([]PXCCluster, error) {
	perconaXtraDBClusters, err := c.getPerconaXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}

	deletingClusters, err := c.getDeletingPXCClusters(ctx, perconaXtraDBClusters)
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

// CreatePXCCluster creates Percona XtraDB cluster with provided parameters.
func (c *K8sClient) CreatePXCCluster(ctx context.Context, params *PXCParams) error {
	if (params.ProxySQL != nil) == (params.HAProxy != nil) {
		return errors.New("pxc cluster must have one and only one proxy type defined")
	}

	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, pxc.PerconaXtraDBClusterKind, params.Name, &cluster)
	if err == nil {
		return fmt.Errorf(clusterWithSameNameExistsErrTemplate, params.Name)
	}

	secretName := fmt.Sprintf(pxcSecretNameTmpl, params.Name)
	secrets, err := generatePXCPasswords()
	if err != nil {
		return err
	}

	storageName := fmt.Sprintf(pxcBackupStorageName, params.Name)

	operators, err := c.CheckOperators(ctx)
	if err != nil {
		return err
	}

	pxcImage := pxcDefaultImage
	if params.PXC.Image != "" {
		pxcImage = params.PXC.Image
	}

	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: common.TypeMeta{
			APIVersion: c.getAPIVersionForPXCOperator(operators.PXCOperatorVersion),
			Kind:       pxc.PerconaXtraDBClusterKind,
		},
		ObjectMeta: common.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-proxysql-pvc", "delete-pxc-pvc"},
		},
		Spec: &pxc.PerconaXtraDBClusterSpec{
			UpdateStrategy:    updateStrategyRollingUpdate,
			CRVersion:         operators.PXCOperatorVersion,
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
				Image: fmt.Sprintf(pxcBackupImageTemplate, operators.PXCOperatorVersion),
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
					Memory: "300M",
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
		podSpec.Image = fmt.Sprintf(pxcProxySQLDefaultImageTemplate, operators.PXCOperatorVersion)
		if params.ProxySQL.Image != "" {
			podSpec.Image = params.ProxySQL.Image
		}
		podSpec.Resources = c.setComputeResources(params.ProxySQL.ComputeResources)
		podSpec.VolumeSpec = c.volumeSpec(params.ProxySQL.DiskSize)
	} else {
		res.Spec.HAProxy = new(pxc.PodSpec)
		podSpec = res.Spec.HAProxy
		podSpec.Image = fmt.Sprintf(pxcHAProxyDefaultImageTemplate, operators.PXCOperatorVersion)
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

// UpdatePXCCluster changes size of provided Percona XtraDB cluster.
func (c *K8sClient) UpdatePXCCluster(ctx context.Context, params *PXCParams) error {
	if (params.ProxySQL != nil) && (params.HAProxy != nil) {
		return errors.New("can't update both proxies, only one should be in use")
	}

	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, pxc.PerconaXtraDBClusterKind, params.Name, &cluster)
	if err != nil {
		return err
	}

	clusterState := c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)

	// Only if cluster is paused, allow resuming it. All other modifications are forbinden.
	if params.Resume && clusterState == ClusterStatePaused {
		cluster.Spec.Pause = false
		return c.kubeCtl.Apply(ctx, &cluster)
	}

	// This is to prevent concurrent updates
	if clusterState != ClusterStateReady {
		return errors.Wrapf(ErrPXCClusterStateUnexpected, "state is %v", cluster.Status.Status) //nolint:wrapcheck
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
		if params.PXC.Image != "" && params.PXC.Image != cluster.Spec.PXC.Image {
			// Let's upgrade the cluster.
			err = c.changeImageInCluster(&cluster, params.PXC.Image)
			if err != nil {
				return err
			}
		}
	}

	if params.ProxySQL != nil {
		cluster.Spec.ProxySQL.Resources = c.updateComputeResources(params.ProxySQL.ComputeResources, cluster.Spec.ProxySQL.Resources)
	}

	if params.HAProxy != nil {
		cluster.Spec.HAProxy.Resources = c.updateComputeResources(params.HAProxy.ComputeResources, cluster.Spec.HAProxy.Resources)
	}

	return c.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, common.DatabaseCluster(&cluster).CRDName(), common.DatabaseCluster(&cluster).GetName(), cluster)
}

// DeletePXCCluster deletes Percona XtraDB cluster with provided name.
func (c *K8sClient) DeletePXCCluster(ctx context.Context, name string) error {
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: common.TypeMeta{
			APIVersion: pxcAPINamespace + "/v1",
			Kind:       pxc.PerconaXtraDBClusterKind,
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

// GetPXCClusterCredentials returns an PXC cluster credentials.
func (c *K8sClient) GetPXCClusterCredentials(ctx context.Context, name string) (*PXCCredentials, error) {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, pxc.PerconaXtraDBClusterKind, name, &cluster)
	if err != nil {
		if errors.Is(err, kubectl.ErrNotFound) {
			return nil, errors.Wrap(ErrNotFound, fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"))
		}
		return nil, errors.Wrap(err, fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"))
	}

	clusterState := c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)
	if clusterState != ClusterStateReady && clusterState != ClusterStateChanging {
		return nil, errors.Wrapf(
			errors.Wrap(ErrPXCClusterStateUnexpected,
				fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"),
			),
			"cluster state is state %v, %v or %v is expected",
			clusterState,
			ClusterStateReady,
			ClusterStateChanging,
		)
	}

	var secret common.Secret
	err = c.kubeCtl.Get(ctx, k8sMetaKindSecret, fmt.Sprintf(pxcSecretNameTmpl, name), &secret)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get XtraDb cluster secrets")
	}
	password := string(secret.Data["root"])

	credentials := &PXCCredentials{
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

// RestartPXCCluster restarts Percona XtraDB cluster with provided name.
// FIXME: https://jira.percona.com/browse/PMM-6980
func (c *K8sClient) RestartPXCCluster(ctx context.Context, name string) error {
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
func (c *K8sClient) getPerconaXtraDBClusters(ctx context.Context) ([]PXCCluster, error) {
	var list pxc.PerconaXtraDBClusterList
	err := c.kubeCtl.Get(ctx, pxc.PerconaXtraDBClusterKind, "", &list)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get Percona XtraDB clusters")
	}

	res := make([]PXCCluster, len(list.Items))
	for i, cluster := range list.Items {
		val := PXCCluster{
			Name: cluster.Name,
			Size: *cluster.Spec.PXC.Size,
			PXC: &PXC{
				Image:            cluster.Spec.PXC.Image,
				DiskSize:         c.getDiskSize(cluster.Spec.PXC.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.PXC.Resources),
			},
			Pause: cluster.Spec.Pause,
		}
		if cluster.Status != nil {
			val.DetailedState = []appStatus{
				{size: cluster.Status.PMM.Size, ready: cluster.Status.PMM.Ready},
				{size: cluster.Status.HAProxy.Size, ready: cluster.Status.HAProxy.Ready},
				{size: cluster.Status.ProxySQL.Size, ready: cluster.Status.ProxySQL.Ready},
				{size: cluster.Status.PXC.Size, ready: cluster.Status.PXC.Ready},
			}
			val.Message = strings.Join(cluster.Status.Messages, ";")
		}

		val.State = c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)

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

func (c *K8sClient) getClusterState(ctx context.Context, cluster common.DatabaseCluster, crAndPodsMatchFunc func(context.Context, common.DatabaseCluster) (bool, error)) ClusterState {
	if cluster == nil {
		return ClusterStateInvalid
	}
	state := cluster.State()
	if state == common.AppStateUnknown {
		return ClusterStateInvalid
	}
	// Handle paused state for operator version >= 1.9.0 and for operator version <= 1.8.0.
	if state == common.AppStatePaused || (cluster.Pause() && state == common.AppStateReady) {
		return ClusterStatePaused
	}

	clusterState, ok := clusterStatesMap[state]
	if !ok {
		c.l.Warnf("failed to recognize cluster state: %q, setting status to ClusterStateChanging", state)
		return ClusterStateChanging
	}
	if clusterState == ClusterStateChanging {
		// Check if cr and pods version matches.
		match, err := crAndPodsMatchFunc(ctx, cluster)
		if err != nil {
			c.l.Warnf("failed to check if cluster %q is upgrading: %v", cluster.GetName(), err)
			return ClusterStateInvalid
		}
		if match {
			return ClusterStateChanging
		}
		return ClusterStateUpgrading
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

// getDeletingPXCClusters returns Percona XtraDB clusters which are not fully deleted yet.
func (c *K8sClient) getDeletingPXCClusters(ctx context.Context, clusters []PXCCluster) ([]PXCCluster, error) {
	runningClusters := make(map[string]struct{}, len(clusters))
	for _, cluster := range clusters {
		runningClusters[cluster.Name] = struct{}{}
	}

	deletingClusters, err := c.getDeletingClusters(ctx, "percona-xtradb-cluster-operator", runningClusters)
	if err != nil {
		return nil, err
	}

	pxcClusters := make([]PXCCluster, len(deletingClusters))
	for i, cluster := range deletingClusters {
		pxcClusters[i] = PXCCluster{
			Name:          cluster.Name,
			Size:          0,
			State:         ClusterStateDeleting,
			PXC:           new(PXC),
			ProxySQL:      new(ProxySQL),
			HAProxy:       new(HAProxy),
			DetailedState: []appStatus{},
		}
	}
	return pxcClusters, nil
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
	err := c.kubeCtl.Get(ctx, psmdb.PerconaServerMongoDBKind, params.Name, &cluster)
	if err == nil {
		return fmt.Errorf(clusterWithSameNameExistsErrTemplate, params.Name)
	}

	extra := extraCRParams{}
	extra.secretName = fmt.Sprintf(psmdbSecretNameTmpl, params.Name)
	extra.secrets, err = generatePSMDBPasswords()
	if err != nil {
		return err
	}

	extra.affinity = new(psmdb.PodAffinity)
	if clusterType := c.GetKubernetesClusterType(ctx); clusterType != MinikubeClusterType {
		extra.affinity.TopologyKey = pointer.ToString("kubernetes.io/hostname")

		if params.Expose {
			// This enables ingress for the cluster and exposes the cluster to the world.
			// The cluster will have an internal IP and a world accessible hostname.
			// This feature cannot be tested with minikube. Please use EKS for testing.
			extra.expose = psmdb.Expose{
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
		extra.affinity.TopologyKey = pointer.ToString(psmdb.AffinityOff)
	}

	extra.operators, err = c.CheckOperators(ctx)
	if err != nil {
		return err
	}

	psmdbOperatorVersion, err := goversion.NewVersion(extra.operators.PsmdbOperatorVersion)
	if err != nil {
		return errors.Wrap(err, "cannot get the PSMDB operator version")
	}

	extra.psmdbImage = psmdbDefaultImage
	if params.Image != "" {
		extra.psmdbImage = params.Image
	}

	// Starting with operator 1.12, the image name doesn't follow a template rule anymore.
	// That's why it should be obtained from the components service and passed as a parameter.
	// If it is empty, try the old format using
	extra.backupImage = params.BackupImage
	if extra.backupImage == "" {
		extra.backupImage = fmt.Sprintf(psmdbBackupImageTemplate, extra.operators.PsmdbOperatorVersion)
	}

	var res interface{}

	switch {
	case psmdbOperatorVersion.GreaterThanOrEqual(v112):
		res = c.makeReq112Plus(params, extra)
	default:
		res = c.makeReq(params, extra)
	}

	if params.PMM != nil {
		extra.secrets["PMM_SERVER_USER"] = []byte(params.PMM.Login)
		extra.secrets["PMM_SERVER_PASSWORD"] = []byte(params.PMM.Password)
	}

	err = c.CreateSecret(ctx, extra.secretName, extra.secrets)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}

	return c.kubeCtl.Apply(ctx, res)
}

// UpdatePSMDBCluster changes size, stops, resumes or upgrades provided percona server for mongodb cluster.
func (c *K8sClient) UpdatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	var cluster psmdb.PerconaServerMongoDB
	err := c.kubeCtl.Get(ctx, psmdb.PerconaServerMongoDBKind, params.Name, &cluster)
	if err != nil {
		return err
	}

	clusterState := c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)
	if params.Resume && clusterState == ClusterStatePaused {
		cluster.Spec.Pause = false
		return c.kubeCtl.Apply(ctx, &cluster)
	}

	// This is to prevent concurrent updates
	if clusterState != ClusterStateReady {
		return errors.Wrap(ErrPSMDBClusterNotReady, "cluster is not in ready state") //nolint:wrapcheck
	}
	if params.Size > 0 {
		cluster.Spec.Replsets[0].Size = params.Size
	}

	if params.Suspend {
		cluster.Spec.Pause = true
	}

	if params.Replicaset != nil {
		cluster.Spec.Replsets[0].Resources = c.updateComputeResources(params.Replicaset.ComputeResources, cluster.Spec.Replsets[0].Resources)
	}
	if params.Image != "" && params.Image != cluster.Spec.Image {
		// We want to upgrade the cluster.
		err = c.changeImageInCluster(&cluster, params.Image)
		if err != nil {
			return err
		}
	}
	return c.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, common.DatabaseCluster(&cluster).CRDName(), common.DatabaseCluster(&cluster).GetName(), cluster)
}

const (
	updateStrategyRollingUpdate = "RollingUpdate"
)

func (c *K8sClient) changeImageInCluster(cluster common.DatabaseCluster, newImage string) error {
	// Check that only tag changed.
	newImageAndTag := strings.Split(newImage, ":")
	if len(newImageAndTag) != 2 {
		return errors.New("image has to have version tag")
	}
	currentImageAndTag := strings.Split(cluster.DatabaseImage(), ":")
	if currentImageAndTag[0] != newImageAndTag[0] {
		return errors.Errorf("expected image is %q, %q was given", currentImageAndTag[0], newImageAndTag[0])
	}
	if currentImageAndTag[1] == newImageAndTag[1] {
		return errors.Errorf("failed to change image: the database version %q is already in use", newImageAndTag[1])
	}

	cluster.SetDatabaseImage(newImage)
	return nil
}

// DeletePSMDBCluster deletes percona server for mongodb cluster with provided name.
func (c *K8sClient) DeletePSMDBCluster(ctx context.Context, name string) error {
	res := &psmdb.PerconaServerMongoDB{
		TypeMeta: common.TypeMeta{
			APIVersion: psmdbAPINamespace + "/v1",
			Kind:       psmdb.PerconaServerMongoDBKind,
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

	psmdbInternalSecrets := []string{"internal-%s-users", "%s-ssl", "%s-ssl-internal", "%s-mongodb-keyfile", "%s-mongodb-encryption-key"}

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
	err := c.kubeCtl.Get(ctx, psmdb.PerconaServerMongoDBKind, name, &cluster)
	if err != nil {
		if errors.Is(err, kubectl.ErrNotFound) {
			return nil, errors.Wrap(ErrNotFound, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
		}
		return nil, errors.Wrap(err, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
	}

	clusterState := c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)
	if clusterState != ClusterStateReady {
		return nil, errors.Wrap(ErrPSMDBClusterNotReady, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
	}

	password := ""
	username := ""
	var secret common.Secret
	err = c.kubeCtl.Get(ctx, k8sMetaKindSecret, fmt.Sprintf(psmdbSecretNameTmpl, name), &secret)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get PSMDB cluster secrets")
	}
	username = string(secret.Data["MONGODB_USER_ADMIN_USER"])
	password = string(secret.Data["MONGODB_USER_ADMIN_PASSWORD"])

	credentials := &PSMDBCredentials{
		Username:   username,
		Password:   password,
		Host:       cluster.Status.Host,
		Port:       27017,
		Replicaset: "rs0",
	}

	return credentials, nil
}

func (c *K8sClient) crVersionMatchesPodsVersion(ctx context.Context, cluster common.DatabaseCluster) (bool, error) {
	podLables := cluster.DatabasePodLabels()
	databaseContainerNames := cluster.DatabaseContainerNames()
	crImage := cluster.DatabaseImage()
	pods, err := c.GetPods(ctx, "-l"+strings.Join(podLables, ","))
	if err != nil {
		return false, err
	}
	if len(pods.Items) == 0 {
		// Avoid stating it versions don't match when there are no pods to check.
		return true, nil
	}
	images := make(map[string]struct{})
	for _, p := range pods.Items {
		for _, containerName := range databaseContainerNames {
			image, err := p.ContainerImage(containerName)
			if err != nil {
				c.l.Debugf("failed to check pods for container image: %v", err)
				continue
			}
			images[image] = struct{}{}
		}
	}
	_, ok := images[crImage]
	return len(images) == 1 && ok, nil
}

func getCRVersion(buf []byte) (*goversion.Version, error) {
	var mols psmdb.MinimumObjectListSpec

	if err := json.Unmarshal(buf, &mols); err != nil {
		return nil, errors.Wrap(err, "cannot decode response to get CR version spec")
	}

	if len(mols.Items) < 1 {
		return nil, ErrEmptyResponse
	}

	return goversion.NewVersion(mols.Items[0].Spec.CrVersion)
}

// getPSMDBClusters returns Percona Server for MongoDB clusters.
func (c *K8sClient) getPSMDBClusters(ctx context.Context) ([]PSMDBCluster, error) {
	buf, err := c.kubeCtl.GetRaw(ctx, psmdb.PerconaServerMongoDBKind, "")
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get percona server MongoDB clusters")
	}

	res := []PSMDBCluster{}

	crVersion, err := getCRVersion(buf)
	if err != nil {
		if errors.Is(err, ErrEmptyResponse) {
			return res, nil
		}
		return nil, errors.Wrap(err, "cannot determine the CR version in list PSMDB clusters call")
	}

	switch {
	case crVersion == nil: // empty list from kubectl get.
		return res, nil
	case crVersion.GreaterThanOrEqual(v112):
		res, err = c.buildPSMDBDBList112(ctx, buf)
	default:
		res, err = c.buildPSMDBDBList110(ctx, buf)
	}

	return res, err
}

func (c *K8sClient) buildPSMDBDBList110(ctx context.Context, buf []byte) ([]PSMDBCluster, error) {
	var list psmdb.PerconaServerMongoDBList

	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, err
	}

	res := make([]PSMDBCluster, len(list.Items))
	for i, cluster := range list.Items {
		val := PSMDBCluster{
			Name:  cluster.Name,
			Size:  cluster.Spec.Replsets[0].Size,
			Pause: cluster.Spec.Pause,
			Replicaset: &Replicaset{
				DiskSize:         c.getDiskSize(cluster.Spec.Replsets[0].VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.Replsets[0].Resources),
			},
			Exposed: cluster.Spec.Sharding.Mongos.Expose.Enabled,
			Image:   cluster.Spec.Image,
		}

		if cluster.Status != nil {
			message := cluster.Status.Message
			conditions := cluster.Status.Conditions
			if message == "" && len(conditions) > 0 {
				message = conditions[len(conditions)-1].Message
			}

			status := make([]appStatus, 0, len(cluster.Status.Replsets)+1)
			for _, rs := range cluster.Status.Replsets {
				status = append(status, appStatus{rs.Size, rs.Ready})
			}
			status = append(status, appStatus{
				size:  cluster.Status.Mongos.Size,
				ready: cluster.Status.Mongos.Ready,
			})
			val.DetailedState = status
			val.Message = message
		}

		val.State = c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)
		res[i] = val
	}
	return res, nil
}

func (c *K8sClient) buildPSMDBDBList112(ctx context.Context, buf []byte) ([]PSMDBCluster, error) {
	var list psmdb.PerconaServerMongoDBList112

	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, err
	}

	res := make([]PSMDBCluster, len(list.Items))
	for i, cluster := range list.Items {

		exposed := cluster.Spec.Sharding.Mongos.Expose.ExposeType == common.ServiceTypeLoadBalancer ||
			cluster.Spec.Sharding.Mongos.Expose.ExposeType == common.ServiceTypeExternalName

		val := PSMDBCluster{
			Name:  cluster.Name,
			Size:  cluster.Spec.Replsets[0].Size,
			Pause: cluster.Spec.Pause,
			Replicaset: &Replicaset{
				DiskSize:         c.getDiskSize(cluster.Spec.Replsets[0].VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.Replsets[0].Resources),
			},
			Exposed: exposed,
			Image:   cluster.Spec.Image,
		}

		val.State = c.getClusterState(ctx, &cluster, c.crVersionMatchesPodsVersion)
		res[i] = val
	}
	return res, nil
}

// getDeletingPSMDBClusters returns Percona Server for MongoDB clusters which are not fully deleted yet.
func (c *K8sClient) getDeletingPSMDBClusters(ctx context.Context, clusters []PSMDBCluster) ([]PSMDBCluster, error) {
	runningClusters := make(map[string]struct{}, len(clusters))
	for _, cluster := range clusters {
		runningClusters[cluster.Name] = struct{}{}
	}

	deletingClusters, err := c.getDeletingClusters(ctx, "percona-server-mongodb-operator", runningClusters)
	if err != nil {
		return nil, err
	}

	pxcClusters := make([]PSMDBCluster, len(deletingClusters))
	for i, cluster := range deletingClusters {
		pxcClusters[i] = PSMDBCluster{
			Name:          cluster.Name,
			Size:          0,
			State:         ClusterStateDeleting,
			Replicaset:    new(Replicaset),
			DetailedState: []appStatus{},
		}
	}
	return pxcClusters, nil
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

// CheckOperators checks installed operator API version.
func (c *K8sClient) CheckOperators(ctx context.Context) (*Operators, error) {
	output, err := c.kubeCtl.Run(ctx, []string{"api-versions"}, "")
	if err != nil {
		return nil, errors.Wrap(err, "can't get api versions list")
	}

	apiVersions := strings.Split(string(output), "\n")

	return &Operators{
		PXCOperatorVersion:   c.getLatestOperatorAPIVersion(apiVersions, pxcAPINamespace),
		PsmdbOperatorVersion: c.getLatestOperatorAPIVersion(apiVersions, psmdbAPINamespace),
	}, nil
}

// getLatestOperatorAPIVersion returns latest installed operator API version.
// It checks for all API versions supported by the operator and based on the latest API version in the list
// figures out the version. Returns empty string if operator API is not installed.
func (c *K8sClient) getLatestOperatorAPIVersion(installedVersions []string, apiPrefix string) string {
	lastVersion, _ := goversion.NewVersion("v0.0.0")
	foundGreatherVersion := false

	for _, apiVersion := range installedVersions {
		if !strings.HasPrefix(apiVersion, apiPrefix) {
			continue
		}
		v := strings.Split(apiVersion, "/")[1]
		versionParts := strings.Split(v, "-")
		if len(versionParts) != 3 {
			continue
		}
		v = strings.Join(versionParts, ".")
		newVersion, err := goversion.NewVersion(v)
		if err != nil {
			c.l.Warnf("can't parse version %s: %s", v, err)
			continue
		}
		if newVersion.GreaterThan(lastVersion) {
			lastVersion = newVersion
			foundGreatherVersion = true
		}
	}
	if foundGreatherVersion {
		return lastVersion.String()
	}
	return ""
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
// kubectl command. For example "-lyour-label=value,next-label=value", "-ntest-namespace".
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

// GetConsumedCPUAndMemory returns consumed CPU and Memory in given namespace. If namespace
// is empty, it tries to get them from all namespaces.
func (c *K8sClient) GetConsumedCPUAndMemory(ctx context.Context, namespace string) (
	cpuMillis uint64, memoryBytes uint64, err error,
) {
	// Get CPU and Memory Requests of Pods' containers.
	if namespace == "" {
		namespace = "--all-namespaces"
	} else {
		namespace = "-n" + namespace
	}

	pods, err := c.GetPods(ctx, namespace)
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

func (c *K8sClient) getAPIVersionForPSMDBOperator(version string) string {
	return fmt.Sprintf(psmdbAPIVersionTemplate, strings.ReplaceAll(version, ".", "-"))
}

func (c *K8sClient) getAPIVersionForPXCOperator(version string) string {
	return fmt.Sprintf(pxcAPIVersionTemplate, strings.ReplaceAll(version, ".", "-"))
}

func (c *K8sClient) fetchOperatorManifest(ctx context.Context, manifestURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch operator manifests")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch operator manifests")
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			c.l.Errorf("failed to close response's body: %v", err)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to fetch operator manifests, http request ended with status %q", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// ApplyOperator applies bundle.yaml which installs CRDs, RBAC and operator's deployment.
func (c *K8sClient) ApplyOperator(ctx context.Context, version string, manifestsURLTemplate string) error {
	bundleURL := fmt.Sprintf(manifestsURLTemplate, version, "bundle.yaml")
	bundle, err := c.fetchOperatorManifest(ctx, bundleURL)
	if err != nil {
		return errors.Wrap(err, "failed to install operator")
	}
	return c.kubeCtl.Apply(ctx, bundle)
}

// PatchAllPSMDBClusters replaces images versions and CrVersion after update of the operator to match version
// of the installed operator.
func (c *K8sClient) PatchAllPSMDBClusters(ctx context.Context, oldVersion, newVersion string) error {
	var list psmdb.PerconaServerMongoDBList
	err := c.kubeCtl.Get(ctx, psmdb.PerconaServerMongoDBKind, "", &list)
	if err != nil {
		return errors.Wrap(err, "couldn't get percona server MongoDB clusters")
	}

	for _, cluster := range list.Items {
		clusterPatch := &psmdb.PerconaServerMongoDB{
			Spec: &psmdb.PerconaServerMongoDBSpec{
				CRVersion: newVersion,
				Image:     strings.Replace(cluster.Spec.Image, oldVersion, newVersion, 1),
				Backup: &psmdb.BackupSpec{
					Image: strings.Replace(cluster.Spec.Backup.Image, oldVersion, newVersion, 1),
				},
			},
		}
		if err := c.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, "perconaservermongodb", cluster.Name, clusterPatch); err != nil {
			return err
		}
	}
	return nil
}

// PatchAllPXCClusters replaces the image versions and crVersion after update of the operator to match version
// of the installed operator.
func (c *K8sClient) PatchAllPXCClusters(ctx context.Context, oldVersion, newVersion string) error {
	var list pxc.PerconaXtraDBClusterList
	err := c.kubeCtl.Get(ctx, pxc.PerconaXtraDBClusterKind, "", &list)
	if err != nil {
		return errors.Wrap(err, "couldn't get percona XtraDB clusters")
	}

	for _, cluster := range list.Items {
		clusterPatch := &pxc.PerconaXtraDBCluster{
			Spec: &pxc.PerconaXtraDBClusterSpec{
				CRVersion: newVersion,
				PXC: &pxc.PodSpec{
					Image: strings.Replace(cluster.Spec.PXC.Image, oldVersion, newVersion, 1),
				},
				Backup: &pxc.PXCScheduledBackup{
					Image: strings.Replace(cluster.Spec.Backup.Image, oldVersion, newVersion, 1),
				},
			},
		}

		if cluster.Spec.HAProxy != nil {
			clusterPatch.Spec.HAProxy = &pxc.PodSpec{
				Image: strings.Replace(cluster.Spec.HAProxy.Image, oldVersion, newVersion, 1),
			}
		} else {
			cluster.Spec.ProxySQL = &pxc.PodSpec{
				Image: strings.Replace(cluster.Spec.ProxySQL.Image, oldVersion, newVersion, 1),
			}
		}

		if err := c.kubeCtl.Patch(ctx, kubectl.PatchTypeMerge, "perconaxtradbcluster", cluster.Name, clusterPatch); err != nil {
			return err
		}
	}
	return nil
}

// UpdateOperator updates images inside operator deployment and also applies new CRDs and RBAC.
func (c *K8sClient) UpdateOperator(ctx context.Context, version, deploymentName, manifestsURLTemplate string) error {
	files := []string{"crd.yaml", "rbac.yaml"}
	for _, file := range files {
		manifestURL := fmt.Sprintf(manifestsURLTemplate, version, file)
		manifest, err := c.fetchOperatorManifest(ctx, manifestURL)
		if err != nil {
			return errors.Wrap(err, "failed to update operator")
		}
		err = c.kubeCtl.Apply(ctx, manifest)
		if err != nil {
			return errors.Wrap(err, "failed to update operator")
		}
	}
	// Change image inside operator deployment.
	var deployment common.Deployment
	err := c.kubeCtl.Get(ctx, "deployment", deploymentName, &deployment)
	if err != nil {
		return errors.Wrap(err, "failed to get operator deployment")
	}
	containerIndex := -1
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == deploymentName {
			containerIndex = i
		}
	}
	if containerIndex < 0 {
		return errors.Errorf("container with name %q not found inside operator deployment", deploymentName)
	}
	imageAndTag := strings.Split(deployment.Spec.Template.Spec.Containers[containerIndex].Image, ":")
	if len(imageAndTag) != 2 {
		return errors.Errorf("container image %q does not have any tag", deployment.Spec.Template.Spec.Containers[containerIndex].Image)
	}
	deployment.Spec.Template.Spec.Containers[containerIndex].Image = imageAndTag[0] + ":" + version
	return c.kubeCtl.Patch(ctx, kubectl.PatchTypeStrategic, "deployment", deploymentName, deployment)
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
			return errors.Wrapf(err, "cannot apply file: %q", path)
		}
	}

	randomCrypto, err := rand.Prime(rand.Reader, 64)
	if err != nil {
		return err
	}

	secretName := fmt.Sprintf("vm-operator-%d", randomCrypto)
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

// RemoveVMOperator deletes the VM Operator installed when the cluster was registered.
func (c *K8sClient) RemoveVMOperator(ctx context.Context) error {
	files := []string{
		"deploy/victoriametrics/crds/crd.yaml",
		"deploy/victoriametrics/operator/manager.yaml",
	}
	for _, path := range files {
		file, err := dbaascontroller.DeployDir.ReadFile(path)
		if err != nil {
			return err
		}
		err = c.kubeCtl.Delete(ctx, file)
		if err != nil {
			return errors.Wrapf(err, "cannot apply file: %q", path)
		}
	}

	return nil
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
			SelectAllByDefault:             true,
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
			ExtraArgs: map[string]string{
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

// CreatePSMDBCluster creates percona server for mongodb cluster with provided parameters.
// func (c *K8sClient) CreatePSMDBClusterOld(ctx context.Context, params *PSMDBParams) error {
func (c *K8sClient) makeReq(params *PSMDBParams, extra extraCRParams) *psmdb.PerconaServerMongoDB {
	res := &psmdb.PerconaServerMongoDB{
		TypeMeta: common.TypeMeta{
			APIVersion: c.getAPIVersionForPSMDBOperator(extra.operators.PsmdbOperatorVersion),
			Kind:       psmdb.PerconaServerMongoDBKind,
		},
		ObjectMeta: common.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-psmdb-pvc"},
		},
		Spec: &psmdb.PerconaServerMongoDBSpec{
			UpdateStrategy: updateStrategyRollingUpdate,
			CRVersion:      extra.operators.PsmdbOperatorVersion,
			Image:          extra.psmdbImage,
			Secrets: &psmdb.SecretsSpec{
				Users: extra.secretName,
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
							Affinity: extra.affinity,
						},
					},
					MultiAZ: psmdb.MultiAZ{
						Affinity: extra.affinity,
					},
				},
				Mongos: &psmdb.ReplsetSpec{
					Arbiter: psmdb.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdb.MultiAZ{
							Affinity: extra.affinity,
						},
					},
					Size: params.Size,
					MultiAZ: psmdb.MultiAZ{
						Affinity: extra.affinity,
					},
					Expose: extra.expose,
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
							Affinity: extra.affinity,
						},
					},
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					PodDisruptionBudget: &common.PodDisruptionBudgetSpec{
						MaxUnavailable: pointer.ToInt(1),
					},
					MultiAZ: psmdb.MultiAZ{
						Affinity: extra.affinity,
					},
				},
			},

			PMM: &psmdb.PmmSpec{
				Enabled: false,
			},

			Backup: &psmdb.BackupSpec{
				Enabled:            true,
				Image:              fmt.Sprintf(psmdbBackupImageTemplate, extra.operators.PsmdbOperatorVersion),
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
					Memory: "300M",
					CPU:    "500m",
				},
			},
		}
	}

	return res
}

func (c *K8sClient) makeReq112Plus(params *PSMDBParams, extra extraCRParams) *psmdb.PerconaServerMongoDB112 { //nolint:funlen
	req := &psmdb.PerconaServerMongoDB112{
		APIVersion: c.getAPIVersionForPSMDBOperator(extra.operators.PsmdbOperatorVersion),
		Kind:       psmdb.PerconaServerMongoDBKind,
		TypeMeta: common.TypeMeta{
			APIVersion: c.getAPIVersionForPSMDBOperator(extra.operators.PsmdbOperatorVersion),
			Kind:       psmdb.PerconaServerMongoDBKind,
		},
		ObjectMeta: common.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-psmdb-pvc"},
		},
		Spec: &psmdb.PSMDB112Spec{
			UpdateStrategy: updateStrategyRollingUpdate,
			CRVersion:      extra.operators.PsmdbOperatorVersion,
			Image:          extra.psmdbImage,
			Secrets: &psmdb.SecretsSpec{
				Users: extra.secretName,
			},
			Sharding: &psmdb.ShardingSpec112{
				Enabled: true,
				ConfigsvrReplSet: &psmdb.ReplsetSpec{
					Size:       3,
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					Arbiter: psmdb.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdb.MultiAZ{
							Affinity: extra.affinity,
						},
					},
					MultiAZ: psmdb.MultiAZ{
						Affinity: extra.affinity,
					},
					Expose: extra.expose,
				},
				Mongos: &psmdb.ReplsetMongosSpec112{
					Size: params.Size,
					MultiAZ: psmdb.MultiAZ{
						Affinity: extra.affinity,
					},
					Expose: psmdb.ExposeSpec{
						ExposeType: common.ServiceTypeLoadBalancer,
					},
				},
			},
			Replsets: []*psmdb.ReplsetSpec112{
				{
					Name:      "rs0",
					Size:      params.Size,
					Resources: c.setComputeResources(params.Replicaset.ComputeResources),
					Arbiter: psmdb.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdb.MultiAZ{
							Affinity: extra.affinity,
						},
					},
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					PodDisruptionBudget: &common.PodDisruptionBudgetSpec{
						MaxUnavailable: pointer.ToInt(1),
					},
					MultiAZ: psmdb.MultiAZ{
						Affinity: extra.affinity,
					},
					Configuration: "      operationProfiling:\n" +
						"        mode: " + string(psmdb.OperationProfilingModeSlowOp) + "\n",
				},
			},
			PMM: &psmdb.PmmSpec{
				Enabled: false,
			},
			Backup: &psmdb.BackupSpec{
				Enabled:            true,
				Image:              extra.backupImage,
				ServiceAccountName: "percona-server-mongodb-operator",
			},
		},
	}

	if params.Replicaset != nil {
		req.Spec.Replsets[0].Resources = c.setComputeResources(params.Replicaset.ComputeResources)
		req.Spec.Sharding.Mongos.Resources = c.setComputeResources(params.Replicaset.ComputeResources)
	}
	if params.PMM != nil {
		req.Spec.PMM = &psmdb.PmmSpec{
			Enabled:    true,
			ServerHost: params.PMM.PublicAddress,
			Image:      pmmClientImage,
			Resources: &common.PodResources{
				Requests: &common.ResourcesList{
					Memory: "300M",
					CPU:    "500m",
				},
			},
		}
	}

	return req
}
