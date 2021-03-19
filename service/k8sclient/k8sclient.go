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
	"fmt"
	"math/rand"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/avast/retry-go"
	"github.com/pkg/errors"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kubectl"
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
	passwordLength           = 24
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
)

const (
	pmmClientImage = "perconalab/pmm-client:dev-latest"

	k8sAPIVersion     = "v1"
	k8sMetaKindSecret = "Secret"

	pxcCRVersion         = "1.7.0"
	pxcBackupImage       = "percona/percona-xtradb-cluster-operator:1.6.0-pxc8.0-backup"
	pxcImage             = "percona/percona-xtradb-cluster:8.0.20-11.1"
	pxcBackupStorageName = "pxc-backup-storage-%s"
	pxcAPIVersion        = "pxc.percona.com/v1-6-0"
	pxcProxySQLImage     = "percona/percona-xtradb-cluster-operator:1.6.0-proxysql"
	pxcSecretNameTmpl    = "dbaas-%s-pxc-secrets"
	defaultPXCSecretName = "my-cluster-secrets"

	psmdbCRVersion         = "1.6.0"
	psmdbBackupImage       = "percona/percona-server-mongodb-operator:1.5.0-backup"
	psmdbImage             = "percona/percona-server-mongodb:4.2.8-8"
	psmdbAPIVersion        = "psmdb.percona.com/v1-6-0"
	psmdbSecretNameTmpl    = "dbaas-%s-psmdb-secrets"
	defaultPSMDBSecretName = "my-cluster-name-secrets"

	// Max size of volume for AWS Elastic Block Storage service is 16TiB
	maxVolumeSizeEBS uint64 = 16 * 1024 * 1024 * 1024 * 1024
)

type kubernetesClusterType uint8

const (
	clusterTypeUnknown kubernetesClusterType = iota
	amazonEKS
	minikube
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

// Option is meant to be bitmask to specify k8sClient's options.
type Option uint64

const (
	// UseCacheOption togles use of cache.
	UseCacheOption Option = Option(kubectl.UseCacheOption)
)

// Operators contains statuses of operators.
type Operators struct {
	Xtradb OperatorStatus
	Psmdb  OperatorStatus
}

// ComputeResources represents container computer resources requests or limits.
type ComputeResources struct {
	CPUM        string
	MemoryBytes string
}

// PXC contains information related to PXC containers in Percona XtraDB cluster.
type PXC struct {
	ComputeResources *ComputeResources
	DiskSize         string
}

// ProxySQL contains information related to ProxySQL containers in Percona XtraDB cluster.
type ProxySQL struct {
	ComputeResources *ComputeResources
	DiskSize         string
}

// Replicaset contains information related to Replicaset containers in PSMDB cluster.
type Replicaset struct {
	ComputeResources *ComputeResources
	DiskSize         string
}

// XtraDBParams contains all parameters required to create or update Percona XtraDB cluster.
type XtraDBParams struct {
	Name             string
	PMMPublicAddress string
	Size             int32
	Suspend          bool
	Resume           bool
	PXC              *PXC
	ProxySQL         *ProxySQL
}

// Cluster contains common information related to cluster.
type Cluster struct {
	Name string
}

// PSMDBParams contains all parameters required to create or update percona server for mongodb cluster.
type PSMDBParams struct {
	Name             string
	PMMPublicAddress string
	Size             int32
	Suspend          bool
	Resume           bool
	Replicaset       *Replicaset
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
	Pause         bool
	DetailedState DetailedState
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
	kubeCtl *kubectl.KubeCtl
	l       logger.Logger
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
func New(ctx context.Context, kubeconfig string, options ...Option) (*K8sClient, error) {
	l := logger.Get(ctx)
	l = l.WithField("component", "K8sClient")
	kubectlOptions := make([]kubectl.Option, 0, len(options))
	for _, option := range options {
		kubectlOptions = append(kubectlOptions, kubectl.Option(option))
	}
	kubeCtl, err := kubectl.NewKubeCtl(ctx, kubeconfig, kubectlOptions...)
	if err != nil {
		return nil, err
	}
	return &K8sClient{
		kubeCtl: kubeCtl,
		l:       l,
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

func randSeq(n int) string {
	rand.Seed(time.Now().UnixNano())
	// PSMDB do not support all special characters in password https://jira.percona.com/browse/K8SPSMDB-364
	symbols := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789~=+%^*/(){}!$|")
	symbolsLen := len(symbols)
	b := make([]rune, n)
	for i := range b {
		b[i] = symbols[rand.Intn(symbolsLen)]
	}
	return string(b)
}

// CreateXtraDBCluster creates Percona XtraDB cluster with provided parameters.
func (c *K8sClient) CreateXtraDBCluster(ctx context.Context, params *XtraDBParams) error {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), params.Name, &cluster)
	if err == nil {
		return fmt.Errorf(clusterWithSameNameExistsErrTemplate, params.Name)
	}

	var secret common.Secret
	err = c.kubeCtl.Get(ctx, k8sMetaKindSecret, defaultPXCSecretName, &secret)
	if err != nil {
		return errors.Wrap(err, "cannot get default PXC secrets")
	}

	secretName := fmt.Sprintf(pxcSecretNameTmpl, params.Name)
	pwd := randSeq(passwordLength)

	// TODO: add a link to ticket to set random password for all other users.
	data := secret.Data
	data["root"] = []byte(pwd)

	err = c.CreateSecret(ctx, secretName, data)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}
	storageName := fmt.Sprintf(pxcBackupStorageName, params.Name)
	res := &pxc.PerconaXtraDBCluster{
		TypeMeta: common.TypeMeta{
			APIVersion: pxcAPIVersion,
			Kind:       string(perconaXtraDBClusterKind),
		},
		ObjectMeta: common.ObjectMeta{
			Name: params.Name,
		},
		Spec: pxc.PerconaXtraDBClusterSpec{
			CRVersion:         pxcCRVersion,
			AllowUnsafeConfig: true,
			SecretsName:       secretName,

			PXC: &pxc.PodSpec{
				Size:       params.Size,
				Resources:  c.setComputeResources(params.PXC.ComputeResources),
				Image:      pxcImage,
				VolumeSpec: c.volumeSpec(params.PXC.DiskSize),
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
				PodDisruptionBudget: &common.PodDisruptionBudgetSpec{
					MaxUnavailable: pointer.ToInt(1),
				},
			},

			ProxySQL: &pxc.PodSpec{
				Enabled:    true,
				Size:       params.Size,
				Resources:  c.setComputeResources(params.ProxySQL.ComputeResources),
				Image:      pxcProxySQLImage,
				VolumeSpec: c.volumeSpec(params.ProxySQL.DiskSize),
				Affinity: &pxc.PodAffinity{
					TopologyKey: pointer.ToString(pxc.AffinityTopologyKeyOff),
				},
			},

			PMM: &pxc.PMMSpec{
				Enabled:    params.PMMPublicAddress != "",
				ServerHost: params.PMMPublicAddress,
				ServerUser: "admin",
				Image:      pmmClientImage,
				Resources: &common.PodResources{
					Requests: &common.ResourcesList{
						Memory: "500M",
						CPU:    "500m",
					},
				},
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

	// This enables ingress for the cluster and exposes the cluster to the world.
	// The cluster will have an internal IP and a world accessible hostname.
	// This feature cannot be tested with minikube. Please use EKS for testing.
	if clusterType := c.getKubernetesClusterType(ctx); clusterType != minikube {
		res.Spec.ProxySQL.ServiceType = common.ServiceTypeLoadBalancer
	}

	return c.kubeCtl.Apply(ctx, res)
}

// UpdateXtraDBCluster changes size of provided Percona XtraDB cluster.
func (c *K8sClient) UpdateXtraDBCluster(ctx context.Context, params *XtraDBParams) error {
	var cluster pxc.PerconaXtraDBCluster
	err := c.kubeCtl.Get(ctx, string(perconaXtraDBClusterKind), params.Name, &cluster)
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
		cluster.Spec.PXC.Size = params.Size
		cluster.Spec.ProxySQL.Size = params.Size
	}

	if params.PXC != nil {
		cluster.Spec.PXC.Resources = c.updateComputeResources(params.PXC.ComputeResources, cluster.Spec.PXC.Resources)
	}

	if params.ProxySQL != nil {
		cluster.Spec.ProxySQL.Resources = c.updateComputeResources(params.ProxySQL.ComputeResources, cluster.Spec.ProxySQL.Resources)
	}

	return c.kubeCtl.Apply(ctx, &cluster)
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

	secret := &common.Secret{
		TypeMeta: common.TypeMeta{
			APIVersion: k8sAPIVersion,
			Kind:       k8sMetaKindSecret,
		},
		ObjectMeta: common.ObjectMeta{
			Name: fmt.Sprintf(pxcSecretNameTmpl, name),
		},
	}

	err = c.kubeCtl.Delete(ctx, secret)
	if err != nil {
		c.l.Errorf("cannot delete secret for %s: %v", name, err)
	}

	return nil
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
	if cluster.Status.Status != pxc.AppStateReady {
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

func (c *K8sClient) getKubernetesClusterType(ctx context.Context) kubernetesClusterType {
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
			return amazonEKS
		}
		if strings.Contains(class.Provisioner, "minikube") {
			return minikube
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

	// TODO: implement logic to handle the case when there is HAProxy instead of ProxySQL.
	_, err = c.kubeCtl.Run(ctx, c.restartDBClusterCmd(name, "proxysql"), nil)
	if err != nil {
		return err
	}

	return nil
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
			Size:    cluster.Spec.ProxySQL.Size,
			State:   getPXCState(cluster.Status.Status),
			Message: strings.Join(cluster.Status.Messages, ";"),
			PXC: &PXC{
				DiskSize:         c.getDiskSize(cluster.Spec.PXC.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.PXC.Resources),
			},
			ProxySQL: &ProxySQL{
				DiskSize:         c.getDiskSize(cluster.Spec.ProxySQL.VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.ProxySQL.Resources),
			},
			Pause: cluster.Spec.Pause,
			DetailedState: []appStatus{
				{size: cluster.Status.PMM.Size, ready: cluster.Status.PMM.Ready},
				{size: cluster.Status.HAProxy.Size, ready: cluster.Status.HAProxy.Ready},
				{size: cluster.Status.ProxySQL.Size, ready: cluster.Status.ProxySQL.Ready},
				{size: cluster.Status.PXC.Size, ready: cluster.Status.PXC.Ready},
			},
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

	var secret common.Secret
	err = c.kubeCtl.Get(ctx, k8sMetaKindSecret, defaultPSMDBSecretName, &secret)
	if err != nil {
		return errors.Wrap(err, "cannot get default PSMDB secrets")
	}

	secretName := fmt.Sprintf(psmdbSecretNameTmpl, params.Name)
	pwd := randSeq(passwordLength)

	data := secret.Data
	data["MONGODB_USER_ADMIN_PASSWORD"] = []byte(pwd)

	err = c.CreateSecret(ctx, secretName, data)
	if err != nil {
		return errors.Wrap(err, "cannot create secret for PXC")
	}

	affinity := new(psmdb.PodAffinity)
	var expose psmdb.Expose
	if clusterType := c.getKubernetesClusterType(ctx); clusterType != minikube {
		affinity.TopologyKey = pointer.ToString("kubernetes.io/hostname")

		// This enables ingress for the cluster and exposes the cluster to the world.
		// The cluster will have an internal IP and a world accessible hostname.
		// This feature cannot be tested with minikube. Please use EKS for testing.
		expose = psmdb.Expose{
			Enabled:    true,
			ExposeType: common.ServiceTypeLoadBalancer,
		}
	} else {
		// https://www.percona.com/doc/kubernetes-operator-for-psmongodb/minikube.html
		// > Install Percona Server for MongoDB on Minikube
		// > ...
		// > set affinity.antiAffinityTopologyKey key to "none"
		// > (the Operator will be unable to spread the cluster on several nodes)
		affinity.TopologyKey = pointer.ToString(psmdb.AffinityOff)
	}
	res := &psmdb.PerconaServerMongoDB{
		TypeMeta: common.TypeMeta{
			APIVersion: psmdbAPIVersion,
			Kind:       string(perconaServerMongoDBKind),
		},
		ObjectMeta: common.ObjectMeta{
			Name: params.Name,
		},
		Spec: psmdb.PerconaServerMongoDBSpec{
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
					EncryptionKeySecret:  "my-cluster-name-mongodb-encryption-key",
					EncryptionCipherMode: psmdb.MongodChiperModeCBC,
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

			PMM: psmdb.PmmSpec{
				Enabled:    params.PMMPublicAddress != "",
				ServerHost: params.PMMPublicAddress,
				Image:      pmmClientImage,
				Resources: &common.PodResources{
					Requests: &common.ResourcesList{
						Memory: "500M",
						CPU:    "500m",
					},
				},
			},

			Backup: psmdb.BackupSpec{
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

	return c.kubeCtl.Apply(ctx, res)
}

// UpdatePSMDBCluster changes size of provided percona server for mongodb cluster.
func (c *K8sClient) UpdatePSMDBCluster(ctx context.Context, params *PSMDBParams) error {
	var cluster psmdb.PerconaServerMongoDB
	err := c.kubeCtl.Get(ctx, string(perconaServerMongoDBKind), params.Name, &cluster)
	if err != nil {
		return errors.Wrap(err, "UpdatePSMDBCluster get error")
	}

	// This is to prevent concurrent updates
	if cluster.Status.Status != psmdb.AppStateReady {
		return errors.Wrapf(ErrPSMDBClusterNotReady, "state is %v", cluster.Status.Status) //nolint:wrapcheck
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

	return c.kubeCtl.Apply(ctx, cluster)
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

	secret := &common.Secret{
		TypeMeta: common.TypeMeta{
			APIVersion: k8sAPIVersion,
			Kind:       k8sMetaKindSecret,
		},
		ObjectMeta: common.ObjectMeta{
			Name: fmt.Sprintf(psmdbSecretNameTmpl, name),
		},
	}

	err = c.kubeCtl.Delete(ctx, secret)
	if err != nil {
		c.l.Errorf("cannot delete secret for %s: %v", name, err)
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
	if cluster.Status.Status != psmdb.AppStateReady {
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

		val := PSMDBCluster{
			Name:    cluster.Name,
			Size:    cluster.Spec.Replsets[0].Size,
			State:   getReplicasetStatus(cluster),
			Pause:   cluster.Spec.Pause,
			Message: message,
			Replicaset: &Replicaset{
				DiskSize:         c.getDiskSize(cluster.Spec.Replsets[0].VolumeSpec),
				ComputeResources: c.getComputeResources(cluster.Spec.Replsets[0].Resources),
			},
			DetailedState: status,
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

func (c *K8sClient) checkOperatorStatus(installedVersions []string, expectedAPIVersion string) (operator OperatorStatus) {
	apiNamespace := strings.Split(expectedAPIVersion, "/")[0]
	installed := false
	for _, version := range installedVersions {
		switch {
		case version == expectedAPIVersion:
			return OperatorStatusOK
		case strings.HasPrefix(version, apiNamespace):
			installed = true
		}
	}
	if installed {
		return OperatorStatusUnsupported
	}
	return OperatorStatusNotInstalled
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
func (c *K8sClient) GetAllClusterResources(ctx context.Context) (
	cpuMillis uint64, memoryBytes uint64, diskSizeBytes uint64, err error,
) {
	clusterType := c.getKubernetesClusterType(ctx)

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
		case minikube:
			storage, ok := node.Status.Allocatable[common.ResourceEphemeralStorage]
			if !ok {
				return 0, 0, 0, errors.Errorf("could not get storage size of the node")
			}
			bytes, err := convertors.StrToBytes(storage)
			if err != nil {
				return 0, 0, 0, errors.Wrapf(err, "could not convert storage size '%s' to bytes", storage)
			}
			diskSizeBytes += bytes
		case amazonEKS:
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
	if clusterType == amazonEKS {
		volumes, err := c.GetPersistentVolumes(ctx)
		if err != nil {
			return 0, 0, 0, errors.Wrap(err, "failed to get persistent volumes")
		}

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
func (c *K8sClient) GetConsumedDiskBytes(ctx context.Context) (consumedBytes uint64, err error) {
	clusterType := c.getKubernetesClusterType(ctx)

	switch clusterType {
	case minikube:
		nodes, err := c.getWorkerNodes(ctx)
		if err != nil {
			return 0, errors.Wrap(err, "can't compute consumed disk size: failed to get worker nodes")
		}
		for _, node := range nodes {
			method := "GET"
			endpoint := fmt.Sprintf("/v1/nodes/%s/proxy/stats/summary", node.Name)
			summary := new(common.NodeSummary)
			err = c.doAPIRequest(ctx, method, endpoint, &summary)
			if err != nil {
				return 0, errors.Wrap(err, "can't compute consumed disk size: failed to get worker nodes summary")
			}
			consumedBytes += summary.Node.FileSystem.UsedBytes
		}
		return consumedBytes, nil
	case amazonEKS:
		volumes, err := c.GetPersistentVolumes(ctx)
		if err != nil {
			return 0, errors.Wrap(err, "failed to get persistent volumes")
		}
		consumedBytes, err := sumVolumesSize(volumes)
		if err != nil {
			return 0, errors.Wrap(err, "failed to sum persistent volumes storage sizes")
		}
		return consumedBytes, nil
	}

	return 0, nil
}

// doAPIRequest starts kubectl proxy, does the request described using given endpoint and method,
// unmarshals the result into the variable out and stops kubectl proxy.
func (c *K8sClient) doAPIRequest(ctx context.Context, method, endpoint string, out interface{}) error {
	if reflect.ValueOf(out).Kind() != reflect.Ptr {
		return errors.New("output expected to be pointer")
	}

	port, err := c.kubeCtl.RunProxy(ctx)
	if err != nil {
		return err
	}
	defer func() {
		err := c.kubeCtl.StopProxy()
		if err != nil {
			c.l.Error(err)
		}
	}()

	client := http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			MaxIdleConns:    1,
			IdleConnTimeout: 10 * time.Second,
		},
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost:"+port+"/api"+endpoint, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create Kubernetes API request")
	}
	var resp *http.Response
	retry.Do(
		func() error {
			resp, err = client.Do(req)
			return err
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to do Kubernetes API request")
	}

	defer resp.Body.Close() //nolint:errcheck
	return json.NewDecoder(resp.Body).Decode(out)
}
