package kube

import (
	"context"

	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	PXCKind = "PerconaXtraDBCluster"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(pxcv1.SchemeGroupVersion,
		&pxcv1.PerconaXtraDBCluster{},
		&pxcv1.PerconaXtraDBClusterList{},
	)

	metav1.AddToGroupVersion(scheme, pxcv1.SchemeGroupVersion)
	return nil
}

type PerconaXtraDBClusterClientInterface interface {
	PXCClusters(namespace string) PerconaXtraDBClusterInterface
}

type PerconaXtraDBClusterClient struct {
	restClient rest.Interface
}

func NewForConfig(c *rest.Config) (*PerconaXtraDBClusterClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &pxcv1.SchemeGroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	AddToScheme(scheme.Scheme)

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &PerconaXtraDBClusterClient{restClient: client}, nil
}

func (c *PerconaXtraDBClusterClient) PXCClusters(namespace string) PerconaXtraDBClusterInterface {
	return &pxcClient{
		restClient: c.restClient,
		namespace:  namespace,
	}
}

type PerconaXtraDBClusterInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*pxcv1.PerconaXtraDBClusterList, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*pxcv1.PerconaXtraDBCluster, error)
	Create(context.Context, *pxcv1.PerconaXtraDBCluster) (*pxcv1.PerconaXtraDBCluster, error)
	Update(context.Context, *pxcv1.PerconaXtraDBCluster) (*pxcv1.PerconaXtraDBCluster, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
}

type pxcClient struct {
	restClient rest.Interface
	namespace  string
}

func (c *pxcClient) List(ctx context.Context, opts metav1.ListOptions) (*pxcv1.PerconaXtraDBClusterList, error) {
	result := &pxcv1.PerconaXtraDBClusterList{}
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(PXCKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*pxcv1.PerconaXtraDBCluster, error) {
	result := &pxcv1.PerconaXtraDBCluster{}
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(PXCKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Create(ctx context.Context, spec *pxcv1.PerconaXtraDBCluster) (*pxcv1.PerconaXtraDBCluster, error) {
	result := &pxcv1.PerconaXtraDBCluster{}
	err := c.restClient.
		Post().
		Namespace(c.namespace).
		Resource(PXCKind).
		Body(spec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Update(ctx context.Context, spec *pxcv1.PerconaXtraDBCluster) (*pxcv1.PerconaXtraDBCluster, error) {
	result := &pxcv1.PerconaXtraDBCluster{}
	err := c.restClient.
		Put().
		Namespace(c.namespace).
		Resource(PXCKind).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Namespace(c.namespace).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}
func (c *pxcClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(PXCKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}
