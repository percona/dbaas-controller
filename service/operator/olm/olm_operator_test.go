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
	"io/ioutil"
	"os"
	"testing"
	"time"

	v1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

func TestGetLatestVersion(t *testing.T) {
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	client, err := k8sclient.New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	defer client.Cleanup() //nolint:errcheck

	olmOperatorService := NewOperatorService(client)

	latest, err := olmOperatorService.getLatestVersion(ctx, olmRepo)
	assert.NoError(t, err)
	assert.NotEmpty(t, latest)
}

func TestSubscribe(t *testing.T) {
	params := OperatorInstallRequest{
		Namespace:              "my-percona-server-mongodb-operator",
		Name:                   "percona-server-mongodb-operator",
		OperatorGroup:          "operatorgroup",
		CatalogSource:          "operatorhubio-catalog",
		CatalogSourceNamespace: "olm",
		Channel:                "stable",
		InstallPlanApproval:    v1alpha1.ApprovalAutomatic,
		StartingCSV:            "percona-server-mongodb-operator.v1.11.0",
	}

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	client, err := k8sclient.New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	defer client.Cleanup() //nolint:errcheck

	olmc := NewOperatorService(client)

	err = olmc.InstallOperator(ctx, params)
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

func TestInstallOlmOperator(t *testing.T) {
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	client, err := k8sclient.New(ctx, string(kubeconfig))
	assert.NoError(t, err)

	t.Cleanup(func() {
		return
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
			// When deleting, some resources might be already deleted by the previous file so the returned error
			// should be considered only as a warning.
			_ = client.Delete(ctx, yamlFile)
		}

		_ = client.Cleanup()
	})

	req := &controllerv1beta1.InstallOLMOperatorRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: string(kubeconfig),
		},
	}

	olms := NewOperatorService(client)
	_, err = olms.InstallOLMOperator(ctx, req)
	assert.NoError(t, err)

	// Wait for the deployments
	_, err = client.Run(ctx, []string{"rollout", "status", "-w", "deployment/olm-operator", "-n", "olm"})
	_, err = client.Run(ctx, []string{"rollout", "status", "-w", "deployment/catalog-operator", "-n", "olm"})

	manifests, err := olms.AvailableOperators(ctx, "percona-server-mongodb-operator")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(manifests.Items))

	subscriptionName := "percona-server-mongodb-operator"
	subscriptionNamespace := "my-percona-server-mongodb-operator"

	t.Run("Subscribe", func(t *testing.T) {
		kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
		require.NoError(t, err)

		ctx := context.Background()

		client, err := k8sclient.New(ctx, string(kubeconfig))
		assert.NoError(t, err)
		defer client.Cleanup() //nolint:errcheck

		// Install PSMDB Operator
		params := OperatorInstallRequest{
			Namespace:              subscriptionNamespace,
			Name:                   subscriptionName,
			OperatorGroup:          "operatorgroup",
			CatalogSource:          "operatorhubio-catalog",
			CatalogSourceNamespace: "olm",
			Channel:                "stable",
			InstallPlanApproval:    v1alpha1.ApprovalManual,
			StartingCSV:            "percona-server-mongodb-operator.v1.11.0",
		}

		err = olms.InstallOperator(ctx, params)
		assert.NoError(t, err)

		var installPlans *v1alpha1.InstallPlanList
		for i := 0; i < 6; i++ {
			installPlans, err = olms.GetInstallPlans(ctx, subscriptionNamespace)
			if len(installPlans.Items) > 0 {
				break
			}
			time.Sleep(30 * time.Second)
		}
		assert.NoError(t, err)
		require.True(t, len(installPlans.Items) > 0)

		olms.ApproveInstallPlan(ctx, subscriptionNamespace, installPlans.Items[0].ObjectMeta.Name)

		t.Log("Waiting for deployment")
		// Loop until the deployment exists and THEN we can wait.
		// Sometimes the test reaches this point too fast and we get an error saying that the
		// deplyment doesn't exists.
		for i := 0; i < 5; i++ {
			_, err = client.Run(ctx, []string{"wait", "--for=condition=Available", "deployment", params.Name, "-n", params.Namespace, "--timeout=380s"})
			if err == nil {
				t.Logf("Deployment ready at try number %d", i)
				break
			}
			time.Sleep(5 * time.Second)
		}
		assert.NoError(t, err)

		err = client.Delete(ctx, []string{"subscription", params.Namespace, "-n", params.Namespace})
		assert.NoError(t, err)
		err = client.Delete(ctx, []string{"operatorgroup", params.OperatorGroup, "-n", params.Namespace})
		assert.NoError(t, err)
		err = client.Delete(ctx, []string{"deployment", params.Name, "-n", params.Namespace})
		assert.NoError(t, err)
		err = client.Delete(ctx, []string{"namespace", params.Namespace})
		assert.NoError(t, err)
	})
}
