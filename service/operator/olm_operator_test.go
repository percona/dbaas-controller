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
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

func TestGetLatestVersion(t *testing.T) {
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	client, err := k8sclient.New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	defer client.Cleanup() //nolint:errcheck

	olmOperatorService := NewOLMOperatorService(client)

	latest, err := olmOperatorService.getLatestVersion(ctx, olmRepo)
	assert.NoError(t, err)
	assert.NotEmpty(t, latest)
}

// func TestAvailableOperators(t *testing.T) {
// 	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
// 	require.NoError(t, err)
//
// 	ctx := context.Background()
//
// 	olmc := NewOLMOperatorServiceFromConfig(string(kubeconfig))
//
// 	aos, err := olmc.AvailableOperators(ctx, "", "")
// 	assert.NoError(t, err)
// 	assert.True(t, len(aos.Items) > 1)
//
// 	aos, err = olmc.AvailableOperators(ctx, "", "metadata.name=percona-server-mongodb-operator")
// 	assert.NoError(t, err)
// 	assert.Equal(t, 1, len(aos.Items))
//
// 	pp.Println(aos)
// }

func TestSubscribe(t *testing.T) {
	params := OperatorInstallRequest{
		Namespace:              "my-percona-server-mongodb-operator",
		Name:                   "percona-server-mongodb-operator",
		OperatorGroup:          "operatorgroup",
		CatalogSource:          "operatorhubio-catalog",
		CatalogSourceNamespace: "olm",
		Channel:                "stable",
		InstallPlanApproval:    ApprovalAutomatic,
		StartingCSV:            "percona-server-mongodb-operator.v1.11.0",
	}

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	client, err := k8sclient.New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	defer client.Cleanup() //nolint:errcheck

	olmc := NewOLMOperatorService(client)

	err = olmc.InstallOperator(ctx, client, params)
	assert.NoError(t, err)

	t.Log("Waiting for deployment")
	// Loop until the deployment exists and THEN we can wait.
	// Sometimes the test reaches this point too fast and we get an error saying that the
	// deplyment doesn't exists.
	for i := 0; i < 5; i++ {
		_, err = client.Run(ctx, []string{
			"wait", "--for=condition=Available", "deployment", "percona-server-mongodb-operator",
			"-n", params.Namespace, "--timeout=180s",
		})
		if err == nil {
			t.Logf("Deployment ready at try number %d", i)
			break
		}
		time.Sleep(5 * time.Second)
	}

	err = client.Delete(ctx, []string{"subscription", params.Namespace, "-n", params.Namespace})
	assert.NoError(t, err)
	err = client.Delete(ctx, []string{"operatorgroup", params.OperatorGroup, "-n", params.Namespace})
	assert.NoError(t, err)
	err = client.Delete(ctx, []string{"deployment", params.Name, "-n", params.Namespace})
	assert.NoError(t, err)
	err = client.Delete(ctx, []string{"namespace", params.Namespace})
	assert.NoError(t, err)
}

//func TestPackageManifest(t *testing.T) {
//	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
//	require.NoError(t, err)
//
//	ctx := context.Background()
//
//	client, err := k8sclient.New(ctx, string(kubeconfig))
//	assert.NoError(t, err)
//	defer client.Cleanup() //nolint:errcheck
//
//	olms := NewOLMOperatorService(client)
//
//	manifests, err := olms.AvailableOperators(
//}

func TestInstallOlmOperator(t *testing.T) {
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	client, err := k8sclient.New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	defer client.Cleanup() //nolint:errcheck
	t.Cleanup(func() {
		// Maintain the order, otherwise the Kubernetes deletetion will stuck in Terminating state.
		err := client.Delete(ctx, []string{"apiservices.apiregistration.k8s.io", "v1.packages.operators.coreos.com"})
		assert.NoError(t, err)
		files := []string{
			"deploy/olm/crds.yaml",
			"deploy/olm/olm.yaml",
		}

		for _, file := range files {
			t.Logf("deleting %q\n", file)
			yamlFile, _ := dbaascontroller.DeployDir.ReadFile(file)
			client.Delete(ctx, yamlFile)
		}
	})
	req := &controllerv1beta1.InstallOLMOperatorRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: string(kubeconfig),
		},
	}

	olms := NewOLMOperatorService(client)
	_, err = olms.InstallOLMOperator(ctx, req)
	assert.NoError(t, err)

	err = waitForTestDeployment(ctx, client, "olm", "packageserver")
	assert.NoError(t, err)
	err = waitForTestDeployment(ctx, client, "olm", "olm-operator")
	assert.NoError(t, err)
}

func waitForTestDeployment(ctx context.Context, client *k8sclient.K8sClient, namespace, name string) error {
	var err error

	for i := 0; i < 15; i++ {
		_, err = client.Run(ctx, []string{
			"wait", "--for=condition=Available", "deployment", name, "-n", namespace, "--timeout=180s",
		})
		if err == nil {
			break
		}
		fmt.Println("waiting 5 seconds")
		time.Sleep(5 * time.Second)
	}

	return err
}
