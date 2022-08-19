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

// Package kube provides client for kubernetes.
package kube

import (
	"bytes"
	"context"
	"io"
	"os"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	defaultAPIURIPath  = "/api"
	defaultAPIsURIPath = "/apis"
)

// configGetter stores kubeconfig string to convert it to the final object
type configGetter struct {
	kubeconfig string
}

type Client struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
}

// NewConfigGetter creates a new configGetter struct
func NewConfigGetter(kubeconfig string) *configGetter {
	return &configGetter{kubeconfig: kubeconfig}
}

// loadFromString takes a kubeconfig and deserializes the contents into Config object.
func (g *configGetter) loadFromString() (*clientcmdapi.Config, error) {
	config, err := clientcmd.Load([]byte(g.kubeconfig))
	if err != nil {
		return nil, err
	}

	if config.AuthInfos == nil {
		config.AuthInfos = make(map[string]*clientcmdapi.AuthInfo)
	}
	if config.Clusters == nil {
		config.Clusters = make(map[string]*clientcmdapi.Cluster)
	}
	if config.Contexts == nil {
		config.Contexts = make(map[string]*clientcmdapi.Context)
	}

	return config, nil
}

// NewFromIncluster returns a client object which uses the service account
// kubernetes gives to pods. It's intended for clients that expect to be
// running inside a pod running on kubernetes. It will return ErrNotInCluster
// if called from a process not running in a kubernetes environment.
func NewFromIncluster() (*Client, error) {
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: clientset, restConfig: c}, nil
}

// NewFromKubeConfigString creates a new client for the given config string.
// It's intended for clients that expect to be running outside of a cluster
func NewFromKubeConfigString(kubeconfig string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromKubeconfigGetter("", NewConfigGetter(kubeconfig).loadFromString)
	if err != nil {
		return nil, err
	}
	config.QPS = 350
	config.Burst = 500
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &Client{clientset: clientset, restConfig: config}, nil
}

func (c *Client) resourceClient(gv schema.GroupVersion) (rest.Interface, error) {
	cfg := c.restConfig
	cfg.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	cfg.GroupVersion = &gv
	if len(gv.Group) == 0 {
		cfg.APIPath = defaultAPIURIPath
	} else {
		cfg.APIPath = defaultAPIsURIPath
	}
	return rest.RESTClientFor(cfg)
}

// Delete deletes object from the k8s cluster
func (c *Client) Delete(ctx context.Context, obj runtime.Object) error {
	groupResources, err := restmapper.GetAPIGroupResources(c.clientset.Discovery())
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	gvk := obj.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := mapper.RESTMapping(gk, gvk.Version)
	if err != nil {
		return err
	}
	namespace, name, err := retrievesMetaFromObject(obj)
	if err != nil {
		return err
	}
	cli, err := c.resourceClient(mapping.GroupVersionKind.GroupVersion())
	if err != nil {
		return err
	}
	helper := resource.NewHelper(cli, mapping)
	err = deleteObject(helper, namespace, name)
	return err
}

// Apply applies object against the k8s cluster
func (c *Client) Apply(ctx context.Context, obj runtime.Object) error {
	groupResources, err := restmapper.GetAPIGroupResources(c.clientset.Discovery())
	if err != nil {
		return err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	gvk := obj.GetObjectKind().GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := mapper.RESTMapping(gk, gvk.Version)
	if err != nil {
		return err
	}
	namespace, name, err := retrievesMetaFromObject(obj)
	if err != nil {
		return err
	}
	cli, err := c.resourceClient(mapping.GroupVersionKind.GroupVersion())
	if err != nil {
		return err
	}
	helper := resource.NewHelper(cli, mapping)
	err = applyObject(helper, namespace, name, obj)
	return err
}

// DeleteFile accepts manifest file contents parses into []runtime.Object
// and deletes them from the cluster
func (c *Client) DeleteFile(ctx context.Context, fileBytes []byte) error {
	objs, err := c.getObjects(fileBytes)
	if err != nil {
		return err
	}
	for i := range objs {
		err := c.Delete(ctx, objs[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// ApplyFile accepts manifest file contents, parses into []runtime.Object
// and applies them against the cluster
func (c *Client) ApplyFile(ctx context.Context, fileBytes []byte) error {
	objs, err := c.getObjects(fileBytes)
	if err != nil {
		return err
	}
	for i := range objs {
		err := c.Apply(ctx, objs[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) getObjects(f []byte) ([]runtime.Object, error) {
	objs := []runtime.Object{}
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(f), 100)
	var err error
	for {
		var rawObj runtime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}

		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return nil, err
		}

		objs = append(objs, &unstructured.Unstructured{Object: unstructuredMap})
	}

	return objs, nil
}

// GetStorageClasses returns all storage classes available in the cluster
func (c *Client) GetStorageClasses(ctx context.Context) (*v1.StorageClassList, error) {
	return c.clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
}

// GetSecret returns k8s secret by provided name
func (c *Client) GetSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	namespace := "default"
	if space := os.Getenv("NAMESPACE"); space != "" {
		namespace = space
	}

	return c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetAPIVersions returns apiversions
func (c *Client) GetAPIVersions(ctx context.Context) ([]string, error) {
	var versions []string
	groupList, err := c.clientset.Discovery().ServerGroups()
	if err != nil {
		return versions, err
	}
	versions = metav1.ExtractGroupVersions(groupList)
	return versions, nil
}

// GetPersistentVolumes returns Persistent Volumes avaiable in the cluster
func (c *Client) GetPersistentVolumes(ctx context.Context) (*corev1.PersistentVolumeList, error) {
	return c.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
}

// GetPods returns list of pods
func (c *Client) GetPods(ctx context.Context, namespace, labelSelector string) (*corev1.PodList, error) {
	options := metav1.ListOptions{}
	if labelSelector != "" {
		parsed, err := metav1.ParseToLabelSelector(labelSelector)
		if err != nil {
			return nil, err
		}
		selector, err := parsed.Marshal()
		if err != nil {
			return nil, err
		}
		options.LabelSelector = string(selector)
		options.LabelSelector = labelSelector
	}
	return c.clientset.CoreV1().Pods(namespace).List(ctx, options)
}

// GetNodes returns list of nodes
func (c *Client) GetNodes(ctx context.Context) (*corev1.NodeList, error) {
	return c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
}

// GetLogs returns logs for pod
func (c *Client) GetLogs(ctx context.Context, pod, container string) (string, error) {
	options := new(corev1.PodLogOptions)
	if container != "" {
		options.Container = container
	}
	buf := new(bytes.Buffer)
	namespace := "default"
	if space := os.Getenv("NAMESPACE"); space != "" {
		namespace = space
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(pod, options)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return buf.String(), err
	}
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return buf.String(), err
	}
	return buf.String(), nil
}

func retrievesMetaFromObject(obj runtime.Object) (namespace, name string, err error) {
	name, err = meta.NewAccessor().Name(obj)
	if err != nil {
		return
	}
	namespace, err = meta.NewAccessor().Namespace(obj)
	if err != nil {
		return
	}
	if namespace == "" {
		namespace = "default"
		if space := os.Getenv("NAMESPACE"); space != "" {
			namespace = space
		}
	}
	return
}

func applyObject(helper *resource.Helper, namespace, name string, obj runtime.Object) error {
	if _, err := helper.Get(namespace, name); err != nil {
		_, err = helper.Create(namespace, false, obj)
		if err != nil {
			return err
		}
	} else {
		_, err = helper.Replace(namespace, name, true, obj)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteObject(helper *resource.Helper, namespace, name string) error {
	if _, err := helper.Get(namespace, name); err == nil {
		_, err = helper.Delete(namespace, name)
		if err != nil {
			return err
		}
	}
	return nil
}
