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

package kubernetes

import (
	"context"
	"os"
	"testing"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"github.com/percona-platform/dbaas-controller/tests"
)

func TestKubernetesClusterAPI(t *testing.T) {
	t.Run("Correct", func(t *testing.T) {
		kubeConfig := os.Getenv("PERCONA_TEST_DBAAS_KUBECONFIG")
		if kubeConfig == "" {
			t.Skip("PERCONA_TEST_DBAAS_KUBECONFIG env variable is not provided")
		}
		ctx := context.TODO()
		response, err := tests.KubernetesClusterAPIClient.CheckKubernetesClusterConnection(ctx,
			&controllerv1beta1.CheckKubernetesClusterConnectionRequest{
				KubeAuth: &controllerv1beta1.KubeAuth{Kubeconfig: kubeConfig},
			},
		)
		require.NoError(t, err)
		require.NotNil(t, response)
	})

	t.Run("Wrong", func(t *testing.T) {
		kubeConfig := `{
			"kind": "Config",
			"apiVersion": "v1",
			"preferences": {},
			"clusters": [
				{
					"name": "minikube",
					"cluster": {
						"server": "https://1.2.3.4:8443",
					}
				}
			],
			"contexts": [
				{
					"name": "minikube",
					"context": {
						"cluster": "minikube",
						"user": "minikube"
					}
				}
			],
			"current-context": "minikube"
		}`
		ctx := context.TODO()
		response, err := tests.KubernetesClusterAPIClient.CheckKubernetesClusterConnection(ctx,
			&controllerv1beta1.CheckKubernetesClusterConnectionRequest{
				KubeAuth: &controllerv1beta1.KubeAuth{Kubeconfig: kubeConfig},
			},
		)

		tests.AssertGRPCErrorRE(t, codes.FailedPrecondition, "Unable to connect to Kubernetes cluster: exit status", err)
		require.Nil(t, response)
	})
}
