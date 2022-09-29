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

// Package operator contains logic related to kubernetes operators.
package olm

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	operatorsv1 "github.com/operator-framework/api/pkg/operators/v1"
	v1 "github.com/operator-framework/api/pkg/operators/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operators "github.com/operator-framework/operator-lifecycle-manager/pkg/package-server/apis/operators"
	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

const (
	olmRepo              = "operator-lifecycle-manager"
	githubAPIURLTemplate = "https://api.github.com/repos/operator-framework/%s/releases/latest"
	baseDownloadURL      = "github.com/operator-framework/operator-lifecycle-manager/releases/download"
	olmNamespace         = "olm"

	// If version is not set, DBaaS controller will choose the latest from the repo.
	// It doesn't work for offline installation.
	latestOLMVersion  = "latest"
	defaultOLMVersion = ""

	APIVersionCoreosV1 = "operators.coreos.com/v1"
)

// ErrEmptyVersionTag Got an empty version tag from GitHub API.
var ErrEmptyVersionTag = errors.New("got an empty version tag from Github")

// OperatorService holds methods to handle the OLM operator.
type OperatorService struct {
	kubeConfig string
	client     *k8sclient.K8sClient
}

// NewOperatorService returns new OperatorService instance.
func NewOperatorService(client *k8sclient.K8sClient) *OperatorService {
	return &OperatorService{
		client: client,
	} //nolint:exhaustruct
}

// NewOperatorServiceFromConfig returns new OperatorService instance and intializes the config.
func NewOperatorServiceFromConfig(kubeConfig string) *OperatorService {
	return &OperatorService{ //nolint:exhaustruct
		kubeConfig: kubeConfig,
	}
}

// InstallOLMOperator installs the OLM in the Kubernetes cluster.
func (o *OperatorService) InstallOLMOperator(ctx context.Context, req *controllerv1beta1.InstallOLMOperatorRequest) (*controllerv1beta1.InstallOLMOperatorResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	response := &controllerv1beta1.InstallOLMOperatorResponse{}

	if isInstalled(ctx, client, olmNamespace) {
		return response, nil // No error, already installed.
	}

	var crdFile, olmFile interface{}

	switch strings.ToLower(req.Version) {
	case latestOLMVersion:
		latestVersion, err := o.getLatestVersion(ctx, olmRepo)
		if err != nil {
			return nil, err
		}
		crdFile = "https://" + path.Join(baseDownloadURL, latestVersion, "crds.yaml")
		olmFile = "https://" + path.Join(baseDownloadURL, latestVersion, "olm.yaml")
	case defaultOLMVersion:
		crdFile, err = dbaascontroller.DeployDir.ReadFile("deploy/olm/crds.yaml")
		if err != nil {
			return nil, err
		}
		olmFile, err = dbaascontroller.DeployDir.ReadFile("deploy/olm/olm.yaml")
		if err != nil {
			return nil, err
		}
	}

	if err := client.Create(ctx, crdFile); err != nil {
		// TODO: revert applied files before return
		return nil, errors.Wrapf(err, "cannot apply %q file", crdFile)
	}
	client.WaitForCondition(ctx, "Established", crdFile)

	if err := client.Create(ctx, olmFile); err != nil {
		// TODO: revert applied files before return
		return nil, errors.Wrapf(err, "cannot apply %q file", olmFile)
	}

	if err := waitForDeployments(ctx, client, "olm"); err != nil {
		return nil, errors.Wrap(err, "error waiting olm deployments")
	}

	if err := waitForPackageServer(ctx, client, "olm"); err != nil {
		return nil, errors.Wrap(err, "error waiting olm package server to become ready")
	}

	return response, nil
}

func (o *OperatorService) GetInstallPlans(ctx context.Context, namespace string) (*operatorsv1alpha1.InstallPlanList, error) {
	data, err := o.client.Run(ctx, []string{"get", "installplans", "-ojson", "-n", namespace})
	if err != nil {
		return nil, errors.Wrap(err, "cannot get operators group list")
	}

	var installPlans operatorsv1alpha1.InstallPlanList
	err = json.Unmarshal(data, &installPlans)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode install plans from the response")
	}

	return &installPlans, nil
}

type approveInstallPlanSpec struct {
	Approved bool `json:"approved"`
}

type approveInstallPlan struct {
	Spec approveInstallPlanSpec `json:"spec"`
}

func (o *OperatorService) ApproveInstallPlan(ctx context.Context, namespace, resourceName string) error {
	res := approveInstallPlan{Spec: approveInstallPlanSpec{Approved: true}}
	return o.client.Patch(ctx, "merge", "installplan", resourceName, namespace, res)
}

func waitForPackageServer(ctx context.Context, client *k8sclient.K8sClient, namespace string) error {
	var err error
	var data []byte

	for i := 0; i < 15; i++ {
		data, err = client.Run(ctx, []string{"get", "csv", "-n", namespace, "packageserver", "-o", "jsonpath='{.status.phase}'"})
		if err == nil && string(data) == "Succeeded" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	return err
}

type configGetter struct {
	kubeconfig string
}

func NewConfigGetter(kubeconfig string) *configGetter {
	return &configGetter{kubeconfig: kubeconfig}
}

// loadFromString takes a kubeconfig and deserializes the contents into Config object.
func (g *configGetter) loadFromString() (*clientcmdapi.Config, error) {
	config, err := clientcmd.Load([]byte(g.kubeconfig))
	if err != nil {
		return nil, err
	}

	if config.AuthInfos == nil {
		config.AuthInfos = make(map[string]*clientcmdapi.AuthInfo)
	}
	if config.Clusters == nil {
		config.Clusters = make(map[string]*clientcmdapi.Cluster)
	}
	if config.Contexts == nil {
		config.Contexts = make(map[string]*clientcmdapi.Context)
	}

	return config, nil
}

// AvailableOperators resturns the list of available operators for a given namespace and filter.
func (o *OperatorService) AvailableOperators(ctx context.Context, name string) (*operators.PackageManifestList, error) {
	data, err := o.client.Run(ctx, []string{"get", "packagemanifest", "-ojson", "-n=olm", "--field-selector", "metadata.name=" + name})
	if err != nil {
		return nil, errors.Wrap(err, "cannot get operators group list")
	}

	var manifestList operators.PackageManifestList
	err = json.Unmarshal(data, &manifestList)
	if err != nil {
		return nil, errors.Wrap(err, "cannot decode package manifest list response")
	}

	return &manifestList, nil
}

type OperatorInstallRequest struct {
	Namespace              string
	Name                   string
	OperatorGroup          string
	CatalogSource          string
	CatalogSourceNamespace string
	Channel                string
	InstallPlanApproval    operatorsv1alpha1.Approval
	StartingCSV            string
}

func (o *OperatorService) InstallOperator(ctx context.Context, params OperatorInstallRequest) error {
	exists, err := namespaceExists(ctx, o.client, params.Namespace)
	if err != nil {
		return errors.Wrapf(err, "cannot determine is the namespace %q exists", params.Namespace)
	}
	if !exists {
		if _, err := o.client.Run(ctx, []string{"create", "namespace", params.Namespace}); err != nil {
			return errors.Wrap(err, "cannot create namespace for subscription")
		}
	}

	ogExists, err := o.operatorGroupExists(ctx, o.client, params.Namespace, params.OperatorGroup)
	if err != nil {
		return errors.Wrap(err, "cannot check if oprator group exists")
	}

	if !ogExists {
		if err := o.createOperatorGroup(ctx, params.Namespace, params.OperatorGroup); err != nil {
			return errors.Wrapf(err, "cannot create operator group %q", params.OperatorGroup)
		}
	}

	return o.createSubscription(ctx, params)
}

func isInstalled(ctx context.Context, client *k8sclient.K8sClient, namespace string) bool {
	if _, err := client.Run(ctx, []string{"get", "deployment", "olm-operator", "-n", namespace}); err == nil {
		return true
	}
	return false
}

func (o *OperatorService) operatorGroupExists(ctx context.Context, client *k8sclient.K8sClient, namespace, name string) (bool, error) {
	var operatorGroupList operatorsv1.OperatorGroupList

	data, err := client.Run(ctx, []string{"get", "operatorgroups", "-ojson", "--field-selector", "metadata.name=" + name})
	if err != nil {
		return false, errors.Wrap(err, "cannot get operators group list")
	}

	if err := json.Unmarshal(data, &operatorGroupList); err != nil {
		return false, errors.Wrap(err, "cannot decode operator group list response")
	}

	if len(operatorGroupList.Items) > 0 {
		return true, nil
	}

	return false, nil
}

func (o *OperatorService) createOperatorGroup(ctx context.Context, namespace, name string) error {
	op := &v1.OperatorGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersionCoreosV1,
			Kind:       operatorsv1.OperatorGroupKind,
		},

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.OperatorGroupSpec{
			TargetNamespaces: []string{namespace},
		},
		Status: v1.OperatorGroupStatus{
			LastUpdated: &metav1.Time{
				Time: time.Now(),
			},
		},
	}

	payload, err := json.Marshal(op)
	if err != nil {
		return errors.Wrap(err, "cannot encode subscription payload as yaml")
	}

	return o.client.Create(ctx, payload)
}

func (o *OperatorService) createSubscription(ctx context.Context, req OperatorInstallRequest) error {
	subscription := &operatorsv1alpha1.Subscription{
		TypeMeta: metav1.TypeMeta{
			Kind:       operatorsv1alpha1.SubscriptionKind,
			APIVersion: operatorsv1alpha1.SubscriptionCRDAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      "my-" + req.Name,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{
			CatalogSource:          req.CatalogSource,
			CatalogSourceNamespace: req.CatalogSourceNamespace,
			Channel:                req.Channel,
			Package:                req.Name,
			InstallPlanApproval:    req.InstallPlanApproval,
			StartingCSV:            req.StartingCSV,
		},
		Status: operatorsv1alpha1.SubscriptionStatus{
			LastUpdated: metav1.Time{
				Time: time.Now(),
			},
		},
	}

	payload, err := json.Marshal(subscription)
	if err != nil {
		return errors.Wrap(err, "cannot encode subscription payload as yaml")
	}

	return o.client.Create(ctx, payload)
}

func namespaceExists(ctx context.Context, client *k8sclient.K8sClient, namespace string) (bool, error) {
	var namespaceList corev1.NamespaceList

	data, err := client.Run(ctx, []string{"get", "namespaces", "--field-selector", "metadata.name=" + namespace, "-ojson"})
	if err != nil {
		return false, errors.Wrap(err, "cannot get namespaces list")
	}

	if err := json.Unmarshal(data, &namespaceList); err != nil {
		return false, errors.Wrap(err, "cannot decode namespaces list response")
	}

	if len(namespaceList.Items) > 0 {
		return true, nil
	}

	return false, nil
}

func waitForDeployments(ctx context.Context, client *k8sclient.K8sClient, namespace string) error {
	if _, err := client.Run(ctx, []string{"rollout", "status", "-w", "deployment/olm-operator", "--namespace", namespace}); err != nil {
		return errors.Wrap(err, "error while waiting for olm component deployment")
	}

	if _, err := client.Run(ctx, []string{"rollout", "status", "-w", "deployment/catalog-operator", "--namespace", namespace}); err != nil {
		return errors.Wrap(err, "error while waiting for olm component deployment")
	}

	return nil
}

func (o OperatorService) getLatestVersion(ctx context.Context, repo string) (string, error) {
	url := fmt.Sprintf(githubAPIURLTemplate, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", errors.Wrap(err, "cannot prepare OLM lastest version request")
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "cannot get OLM latest version")
	}

	defer response.Body.Close() //nolint:errcheck

	type jsonResponse struct {
		TagName string `json:"tag_name"` // nolint:tagliatelle
	}
	var resp *jsonResponse

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "cannot read OLM latest version response")
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", errors.Wrap(err, "cannot decode OLM latest version response")
	}

	if resp.TagName != "" {
		return resp.TagName, nil
	}

	return "", ErrEmptyVersionTag
}
