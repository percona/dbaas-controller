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
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestKubeClient(t *testing.T) {
	t.Parallel()
	deployment := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: echo-deployment
  labels:
    app: echo
spec:
  replicas: 3
  selector:
    matchLabels:
      app: echo
  template:
    metadata:
      labels:
        app: echo
    spec:
      containers:
      - name: echo
        image: alpine
        command: ['sh', '-c', '--']
        args: ["while true; do echo 'Hello' && sleep 10; done"]
        ports:
        - containerPort: 80
`
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	namespace := os.Getenv("NAMESPACE")
	require.NoError(t, err)
	k, err := NewFromKubeConfigString(string(kubeconfig))
	assert.NoError(t, err)

	podList, err := k.GetPods(context.Background(), "non-existent-namespace", "")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(podList.Items))

	err = k.ApplyFile(context.Background(), []byte(deployment))

	assert.NoError(t, err)

	cs, err := k.GetStorageClasses(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, cs.Items)

	versions, err := k.GetAPIVersions(context.Background())
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(versions))

	_, err = k.GetPersistentVolumes(context.Background())
	assert.NoError(t, err)
	time.Sleep(8 * time.Second)

	pods, err := k.GetPods(context.Background(), namespace, "")
	assert.NoError(t, err)

	nodes, err := k.GetNodes(context.Background())
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(nodes.Items))

	logs, err := k.GetLogs(context.Background(), pods.Items[0].Name, pods.Items[0].Spec.Containers[0].Name)
	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(logs))

	assert.NoError(t, k.DeleteFile(context.Background(), []byte(deployment)))
	time.Sleep(time.Second)
}

func TestInCluster(t *testing.T) {
	t.Parallel()
	_, err := NewFromIncluster()
	require.Error(t, err)
}

func TestConfigGetter(t *testing.T) {
	t.Parallel()
	g := NewConfigGetter("")
	c, err := g.loadFromString()
	require.NoError(t, err)
	expected := &clientcmdapi.Config{
		Preferences: clientcmdapi.Preferences{
			Extensions: make(map[string]runtime.Object),
		},
		AuthInfos:  make(map[string]*clientcmdapi.AuthInfo),
		Clusters:   make(map[string]*clientcmdapi.Cluster),
		Contexts:   make(map[string]*clientcmdapi.Context),
		Extensions: make(map[string]runtime.Object),
	}
	require.Equal(t, expected, c)
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)
	g = NewConfigGetter(string(kubeconfig))
	c, err = g.loadFromString()
	require.NoError(t, err)
	assert.NotEqual(t, 0, len(c.AuthInfos))
	assert.NotEqual(t, 0, len(c.Clusters))
	assert.NotEqual(t, 0, len(c.Contexts))
}

func TestGetSecret(t *testing.T) {
	t.Parallel()
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)
	k, err := NewFromKubeConfigString(string(kubeconfig))
	require.NoError(t, err)
	secret := &corev1.Secret{ //nolint: exhaustruct
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "supersecret",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"username": []byte("hello"),
			"password": []byte("pa$$w0rd"),
		},
	}
	err = k.Apply(context.Background(), secret)
	assert.NoError(t, err)

	s, err := k.GetSecret(context.Background(), "supersecret")
	assert.NoError(t, err)
	assert.Equal(t, secret.Data["username"], s.Data["username"])
	assert.Equal(t, secret.Data["password"], s.Data["password"])

	err = k.Delete(context.Background(), secret)
	assert.NoError(t, err)

}
