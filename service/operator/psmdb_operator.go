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
