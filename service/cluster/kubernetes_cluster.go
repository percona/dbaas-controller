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

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/kubectl"
)

// KubernetesClusterService implements methods of gRPC server and other business logic related to kubernetes clusters.
type KubernetesClusterService struct {
	p *message.Printer
}

// NewKubernetesClusterService returns new KubernetesClusterService instance.
func NewKubernetesClusterService(p *message.Printer) *KubernetesClusterService {
	return &KubernetesClusterService{p: p}
}

func (k KubernetesClusterService) CheckKubernetesClusterConnection(ctx context.Context, req *controllerv1beta1.CheckKubernetesClusterConnectionRequest) (*controllerv1beta1.CheckKubernetesClusterConnectionResponse, error) {
	kubeCtl, err := kubectl.NewKubeCtl(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, k.p.Sprintf("Unable to connect to kubernetes cluster: %s", err))
	}
	defer kubeCtl.Cleanup() //nolint:errcheck
	return new(controllerv1beta1.CheckKubernetesClusterConnectionResponse), nil
}
