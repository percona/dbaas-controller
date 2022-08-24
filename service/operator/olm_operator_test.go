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
	"io/ioutil"
	"os"
	"testing"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

func TestGetLatestVersion(t *testing.T) {
	ctx := context.Background()

	olmOperatorService := NewOLMOperatorService()

	latest, err := olmOperatorService.getLatestVersion(ctx, olmRepo)
	assert.NoError(t, err)
	assert.NotEmpty(t, latest)
}

func TestInstallOlmOperator(t *testing.T) {
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	ctx := context.Background()

	t.Cleanup(func() {
		// Cleanup the cluster
		req := &controllerv1beta1.InstallOLMOperatorRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{
				Kubeconfig: string(kubeconfig),
			},
		}
		client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
		assert.NoError(t, err)
		defer client.Cleanup() //nolint:errcheck

		files := []string{
			"deploy/olm/pxc/percona-xtradb-cluster-operator.yaml",
			"deploy/olm/psmdb/percona-server-mongodb-operator.yaml",
			"deploy/olm/crds.yaml",
		}

		for _, file := range files {
			t.Logf("deleting %q\n", file)
			yamlFile, _ := dbaascontroller.DeployDir.ReadFile(file)
			err := client.Delete(ctx, yamlFile)
			assert.NoError(t, err)
		}
	})
	req := &controllerv1beta1.InstallOLMOperatorRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: string(kubeconfig),
		},
	}

	olms := NewOLMOperatorService()

	_, err = olms.InstallOLMOperator(ctx, req)
	assert.NoError(t, err)

	DefaultPSMDBOperatorURLTemplate := "https://raw.githubusercontent.com/percona/percona-server-mongodb-operator/v%s/deploy/%s"
	psmdbOperatorService := NewPSMDBOperatorService(DefaultPSMDBOperatorURLTemplate)
	_, err = psmdbOperatorService.InstallPSMDBOperator(ctx, &controllerv1beta1.InstallPSMDBOperatorRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: string(kubeconfig),
		},
		Version: "1.10.0",
	})
	assert.NoError(t, err)

	DefaultPXCOperatorURLTemplate := "https://raw.githubusercontent.com/percona/percona-xtradb-cluster-operator/v%s/deploy/%s"
	pxcOperatorService := NewPXCOperatorService(DefaultPXCOperatorURLTemplate)
	_, err = pxcOperatorService.InstallPXCOperator(ctx, &controllerv1beta1.InstallPXCOperatorRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: string(kubeconfig),
		},
		Version: "1.10.0",
	})
	assert.NoError(t, err)

	// Cleanup the cluster
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	assert.NoError(t, err)
	defer client.Cleanup() //nolint:errcheck

	files := []string{
		"deploy/olm/pxc/percona-xtradb-cluster-operator.yaml",
		"deploy/olm/psmdb/percona-server-mongodb-operator.yaml",
		"deploy/olm/crds.yaml",
	}

	for _, file := range files {
		t.Logf("deleting %q\n", file)
		yamlFile, _ := dbaascontroller.DeployDir.ReadFile(file)
		err := client.Delete(ctx, yamlFile)
		assert.NoError(t, err)
	}
}
