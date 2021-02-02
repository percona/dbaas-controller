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

// Package logs contains implementation of API for getting logs out of
// Kubernetes cluster workloads.
package logs

import (
	"context"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/percona-platform/dbaas-controller/service/k8sclient"
	"github.com/pkg/errors"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LogsService struct {
	p *message.Printer
}

func NewService(p *message.Printer) *LogsService {
	return &LogsService{
		p: p,
	}
}

// GetLogs first tries to get logs and events only from failing pods/containers.
// If no such logs/events are found, it returns all logs and events.
func (s *LogsService) GetLogs(ctx context.Context, req *controllerv1beta1.GetLogsRequest) (*controllerv1beta1.GetLogsResponse, error) {
	client, ok := ctx.Value("k8sclient").(*k8sclient.K8sClient)
	if !ok {
		return nil, status.Error(codes.Internal, "failed to get k8s client")
	}
	logs, err := client.GetLogs(ctx, req.ClusterName)
	if err != nil {
		return nil, status.Error(codes.Internal, errors.Wrap(err, "failed to get logs").Error())
	}
	response := make([]*controllerv1beta1.Logs, 0, len(logs))
	for pod, containers := range logs {
		for container, clogs := range containers {
			response = append(response, &controllerv1beta1.Logs{
				Pod:       pod,
				Container: container,
				Logs:      clogs,
			})
		}
	}
	return &controllerv1beta1.GetLogsResponse{
		Logs: response,
	}, nil
}
