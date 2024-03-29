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
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	goversion "github.com/hashicorp/go-version"
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	pmmversion "github.com/percona/pmm/version"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kube"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kubectl"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/monitoring"
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

	appStateUnknown  string = "unknown"
	appStateInit     string = "initializing"
	appStatePaused   string = "paused"
	appStateStopping string = "stopping"
	appStateReady    string = "ready"
	appStateError    string = "error"
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
	pxcCRFile               = "/srv/dbaas/crs/pxc.cr.yml"
	psmdbCRFile             = "/srv/dbaas/crs/psmdb.cr.yml"
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

type extraCRParams struct {
	secretName  string
	secrets     map[string][]byte
	psmdbImage  string
	backupImage string
	affinity    *psmdbv1.PodAffinity
	expose      psmdbv1.Expose
	operators   *Operators
}

// clustertatesMap matches pxc and psmdb app states to cluster states.
var clusterStatesMap = map[string]ClusterState{ //nolint:gochecknoglobals
	appStateInit:     ClusterStateChanging,
	appStateReady:    ClusterStateReady,
	appStateError:    ClusterStateFailed,
	appStatePaused:   ClusterStatePaused,
	appStateStopping: ClusterStateChanging,
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

var pmmClientImage string //nolint:gochecknoglobals

// K8sClient is a client for Kubernetes.
type K8sClient struct {
	kubeCtl    *kubectl.KubeCtl
	kube       *kube.Client
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

	v, err := goversion.NewVersion(pmmversion.PMMVersion) //nolint: varnamelen
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

	kube, err := kube.NewFromKubeConfigString(kubeconfig)
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		kube:    kube,
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

// NewIncluster returns new K8Client object.
func NewIncluster(ctx context.Context) (*K8sClient, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "K8sClient")

	kube, err := kube.NewFromIncluster()
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		kube: kube,
		l:    l,
		client: &http.Client{
			Timeout: time.Second * 5,
			Transport: &http.Transport{
				MaxIdleConns:    1,
				IdleConnTimeout: 10 * time.Second,
			},
		},
	}, nil
}

// Cleanup removes temporary files created by that object.
func (c *K8sClient) Cleanup() error {
	return c.kubeCtl.Cleanup()
}

func (c *K8sClient) Run(ctx context.Context, params []string) ([]byte, error) {
	return c.kubeCtl.Run(ctx, params, nil)
}

func (c *K8sClient) Apply(ctx context.Context, res interface{}) error {
	return c.kubeCtl.Apply(ctx, res)
}

func (c *K8sClient) Patch(ctx context.Context, patchType kubectl.PatchType, resourceType, resourceName, namespace string, res interface{}) error {
	return c.kubeCtl.Patch(ctx, patchType, resourceType, resourceName, namespace, res)
}

func (c *K8sClient) Delete(ctx context.Context, res interface{}) error {
	return c.kubeCtl.Delete(ctx, res)
}

// GetKubeconfig generates kubeconfig compatible with kubectl for incluster created clients.
func (c *K8sClient) GetKubeconfig(ctx context.Context) (string, error) {
	secret, err := c.kube.GetSecretsForServiceAccount(ctx, "pmm-service-account")
	if err != nil {
		c.l.Errorf("failed getting service account: %v", err)
		return "", err
	}
	kubeConfig, err := c.kube.GenerateKubeConfig(secret)
	if err != nil {
		c.l.Errorf("failed generating kubeconfig: %v", err)
		return "", err
	}
	return string(kubeConfig), nil
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
	secret := &corev1.Secret{ //nolint: exhaustruct
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8sAPIVersion,
			Kind:       k8sMetaKindSecret,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	return c.kube.Apply(ctx, secret)
}

// CreatePXCCluster creates Percona XtraDB cluster with provided parameters.
func (c *K8sClient) CreatePXCCluster(ctx context.Context, params *PXCParams) error {
	if (params.ProxySQL != nil) == (params.HAProxy != nil) {
		return errors.New("pxc cluster must have one and only one proxy type defined")
	}

	_, err := c.kube.GetPXCCluster(ctx, params.Name)
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
	if params.PMM != nil {
		secrets["pmmserver"] = []byte(params.PMM.Password)
	}

	var serviceType corev1.ServiceType
	// This enables ingress for the cluster and exposes the cluster to the world.
	// The cluster will have an internal IP and a world accessible hostname.
	// This feature cannot be tested with minikube. Please use EKS for testing.
	if clusterType := c.GetKubernetesClusterType(ctx); clusterType != MinikubeClusterType && params.Expose {
		serviceType = corev1.ServiceTypeLoadBalancer
	} else {
		serviceType = corev1.ServiceTypeNodePort
	}

	spec, err := c.createPXCSpecFromParams(params, &secretName, operators.PXCOperatorVersion, storageName, serviceType)
	if err != nil {
		return err
	}

	err = c.CreateSecret(ctx, secretName, secrets)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}

	return c.kube.Apply(ctx, spec)
}

// UpdatePXCCluster changes size of provided Percona XtraDB cluster.
func (c *K8sClient) UpdatePXCCluster(ctx context.Context, params *PXCParams) error {
	if (params.ProxySQL != nil) && (params.HAProxy != nil) {
		return errors.New("can't update both proxies, only one should be in use")
	}

	cluster, err := c.kube.GetPXCCluster(ctx, params.Name)
	if err != nil {
		return err
	}
	cluster.Kind = kube.PXCKind
	cluster.APIVersion = pxcAPINamespace + "/v1"

	clusterInfo := kube.NewDBClusterInfoFromPXC(cluster)

	clusterState := c.getClusterState(ctx, clusterInfo, c.crVersionMatchesPodsVersion)

	// Only if cluster is paused, allow resuming it. All other modifications are forbinden.
	if params.Resume && clusterState == ClusterStatePaused {
		cluster.Spec.Pause = false
		return c.kube.Apply(ctx, cluster)
	}

	// This is to prevent concurrent updates
	if clusterState != ClusterStateReady {
		return errors.Wrapf(ErrPXCClusterStateUnexpected, "state is %v", cluster.Status.Status) //nolint:wrapcheck
	}

	if params.Suspend {
		cluster.Spec.Pause = true
	}

	if params.Size > 0 {
		cluster.Spec.PXC.Size = params.Size
		if cluster.Spec.ProxySQL != nil {
			cluster.Spec.ProxySQL.Size = params.Size
		} else {
			cluster.Spec.HAProxy.Size = params.Size
		}
	}

	if params.PXC != nil {
		cluster.Spec.PXC.Resources = c.updateComputeResources(params.PXC.ComputeResources, cluster.Spec.PXC.Resources)
		if params.PXC.Image != "" && params.PXC.Image != cluster.Spec.PXC.Image {
			// Let's upgrade the cluster.
			err = c.validateImage(cluster.Spec.PXC.Image, params.PXC.Image)
			if err != nil {
				return err
			}
			cluster.Spec.PXC.Image = params.PXC.Image
		}
	}

	if params.ProxySQL != nil {
		cluster.Spec.ProxySQL.Resources = c.updateComputeResources(params.ProxySQL.ComputeResources, cluster.Spec.ProxySQL.Resources)
	}

	if params.HAProxy != nil {
		cluster.Spec.HAProxy.Resources = c.updateComputeResources(params.HAProxy.ComputeResources, cluster.Spec.HAProxy.Resources)
	}

	patch, err := json.Marshal(cluster)
	if err != nil {
		return err
	}
	_, err = c.kube.PatchPXCCluster(ctx, cluster.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// DeletePXCCluster deletes Percona XtraDB cluster with provided name.
func (c *K8sClient) DeletePXCCluster(ctx context.Context, name string) error {
	spec := &pxcv1.PerconaXtraDBCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: pxcAPINamespace + "/v1",
			Kind:       kube.PXCKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := c.kube.Delete(ctx, spec)
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
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: k8sAPIVersion,
			Kind:       k8sMetaKindSecret,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: secretName,
		},
	}

	return c.kube.Delete(ctx, secret)
}

// GetPXCClusterCredentials returns an PXC cluster credentials.
func (c *K8sClient) GetPXCClusterCredentials(ctx context.Context, name string) (*PXCCredentials, error) {
	cluster, err := c.kube.GetPXCCluster(ctx, name)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return nil, errors.Wrap(ErrNotFound, fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"))
		}
		return nil, errors.Wrap(err, fmt.Sprintf(canNotGetCredentialsErrTemplate, "XtraDb"))
	}

	clusterInfo := kube.NewDBClusterInfoFromPXC(cluster)
	clusterState := c.getClusterState(ctx, clusterInfo, c.crVersionMatchesPodsVersion)
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

	secret, err := c.kube.GetSecret(ctx, fmt.Sprintf(pxcSecretNameTmpl, name))
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

// GetKubernetesClusterType returns k8s cluster type based on storage class.
func (c *K8sClient) GetKubernetesClusterType(ctx context.Context) KubernetesClusterType {
	sc, err := c.kube.GetStorageClasses(ctx)
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
		if strings.Contains(class.Provisioner, "minikube") || strings.Contains(class.Provisioner, "kubevirt.io/hostpath-provisioner") || strings.Contains(class.Provisioner, "standard") {
			return MinikubeClusterType
		}
	}

	return clusterTypeUnknown
}

// RestartPXCCluster restarts Percona XtraDB cluster with provided name.
// FIXME: https://jira.percona.com/browse/PMM-6980
func (c *K8sClient) RestartPXCCluster(ctx context.Context, name string) error {
	c.l.Info(name)
	_, err := c.kube.RestartStatefulSet(ctx, name+"-"+"pxc")
	if err != nil {
		return err
	}

	for _, proxy := range []string{"proxysql", "haproxy"} {
		if _, err := c.kube.GetStatefulSet(ctx, name+"-"+proxy); err == nil {
			_, err = c.kube.RestartStatefulSet(ctx, name+"-"+proxy)
			return err
		}
	}
	return nil

	// return errors.New("failed to restart pxc cluster proxy statefulset")
}

// getPerconaXtraDBClusters returns Percona XtraDB clusters.
func (c *K8sClient) getPerconaXtraDBClusters(ctx context.Context) ([]PXCCluster, error) {
	list, err := c.kube.ListPXCClusters(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get Percona XtraDB clusters")
	}

	res := make([]PXCCluster, len(list.Items))
	for i, cluster := range list.Items {
		val := PXCCluster{
			Name: cluster.Name,
			Size: cluster.Spec.PXC.Size,
			PXC: &PXC{
				Image:            cluster.Spec.PXC.Image,
				DiskSize:         c.getPXCDiskSize(cluster.Spec.PXC.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.PXC.Resources),
			},
			Pause: cluster.Spec.Pause,
		}
		if len(cluster.Status.Conditions) > 0 {
			val.DetailedState = []appStatus{
				//{size: cluster.Status.Size, ready: cluster.Status.PMM.Status == "ready"},
				{size: cluster.Status.HAProxy.Size, ready: cluster.Status.HAProxy.Ready},
				{size: cluster.Status.ProxySQL.Size, ready: cluster.Status.ProxySQL.Ready},
				{size: cluster.Status.PXC.Size, ready: cluster.Status.PXC.Ready},
			}
			val.Message = strings.Join(cluster.Status.Messages, ";")
		}

		clusterInfo := kube.NewDBClusterInfoFromPXC(&cluster)

		val.State = c.getClusterState(ctx, clusterInfo, c.crVersionMatchesPodsVersion)
		if cluster.Spec.ProxySQL != nil {
			val.ProxySQL = &ProxySQL{
				DiskSize:         c.getPXCDiskSize(cluster.Spec.ProxySQL.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.ProxySQL.Resources),
			}
			val.Exposed = cluster.Spec.ProxySQL.ServiceType != "" &&
				cluster.Spec.ProxySQL.ServiceType != corev1.ServiceTypeClusterIP
			res[i] = val
			continue
		}
		if cluster.Spec.HAProxy != nil {
			val.HAProxy = &HAProxy{
				ComputeResources: c.getComputeResources(cluster.Spec.HAProxy.Resources),
			}
			val.Exposed = cluster.Spec.HAProxy.ServiceType != "" &&
				cluster.Spec.HAProxy.ServiceType != corev1.ServiceTypeClusterIP
		}
		res[i] = val
	}
	return res, nil
}

func (c *K8sClient) getClusterState(ctx context.Context, cluster kube.DBCluster, crAndPodsMatchFunc func(context.Context, kube.DBCluster) (bool, error)) ClusterState {
	state := cluster.State
	if state == appStateUnknown {
		return ClusterStateInvalid
	}
	// Handle paused state for operator version >= 1.9.0 and for operator version <= 1.8.0.
	if state == appStatePaused || (cluster.Pause && state == appStateReady) {
		return ClusterStatePaused
	}

	clusterState, ok := clusterStatesMap[string(state)]
	if !ok {
		c.l.Warnf("failed to recognize cluster state: %q, setting status to ClusterStateChanging", state)
		return ClusterStateChanging
	}
	if clusterState == ClusterStateChanging {
		// Check if cr and pods version matches.
		match, err := crAndPodsMatchFunc(ctx, cluster)
		if err != nil {
			c.l.Warnf("failed to check if cluster %q is upgrading: %v", cluster.Name, err)
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
	list, err := c.kube.GetPods(ctx, "", "")
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
	_, err := c.kube.GetPSMDBCluster(ctx, params.Name)
	if err == nil {
		return fmt.Errorf(clusterWithSameNameExistsErrTemplate, params.Name)
	}

	extra := extraCRParams{}
	extra.secretName = fmt.Sprintf(psmdbSecretNameTmpl, params.Name)
	extra.secrets, err = generatePSMDBPasswords()
	if err != nil {
		return err
	}

	extra.affinity = new(psmdbv1.PodAffinity)
	extra.expose = psmdbv1.Expose{
		Enabled:    false,
		ExposeType: corev1.ServiceTypeClusterIP,
	}
	if clusterType := c.GetKubernetesClusterType(ctx); clusterType != MinikubeClusterType {
		extra.affinity.TopologyKey = pointer.ToString("kubernetes.io/hostname")

		if params.Expose {
			// This enables ingress for the cluster and exposes the cluster to the world.
			// The cluster will have an internal IP and a world accessible hostname.
			// This feature cannot be tested with minikube. Please use EKS for testing.
			extra.expose = psmdbv1.Expose{
				Enabled:    true,
				ExposeType: corev1.ServiceTypeLoadBalancer,
			}
		}
	} else {
		// https://www.percona.com/doc/kubernetes-operator-for-psmongodb/minikube.html
		// > Install Percona Server for MongoDB on Minikube
		// > ...
		// > set affinity.antiAffinityTopologyKey key to "none"
		// > (the Operator will be unable to spread the cluster on several nodes)
		extra.affinity.TopologyKey = pointer.ToString(psmdbv1.AffinityOff)
		if params.Expose {
			// Expose services for minikube using NodePort
			// This requires additional configuration for minikube and has limitations
			// on MacOs
			extra.expose = psmdbv1.Expose{
				Enabled:    true,
				ExposeType: corev1.ServiceTypeNodePort,
			}
		}

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

	if params.PMM != nil {
		extra.secrets["PMM_SERVER_USER"] = []byte(params.PMM.Login)
		extra.secrets["PMM_SERVER_PASSWORD"] = []byte(params.PMM.Password)
	}

	spec, err := c.createPSMDBSpec(psmdbOperatorVersion, params, &extra)
	if err != nil {
		return err
	}
	err = c.CreateSecret(ctx, extra.secretName, extra.secrets)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}

	return c.kube.Apply(ctx, spec)
}

// UpdatePSMDBCluster changes size, stops, resumes or upgrades provided percona server for mongodb cluster.
func (c *K8sClient) UpdatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	cluster, err := c.kube.GetPSMDBCluster(ctx, params.Name)
	if err != nil {
		return err
	}
	cluster.Kind = kube.PSMDBKind
	cluster.APIVersion = psmdbAPINamespace + "/v1"
	clusterInfo := kube.NewDBClusterInfoFromPSMDB(cluster)
	clusterState := c.getClusterState(ctx, clusterInfo, c.crVersionMatchesPodsVersion)
	if params.Resume && clusterState == ClusterStatePaused {
		cluster.Spec.Pause = false
		return c.kube.Apply(ctx, cluster)
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
		err = c.validateImage(cluster.Spec.Image, params.Image)
		if err != nil {
			return err
		}
		cluster.Spec.Image = params.Image
	}
	patch, err := json.Marshal(cluster)
	if err != nil {
		return err
	}
	_, err = c.kube.PatchPSMDBCluster(ctx, cluster.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

const (
	updateStrategyRollingUpdate = "RollingUpdate"
)

func (c *K8sClient) validateImage(crImage, newImage string) error {
	// Check that only tag changed.
	newImageAndTag := strings.Split(newImage, ":")
	if len(newImageAndTag) != 2 {
		return errors.New("image has to have version tag")
	}
	currentImageAndTag := strings.Split(crImage, ":")
	if currentImageAndTag[0] != newImageAndTag[0] {
		return errors.Errorf("expected image is %q, %q was given", currentImageAndTag[0], newImageAndTag[0])
	}
	if currentImageAndTag[1] == newImageAndTag[1] {
		return errors.Errorf("failed to change image: the database version %q is already in use", newImageAndTag[1])
	}

	return nil
}

// DeletePSMDBCluster deletes percona server for mongodb cluster with provided name.
func (c *K8sClient) DeletePSMDBCluster(ctx context.Context, name string) error {
	spec := &psmdbv1.PerconaServerMongoDB{
		TypeMeta: metav1.TypeMeta{
			APIVersion: psmdbAPINamespace + "/v1",
			Kind:       kube.PSMDBKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	err := c.kube.Delete(ctx, spec)
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
	if _, err := c.kube.GetStatefulSet(ctx, name+"-rs0"); err == nil {
		_, err = c.kube.RestartStatefulSet(ctx, name+"-rs0")
		return err
	}
	return nil
}

// GetPSMDBClusterCredentials returns a PSMDB cluster.
func (c *K8sClient) GetPSMDBClusterCredentials(ctx context.Context, name string) (*PSMDBCredentials, error) {
	cluster, err := c.kube.GetPSMDBCluster(ctx, name)
	if err != nil {
		if apiErrors.IsNotFound(err) {
			return nil, errors.Wrap(ErrNotFound, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
		}
		return nil, errors.Wrap(err, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
	}

	clusterInfo := kube.NewDBClusterInfoFromPSMDB(cluster)

	clusterState := c.getClusterState(ctx, clusterInfo, c.crVersionMatchesPodsVersion)
	if clusterState != ClusterStateReady {
		return nil, errors.Wrap(ErrPSMDBClusterNotReady, fmt.Sprintf(canNotGetCredentialsErrTemplate, "PSMDB"))
	}

	password := ""
	username := ""
	secret, err := c.kube.GetSecret(ctx, fmt.Sprintf(psmdbSecretNameTmpl, name))
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

func (c *K8sClient) crVersionMatchesPodsVersion(ctx context.Context, cluster kube.DBCluster) (bool, error) {
	podLables := cluster.PodLabels
	pods, err := c.GetPods(ctx, "", strings.Join(podLables, ","))
	if err != nil {
		return false, err
	}
	if len(pods.Items) == 0 {
		// Avoid stating it versions don't match when there are no pods to check.
		return true, nil
	}
	images := make(map[string]struct{})
	for _, p := range pods.Items {
		for _, containerName := range cluster.ContainerNames {
			var imageName string
			for _, c := range p.Spec.Containers {
				if c.Name == containerName {
					imageName = c.Image
				}
			}
			if imageName == "" {
				c.l.Debugf("failed to check pods for container image: %v", err)
				continue
			}
			images[imageName] = struct{}{}
		}
	}
	_, ok := images[cluster.CRImage]
	return len(images) == 1 && ok, nil
}

// getPSMDBClusters returns Percona Server for MongoDB clusters.
func (c *K8sClient) getPSMDBClusters(ctx context.Context) ([]PSMDBCluster, error) {
	list, err := c.kube.ListPSMDBClusters(ctx)
	res := make([]PSMDBCluster, len(list.Items))
	if err != nil {
		return res, err
	}
	for i, cluster := range list.Items {

		val := PSMDBCluster{
			Name:  cluster.Name,
			Size:  cluster.Spec.Replsets[0].Size,
			Pause: cluster.Spec.Pause,
			Replicaset: &Replicaset{
				DiskSize:         c.getPSMDBDiskSize(cluster.Spec.Replsets[0].VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.Replsets[0].Resources),
			},
			Exposed: cluster.Spec.Sharding.Mongos.Expose.ExposeType != corev1.ServiceTypeClusterIP,
			Image:   cluster.Spec.Image,
		}

		if len(cluster.Status.Conditions) > 0 {
			message := cluster.Status.Message
			conditions := cluster.Status.Conditions
			if message == "" && len(conditions) > 0 {
				message = conditions[len(conditions)-1].Message
			}

			status := make([]appStatus, 0, len(cluster.Status.Replsets)+1)
			for _, rs := range cluster.Status.Replsets {
				status = append(status, appStatus{rs.Size, rs.Ready})
			}
			if val.Size != 1 {
				status = append(status, appStatus{
					size:  int32(cluster.Status.Mongos.Size),
					ready: int32(cluster.Status.Mongos.Ready),
				})
			}
			val.DetailedState = status
			val.Message = message
		}

		clusterInfo := kube.NewDBClusterInfoFromPSMDB(&cluster)
		val.State = c.getClusterState(ctx, clusterInfo, c.crVersionMatchesPodsVersion)
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

func (c *K8sClient) getComputeResources(resources corev1.ResourceRequirements) *ComputeResources {
	res := new(ComputeResources)
	cpuLimit, ok := resources.Limits[corev1.ResourceCPU]
	cpu := (&cpuLimit).String()
	if ok && cpu != "" {
		res.CPUM = cpu
	}
	memLimit, ok := resources.Limits[corev1.ResourceMemory]
	mem := (&memLimit).String()
	if ok && mem != "" {
		res.MemoryBytes = mem
	}
	return res
}

func (c *K8sClient) setComputeResources(res *ComputeResources) corev1.ResourceRequirements {
	req := corev1.ResourceRequirements{}
	if res == nil {
		return req
	}
	req.Limits = corev1.ResourceList{}
	if res.CPUM != "" {
		req.Limits[corev1.ResourceCPU] = resource.MustParse(res.CPUM)
	}
	if res.MemoryBytes != "" {
		req.Limits[corev1.ResourceMemory] = resource.MustParse(res.MemoryBytes)
	}
	return req
}

func (c *K8sClient) updateComputeResources(res *ComputeResources, podResources corev1.ResourceRequirements) corev1.ResourceRequirements {
	if res == nil {
		return podResources
	}
	if (&podResources).Size() == 0 {
		podResources = corev1.ResourceRequirements{}
	}

	podResources.Limits[corev1.ResourceCPU] = resource.MustParse(res.CPUM)
	podResources.Limits[corev1.ResourceMemory] = resource.MustParse(res.MemoryBytes)
	return podResources
}

func (c *K8sClient) getPXCDiskSize(volumeSpec *pxcv1.VolumeSpec) string {
	if volumeSpec == nil || volumeSpec.PersistentVolumeClaim == nil {
		return "0"
	}
	quantity, ok := volumeSpec.PersistentVolumeClaim.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		return "0"
	}
	return quantity.String()
}

func (c *K8sClient) getPSMDBDiskSize(volumeSpec *psmdbv1.VolumeSpec) string {
	if volumeSpec == nil || volumeSpec.PersistentVolumeClaim == nil {
		return "0"
	}
	quantity, ok := volumeSpec.PersistentVolumeClaim.Resources.Requests[corev1.ResourceStorage]
	if !ok {
		return "0"
	}
	return quantity.String()
}

func (c *K8sClient) pxcVolumeSpec(diskSize string) *pxcv1.VolumeSpec {
	return &pxcv1.VolumeSpec{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(diskSize),
				},
			},
		},
	}
}

func (c *K8sClient) volumeSpec(diskSize string) *psmdbv1.VolumeSpec {
	return &psmdbv1.VolumeSpec{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimSpec{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(diskSize),
				},
			},
		},
	}
}

// CheckOperators checks installed operator API version.
func (c *K8sClient) CheckOperators(ctx context.Context) (*Operators, error) {
	apiVersions, err := c.kube.GetAPIVersions(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "can't get api versions list")
	}

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
func sumVolumesSize(pvs *corev1.PersistentVolumeList) (sum uint64, err error) {
	for _, pv := range pvs.Items {
		bytes, err := convertors.StrToBytes(pv.Spec.Capacity.Storage().String())
		if err != nil {
			return 0, err
		}
		sum += bytes
	}
	return
}

// GetPersistentVolumes returns list of persistent volumes.
func (c *K8sClient) GetPersistentVolumes(ctx context.Context) (*corev1.PersistentVolumeList, error) {
	return c.kube.GetPersistentVolumes(ctx)
}

// GetPods returns list of pods based on given filters. Filters are args to
// kubectl command. For example "-lyour-label=value,next-label=value", "-ntest-namespace".
func (c *K8sClient) GetPods(ctx context.Context, namespace string, filters ...string) (*corev1.PodList, error) {
	podList, err := c.kube.GetPods(ctx, namespace, strings.Join(filters, ""))
	return podList, err
}

// GetLogs returns logs as slice of log lines - strings - for given pod's container.
func (c *K8sClient) GetLogs(
	ctx context.Context,
	containerStatuses []corev1.ContainerStatus,
	pod,
	container string,
) ([]string, error) {
	if common.IsContainerInState(containerStatuses, common.ContainerStateWaiting, container) {
		return []string{}, nil
	}
	stdout, err := c.kube.GetLogs(ctx, pod, container)
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
	stdout, err := c.kube.GetEvents(ctx, pod)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't describe pod")
	}
	lines := strings.Split(string(stdout), "\n")
	return lines, nil
}

// getWorkerNodes returns list of cluster workers nodes.
func (c *K8sClient) getWorkerNodes(ctx context.Context) ([]corev1.Node, error) {
	nodes, err := c.kube.GetNodes(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "could not get nodes of Kubernetes cluster")
	}
	forbidenTaints := map[string]corev1.TaintEffect{
		"node.cloudprovider.kubernetes.io/uninitialized": corev1.TaintEffectNoSchedule,
		"node.kubernetes.io/unschedulable":               corev1.TaintEffectNoSchedule,
		"node-role.kubernetes.io/master":                 corev1.TaintEffectNoSchedule,
	}
	workers := make([]corev1.Node, 0, len(nodes.Items))
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
func (c *K8sClient) GetAllClusterResources(ctx context.Context, clusterType KubernetesClusterType, volumes *corev1.PersistentVolumeList) (
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
			storage, ok := node.Status.Allocatable[corev1.ResourceEphemeralStorage]
			if !ok {
				return 0, 0, 0, errors.Errorf("could not get storage size of the node")
			}
			bytes, err := convertors.StrToBytes(storage.String())
			if err != nil {
				return 0, 0, 0, errors.Wrapf(err, "could not convert storage size '%s' to bytes", storage.String())
			}
			diskSizeBytes += bytes
		case AmazonEKSClusterType:
			// See https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/#scheduler.
			if common.IsNodeInCondition(node, corev1.NodeDiskPressure) {
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
func getResources(resources corev1.ResourceList) (cpuMillis uint64, memoryBytes uint64, err error) {
	cpu, ok := resources[corev1.ResourceCPU]
	if ok {
		cpuMillis, err = convertors.StrToMilliCPU(cpu.String())
		if err != nil {
			return 0, 0, errors.Wrapf(err, "failed to convert '%s' to millicpus", cpu.String())
		}
	}
	memory, ok := resources[corev1.ResourceMemory]
	if ok {
		memoryBytes, err = convertors.StrToBytes(memory.String())
		if err != nil {
			return 0, 0, errors.Wrapf(err, "failed to convert '%s' to bytes", memory.String())
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
	pods, err := c.GetPods(ctx, namespace)
	if err != nil {
		return 0, 0, errors.Wrap(err, "failed to get consumed resources")
	}
	for _, ppod := range pods.Items {
		if ppod.Status.Phase != corev1.PodRunning {
			continue
		}
		nonTerminatedInitContainers := make([]corev1.Container, 0, len(ppod.Spec.InitContainers))
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
func (c *K8sClient) GetConsumedDiskBytes(ctx context.Context, clusterType KubernetesClusterType, volumes *corev1.PersistentVolumeList) (consumedBytes uint64, err error) {
	//nolint: cyclop
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
	return c.kube.ApplyFile(ctx, bundle)
}

// PatchAllPSMDBClusters replaces images versions and CrVersion after update of the operator to match version
// of the installed operator.
func (c *K8sClient) PatchAllPSMDBClusters(ctx context.Context, oldVersion, newVersion string) error {
	list, err := c.kube.ListPSMDBClusters(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't get percona server MongoDB clusters")
	}

	for _, cluster := range list.Items {
		clusterPatch := &kube.OperatorPatch{
			Spec: kube.Spec{
				CRVersion: newVersion,
				Image:     strings.Replace(cluster.Spec.Image, oldVersion, newVersion, 1),
				UpgradeOptions: kube.UpgradeOptions{
					Apply:    "recommended",
					Schedule: "0 4 * * *",
				},
			},
		}
		patch, err := json.Marshal(clusterPatch)
		if err != nil {
			return err
		}
		if _, err := c.kube.PatchPSMDBCluster(ctx, cluster.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// PatchAllPXCClusters replaces the image versions and crVersion after update of the operator to match version
// of the installed operator.
func (c *K8sClient) PatchAllPXCClusters(ctx context.Context, oldVersion, newVersion string) error {
	list, err := c.kube.ListPXCClusters(ctx)
	if err != nil {
		return errors.Wrap(err, "couldn't get percona XtraDB clusters")
	}

	for _, cluster := range list.Items {
		clusterPatch := &kube.PXCOperatorPatch{
			Spec: kube.PXCOperatorSpec{
				CRVersion: newVersion,
				PXC: kube.PXCSpec{
					Image: strings.Replace(cluster.Spec.PXC.Image, oldVersion, newVersion, 1),
				},
				Backup: kube.ImageSpec{
					Image: strings.Replace(cluster.Spec.Backup.Image, oldVersion, newVersion, 1),
				},
			},
		}

		if cluster.Spec.HAProxy != nil {
			clusterPatch.Spec.HAProxy = kube.ImageSpec{
				Image: strings.Replace(cluster.Spec.HAProxy.Image, oldVersion, newVersion, 1),
			}
		} else {
			clusterPatch.Spec.ProxySQL = kube.ImageSpec{
				Image: strings.Replace(cluster.Spec.ProxySQL.Image, oldVersion, newVersion, 1),
			}
		}

		patch, err := json.Marshal(clusterPatch)
		if err != nil {
			return err
		}
		if _, err := c.kube.PatchPXCCluster(ctx, cluster.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
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
		err = c.kube.ApplyFile(ctx, manifest)
		if err != nil {
			return errors.Wrap(err, "failed to update operator")
		}
	}
	// Change image inside operator deployment.
	deployment, err := c.kube.GetDeployment(ctx, deploymentName)
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
	return c.kube.PatchDeployment(ctx, deploymentName, deployment)
}

func (c *K8sClient) CreateVMOperator(ctx context.Context, params *PMM) error {
	files := []string{
		"deploy/victoriametrics/crs/vmnodescrape.yaml",
		"deploy/victoriametrics/crs/vmpodscrape.yaml",
		"deploy/victoriametrics/kube-state-metrics/service-account.yaml",
		"deploy/victoriametrics/kube-state-metrics/cluster-role.yaml",
		"deploy/victoriametrics/kube-state-metrics/cluster-role-binding.yaml",
		"deploy/victoriametrics/kube-state-metrics/deployment.yaml",
		"deploy/victoriametrics/kube-state-metrics/service.yaml",
		"deploy/victoriametrics/kube-state-metrics.yaml",
	}
	for _, path := range files {
		file, err := dbaascontroller.DeployDir.ReadFile(path)
		if err != nil {
			return err
		}
		err = c.kube.ApplyFile(ctx, file)
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
	return c.kube.Apply(ctx, vmagent)
}

// RemoveVMOperator deletes the VM Operator installed when the cluster was registered.
func (c *K8sClient) RemoveVMOperator(ctx context.Context) error {
	files := []string{
		"deploy/victoriametrics/kube-state-metrics.yaml",
		"deploy/victoriametrics/kube-state-metrics/cluster-role-binding.yaml",
		"deploy/victoriametrics/kube-state-metrics/cluster-role.yaml",
		"deploy/victoriametrics/kube-state-metrics/deployment.yaml",
		"deploy/victoriametrics/kube-state-metrics/service-account.yaml",
		"deploy/victoriametrics/kube-state-metrics/service.yaml",
		"deploy/victoriametrics/crs/vmagent_rbac.yaml",
		"deploy/victoriametrics/crs/vmnodescrape.yaml",
		"deploy/victoriametrics/crs/vmpodscrape.yaml",
	}
	for _, path := range files {
		file, err := dbaascontroller.DeployDir.ReadFile(path)
		if err != nil {
			return err
		}
		err = c.kube.DeleteFile(ctx, file)
		if err != nil {
			return errors.Wrapf(err, "cannot apply file: %q", path)
		}
	}

	return nil
}

// Create the resource from the specs.
func (c *K8sClient) Create(ctx context.Context, resource interface{}) error {
	var err error

	switch res := resource.(type) {
	case string:
		_, err = c.kubeCtl.Run(ctx, []string{"create", "-f", res}, nil)
	case []byte:
		_, err = c.kubeCtl.Run(ctx, []string{"create", "-f", "-"}, res)
	}
	if err != nil {
		return errors.Wrap(err, "cannot create resource")
	}

	return nil
}

// WaitForCondition waits until the condition is met for the specified resource.
func (c *K8sClient) WaitForCondition(ctx context.Context, condition string, resource interface{}) error {
	var err error

	condition = "--for=condition=" + condition

	switch res := resource.(type) {
	case string:
		_, err = c.kubeCtl.Run(ctx, []string{"wait", "-f", res}, nil)
	case []byte:
		_, err = c.kubeCtl.Run(ctx, []string{"wait", "-f", "-"}, res)
	}
	if err != nil {
		return errors.Wrapf(err, "error while waiting for condition %q", condition)
	}

	return nil
}

func vmAgentSpec(params *PMM, secretName string) *monitoring.VMAgent {
	return &monitoring.VMAgent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "VMAgent",
			APIVersion: "operator.victoriametrics.com/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pmm-vmagent-" + secretName,
		},
		Spec: monitoring.VMAgentSpec{
			ServiceScrapeNamespaceSelector: new(metav1.LabelSelector),
			ServiceScrapeSelector:          new(metav1.LabelSelector),
			PodScrapeNamespaceSelector:     new(metav1.LabelSelector),
			PodScrapeSelector:              new(metav1.LabelSelector),
			ProbeSelector:                  new(metav1.LabelSelector),
			ProbeNamespaceSelector:         new(metav1.LabelSelector),
			StaticScrapeSelector:           new(metav1.LabelSelector),
			StaticScrapeNamespaceSelector:  new(metav1.LabelSelector),
			ReplicaCount:                   1,
			SelectAllByDefault:             true,
			Resources: &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("250m"),
					corev1.ResourceMemory: resource.MustParse("350Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("850Mi"),
				},
			},
			ExtraArgs: map[string]string{
				"memory.allowedPercent": "40",
			},
			RemoteWrite: []monitoring.VMAgentRemoteWriteSpec{
				{
					URL: fmt.Sprintf("%s/victoriametrics/api/v1/write", params.PublicAddress),
					TLSConfig: &monitoring.TLSConfig{
						InsecureSkipVerify: true,
					},
					BasicAuth: &monitoring.BasicAuth{
						Username: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
							Key: "username",
						},
						Password: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
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

func (c *K8sClient) getPSMDBSpec(params *PSMDBParams, extra extraCRParams) *psmdbv1.PerconaServerMongoDB {
	maxUnavailable := intstr.FromInt(1)
	res := &psmdbv1.PerconaServerMongoDB{
		TypeMeta: metav1.TypeMeta{
			APIVersion: c.getAPIVersionForPSMDBOperator(extra.operators.PsmdbOperatorVersion),
			Kind:       kube.PSMDBKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-psmdb-pvc"},
		},
		Spec: psmdbv1.PerconaServerMongoDBSpec{
			UpdateStrategy: updateStrategyRollingUpdate,
			CRVersion:      extra.operators.PsmdbOperatorVersion,
			Image:          extra.psmdbImage,
			Secrets: &psmdbv1.SecretsSpec{
				Users: extra.secretName,
			},
			Sharding: psmdbv1.Sharding{
				Enabled: true,
				ConfigsvrReplSet: &psmdbv1.ReplsetSpec{
					Size:       params.Size,
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					Arbiter: psmdbv1.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdbv1.MultiAZ{
							Affinity: extra.affinity,
						},
					},
					MultiAZ: psmdbv1.MultiAZ{
						Affinity: extra.affinity,
					},
				},
				Mongos: &psmdbv1.MongosSpec{
					Size: params.Size,
					MultiAZ: psmdbv1.MultiAZ{
						Affinity: extra.affinity,
					},
					Expose: psmdbv1.MongosExpose{
						ExposeType: extra.expose.ExposeType,
					},
				},
			},
			Replsets: []*psmdbv1.ReplsetSpec{
				// Note: in case to support single node environments
				// we need to expose primary mongodb node
				{
					Name: "rs0",
					Size: params.Size,
					Arbiter: psmdbv1.Arbiter{
						Enabled: false,
						Size:    1,
						MultiAZ: psmdbv1.MultiAZ{
							Affinity: extra.affinity,
						},
					},
					VolumeSpec: c.volumeSpec(params.Replicaset.DiskSize),
					MultiAZ: psmdbv1.MultiAZ{
						PodDisruptionBudget: &psmdbv1.PodDisruptionBudgetSpec{
							MaxUnavailable: &maxUnavailable,
						},
						Affinity:  extra.affinity,
						Resources: c.setComputeResources(params.Replicaset.ComputeResources),
					},
					Configuration: psmdbv1.MongoConfiguration("      operationProfiling:\n" +
						"        mode: " + string(psmdbv1.OperationProfilingModeSlowOp) + "\n"),
				},
			},

			PMM: psmdbv1.PMMSpec{
				Enabled: false,
			},
			Mongod: &psmdbv1.MongodSpec{
				Net: &psmdbv1.MongodSpecNet{
					Port: 27017,
				},
				OperationProfiling: &psmdbv1.MongodSpecOperationProfiling{
					Mode:              psmdbv1.OperationProfilingModeSlowOp,
					SlowOpThresholdMs: 100,
					RateLimit:         100,
				},
				Security: &psmdbv1.MongodSpecSecurity{
					RedactClientLogData:  false,
					EnableEncryption:     pointer.ToBool(true),
					EncryptionKeySecret:  fmt.Sprintf("%s-mongodb-encryption-key", params.Name),
					EncryptionCipherMode: psmdbv1.MongodChiperModeCBC,
				},
				SetParameter: &psmdbv1.MongodSpecSetParameter{
					TTLMonitorSleepSecs: 60,
				},
				Storage: &psmdbv1.MongodSpecStorage{
					Engine: psmdbv1.StorageEngineWiredTiger,
					MMAPv1: &psmdbv1.MongodSpecMMAPv1{
						NsSize:     16,
						Smallfiles: false,
					},
					WiredTiger: &psmdbv1.MongodSpecWiredTiger{
						CollectionConfig: &psmdbv1.MongodSpecWiredTigerCollectionConfig{
							BlockCompressor: &psmdbv1.WiredTigerCompressorSnappy,
						},
						EngineConfig: &psmdbv1.MongodSpecWiredTigerEngineConfig{
							DirectoryForIndexes: false,
							JournalCompressor:   &psmdbv1.WiredTigerCompressorSnappy,
						},
						IndexConfig: &psmdbv1.MongodSpecWiredTigerIndexConfig{
							PrefixCompression: true,
						},
					},
				},
			},

			Backup: psmdbv1.BackupSpec{
				Enabled:            true,
				Image:              extra.backupImage,
				ServiceAccountName: "percona-server-mongodb-operator",
			},
		},
	}

	if params.Replicaset != nil {
		res.Spec.Replsets[0].Resources = c.setComputeResources(params.Replicaset.ComputeResources)
		res.Spec.Sharding.Mongos.Resources = c.setComputeResources(params.Replicaset.ComputeResources)
		// For single node clusters, the operator creates a single instance replicaset.
		// This is an unsafe configuration and the expose config should be applied to the replicaset
		// instead of to the mongos.
		if params.Size == 1 {
			res.Spec.UnsafeConf = true
			res.Spec.Sharding.Enabled = false
			if params.Expose {
				res.Spec.Replsets[0].Expose.Enabled = true
				res.Spec.Sharding.Mongos.Expose.ExposeType = corev1.ServiceTypeClusterIP
			}
		}
	}

	if params.PMM != nil {
		res.Spec.PMM = psmdbv1.PMMSpec{
			Enabled:    true,
			ServerHost: params.PMM.PublicAddress,
			Image:      pmmClientImage,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("300M"),
					corev1.ResourceCPU:    resource.MustParse("500m"),
				},
			},
		}
	}

	return res
}

func (c *K8sClient) createPSMDBSpec(operator *goversion.Version, params *PSMDBParams, extra *extraCRParams) (*psmdbv1.PerconaServerMongoDB, error) {
	spec := new(psmdbv1.PerconaServerMongoDB)
	bytes, err := ioutil.ReadFile(psmdbCRFile)
	if err == nil {
		err = c.unmarshalTemplate(bytes, spec)
		if err != nil {
			return nil, err
		}
		if spec.Spec.Secrets.Users != "" {
			extra.secretName = spec.Spec.Secrets.Users
		}
		if spec.Spec.Secrets.Users == "" {
			spec.Spec.Secrets.Users = extra.secretName
		}
		return c.overridePSMDBSpec(spec, params, *extra), nil
	}
	return c.getPSMDBSpec(params, *extra), nil
}

func (c *K8sClient) createPXCSpecFromParams(params *PXCParams, secretName *string, pxcOperatorVersion, storageName string, serviceType corev1.ServiceType) (*pxcv1.PerconaXtraDBCluster, error) {
	spec := new(pxcv1.PerconaXtraDBCluster)

	bytes, err := ioutil.ReadFile(pxcCRFile)
	if err == nil {
		c.l.Debug("found pxc cr template")
		err = c.unmarshalTemplate(bytes, spec)
		if err != nil {
			return nil, err
		}
		if spec.Spec.SecretsName != "" {
			*secretName = spec.Spec.SecretsName
		}
		if spec.Spec.SecretsName == "" {
			spec.Spec.SecretsName = *secretName
		}
		return c.overridePXCSpec(spec, params, storageName, pxcOperatorVersion), nil

	}
	c.l.Debug("failed openint cr template file. Fallback to defaults")
	return c.getDefaultPXCSpec(params, *secretName, pxcOperatorVersion, storageName, serviceType), nil
}

func (c *K8sClient) overridePSMDBSpec(spec *psmdbv1.PerconaServerMongoDB, params *PSMDBParams, extra extraCRParams) *psmdbv1.PerconaServerMongoDB {
	spec.Spec.Image = extra.psmdbImage
	spec.ObjectMeta.Name = params.Name
	spec.Spec.Sharding.ConfigsvrReplSet.Size = params.Size

	spec.Spec.Replsets[0].Resources = c.setComputeResources(params.Replicaset.ComputeResources)
	spec.Spec.Sharding.Mongos.Resources = c.setComputeResources(params.Replicaset.ComputeResources)
	spec.Spec.Sharding.ConfigsvrReplSet.VolumeSpec = c.volumeSpec(params.Replicaset.DiskSize)
	// FIXME: implement better solution
	if spec.Spec.Backup.Image == "" {
		spec.Spec.Backup = psmdbv1.BackupSpec{
			Enabled:            true,
			Image:              extra.backupImage,
			ServiceAccountName: "percona-server-mongodb-operator",
		}
	}
	if spec.Spec.Backup.Image == "" {
		spec.Spec.Backup.Image = extra.backupImage
	}
	if !params.Expose {
		spec.Spec.Sharding.Mongos.Expose.ExposeType = corev1.ServiceTypeClusterIP
	}

	if params.Size == 1 {
		spec.Spec.UnsafeConf = true
		if params.Expose {
			spec.Spec.Replsets[0].Expose.Enabled = true
			spec.Spec.Replsets[0].Expose.ExposeType = corev1.ServiceTypeClusterIP
			spec.Spec.Sharding.Enabled = false
		}
	}
	// Always override PMM spec
	if params.PMM != nil {
		spec.Spec.PMM = psmdbv1.PMMSpec{
			Enabled:    true,
			ServerHost: params.PMM.PublicAddress,
			Image:      pmmClientImage,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("300M"),
					corev1.ResourceCPU:    resource.MustParse("500m"),
				},
			},
		}
	}

	return spec
}

func (c *K8sClient) overridePXCSpec(spec *pxcv1.PerconaXtraDBCluster, params *PXCParams, storageName, pxcOperatorVersion string) *pxcv1.PerconaXtraDBCluster {
	if params.PXC.Image != "" {
		spec.Spec.PXC.PodSpec.Image = params.PXC.Image
	}
	spec.ObjectMeta.Name = params.Name
	spec.Spec.PXC.PodSpec.Size = params.Size
	spec.Spec.PXC.PodSpec.Resources = c.setComputeResources(params.PXC.ComputeResources)
	if spec.Spec.PXC.PodSpec.VolumeSpec != nil && spec.Spec.PXC.PodSpec.VolumeSpec.PersistentVolumeClaim != nil && spec.Spec.PXC.PodSpec.VolumeSpec.PersistentVolumeClaim.StorageClassName != nil {
		spec.Spec.PXC.PodSpec.VolumeSpec.PersistentVolumeClaim.Resources.Requests = corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse(params.PXC.DiskSize),
		}
	} else {
		spec.Spec.PXC.PodSpec.VolumeSpec = c.pxcVolumeSpec(params.PXC.DiskSize)
	}
	if spec.Spec.Backup == nil {
		spec.Spec.Backup = &pxcv1.PXCScheduledBackup{
			Image: fmt.Sprintf(pxcBackupImageTemplate, pxcOperatorVersion),
			Schedule: []pxcv1.PXCScheduledBackupSchedule{{
				Name:        "test",
				Schedule:    "*/30 * * * *",
				Keep:        3,
				StorageName: storageName,
			}},
			Storages: map[string]*pxcv1.BackupStorageSpec{
				storageName: {
					Type:   pxcv1.BackupStorageFilesystem,
					Volume: c.pxcVolumeSpec(params.PXC.DiskSize),
				},
			},
			ServiceAccountName: "percona-xtradb-cluster-operator",
		}
	}
	if spec.Spec.Backup.Image == "" {
		spec.Spec.Backup.Image = fmt.Sprintf(pxcBackupImageTemplate, pxcOperatorVersion)
	}
	if len(spec.Spec.Backup.Storages) == 0 {
		spec.Spec.Backup.Storages = map[string]*pxcv1.BackupStorageSpec{
			storageName: {
				Type:   pxcv1.BackupStorageFilesystem,
				Volume: c.pxcVolumeSpec(params.PXC.DiskSize),
			},
		}
	}
	if !params.Expose {
		spec.Spec.PXC.Expose = pxcv1.ServiceExpose{Enabled: false}
	}
	if params.ProxySQL != nil && spec.Spec.ProxySQL != nil {
		spec.Spec.ProxySQL.Resources = c.setComputeResources(params.ProxySQL.ComputeResources)
		spec.Spec.ProxySQL.VolumeSpec = c.pxcVolumeSpec(params.ProxySQL.DiskSize)
	}
	if params.HAProxy != nil && spec.Spec.HAProxy != nil {
		spec.Spec.HAProxy.Resources = c.setComputeResources(params.HAProxy.ComputeResources)
		if params.HAProxy.Image != "" {
			spec.Spec.HAProxy.Image = params.HAProxy.Image
		}
	}
	// Always override defaults for PMM by specified by user
	if params.PMM != nil {
		spec.Spec.PMM = &pxcv1.PMMSpec{
			Enabled:         true,
			ServerHost:      params.PMM.PublicAddress,
			ServerUser:      params.PMM.Login,
			Image:           pmmClientImage,
			ImagePullPolicy: corev1.PullPolicy(string(pullPolicy)),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("300M"),
					corev1.ResourceCPU:    resource.MustParse("500m"),
				},
			},
		}
	}

	return spec
}

func (c *K8sClient) getDefaultPXCSpec(params *PXCParams, secretName, pxcOperatorVersion, storageName string, serviceType corev1.ServiceType) *pxcv1.PerconaXtraDBCluster {
	pxcImage := pxcDefaultImage
	if params.PXC.Image != "" {
		pxcImage = params.PXC.Image
	}
	maxUnavailable := intstr.FromInt(1)
	spec := &pxcv1.PerconaXtraDBCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: c.getAPIVersionForPXCOperator(pxcOperatorVersion),
			Kind:       kube.PXCKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       params.Name,
			Finalizers: []string{"delete-proxysql-pvc", "delete-pxc-pvc"},
		},
		Spec: pxcv1.PerconaXtraDBClusterSpec{
			UpdateStrategy:    updateStrategyRollingUpdate,
			CRVersion:         pxcOperatorVersion,
			AllowUnsafeConfig: true,
			SecretsName:       secretName,

			PXC: &pxcv1.PXCSpec{
				PodSpec: &pxcv1.PodSpec{
					Size:            params.Size,
					Resources:       c.setComputeResources(params.PXC.ComputeResources),
					Image:           pxcImage,
					ImagePullPolicy: corev1.PullPolicy(string(pullPolicy)),
					VolumeSpec:      c.pxcVolumeSpec(params.PXC.DiskSize),
					Affinity: &pxcv1.PodAffinity{
						TopologyKey: pointer.ToString(pxcv1.AffinityTopologyKeyOff),
					},
					PodDisruptionBudget: &pxcv1.PodDisruptionBudgetSpec{
						MaxUnavailable: &maxUnavailable,
					},
				},
			},

			PMM: &pxcv1.PMMSpec{
				Enabled: false,
			},

			Backup: &pxcv1.PXCScheduledBackup{
				Image: fmt.Sprintf(pxcBackupImageTemplate, pxcOperatorVersion),
				Schedule: []pxcv1.PXCScheduledBackupSchedule{{
					Name:        "test",
					Schedule:    "*/30 * * * *",
					Keep:        3,
					StorageName: storageName,
				}},
				Storages: map[string]*pxcv1.BackupStorageSpec{
					storageName: {
						Type:   pxcv1.BackupStorageFilesystem,
						Volume: c.pxcVolumeSpec(params.PXC.DiskSize),
					},
				},
				ServiceAccountName: "percona-xtradb-cluster-operator",
			},
		},
	}

	if params.PMM != nil {
		spec.Spec.PMM = &pxcv1.PMMSpec{
			Enabled:         true,
			ServerHost:      params.PMM.PublicAddress,
			ServerUser:      params.PMM.Login,
			Image:           pmmClientImage,
			ImagePullPolicy: corev1.PullPolicy(string(pullPolicy)),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("300M"),
					corev1.ResourceCPU:    resource.MustParse("500m"),
				},
			},
		}
	}

	podSpec := pxcv1.PodSpec{
		Enabled:         true,
		ImagePullPolicy: corev1.PullPolicy(string(pullPolicy)),
		Size:            params.Size,
		Affinity: &pxcv1.PodAffinity{
			TopologyKey: pointer.ToString(pxcv1.AffinityTopologyKeyOff),
		},
	}
	if len(serviceType) > 0 {
		podSpec.ServiceType = serviceType
	}
	if params.ProxySQL != nil {
		spec.Spec.ProxySQL = &podSpec
		spec.Spec.ProxySQL.Image = fmt.Sprintf(pxcProxySQLDefaultImageTemplate, pxcOperatorVersion)
		if params.ProxySQL.Image != "" {
			spec.Spec.ProxySQL.Image = params.ProxySQL.Image
		}
		spec.Spec.ProxySQL.Resources = c.setComputeResources(params.ProxySQL.ComputeResources)
		spec.Spec.ProxySQL.VolumeSpec = c.pxcVolumeSpec(params.ProxySQL.DiskSize)
	} else {
		spec.Spec.HAProxy = new(pxcv1.HAProxySpec)
		podSpec.Image = fmt.Sprintf(pxcHAProxyDefaultImageTemplate, pxcOperatorVersion)
		if params.HAProxy.Image != "" {
			podSpec.Image = params.HAProxy.Image
		}
		podSpec.Resources = c.setComputeResources(params.HAProxy.ComputeResources)
		spec.Spec.HAProxy.PodSpec = podSpec
	}

	return spec
}

func (c *K8sClient) unmarshalTemplate(body []byte, out interface{}) error {
	var yamlObj interface{}
	err := yaml.Unmarshal(body, &yamlObj)
	if err != nil {
		return err
	}
	yamlObj = convert(yamlObj)
	jsonData, err := json.Marshal(yamlObj)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, out)
}

func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := make(map[string]interface{})
		for k, v := range x {
			m2[k.(string)] = convert(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}
