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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestKubernetesClusterServiceCheckConnection(t *testing.T) {
	t.Run("Wrong kube config", func(t *testing.T) {
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
		AssertGRPCErrorRE(t, codes.FailedPrecondition, "Unable to connect to Kubernetes cluster: exit status 1", err)
	})
}

// AssertGRPCErrorRE checks that actual error has expected gRPC error code, and error messages
// matches expected regular expression.
func AssertGRPCErrorRE(tb testing.TB, expectedCode codes.Code, expectedMessageRE string, actual error) {
	tb.Helper()

	s, ok := status.FromError(actual)
	if !assert.True(tb, ok, "expected gRPC Status, got %T:\n%s", actual, actual) {
		return
	}
	assert.Equal(tb, int(expectedCode), int(s.Code()), "gRPC status codes are not equal") // int() to log in decimal, not hex
	assert.Regexp(tb, expectedMessageRE, s.Message(), "gRPC status message does not match")
}
