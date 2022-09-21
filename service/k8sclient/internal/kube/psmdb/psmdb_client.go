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
	"sync"

	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	PSMDBKind    = "PerconaServerMongoDB"
	psmdbAPIKind = "perconaservermongodbs"
)

type PerconaServerMongoDBClientInterface interface {
	PSMDBClusters(namespace string) PerconaServerMongoDBInterface
}

type PerconaServerMongoDBClient struct {
	restClient rest.Interface
}

var addToScheme sync.Once

func NewForConfig(c *rest.Config) (*PerconaServerMongoDBClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &psmdbv1.SchemeGroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	addToScheme.Do(func() {
		psmdbv1.SchemeBuilder.AddToScheme(scheme.Scheme)
		metav1.AddToGroupVersion(scheme.Scheme, psmdbv1.SchemeGroupVersion)
	})

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
	Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions) (*psmdbv1.PerconaServerMongoDB, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
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

func (c *psmdbClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*psmdbv1.PerconaServerMongoDB, error) {
	result := new(psmdbv1.PerconaServerMongoDB)
	err := c.restClient.
		Patch(pt).
		Namespace(c.namespace).
		Resource(psmdbAPIKind).
		Name(name).
		Body(data).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
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
