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

// Package psmdb contains API Schema definitions for the psmdb v1 API group.
package psmdb

import (
	"github.com/percona-platform/dbaas-controller/service/k8sclient/common"
)

// PerconaServerMongoDBList112 holds a list of PSMDB objects.
type PerconaServerMongoDBList112 struct {
	common.TypeMeta // anonymous for embedding

	Items []PerconaServerMongoDB112 `json:"items"`
}

// Ensure it implements the DatabaseCluster interface.
var _ common.DatabaseCluster = (*PerconaServerMongoDB112)(nil)

// PerconaServerMongoDB112 is the Schema for the perconaservermongodbs 1.12+ API.
type PerconaServerMongoDB112 struct {
	common.TypeMeta   // anonymous for embedding
	common.ObjectMeta `json:"metadata,omitempty"`

	APIVersion string                      `json:"apiVersion,omitempty"`
	Kind       string                      `json:"kind,omitempty"`
	Spec       *PSMDB112Spec               `json:"spec,omitempty"`
	Status     *PerconaServerMongoDBStatus `json:"status,omitempty"`
}

// Nonvoting Non voting members.
type Nonvoting struct {
	Enabled             bool                            `json:"enabled,omitempty"`
	Size                int                             `json:"size,omitempty"`
	Affinity            *PodAffinity                    `json:"affinity,omitempty"`
	PodDisruptionBudget *common.PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	Resources           common.ResourceRequirements     `json:"resources,omitempty"`
	VolumeSpec          *common.VolumeSpec              `json:"volumeSpec,omitempty"`
}

// PSMDB112Spec defines the PSMDB spec section for the operator parameters.
type PSMDB112Spec struct {
	CRVersion                 string            `json:"crVersion,omitempty"`
	Image                     string            `json:"image,omitempty"`
	ImagePullPolicy           string            `json:"imagePullPolicy,omitempty"`
	UpdateStrategy            string            `json:"updateStrategy,omitempty"`
	AllowUnsafeConfigurations bool              `json:"allowUnsafeConfigurations,omitempty"`
	Pause                     bool              `json:"pause"`
	UpgradeOptions            *UpgradeOptions   `json:"upgradeOptions,omitempty"`
	Secrets                   *SecretsSpec      `json:"secrets,omitempty"`
	PMM                       *PmmSpec          `json:"pmm,omitempty"`
	Replsets                  []*ReplsetSpec112 `json:"replsets,omitempty"`
	Sharding                  *ShardingSpec112  `json:"sharding,omitempty"`
	Backup                    *BackupSpec       `json:"backup,omitempty"`
}

// ShardingSpec112 holds fields to configure the shards.
type ShardingSpec112 struct {
	Enabled            bool                          `json:"enabled"`
	ConfigsvrReplSet   *ReplsetSpec                  `json:"configsvrReplSet"`
	Mongos             *ReplsetMongosSpec112         `json:"mongos"`
	OperationProfiling *MongodSpecOperationProfiling `json:"operationProfiling"`
	Expose             *Expose                       `json:"expose"`
}

// ExposeSpec holds information about how the cluster is exposed to the worl via ingress.
type ExposeSpec struct {
	ExposeType common.ServiceType `json:"exposeType"`
}

// ReplsetSpec112 holds information about the replicasets.
type ReplsetSpec112 struct {
	Affinity            *PodAffinity                    `json:"affinity,omitempty"` // Operator 1.12+
	Arbiter             Arbiter                         `json:"arbiter,omitempty"`
	ClusterRole         clusterRole                     `json:"clusterRole,omitempty"`
	Expose              Expose                          `json:"expose,omitempty"`
	LivenessProbe       *livenessProbeExtended          `json:"livenessProbe,omitempty"`
	Name                string                          `json:"name,omitempty"`
	Nonvoting           *Nonvoting                      `json:"nonvoting,omitempty"` // Operator 1.12+
	PodDisruptionBudget *common.PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	Resources           *common.PodResources            `json:"resources,omitempty"`
	Size                int32                           `json:"size"`
	VolumeSpec          *common.VolumeSpec              `json:"volumeSpec,omitempty"`
	// ConfigurationOptions options that will be passed as defined in MongoDB configuration file.
	// See https://github.com/percona/percona-server-mongodb-operator/blob/b304b6c5bb0df2e6e7dac637d23f10fbcbd4800e/pkg/apis/psmdb/v1/psmdb_types.go#L353-L367
	// It must be a multi line string with indentation to produce a map, like:
	// operationProfiling:
	//     mode: slowOp
	Configuration string `json:"configuration,omitempty"` // Operator 1.12+
	MultiAZ
}

// ReplsetMongosSpec112 holds the fields to describe replicaset's Mongos specs.
type ReplsetMongosSpec112 struct {
	Expose              ExposeSpec                      `json:"expose,omitempty"`
	Size                int32                           `json:"size"`
	Resources           *common.PodResources            `json:"resources,omitempty"`
	Name                string                          `json:"name,omitempty"`
	ClusterRole         clusterRole                     `json:"clusterRole,omitempty"`
	VolumeSpec          *common.VolumeSpec              `json:"volumeSpec,omitempty"`
	LivenessProbe       *livenessProbeExtended          `json:"livenessProbe,omitempty"`
	PodDisruptionBudget *common.PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	MultiAZ
}

// SetDatabaseImage sets database image to appropriate image field.
func (p *PerconaServerMongoDB112) SetDatabaseImage(image string) {
	if p.Spec == nil {
		p.Spec = new(PSMDB112Spec)
	}
	p.Spec.Image = image
}

// DatabaseImage returns image of database software used.
func (p *PerconaServerMongoDB112) DatabaseImage() string {
	return p.Spec.Image
}

// GetName returns name of the cluster.
func (p *PerconaServerMongoDB112) GetName() string {
	return p.Name
}

// CRDName returns name of Custom Resource Definition -> cluster's kind.
func (p *PerconaServerMongoDB112) CRDName() string {
	return string(PerconaServerMongoDBKind)
}

// DatabaseContainerNames returns container names that actually run the database.
func (p *PerconaServerMongoDB112) DatabaseContainerNames() []string {
	return []string{"mongos", "mongod"}
}

// DatabasePodLabels return list of labels to get pods where database is running.
func (p *PerconaServerMongoDB112) DatabasePodLabels() []string {
	return []string{"app.kubernetes.io/instance=" + p.Name, "app.kubernetes.io/part-of=percona-server-mongodb"}
}

// Pause returns bool indicating if cluster should be paused.
func (p *PerconaServerMongoDB112) Pause() bool {
	if p.Spec == nil {
		return false
	}
	return p.Spec.Pause
}

// State returns the cluster state.
func (p *PerconaServerMongoDB112) State() common.AppState {
	if p.Status == nil {
		return common.AppStateUnknown
	}
	return p.Status.Status
}
