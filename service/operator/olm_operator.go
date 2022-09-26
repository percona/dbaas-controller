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
package operator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"

	v1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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
)

// ErrEmptyVersionTag Got an empty version tag from GitHub API.
var ErrEmptyVersionTag = errors.New("got an empty version tag from Github")

// OLMOperatorService holds methods to handle the OLM operator.
type OLMOperatorService struct {
	kubeConfig string
	client     *k8sclient.K8sClient
}

// NewOLMOperatorService returns new OLMOperatorService instance.
func NewOLMOperatorService(client *k8sclient.K8sClient) *OLMOperatorService {
	return &OLMOperatorService{
		client: client,
	} //nolint:exhaustruct
}

// NewOLMOperatorServiceFromConfig returns new OLMOperatorService instance and intializes the config.
func NewOLMOperatorServiceFromConfig(kubeConfig string) *OLMOperatorService {
	return &OLMOperatorService{ //nolint:exhaustruct
		kubeConfig: kubeConfig,
	}
}

// InstallOLMOperator installs the OLM in the Kubernetes cluster.
func (o *OLMOperatorService) InstallOLMOperator(ctx context.Context, req *controllerv1beta1.InstallOLMOperatorRequest) (*controllerv1beta1.InstallOLMOperatorResponse, error) {
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

func (o *OLMOperatorService) GetInstallPlans(ctx context.Context, namespace string) (*v1alpha1.InstallPlanList, error) {
	data, err := o.client.Run(ctx, []string{"get", "installplans", "-ojson", "-n", namespace})
	if err != nil {
		return nil, errors.Wrap(err, "cannot get operators group list")
	}

	var installPlans v1alpha1.InstallPlanList
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

func (o *OLMOperatorService) ApproveInstallPlan(ctx context.Context, namespace, resourceName string) error {
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
func (o *OLMOperatorService) AvailableOperators(ctx context.Context, name string) (*PackageManifests, error) {
	data, err := o.client.Run(ctx, []string{"get", "packagemanifest", "-ojson", "-n=olm", "--field-selector", "metadata.name=" + name})
	if err != nil {
		return nil, errors.Wrap(err, "cannot get operators group list")
	}

	var manifestList PackageManifests
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
	InstallPlanApproval    Approval
	StartingCSV            string
}

func (o *OLMOperatorService) InstallOperator(ctx context.Context, client *k8sclient.K8sClient, params OperatorInstallRequest) error {
	exists, err := namespaceExists(ctx, client, params.Namespace)
	if err != nil {
		return errors.Wrapf(err, "cannot determine is the namespace %q exists", params.Namespace)
	}
	if !exists {
		if _, err := client.Run(ctx, []string{"create", "namespace", params.Namespace}); err != nil {
			return errors.Wrap(err, "cannot create namespace for subscription")
		}
	}

	ogExists, err := o.operatorGroupExists(ctx, client, params.Namespace, params.OperatorGroup)
	if err != nil {
		return errors.Wrap(err, "cannot check if oprator group exists")
	}

	if !ogExists {
		if err := o.createOperatorGroup(ctx, client, params.Namespace, params.OperatorGroup); err != nil {
			return errors.Wrapf(err, "cannot create operator group %q", params.OperatorGroup)
		}
	}

	return createSubscription(ctx, client, params)
}

func isInstalled(ctx context.Context, client *k8sclient.K8sClient, namespace string) bool {
	if _, err := client.Run(ctx, []string{"get", "deployment", "olm-operator", "-n", namespace}); err == nil {
		return true
	}
	return false
}

func (o *OLMOperatorService) operatorGroupExists(ctx context.Context, client *k8sclient.K8sClient, namespace, name string) (bool, error) {
	var operatorGroupList OperatorGroupList

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

func (o *OLMOperatorService) createOperatorGroup(ctx context.Context, client *k8sclient.K8sClient, namespace, name string) error {
	op := &OperatorGroup{
		APIVersion: APIVersionCoreosV1,
		Kind:       ObjectKindOperatorGroup,

		Metadata: OperatorGroupMetadata{
			Name:      name,
			Namespace: namespace,
		},
		Spec: OperatorGroupSpec{
			TargetNamespaces: []string{namespace},
		},
	}

	payload, err := yaml.Marshal(op)
	if err != nil {
		return errors.Wrap(err, "cannot encode subscription payload as yaml")
	}
	return client.Create(ctx, payload)
}

func createSubscription(ctx context.Context, client *k8sclient.K8sClient, req OperatorInstallRequest) error {
	subs := Subscription{
		APIVersion: APIVersionCoreosV1Alpha1,
		Kind:       ObjectKindSubscription,
		Metadata: SubscriptionMetadata{
			Name:      "my-" + req.Name,
			Namespace: req.Namespace,
		},
		Spec: SubscriptionSpec{
			Channel:             req.Channel,
			Name:                req.Name,
			Source:              req.CatalogSource,
			SourceNamespace:     req.CatalogSourceNamespace,
			InstallPlanApproval: req.InstallPlanApproval,
			// Only used to test a specific operator version.
			// StartingCSV:         req.StartingCSV,
		},
	}

	payload, err := yaml.Marshal(subs)
	if err != nil {
		return errors.Wrap(err, "cannot encode subscription payload as yaml")
	}

	return client.Create(ctx, payload)
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

func (o OLMOperatorService) getLatestVersion(ctx context.Context, repo string) (string, error) {
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
