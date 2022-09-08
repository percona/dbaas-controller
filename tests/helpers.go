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

package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

func WaitForClusterState(ctx context.Context, kubeconfig string, name string, state controllerv1beta1.DBClusterState) error {
	for {
		clusters, err := PXCClusterAPIClient.ListPXCClusters(Context, &controllerv1beta1.ListPXCClustersRequest{
			KubeAuth: &controllerv1beta1.KubeAuth{
				Kubeconfig: kubeconfig,
			},
		})
		if err != nil {
			return errors.Wrap(err, "cannot get clusters list")
		}

		if len(clusters.Clusters) > 0 && clusters.Clusters[0].State == state && clusters.Clusters[0].Name == name {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for the cluster to be ready")
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}
}
