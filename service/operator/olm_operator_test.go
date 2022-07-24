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
	"path"
	"testing"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	req := &controllerv1beta1.InstallOLMOperatorRequest{
		KubeAuth: &controllerv1beta1.KubeAuth{
			Kubeconfig: string(kubeconfig),
		},
	}

	olms := NewOLMOperatorService()

	_, err = olms.InstallOLMOperator(ctx, req)
	assert.NoError(t, err)

	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		//	return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	uris := []string{
		path.Join(baseDownloadURL, "crds.yaml"),
		path.Join(baseDownloadURL, "olm.yaml"),
	}

	for _, uri := range uris {
		err := client.Delete(ctx, "https://"+uri)
		assert.NoError(t, err)
	}
}
