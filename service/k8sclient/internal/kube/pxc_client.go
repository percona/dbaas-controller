package kube

import (
	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

const (
	pxcKind = "PerconaXtraDBCluster"
)

type PerconaXtraDBClusterInterface interface {
	List(opts metav1.ListOptions) (*pxcv1.PerconaXtraDBClusterList, error)
	Get(name string, options metav1.GetOptions) (*pxcv1.PerconaXtraDBCluster, error)
	Create(*pxcv1.PerconaXtraDBCluster) (*pxcv1.PerconaXtraDBCluster, error)
	Update(*pxcv1.PerconaXtraDBCluster) (*pxcv1.PerconaXtraDBCluster, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Delete(name string, options metav1.DeleteOptions) error
}

type pxcClient struct {
	restClient rest.Interface
	namespace  string
}

func (c *pxcClient) List(opts metav1.ListOptions) (*pxcv1.PerconaXtraDBClusterList, error) {
	result := &pxcv1.PerconaXtraDBClusterList{}
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(pxcKind).
		VersionedParams(&opts, scheme.ParameterCoded).
		Do().
		Into(result)
}
