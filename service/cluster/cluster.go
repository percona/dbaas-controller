// Package cluster TODO
package cluster

import (
	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
)

// TODO: Implement your gRPC server

// Service implements methods of gRPC server and other business logic.
type Service struct {
}

// New returns new Service instance.
func New() *Service {
	// TODO remove it once it is really used
	_ = controllerv1beta1.NewXtraDBClusterAPIClient(nil)

	return new(Service)
}
