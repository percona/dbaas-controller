package logs

import (
	"context"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

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

	return response, nil
}
