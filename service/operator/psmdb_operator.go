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

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dbaascontroller "github.com/percona-platform/dbaas-controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

const psmdbOperatorDeploymentName = "percona-server-mongodb-operator"

type PSMDBOperatorService struct {
	manifestsURLTemplate string
}

// NewPSMDBOperatorService returns new PSMDBOperatorService instance.
func NewPSMDBOperatorService(url string) *PSMDBOperatorService {
	return &PSMDBOperatorService{manifestsURLTemplate: url}
}

func (x PSMDBOperatorService) InstallPSMDBOperator(ctx context.Context, req *controllerv1beta1.InstallPSMDBOperatorRequest) (*controllerv1beta1.InstallPSMDBOperatorResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck
	// Try to get operator versions to see if we should upgrade or install.
	operators, err := client.CheckOperators(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// NOTE: This does not handle corner case when user has deployed database clusters and operator is no longer installed.
	if operators.PsmdbOperatorVersion != "" {
		// TODO: If operator is installed, try to upgrade it?
		return new(controllerv1beta1.InstallPSMDBOperatorResponse), nil
	}
	req.Version = "1.11.0"

	yamlFile, err := dbaascontroller.DeployDir.ReadFile("deploy/olm/psmdb/percona-server-mongodb-operator.yaml")
	if err != nil {
		return nil, err
	}

	if err := client.Create(ctx, yamlFile); err != nil {
		return nil, errors.Wrap(err, "cannot install the PXC operator via OLM")
	}

	return new(controllerv1beta1.InstallPSMDBOperatorResponse), nil
}
