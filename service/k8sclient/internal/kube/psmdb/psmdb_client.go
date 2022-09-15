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

// Package psmdb provides PSMDB client for kubernetes.
package psmdb

import (
	"context"

	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	PSMDBKind    = "PerconaServerMongoDB"
	psmdbAPIKind = "perconaservermongodbs"
)

var (
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
	SchemeGroupVersion = schema.GroupVersion{Group: "psmdb.percona.com", Version: "v1"}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		new(psmdbv1.PerconaServerMongoDB),
		new(psmdbv1.PerconaServerMongoDBList),
	)

	metav1.AddToGroupVersion(scheme, psmdbv1.SchemeGroupVersion)
	return nil
}

type PerconaServerMongoDBClientInterface interface {
	PSMDBClusters(namespace string) PerconaServerMongoDBInterface
}

type PerconaServerMongoDBClient struct {
	restClient rest.Interface
}

func NewForConfig(c *rest.Config) (*PerconaServerMongoDBClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &SchemeGroupVersion
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

func (c *PerconaServerMongoDBClient) PSMDBClusters(namespace string) PerconaServerMongoDBInterface {
	return &psmdbClient{
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

type psmdbClient struct {
	restClient rest.Interface
	namespace  string
}

func (c *psmdbClient) List(ctx context.Context, opts metav1.ListOptions) (*psmdbv1.PerconaServerMongoDBList, error) {
	result := new(psmdbv1.PerconaServerMongoDBList)
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(psmdbAPIKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *psmdbClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*psmdbv1.PerconaServerMongoDB, error) {
	result := new(psmdbv1.PerconaServerMongoDB)
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(psmdbAPIKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *psmdbClient) Create(ctx context.Context, spec *psmdbv1.PerconaServerMongoDB) (*psmdbv1.PerconaServerMongoDB, error) {
	result := new(psmdbv1.PerconaServerMongoDB)
	err := c.restClient.
		Post().
		Namespace(c.namespace).
		Resource(psmdbAPIKind).
		Body(spec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *psmdbClient) Update(ctx context.Context, spec *psmdbv1.PerconaServerMongoDB) (*psmdbv1.PerconaServerMongoDB, error) {
	result := new(psmdbv1.PerconaServerMongoDB)
	err := c.restClient.
		Put().
		Namespace(c.namespace).
		Resource(psmdbAPIKind).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *psmdbClient) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.restClient.
		Delete().
		Namespace(c.namespace).
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}
func (c *psmdbClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(psmdbAPIKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}
