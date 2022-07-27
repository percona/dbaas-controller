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

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
)

// ErrEmptyVersionTag Got an empty version tag from GitHub API.
var ErrEmptyVersionTag = errors.New("got an empty version tag from Github")

// OLMOperatorService holds methods to handle the OLM operator.
type OLMOperatorService struct{}

// NewOLMOperatorService returns new OLMOperatorService instance.
func NewOLMOperatorService() *OLMOperatorService {
	return &OLMOperatorService{}
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

	return response, nil
}

func isInstalled(ctx context.Context, client *k8sclient.K8sClient, namespace string) bool {
	if _, err := client.Run(ctx, []string{"get", "deployment", "olm-operator", "-n", namespace}); err == nil {
		return true
	}
	return false
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
