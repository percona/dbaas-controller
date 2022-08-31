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

// Package k8sclient provides client for kubernetes.
package k8sclient

import (
	"io/ioutil"
	"os"
	"testing"

	goversion "github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/psmdb"
	"github.com/percona-platform/dbaas-controller/service/k8sclient/internal/pxc"
	"github.com/percona-platform/dbaas-controller/utils/app"
)

func TestPXCSpec(t *testing.T) {
	t.Parallel()
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)
	ctx := app.Context()

	client, err := New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	params := &PXCParams{
		Name: "testCluster",
		Size: 1,
		PXC: &PXC{
			DiskSize: "1000000000",
			Image:    "percona/percona-xtradb-cluster:8.0.23-14.1",
			ComputeResources: &ComputeResources{
				CPUM:        "600m",
				MemoryBytes: "1G",
			},
		},
		ProxySQL: &ProxySQL{
			DiskSize: "1000000000",
			ComputeResources: &ComputeResources{
				CPUM:        "250m",
				MemoryBytes: "500M",
			},
		},
		PMM:               new(PMM),
		VersionServiceURL: "https://check.percona.com",
	}
	t.Run("should fallback to default spec once template does not exist", func(t *testing.T) {
		t.Parallel()

		spec, err := client.createPXCSpecFromParams(params, "secret", "1.11.0", "storage", "")
		assert.NoError(t, err)
		defaultSpec := client.getDefaultPXCSpec(params, "secret", "1.11.0", "storage", "")
		assert.Equal(t, defaultSpec, spec)
	})

	t.Run("test overrideSpec", func(t *testing.T) {
		rawSpec := `apiVersion: pxc.percona.com/v1
kind: PerconaXtraDBCluster
metadata:
  finalizers:
  - delete-proxysql-pvc
  - delete-pxc-pvc
  name: cns
  generation: 1
  namespace: default
spec:
  allowUnsafeConfigurations: true
  backup:
    image: percona/percona-xtradb-cluster-operator:1.11.0-pxc8.0-backup
    schedule:
    - keep: 3
      name: test
      schedule: '0 0 * * *'
      storageName: pxc-backup-storage-cns
    serviceAccountName: percona-xtradb-cluster-operator
    storages:
      pxc-backup-storage-cns:
        s3:
          bucket: ""
          credentialsSecret: ""
        type: filesystem
        volume:
          persistentVolumeClaim:
            resources:
              requests:
                storage: "25000000000"
  crVersion: 1.11.0
  haproxy:
    affinity:
      antiAffinityTopologyKey: none
    enabled: true
    image: percona/percona-xtradb-cluster-operator:1.11.0-haproxy
    imagePullPolicy: IfNotPresent
    resources:
      limits:
        cpu: 500m
        memory: "500000000"
    size: 3
  pause: false
  pmm:
    enabled: true
    image: percona/pmm-client:2
    imagePullPolicy: IfNotPresent
    resources:
      requests:
        cpu: 500m
        memory: 300M
    serverHost: ec2-54-200-65-164.us-west-2.compute.amazonaws.com:8443
    serverUser: api_key
  pxc:
    affinity:
      antiAffinityTopologyKey: none
    image: percona/percona-xtradb-cluster:8.0.27-18.1
    imagePullPolicy: IfNotPresent
    podDisruptionBudget:
      maxUnavailable: 1
    configuration: |
      [mysqld]
      sql-mode = 'ONLY_FULL_GROUP_BY,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'
      innodb_buffer_pool_size=1G
      skip_name_resolve
      innodb_log_file_size=2G
    expose:
      enabled: true
      type: LoadBalancer
      trafficPolicy: Cluster
      loadBalancerSourceRanges:
        - 181.170.213.40/32
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-nlb-target-type: ip
        service.beta.kubernetes.io/aws-load-balancer-scheme: internet-facing
        service.beta.kubernetes.io/aws-load-balancer-target-group-attributes: preserve_client_ip.enabled=true
        service.beta.kubernetes.io/aws-load-balancer-type: external
    resources:
      limits:
        cpu: 1500m
        memory: "3000000000"
    size: 3
    volumeSpec:
      persistentVolumeClaim:
        storageClassName: gp2-enc
        resources:
          requests:
            storage: "25000000000"
  secretsName: dbaas-cns-pxc-secrets
  updateStrategy: RollingUpdate
`
		t.Parallel()
		spec := new(pxc.PerconaXtraDBCluster)
		err := client.unmarshalTemplate([]byte(rawSpec), spec)
		mysqlConfig := "[mysqld]\nsql-mode = 'ONLY_FULL_GROUP_BY,ERROR_FOR_DIVISION_BY_ZERO,NO_ENGINE_SUBSTITUTION'\ninnodb_buffer_pool_size=1G\nskip_name_resolve\ninnodb_log_file_size=2G\n"
		assert.NoError(t, err)
		params.Expose = true
		spec = client.overridePXCSpec(spec, params, "pxc-backup-storage-cns", "1.11.0")
		assert.Equal(t, mysqlConfig, spec.Spec.PXC.Configuration)
		assert.True(t, spec.Spec.PXC.Expose.Enabled)
		assert.NotEqual(t, 0, len(spec.Spec.PXC.Expose.Annotations))
		params.Expose = false
		spec = client.overridePXCSpec(spec, params, "pxc-backup-storage-cns", "1.11.0")
		assert.False(t, spec.Spec.PXC.Expose.Enabled)
		assert.Equal(t, 0, len(spec.Spec.PXC.Expose.Annotations))
	})
}
func TestPSMDBSpec(t *testing.T) {
	t.Parallel()
	kubeconfig, err := ioutil.ReadFile(os.Getenv("HOME") + "/.kube/config")
	require.NoError(t, err)
	ctx := app.Context()

	client, err := New(ctx, string(kubeconfig))
	assert.NoError(t, err)
	params := &PSMDBParams{
		Name: "psmdb-cluster",
		Size: 3,
		Replicaset: &Replicaset{
			DiskSize: "1000000000",
		},
		PMM:         new(PMM),
		BackupImage: "percona/percona-backup-mongodb:1.7.0",
	}
	extra := extraCRParams{
		expose: psmdb.Expose{
			Enabled:    true,
			ExposeType: common.ServiceTypeLoadBalancer,
		},
	}
	operator, _ := goversion.NewVersion("1.12.0")
	extra.operators = &Operators{
		PsmdbOperatorVersion: "1.12.0",
	}
	assert.NoError(t, err)
	t.Run("should fallback to default spec once template does not exist", func(t *testing.T) {
		t.Parallel()
		spec, err := client.createPSMDBSpec(operator, params, extra)
		assert.NoError(t, err)
		defaultSpec := client.getPSMDBSpec112Plus(params, extra)
		assert.Equal(t, defaultSpec, spec)
		params.Expose = false
		spec = client.overridePSMDBSpec(spec, params, extra)
		assert.False(t, spec.Spec.Sharding.Mongos.Expose.Enabled)
		assert.Empty(t, spec.Spec.Sharding.Mongos.Expose.ExposeType)
	})

}
