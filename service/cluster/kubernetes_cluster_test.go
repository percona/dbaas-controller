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

package cluster

import (
	"context"
	"testing"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"

	"github.com/percona-platform/dbaas-controller/utils/testutil"
)

func TestKubernetesClusterServiceCheckConnection(t *testing.T) {
	t.Parallel()
	t.Run("Wrong kube config", func(t *testing.T) {
		t.Parallel()
		i18nPrinter := message.NewPrinter(language.English)
		k := NewKubernetesClusterService(i18nPrinter)
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
		_, err := k.CheckKubernetesClusterConnection(context.TODO(), &controllerv1beta1.CheckKubernetesClusterConnectionRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{Kubeconfig: kubeConfig},
		})
		require.Error(t, err)
		testutil.AssertGRPCErrorRE(t, codes.FailedPrecondition, "Unable to connect to Kubernetes cluster: exit status 1", err)
	})
}
