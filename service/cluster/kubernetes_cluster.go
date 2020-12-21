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

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

var operatorStatusesMap = map[k8sclient.OperatorStatus]controllerv1beta1.OperatorsStatus{
	k8sclient.OperatorStatusOK:           controllerv1beta1.OperatorsStatus_OPERATORS_STATUS_OK,
	k8sclient.OperatorStatusUnsupported:  controllerv1beta1.OperatorsStatus_OPERATORS_STATUS_UNSUPPORTED,
	k8sclient.OperatorStatusNotInstalled: controllerv1beta1.OperatorsStatus_OPERATORS_STATUS_NOT_INSTALLED,
}

// KubernetesClusterService implements methods of gRPC server and other business logic related to kubernetes clusters.
type KubernetesClusterService struct {
	p *message.Printer
}

// NewKubernetesClusterService returns new KubernetesClusterService instance.
func NewKubernetesClusterService(p *message.Printer) *KubernetesClusterService {
	return &KubernetesClusterService{p: p}
}

// CheckKubernetesClusterConnection checks connection with kubernetes cluster.
func (k KubernetesClusterService) CheckKubernetesClusterConnection(ctx context.Context, req *controllerv1beta1.CheckKubernetesClusterConnectionRequest) (*controllerv1beta1.CheckKubernetesClusterConnectionResponse, error) {
	k8Client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, k.p.Sprintf("Unable to connect to Kubernetes cluster: %s", err))
	}
	defer k8Client.Cleanup() //nolint:errcheck

	resp := &controllerv1beta1.CheckKubernetesClusterConnectionResponse{
		Operators: &controllerv1beta1.Operators{
			Xtradb: new(controllerv1beta1.Operator),
			Psmdb:  new(controllerv1beta1.Operator),
		},
		Status: controllerv1beta1.KubernetesClusterStatus_KUBERNETES_CLUSTER_STATUS_OK,
	}

	operators, err := k8Client.CheckOperators(ctx)
	if err != nil {
		return resp, nil
	}

	resp.Operators.Xtradb.Status = operatorStatusesMap[operators.Xtradb]
	resp.Operators.Psmdb.Status = operatorStatusesMap[operators.Psmdb]

	return resp, nil
}
