package psmdb

import "github.com/percona-platform/dbaas-controller/service/k8sclient/common"

// PerconaServerMongoDB is the Schema for the perconaservermongodbs 1.12+ API.
type PerconaServerMongoDB112 struct {
	common.TypeMeta   // anonymous for embedding
	common.ObjectMeta `json:"metadata,omitempty"`

	APIVersion string        `json:"apiVersion,omitempty"`
	Kind       string        `json:"kind,omitempty"`
	Spec       *PSMDB112Spec `json:"spec,omitempty"`
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

// Spec defines the PSMDB operator parameters.
type PSMDB112Spec struct {
	CRVersion                 string          `json:"crVersion,omitempty"`
	Image                     string          `json:"image,omitempty"`
	ImagePullPolicy           string          `json:"imagePullPolicy,omitempty"`
	AllowUnsafeConfigurations bool            `json:"allowUnsafeConfigurations,omitempty"`
	UpdateStrategy            string          `json:"updateStrategy,omitempty"`
	UpgradeOptions            *UpgradeOptions `json:"upgradeOptions,omitempty"`
	Secrets                   *SecretsSpec    `json:"secrets,omitempty"`
	PMM                       *PmmSpec        `json:"pmm,omitempty"`
	Replsets                  []*ReplsetSpec  `json:"replsets,omitempty"`
	Sharding                  *ShardingSpec   `json:"sharding,omitempty"`
	Backup                    *BackupSpec     `json:"backup,omitempty"`
}
