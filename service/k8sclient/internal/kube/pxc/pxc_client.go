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

// Package pxc provides PXC client for kubernetes.
package pxc

import (
	"context"
	"sync"

	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	PXCKind = "PerconaXtraDBCluster"
	apiKind = "perconaxtradbclusters"
)

type PerconaXtraDBClusterClientInterface interface {
	PXCClusters(namespace string) PerconaXtraDBClusterInterface
}

type PerconaXtraDBClusterClient struct {
	restClient rest.Interface
}

var addToScheme sync.Once

func NewForConfig(c *rest.Config) (*PerconaXtraDBClusterClient, error) {
	config := *c
	config.ContentConfig.GroupVersion = &pxcv1.SchemeGroupVersion
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()

	addToScheme.Do(func() {
		pxcv1.SchemeBuilder.AddToScheme(scheme.Scheme)
		metav1.AddToGroupVersion(scheme.Scheme, pxcv1.SchemeGroupVersion)
	})

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
	Patch(context.Context, string, types.PatchType, []byte, metav1.PatchOptions) (*pxcv1.PerconaXtraDBCluster, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
}

type pxcClient struct {
	restClient rest.Interface
	namespace  string
}

func (c *pxcClient) List(ctx context.Context, opts metav1.ListOptions) (*pxcv1.PerconaXtraDBClusterList, error) {
	result := new(pxcv1.PerconaXtraDBClusterList)
	err := c.restClient.
		Get().
		Namespace(c.namespace).
		Resource(apiKind).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *pxcClient) Get(ctx context.Context, name string, opts metav1.GetOptions) (*pxcv1.PerconaXtraDBCluster, error) {
	result := new(pxcv1.PerconaXtraDBCluster)
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

func (c *pxcClient) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*pxcv1.PerconaXtraDBCluster, error) {
	result := new(pxcv1.PerconaXtraDBCluster)
	err := c.restClient.
		Patch(pt).
		Namespace(c.namespace).
		Resource(apiKind).
		Name(name).
		Body(data).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return result, err
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
