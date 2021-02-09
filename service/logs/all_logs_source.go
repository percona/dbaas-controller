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
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

// overallLinesLimit defines how many last lines of logs we should return upon
// upon calling allLogsSource's method getLogs.
const overallLinesLimit = 1000

// allLogsSource implements source interface, it gets all logs from all
// cluster's containers. It also gets events out of all cluster's pods.
type allLogsSource struct{}

// getLogs gets all logs from all cluster's containers and events from all pods.
func (a *allLogsSource) getLogs(ctx context.Context, client *k8sclient.K8sClient, clusterName string) ([]*controllerv1beta1.Logs, error) {
	pods, err := client.GetClusterPods(ctx, clusterName)
	if err != nil {
		return nil, status.Error(codes.Internal, errors.Wrap(err, "failed to get pods").Error())
	}
	// Every pod has at least one contaier, set cap to that value.
	response := make([]*controllerv1beta1.Logs, 0, len(pods.Items))
	for _, pod := range pods.Items {
		// Get all logs from all pod's containers.
		for _, container := range pod.Spec.Containers {
			logs, err := client.GetLogs(ctx, pod.Name, container.Name)
			if err != nil {
				return nil, status.Error(codes.Internal, errors.Wrap(err, "failed to get logs").Error())
			}
			response = append(response, &controllerv1beta1.Logs{
				Pod:       pod.Name,
				Container: container.Name,
				Logs:      logs,
			})
		}
		// Get pod's events.
		events, err := client.GetEvents(ctx, pod.Name)
		if err != nil {
			return nil, status.Error(codes.Internal, errors.Wrap(err, "failed to get events").Error())
		}
		response = append(response, &controllerv1beta1.Logs{
			Pod:       pod.Name,
			Container: "",
			Logs:      events,
		})
	}

	// Limit number of overall log lines.
	limitLines(response, overallLinesLimit)
	return response, nil
}

func sum(counts []int) int {
	sum := 0
	for _, count := range counts {
		sum += count
	}
	return sum
}

// limitLines limits each entry's logs lines count in the way the overall sum of
// all log lines is equal to given limit.
func limitLines(logs []*controllerv1beta1.Logs, limit int) {
	counts := make([]int, len(logs))
	lastSum := -1
	for sum(counts) < limit && sum(counts) > lastSum {
		lastSum = sum(counts)
		for i, item := range logs {
			if counts[i] < len(item.Logs) {
				counts[i]++
				if sum(counts) == limit {
					break
				}
			}
		}
	}
	// Do the actual slicing.
	for i, item := range logs {
		logs[i].Logs = item.Logs[len(item.Logs)-counts[i]:]
	}
}
