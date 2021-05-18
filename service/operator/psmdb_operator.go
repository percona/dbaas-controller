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

package operator

import (
	"context"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

type PSMDBOperatorService struct {
	p *message.Printer
}

// NewPSMDBOperatorService returns new PSMDBOperatorService instance.
func NewPSMDBOperatorService(p *message.Printer) *PSMDBOperatorService {
	return &PSMDBOperatorService{p: p}
}

func (x PSMDBOperatorService) InstallPSMDBOperator(ctx context.Context, req *controllerv1beta1.InstallPSMDBOperatorRequest) (*controllerv1beta1.InstallPSMDBOperatorResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.InstallPSMDBOperator(ctx)
	if err != nil {
		return nil, err
	}

	return &controllerv1beta1.InstallPSMDBOperatorResponse{}, nil
}
