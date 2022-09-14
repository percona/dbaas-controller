package kube

import (
	"context"

	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	PXCKind = "PerconaServerMongoDB"
	apiKind = "perconaxtradbclusters"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(psmdbv1.SchemeGroupVersion,
		&psmdbv1.PerconaServerMongoDB{},
		&psmdbv1.PerconaServerMongoDBList{},
	)

	metav1.AddToGroupVersion(scheme, psmdbv1.SchemeGroupVersion)
	return nil
}

type PerconaServerMongoDBClientInterface interface {
	PXCClusters(namespace string) PerconaServerMongoDBInterface
}

type PerconaServerMongoDBClient struct {
	restClient rest.Interface
}

func NewForConfig(c *rest.Config) (*PerconaServerMongoDBClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &psmdbv1.SchemeGroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	AddToScheme(scheme.Scheme)

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}

	return &PerconaServerMongoDBClient{restClient: client}, nil
}

func (c *PerconaServerMongoDBClient) PXCClusters(namespace string) PerconaServerMongoDBInterface {
	return &pxcClient{
		restClient: c.restClient,
		namespace:  namespace,
	}
}

type PerconaServerMongoDBInterface interface {
	List(ctx context.Context, opts metav1.ListOptions) (*psmdbv1.PerconaServerMongoDBList, error)
	Get(ctx context.Context, name string, options metav1.GetOptions) (*psmdbv1.PerconaServerMongoDB, error)
	Create(context.Context, *psmdbv1.PerconaServerMongoDB) (*psmdbv1.PerconaServerMongoDB, error)
	Update(context.Context, *psmdbv1.PerconaServerMongoDB) (*psmdbv1.PerconaServerMongoDB, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Delete(ctx context.Context, name string, options metav1.DeleteOptions) error
}

type pxcClient struct {
	restClient rest.Interface
	namespace  string
}

func (c *pxcClient) List(ctx context.Context, opts metav1.ListOptions) (*psmdbv1.PerconaServerMongoDBList, error) {
	result := &psmdbv1.PerconaServerMongoDBList{}
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(apiKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*psmdbv1.PerconaServerMongoDB, error) {
	result := &psmdbv1.PerconaServerMongoDB{}
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(apiKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Create(ctx context.Context, spec *psmdbv1.PerconaServerMongoDB) (*psmdbv1.PerconaServerMongoDB, error) {
	result := &psmdbv1.PerconaServerMongoDB{}
	err := c.restClient.
		Post().
		Namespace(c.namespace).
		Resource(apiKind).
		Body(spec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Update(ctx context.Context, spec *psmdbv1.PerconaServerMongoDB) (*psmdbv1.PerconaServerMongoDB, error) {
	result := &psmdbv1.PerconaServerMongoDB{}
	err := c.restClient.
		Put().
		Namespace(c.namespace).
		Resource(apiKind).
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
		Resource(apiKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}
