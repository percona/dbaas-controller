package operator

import (
	"context"

	controllerv1beta1 "github.com/percona-platform/dbaas-api/gen/controller"
	"golang.org/x/text/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/percona-platform/dbaas-controller/service/k8sclient"
)

type XtraDBOperatorService struct {
	p *message.Printer
}

// NewXtraDBOperatorService returns new XtraDBOperatorService instance.
func NewXtraDBOperatorService(p *message.Printer) *XtraDBOperatorService {
	return &XtraDBOperatorService{p: p}
}

func (x XtraDBOperatorService) InstallXtraDBOperator(ctx context.Context, req *controllerv1beta1.InstallXtraDBOperatorRequest) (*controllerv1beta1.InstallXtraDBOperatorResponse, error) {
	client, err := k8sclient.New(ctx, req.KubeAuth.Kubeconfig)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	defer client.Cleanup() //nolint:errcheck

	err = client.InstallXtraDBOperator(ctx)
	if err != nil {
		return nil, err
	}

	return &controllerv1beta1.InstallXtraDBOperatorResponse{}, nil
}
