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

package k8sclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	goversion "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
	"github.com/percona-platform/dbaas-controller/utils/app"
	"github.com/percona-platform/dbaas-controller/utils/logger"
)

// VersionServiceClient represents a client for Version Service API.
type VersionServiceClient struct {
	url  string
	http *http.Client
}

// NewVersionServiceClient creates a new client for given version service URL.
func NewVersionServiceClient(url string) *VersionServiceClient {
	return &VersionServiceClient{
		url: url,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// componentsParams contains params to filter components in version service API.
type componentsParams struct {
	product        string
	productVersion string
}

type matrix struct {
	PXCOperator   map[string]interface{} `json:"pxcOperator,omitempty"`
	PSMDBOperator map[string]interface{} `json:"psmdbOperator,omitempty"`
}

type Version struct {
	Product        string `json:"product"`
	ProductVersion string `json:"operator"`
	Matrix         matrix `json:"matrix"`
}

// VersionServiceResponse represents response from version service API.
type VersionServiceResponse struct {
	Versions []Version `json:"versions"`
}

var errNoVersionsFound error = errors.New("no versions to compare current version with found")

func latest(m map[string]interface{}) (*goversion.Version, error) {
	if len(m) == 0 {
		return nil, errNoVersionsFound
	}
	latest := goversion.Must(goversion.NewVersion("0.0.0"))
	for version := range m {
		parsedVersion, err := goversion.NewVersion(version)
		if err != nil {
			return nil, err
		}
		if parsedVersion.GreaterThan(latest) {
			latest = parsedVersion
		}
	}
	return latest, nil
}

func latestProduct(s []Version) (*goversion.Version, error) {
	if len(s) == 0 {
		return nil, errNoVersionsFound
	}
	latest := goversion.Must(goversion.NewVersion("0.0.0"))
	for _, version := range s {
		parsedVersion, err := goversion.NewVersion(version.ProductVersion)
		if err != nil {
			return nil, err
		}
		if parsedVersion.GreaterThan(latest) {
			latest = parsedVersion
		}
	}
	return latest, nil
}

// Matrix calls version service with given params and returns components matrix.
func (c *VersionServiceClient) Matrix(ctx context.Context, params componentsParams) (*VersionServiceResponse, error) {
	baseURL, err := url.Parse(c.url)
	if err != nil {
		return nil, err
	}
	paths := []string{baseURL.Path, params.product}
	if params.productVersion != "" {
		paths = append(paths, params.productVersion)
	}
	baseURL.Path = path.Join(paths...)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var vsResponse VersionServiceResponse
	err = json.Unmarshal(body, &vsResponse)
	if err != nil {
		return nil, err
	}

	return &vsResponse, nil
}

// LatestOperatorVersion return latest PXC and PSMDB operators for given PMM version.
func (c *VersionServiceClient) LatestOperatorVersion(ctx context.Context, pmmVersion string) (*goversion.Version, *goversion.Version, error) {
	if pmmVersion == "" {
		return nil, nil, errors.New("given PMM version is empty")
	}
	params := componentsParams{
		product:        "pmm-server",
		productVersion: pmmVersion,
	}
	resp, err := c.Matrix(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	if len(resp.Versions) != 1 {
		return nil, nil, nil // no deps for the PMM version passed to c.Matrix
	}
	pmmVersionDeps := resp.Versions[0]
	latestPSMDBOperator, err := latest(pmmVersionDeps.Matrix.PSMDBOperator)
	if err != nil {
		return nil, nil, err
	}
	latestPXCOperator, err := latest(pmmVersionDeps.Matrix.PXCOperator)
	if err != nil {
		return nil, nil, err
	}
	return latestPXCOperator, latestPSMDBOperator, nil
}

const (
	consumedResourcesTestPodsManifestPath string = "../../deploy/test-pods.yaml"
)

// pod is struct just for testing purposes. It contains expected pod and
// container names.
type pod struct {
	name       string
	containers []string
}

func TestK8sClient(t *testing.T) {
	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	client, err := New(ctx, string(kubeconfig))
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
	})

	l := logger.Get(ctx)

	versionService := NewVersionServiceClient("https://check-dev.percona.com/versions/v1")
	pmmVersions, err := versionService.Matrix(ctx, componentsParams{product: "pmm-server"})
	require.NoError(t, err)
	latestPMMVersion, err := latestProduct(pmmVersions.Versions)
	require.NoError(t, err)
	pxc, psmdb, err := versionService.LatestOperatorVersion(ctx, latestPMMVersion.String())
	require.NoError(t, err)

	err = client.ApplyOperator(ctx, pxc.String(), app.DefaultPXCOperatorURLTemplate)
	require.NoError(t, err)

	err = client.ApplyOperator(ctx, psmdb.String(), app.DefaultPSMDBOperatorURLTemplate)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = client.kubeCtl.Run(ctx, []string{"wait", "--for=condition=Available", "deployment", "percona-xtradb-cluster-operator"}, nil)
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	require.NoError(t, err)
	var res interface{}
	err = client.kubeCtl.Get(ctx, "deployment", "percona-xtradb-cluster-operator", &res)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = client.kubeCtl.Run(ctx, []string{"wait", "--for=condition=Available", "deployment", "percona-server-mongodb-operator"}, nil)
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}
	require.NoError(t, err)
	err = client.kubeCtl.Get(ctx, "deployment", "percona-server-mongodb-operator", &res)
	require.NoError(t, err)

	t.Run("Get non-existing clusters", func(t *testing.T) {
		t.Parallel()
		_, err := client.GetPSMDBClusterCredentials(ctx, "d0ca1166b638c-psmdb")
		assert.EqualError(t, errors.Cause(err), ErrNotFound.Error())
		_, err = client.GetXtraDBClusterCredentials(ctx, "871f766d43f8e-xtradb")
		assert.EqualError(t, errors.Cause(err), ErrNotFound.Error())
	})

	var pmm *PMM
	t.Run("XtraDB", func(t *testing.T) {
		t.Parallel()
		name := "test-cluster-xtradb"
		_ = client.DeleteXtraDBCluster(ctx, name)

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster == nil
		})

		l.Info("No XtraDB Clusters running")

		err := client.CreateXtraDBCluster(ctx, &XtraDBParams{
			Name: name,
			Size: 1,
			PXC: &PXC{
				DiskSize: "1000000000",
				Image:    "percona/percona-xtradb-cluster:8.0.20-11.1",
			},
			ProxySQL:          &ProxySQL{DiskSize: "1000000000"},
			PMM:               pmm,
			VersionServiceURL: "https://check.percona.com",
		})
		require.NoError(t, err)

		l.Info("XtraDB Cluster is created")

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State != ClusterStateInvalid
		})

		t.Run("Make sure XtraDB cluster is in changing state right after creation", func(t *testing.T) {
			cluster, err := getXtraDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, ClusterStateChanging, cluster.State)
		})

		t.Run("Get credentials of cluster that is not Ready", func(t *testing.T) {
			_, err := client.GetXtraDBClusterCredentials(ctx, name)
			assert.EqualError(t, errors.Cause(err), ErrXtraDBClusterNotReady.Error())
		})

		t.Run("Create cluster with the same name", func(t *testing.T) {
			err = client.CreateXtraDBCluster(ctx, &XtraDBParams{
				Name:     name,
				Size:     1,
				PXC:      &PXC{DiskSize: "1000000000"},
				ProxySQL: &ProxySQL{DiskSize: "1000000000"},
				PMM:      pmm,
			})
			require.Error(t, err)
			assert.Equal(t, err.Error(), fmt.Sprintf(clusterWithSameNameExistsErrTemplate, name))
		})

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		t.Run("All pods are ready", func(t *testing.T) {
			cluster, err := getXtraDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, int32(2), cluster.DetailedState.CountReadyPods())
			assert.Equal(t, int32(2), cluster.DetailedState.CountAllPods())
		})

		t.Run("Get logs", func(t *testing.T) {
			pods, err := client.GetPods(ctx, "-lapp.kubernetes.io/instance="+name)
			require.NoError(t, err)

			expectedPods := []pod{
				{
					name:       name + "-proxysql-0",
					containers: []string{"pmm-client", "proxysql", "pxc-monit", "proxysql-monit"},
				},
				{
					name:       name + "-pxc-0",
					containers: []string{"pxc", "pmm-client", "pxc-init"},
				},
			}
			for _, ppod := range pods.Items {
				var foundPod pod
				assert.Conditionf(t,
					func(ppod common.Pod) assert.Comparison {
						return func() bool {
							for _, expectedPod := range expectedPods {
								if ppod.Name == expectedPod.name {
									foundPod = expectedPod
									return true
								}
							}
							return false
						}
					}(ppod),
					"pod name '%s' was not expected",
					ppod.Name,
				)

				for _, container := range ppod.Spec.Containers {
					assert.Conditionf(
						t,
						func(container common.ContainerSpec) assert.Comparison {
							return func() bool {
								for _, expectedContainerName := range foundPod.containers {
									if expectedContainerName == container.Name {
										return true
									}
								}
								return false
							}
						}(container),
						"container name '%s' was not expected",
						container.Name,
					)

					logs, err := client.GetLogs(ctx, ppod.Status.ContainerStatuses, ppod.Name, container.Name)
					require.NoError(t, err, "failed to get logs")
					assert.Greater(t, len(logs), 0)
					for _, l := range logs {
						assert.False(t, strings.Contains(l, "\n"), "new lines should have been removed")
					}
				}
			}
		})

		t.Run("Upgrade Xtradb", func(t *testing.T) {
			err = client.UpdateXtraDBCluster(ctx, &XtraDBParams{
				Name: name,
				Size: 1,
				PXC: &PXC{
					DiskSize: "1000000000",
					Image:    "percona/percona-xtradb-cluster:8.0.20-11.2",
				},
				ProxySQL:          &ProxySQL{DiskSize: "1000000000"},
				PMM:               pmm,
				VersionServiceURL: "https://check.percona.com",
			})
			require.NoError(t, err)
			assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
				return cluster != nil && cluster.State == ClusterStateUpgrading
			})
			l.Infof("upgrade of XtraDB cluster %q has begun", name)

			assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
				return cluster != nil && cluster.State == ClusterStateReady
			})
			l.Infof("XtraDB cluster %q has been upgraded", name)

			cluster, err := getXtraDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, "percona/percona-xtradb-cluster:8.0.20-11.2", cluster.PXC.Image)
		})

		err = client.RestartXtraDBCluster(ctx, name)
		require.NoError(t, err)
		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateChanging
		})

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})
		l.Info("XtraDB Cluster is restarted")

		err = client.UpdateXtraDBCluster(ctx, &XtraDBParams{
			Name: name,
			Size: 3,
		})
		require.NoError(t, err)
		l.Info("XtraDB Cluster is updated")

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(3), cluster.Size)
				return true
			}
			return false
		})

		err = client.DeleteXtraDBCluster(ctx, name)
		require.NoError(t, err)

		assertListXtraDBCluster(ctx, t, client, name, func(cluster *XtraDBCluster) bool {
			return cluster == nil
		})
		l.Info("XtraDB Cluster is deleted")
	})

	t.Run("Create XtraDB with HAProxy", func(t *testing.T) {
		t.Parallel()
		clusterName := "test-pxc-haproxy"
		err := client.CreateXtraDBCluster(ctx, &XtraDBParams{
			Name:    clusterName,
			Size:    1,
			PXC:     &PXC{DiskSize: "1000000000"},
			HAProxy: new(HAProxy),
			PMM:     pmm,
		})
		require.NoError(t, err)
		assertListXtraDBCluster(ctx, t, client, clusterName, func(cluster *XtraDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		// Test listing.
		clusters, err := client.ListXtraDBClusters(ctx)
		require.NoError(t, err)
		assert.Conditionf(t,
			func(clusters []XtraDBCluster, clusterName string) assert.Comparison {
				return func() bool {
					for _, cluster := range clusters {
						if cluster.Name == clusterName {
							return true
						}
					}
					return false
				}
			}(clusters, clusterName),
			"cluster '%s' was not found",
			clusterName,
		)

		err = client.DeleteXtraDBCluster(ctx, clusterName)
		require.NoError(t, err)
	})

	t.Run("PSMDB", func(t *testing.T) {
		t.Parallel()
		name := "test-cluster-psmdb"
		_ = client.DeletePSMDBCluster(ctx, name)

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster == nil
		})

		l.Info("No PSMDB Clusters running")

		err := client.CreatePSMDBCluster(ctx, &PSMDBParams{
			Name:       name,
			Size:       3,
			Replicaset: &Replicaset{DiskSize: "1000000000"},
			PMM:        pmm,
		})
		require.NoError(t, err)

		l.Info("PSMDB Cluster is created")

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State != ClusterStateInvalid
		})

		t.Run("Make sure PSMDB cluster is in changing state right after creation", func(t *testing.T) {
			cluster, err := getPSMDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, ClusterStateChanging, cluster.State)
		})

		t.Run("Get credentials of cluster that is not Ready", func(t *testing.T) {
			_, err := client.GetPSMDBClusterCredentials(ctx, name)
			assert.EqualError(t, errors.Cause(err), ErrPSMDBClusterNotReady.Error())
		})

		t.Run("Create cluster with the same name", func(t *testing.T) {
			err = client.CreatePSMDBCluster(ctx, &PSMDBParams{
				Name:              name,
				Size:              1,
				Replicaset:        &Replicaset{DiskSize: "1000000000"},
				PMM:               pmm,
				Image:             "percona/percona-server-mongodb:4.4.5-7",
				VersionServiceURL: "https://check.percona.com",
			})
			require.Error(t, err)
			assert.Equal(t, err.Error(), fmt.Sprintf(clusterWithSameNameExistsErrTemplate, name))
		})

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})

		t.Run("All pods are ready", func(t *testing.T) {
			cluster, err := getPSMDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, int32(9), cluster.DetailedState.CountReadyPods())
			assert.Equal(t, int32(9), cluster.DetailedState.CountAllPods())
		})

		t.Run("Upgrade PSMDB", func(t *testing.T) {
			err = client.UpdatePSMDBCluster(ctx, &PSMDBParams{
				Name:              name,
				Size:              3,
				Replicaset:        &Replicaset{DiskSize: "1000000000"},
				PMM:               pmm,
				Image:             "percona/percona-server-mongodb:4.4.6-8",
				VersionServiceURL: "https://check.percona.com",
			})
			require.NoError(t, err)

			assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
				return cluster != nil && cluster.State == ClusterStateUpgrading
			})
			l.Infof("upgrade of PSMDB cluster %q has begun", name)

			assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
				return cluster != nil && cluster.State == ClusterStateReady
			})
			l.Infof("PSMDB Cluster %q has been upgraded", name)

			cluster, err := getPSMDBCluster(ctx, client, name)
			require.NoError(t, err)
			assert.Equal(t, "percona/percona-server-mongodb:4.4.6-8", cluster.Image)
		})

		err = client.RestartPSMDBCluster(ctx, name)
		require.NoError(t, err)

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateChanging
		})

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster != nil && cluster.State == ClusterStateReady
		})
		l.Info("PSMDB Cluster is restarted")

		err = client.UpdatePSMDBCluster(ctx, &PSMDBParams{
			Name:  name,
			Size:  5,
			Image: "percona/percona-server-mongodb:4.4.6-8",
		})
		require.NoError(t, err)
		l.Info("PSMDB Cluster is updated")

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			if cluster != nil && cluster.State == ClusterStateReady {
				assert.Equal(t, int32(5), cluster.Size)
				return true
			}
			return false
		})

		err = client.DeletePSMDBCluster(ctx, name)
		require.NoError(t, err)

		assertListPSMDBCluster(ctx, t, client, name, func(cluster *PSMDBCluster) bool {
			return cluster == nil
		})
		l.Info("PSMDB Cluster is deleted")
	})

	t.Run("CheckOperators", func(t *testing.T) {
		t.Parallel()
		operators, err := client.CheckOperators(ctx)
		require.NoError(t, err)
		require.NotNil(t, operators)
		_, err = goversion.NewVersion(operators.PsmdbOperatorVersion)
		require.NoError(t, err)
		_, err = goversion.NewVersion(operators.XtradbOperatorVersion)
		require.NoError(t, err)
	})
}

// ErrNoSuchCluster indicates that no cluster with given name was found.
var ErrNoSuchCluster error = errors.New("no cluster found with given name")

func getPSMDBCluster(ctx context.Context, client *K8sClient, name string) (*PSMDBCluster, error) {
	l := logger.Get(ctx)
	clusters, err := client.ListPSMDBClusters(ctx)
	if err != nil {
		return nil, err
	}
	l.Debug(clusters)
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, ErrNoSuchCluster
}

func getXtraDBCluster(ctx context.Context, client *K8sClient, name string) (*XtraDBCluster, error) {
	l := logger.Get(ctx)
	clusters, err := client.ListXtraDBClusters(ctx)
	if err != nil {
		return nil, err
	}
	l.Debug(clusters)
	for _, c := range clusters {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, ErrNoSuchCluster
}

func assertListXtraDBCluster(ctx context.Context, t *testing.T, client *K8sClient, name string, conditionFunc func(cluster *XtraDBCluster) bool) {
	t.Helper()
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	for {
		time.Sleep(5 * time.Second)
		cluster, err := getXtraDBCluster(timeoutCtx, client, name)
		if !errors.Is(err, ErrNoSuchCluster) {
			require.NoError(t, err)
		}

		if conditionFunc(cluster) {
			break
		}
	}
}

func assertListPSMDBCluster(ctx context.Context, t *testing.T, client *K8sClient, name string, conditionFunc func(cluster *PSMDBCluster) bool) {
	t.Helper()
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	for {
		time.Sleep(1 * time.Second)
		cluster, err := getPSMDBCluster(timeoutCtx, client, name)
		if !errors.Is(err, ErrNoSuchCluster) {
			require.NoError(t, err)
		}

		if conditionFunc(cluster) {
			break
		}
	}
}

func TestGetConsumedCPUAndMemory(t *testing.T) {
	t.Parallel()
	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	client, err := New(ctx, string(kubeconfig))
	require.NoError(t, err)

	b := make([]byte, 4)
	n, err := rand.Read(b)
	require.NoError(t, err)
	require.Equal(t, 4, n)
	consumedResourcesTestNamespace := "consumed-resources-test-" + hex.EncodeToString(b)

	t.Cleanup(func() {
		_, err := client.kubeCtl.Run(ctx, []string{"delete", "ns", consumedResourcesTestNamespace}, nil)
		require.NoError(t, err)
		err = client.Cleanup()
		require.NoError(t, err)
	})

	_, err = client.kubeCtl.Run(ctx, []string{"create", "ns", consumedResourcesTestNamespace}, nil)
	require.NoError(t, err)

	args := []string{
		"apply", "-f", consumedResourcesTestPodsManifestPath,
		"-n" + consumedResourcesTestNamespace,
	}
	_, err = client.kubeCtl.Run(ctx, args, nil)
	require.NoError(t, err)
	args = []string{
		"wait", "--for=condition=ready", "--timeout=20s",
		"pods", "hello1", "hello2", "-n" + consumedResourcesTestNamespace,
	}
	_, err = client.kubeCtl.Run(ctx, args, nil)
	require.NoError(t, err)

	cpuMillis, memoryBytes, err := client.GetConsumedCPUAndMemory(ctx, consumedResourcesTestNamespace)
	require.NoError(t, err)
	assert.Equal(t, uint64(40), cpuMillis)
	assert.Equal(t, uint64(192928615), memoryBytes)

	// Test we dont include succeeded and failed pods into consumed resources.
	// Wait for test pods to be completed:
	timeout, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	for {
		time.Sleep(3 * time.Second)
		select {
		case <-timeout.Done():
			t.Error("Timeout waiting for hello1 and hello2 pods to complete!")
			return
		default:
		}
		list, err := client.GetPods(ctx, "-n"+consumedResourcesTestNamespace, "hello1", "hello2")
		require.NoError(t, err)
		var failed, succeeded bool
		for _, pod := range list.Items {
			if pod.Name == "hello1" {
				succeeded = pod.Status.Phase == common.PodPhaseSucceded
				continue
			}
			if pod.Name == "hello2" {
				failed = pod.Status.Phase == common.PodPhaseFailed
			}
		}

		if failed && succeeded {
			break
		}
	}

	cpuMillis, memoryBytes, err = client.GetConsumedCPUAndMemory(ctx, consumedResourcesTestNamespace)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), cpuMillis)
	assert.Equal(t, uint64(0), memoryBytes)
}

func TestGetAllClusterResources(t *testing.T) {
	t.Parallel()
	ctx := app.Context()

	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)

	client, err := New(ctx, string(kubeconfig))
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Cleanup()
		require.NoError(t, err)
	})

	// test getWorkerNodes
	nodes, err := client.getWorkerNodes(ctx)
	require.NoError(t, err)
	require.NotNil(t, nodes)
	assert.Greater(t, len(nodes), 0)
	for _, node := range nodes {
		cpu, ok := node.Status.Allocatable[common.ResourceCPU]
		assert.Truef(t, ok, "no value in node.Status.Allocatable under key %s", common.ResourceCPU)
		assert.NotEmpty(t, cpu)
		memory, ok := node.Status.Allocatable[common.ResourceMemory]
		assert.Truef(t, ok, "no value in node.Status.Allocatable under key %s", common.ResourceMemory)
		assert.NotEmpty(t, memory)
	}

	clusterType := client.GetKubernetesClusterType(ctx)
	var volumes *common.PersistentVolumeList
	if clusterType == AmazonEKSClusterType {
		volumes, err = client.GetPersistentVolumes(ctx)
		require.NoError(t, err)
	}
	cpuMillis, memoryBytes, storageBytes, err := client.GetAllClusterResources(ctx, clusterType, volumes)
	require.NoError(t, err)
	// We check 1 CPU because it is hard to imagine somebody running cluster with less CPU allocatable.
	assert.GreaterOrEqual(
		t, cpuMillis, uint64(len(nodes)*1000),
		"expected to have at lease 1 CPU per node available to be allocated by pods",
	)

	// The same for memory, hard to imagine having less than 1 GB allocatable per node.
	assert.GreaterOrEqual(
		t, memoryBytes, uint64(len(nodes))*1000*1000*1000,
		"expected to have at lease 1GB of memory per node available to be allocated by pods",
	)

	// The same for storage, hard to imagine having less than 4GB of storage per node.
	assert.GreaterOrEqual(
		t, storageBytes, uint64(len(nodes))*1000*1000*1000*4,
		"expected to have at lease 4GB of storage per node.",
	)
}

func TestVMAgentSpec(t *testing.T) {
	t.Parallel()
	expected := `{
  "kind": "VMAgent",
  "apiVersion": "operator.victoriametrics.com/v1beta1",
  "metadata": {
    "name": "pmm-vmagent-rws-basic-auth"
  },
  "spec": {
    "serviceScrapeNamespaceSelector": {},
    "serviceScrapeSelector": {},
    "podScrapeNamespaceSelector": {},
    "podScrapeSelector": {},
    "probeSelector": {},
    "probeNamespaceSelector": {},
    "staticScrapeSelector": {},
    "staticScrapeNamespaceSelector": {},
    "replicaCount": 1,
    "resources": {
      "requests": {
        "memory": "350Mi",
        "cpu": "250m"
      },
      "limits": {
        "memory": "850Mi",
        "cpu": "500m"
      }
    },
    "additionalArgs": {
      "memory.allowedPercent": "40"
    },
    "remoteWrite": [
      {
        "url": "http://vmsingle-example-vmsingle-pvc.default.svc:8429/victoriametrics/api/v1/write",
        "basicAuth": {
          "username": {
            "name": "rws-basic-auth",
            "key": "username"
          },
          "password": {
            "name": "rws-basic-auth",
            "key": "password"
          }
        },
        "tlsConfig": {
          "insecureSkipVerify": true
        }
      }
    ]
  }
}
`
	spec := vmAgentSpec(
		&PMM{PublicAddress: "http://vmsingle-example-vmsingle-pvc.default.svc:8429"},
		"rws-basic-auth",
	)
	var inBuf bytes.Buffer
	e := json.NewEncoder(&inBuf)
	e.SetIndent("", "  ")
	err := e.Encode(spec)
	require.NoError(t, err)
	assert.Equal(t, expected, inBuf.String())
}
