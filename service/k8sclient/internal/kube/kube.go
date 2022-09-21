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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
	pxcv1 "github.com/percona/percona-xtradb-cluster-operator/pkg/apis/pxc/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	v1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/reference"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kube/psmdb"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/kube/pxc"
)

const (
	defaultAPIURIPath  = "/api"
	defaultAPIsURIPath = "/apis"
	defaultQPSLimit    = 100
	defaultBurstLimit  = 150
	dbaasToolPath      = "/opt/dbaas-tools/bin"
	PXCKind            = pxc.PXCKind
	PSMDBKind          = psmdb.PSMDBKind
	defaultChunkSize   = 500
)

// Each level has 2 spaces for PrefixWriter
const (
	LEVEL_0 = iota
	LEVEL_1
	LEVEL_2
	LEVEL_3
	LEVEL_4
)

var restartTemplate = `{
    "spec": {
        "template": {
            "metadata": {
                "annotations": {
                    "kubectl.kubernetes.io/restartedAt": "%s"
                }
            }
        }
    }
}`

// SortableEvents implements sort.Interface for []api.Event based on the Timestamp field
type SortableEvents []corev1.Event

func (list SortableEvents) Len() int {
	return len(list)
}

func (list SortableEvents) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list SortableEvents) Less(i, j int) bool {
	return list[i].LastTimestamp.Time.Before(list[j].LastTimestamp.Time)
}

type BackupSpec struct {
	Image string `json:"image"`
}
type PXCSpec struct {
	Image string `json:"image"`
}
type UpgradeOptions struct {
	VersionServiceEndpoint string `json:"versionServiceEndpoint,omitempty"`
	Apply                  string `json:"apply,omitempty"`
	Schedule               string `json:"schedule,omitempty"`
	SetFCV                 bool   `json:"setFCV,omitempty"`
}

type Spec struct {
	CRVersion      string         `json:"crVersion"`
	Image          string         `json:"image"`
	PXCSpec        PXCSpec        `json:"pxc"`
	UpgradeOptions UpgradeOptions `json:"upgradeOptions"`
	Backup         BackupSpec     `json:"backup"`
}
type OperatorPatch struct {
	Spec Spec `json:"spec"`
}

// configGetter stores kubeconfig string to convert it to the final object
type configGetter struct {
	kubeconfig string
}

type Client struct {
	clientset   *kubernetes.Clientset
	pxcClient   *pxc.PerconaXtraDBClusterClient
	psmdbClient *psmdb.PerconaServerMongoDBClient
	restConfig  *rest.Config
	namespace   string
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
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	config.QPS = defaultQPSLimit
	config.Burst = defaultBurstLimit
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c := &Client{
		clientset:  clientset,
		restConfig: config,
	}
	err = c.setup()
	return c, err
}

// NewFromKubeConfigString creates a new client for the given config string.
// It's intended for clients that expect to be running outside of a cluster
func NewFromKubeConfigString(kubeconfig string) (*Client, error) {
	config, err := clientcmd.BuildConfigFromKubeconfigGetter("", NewConfigGetter(kubeconfig).loadFromString)
	if err != nil {
		return nil, err
	}
	config.QPS = defaultQPSLimit
	config.Burst = defaultBurstLimit
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	c := &Client{
		clientset:  clientset,
		restConfig: config,
	}
	err = c.setup()
	return c, err
}

func (c *Client) setup() error {
	namespace := "default"
	if space := os.Getenv("NAMESPACE"); space != "" {
		namespace = space
	}
	// Set PATH variable to make aws-iam-authenticator executable
	path := fmt.Sprintf("%s:%s", os.Getenv("PATH"), dbaasToolPath)
	os.Setenv("PATH", path)
	c.namespace = namespace
	return c.initOperatorClients()
}

func (c *Client) initOperatorClients() error {
	pxcClient, err := pxc.NewForConfig(c.restConfig)
	if err != nil {
		return err
	}
	psmdbClient, err := psmdb.NewForConfig(c.restConfig)
	if err != nil {
		return err
	}
	c.pxcClient = pxcClient
	c.psmdbClient = psmdbClient
	return nil
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
	namespace, name, err := c.retrieveMetaFromObject(obj)
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
	namespace, name, err := c.retrieveMetaFromObject(obj)
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
		if err != nil {
			return nil, err
		}
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
	return c.clientset.CoreV1().Secrets(c.namespace).Get(ctx, name, metav1.GetOptions{})
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
	defaultLogLines := int64(3000)
	options := new(corev1.PodLogOptions)
	if container != "" {
		options.Container = container
	}
	options.TailLines = &defaultLogLines
	buf := new(bytes.Buffer)

	req := c.clientset.CoreV1().Pods(c.namespace).GetLogs(pod, options)
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

// GetStatefulSet finds statefulset by name.
func (c *Client) GetStatefulSet(ctx context.Context, name string) (*appsv1.StatefulSet, error) {
	return c.clientset.AppsV1().StatefulSets(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

// RestartStatefulSet finds statefulset by name and restarts it.
func (c *Client) RestartStatefulSet(ctx context.Context, name string) (*appsv1.StatefulSet, error) {
	patchData := fmt.Sprintf(restartTemplate, time.Now().UTC().Format(time.RFC3339))
	return c.clientset.AppsV1().StatefulSets(c.namespace).Patch(ctx, name, types.StrategicMergePatchType, []byte(patchData), metav1.PatchOptions{})
}

// ListPXCClusters returns list of managed PCX clusters.
func (c *Client) ListPXCClusters(ctx context.Context) (*pxcv1.PerconaXtraDBClusterList, error) {
	return c.pxcClient.PXCClusters(c.namespace).List(ctx, metav1.ListOptions{})
}

// GetPXCClusters returns PXC clusters by provided name.
func (c *Client) GetPXCCluster(ctx context.Context, name string) (*PXCCluster, error) {
	cluster, err := c.pxcClient.PXCClusters(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	pxc := PXCCluster(*cluster)
	return &pxc, nil
}

// PatchPXCCluster patches CR of managed PXC cluster.
func (c *Client) PatchPXCCluster(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*pxcv1.PerconaXtraDBCluster, error) {
	return c.pxcClient.PXCClusters(c.namespace).Patch(ctx, name, pt, data, opts)
}

// ListPSMDBClusters returns list of managed PSMDB clusters.
func (c *Client) ListPSMDBClusters(ctx context.Context) (*psmdbv1.PerconaServerMongoDBList, error) {
	return c.psmdbClient.PSMDBClusters(c.namespace).List(ctx, metav1.ListOptions{})
}

// GetPSMDBCluster returns PSMDB cluster by provided name,
func (c *Client) GetPSMDBCluster(ctx context.Context, name string) (*psmdbv1.PerconaServerMongoDB, error) {
	return c.psmdbClient.PSMDBClusters(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

// PAtchPSMDBCluster patches CR of managed PSMDB cluster.
func (c *Client) PatchPSMDBCluster(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions) (*psmdbv1.PerconaServerMongoDB, error) {
	return c.psmdbClient.PSMDBClusters(c.namespace).Patch(ctx, name, pt, data, opts)
}

// GetDeployment finds deployment.
func (c *Client) GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
}

// PatchDeployment patches k8s deployment
func (c *Client) PatchDeployment(ctx context.Context, name string, deployment *appsv1.Deployment) error {
	patch, err := json.Marshal(deployment)
	if err != nil {
		return err
	}
	_, err = c.clientset.AppsV1().Deployments(c.namespace).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (c *Client) GetEvents(ctx context.Context, name string) (string, error) {
	pod, err := c.clientset.CoreV1().Pods(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		eventsInterface := c.clientset.CoreV1().Events(c.namespace)
		selector := eventsInterface.GetFieldSelector(&name, &c.namespace, nil, nil)
		initialOpts := metav1.ListOptions{
			FieldSelector: selector.String(),
			Limit:         defaultChunkSize,
		}
		events := &corev1.EventList{}
		err2 := resource.FollowContinue(&initialOpts,
			func(options metav1.ListOptions) (runtime.Object, error) {
				newList, err := eventsInterface.List(ctx, options)
				if err != nil {
					return nil, resource.EnhanceListError(err, options, "events")
				}
				events.Items = append(events.Items, newList.Items...)
				return newList, nil
			})

		if err2 == nil && len(events.Items) > 0 {
			return tabbedString(func(out io.Writer) error {
				w := NewPrefixWriter(out)
				w.Write(0, "Pod '%v': error '%v', but found events.\n", name, err)
				DescribeEvents(events, w)
				return nil
			})
		}
		return "", err
	}

	var events *corev1.EventList
	if ref, err := reference.GetReference(scheme.Scheme, pod); err != nil {
		fmt.Printf("Unable to construct reference to '%#v': %v", pod, err)
	} else {
		ref.Kind = ""
		if _, isMirrorPod := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirrorPod {
			ref.UID = types.UID(pod.Annotations[corev1.MirrorPodAnnotationKey])
		}
		events, _ = searchEvents(c.clientset.CoreV1(), ref, defaultChunkSize)
	}
	return tabbedString(func(out io.Writer) error {
		w := NewPrefixWriter(out)
		w.Write(LEVEL_0, name+" ")
		DescribeEvents(events, w)
		return nil
	})

}
func tabbedString(f func(io.Writer) error) (string, error) {
	out := new(tabwriter.Writer)
	buf := &bytes.Buffer{}
	out.Init(buf, 0, 8, 2, ' ', 0)

	err := f(out)
	if err != nil {
		return "", err
	}

	out.Flush()
	str := string(buf.String())
	return str, nil
}
func DescribeEvents(el *corev1.EventList, w PrefixWriter) {
	if len(el.Items) == 0 {
		w.Write(LEVEL_0, "Events:\t<none>\n")
		return
	}
	w.Flush()
	sort.Sort(SortableEvents(el.Items))
	w.Write(LEVEL_0, "Events:\n  Type\tReason\tAge\tFrom\tMessage\n")
	w.Write(LEVEL_1, "----\t------\t----\t----\t-------\n")
	for _, e := range el.Items {
		var interval string
		firstTimestampSince := translateMicroTimestampSince(e.EventTime)
		if e.EventTime.IsZero() {
			firstTimestampSince = translateTimestampSince(e.FirstTimestamp)
		}
		if e.Series != nil {
			interval = fmt.Sprintf("%s (x%d over %s)", translateMicroTimestampSince(e.Series.LastObservedTime), e.Series.Count, firstTimestampSince)
		} else if e.Count > 1 {
			interval = fmt.Sprintf("%s (x%d over %s)", translateTimestampSince(e.LastTimestamp), e.Count, firstTimestampSince)
		} else {
			interval = firstTimestampSince
		}
		source := e.Source.Component
		if source == "" {
			source = e.ReportingController
		}
		w.Write(LEVEL_1, "%v\t%v\t%s\t%v\t%v\n",
			e.Type,
			e.Reason,
			interval,
			source,
			strings.TrimSpace(e.Message),
		)
	}
}

// searchEvents finds events about the specified object.
// It is very similar to CoreV1.Events.Search, but supports the Limit parameter.
func searchEvents(client corev1client.EventsGetter, objOrRef runtime.Object, limit int64) (*corev1.EventList, error) {
	ref, err := reference.GetReference(scheme.Scheme, objOrRef)
	if err != nil {
		return nil, err
	}
	stringRefKind := string(ref.Kind)
	var refKind *string
	if len(stringRefKind) > 0 {
		refKind = &stringRefKind
	}
	stringRefUID := string(ref.UID)
	var refUID *string
	if len(stringRefUID) > 0 {
		refUID = &stringRefUID
	}

	e := client.Events(ref.Namespace)
	fieldSelector := e.GetFieldSelector(&ref.Name, &ref.Namespace, refKind, refUID)
	initialOpts := metav1.ListOptions{FieldSelector: fieldSelector.String(), Limit: limit}
	eventList := &corev1.EventList{}
	err = resource.FollowContinue(&initialOpts,
		func(options metav1.ListOptions) (runtime.Object, error) {
			newEvents, err := e.List(context.TODO(), options)
			if err != nil {
				return nil, resource.EnhanceListError(err, options, "events")
			}
			eventList.Items = append(eventList.Items, newEvents.Items...)
			return newEvents, nil
		})
	return eventList, err
}

func (c *Client) retrieveMetaFromObject(obj runtime.Object) (namespace, name string, err error) {
	name, err = meta.NewAccessor().Name(obj)
	if err != nil {
		return
	}
	namespace, err = meta.NewAccessor().Namespace(obj)
	if err != nil {
		return
	}
	if namespace == "" {
		namespace = c.namespace
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

// translateMicroTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateMicroTimestampSince(timestamp metav1.MicroTime) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}
