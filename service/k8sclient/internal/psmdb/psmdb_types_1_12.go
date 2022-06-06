package psmdb

type PerconaServerMongoDB112 struct {
	APIVersion string       `json:"apiVersion"`
	Kind       string       `json:"kind"`
	Metadata   *Metadata112 `json:"metadata"`
	Spec       *Spec112     `json:"spec"`
}

type Metadata112 struct {
	Name       string   `json:"name"`
	Finalizers []string `json:"finalizers"`
}

type UpgradeOptions112 struct {
	VersionServiceEndpoint string `json:"versionServiceEndpoint"`
	Apply                  string `json:"apply"`
	Schedule               string `json:"schedule"`
	SetFCV                 bool   `json:"setFCV"`
}

type Secrets112 struct {
	Users         string `json:"users"`
	EncryptionKey string `json:"encryptionKey"`
}

type Pmm112 struct {
	Enabled    bool   `json:"enabled"`
	Image      string `json:"image"`
	ServerHost string `json:"serverHost"`
}

type Affinity112 struct {
	AntiAffinityTopologyKey string `json:"antiAffinityTopologyKey"`
}

type PodDisruptionBudget112 struct {
	MaxUnavailable int `json:"maxUnavailable"`
}

type Expose112 struct {
	Enabled    bool   `json:"enabled"`
	ExposeType string `json:"exposeType"`
}

type Limits112 struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

type Requests112 struct {
	CPU     string `json:"cpu"`
	Memory  string `json:"memory"`
	Storage string `json:"storage"`
}

type Resources112 struct {
	Limits   *Limits112   `json:"limits"`
	Requests *Requests112 `json:"requests"`
}

type PersistentVolumeClaim112 struct {
	Resources Resources112 `json:"resources"`
}

type VolumeSpec112 struct {
	PersistentVolumeClaim PersistentVolumeClaim112 `json:"persistentVolumeClaim"`
}

type Nonvoting112 struct {
	Enabled             bool                   `json:"enabled"`
	Size                int                    `json:"size"`
	Affinity            Affinity112            `json:"affinity"`
	PodDisruptionBudget PodDisruptionBudget112 `json:"podDisruptionBudget"`
	Resources           Resources112           `json:"resources"`
	VolumeSpec          VolumeSpec112          `json:"volumeSpec"`
}

type Arbiter112 struct {
	Enabled  bool         `json:"enabled"`
	Size     int          `json:"size"`
	Affinity *Affinity112 `json:"affinity"`
}

type Replsets112 struct {
	Name                string                  `json:"name"`
	Size                int                     `json:"size"`
	Affinity            *Affinity112            `json:"affinity"`
	PodDisruptionBudget *PodDisruptionBudget112 `json:"podDisruptionBudget"`
	Expose              Expose112               `json:"expose"`
	Resources           *Resources112           `json:"resources"`
	VolumeSpec          *VolumeSpec112          `json:"volumeSpec"`
	Nonvoting           Nonvoting112            `json:"nonvoting"`
	Arbiter             Arbiter112              `json:"arbiter"`
}

type ConfigsvrReplSet112 struct {
	Size                int                    `json:"size"`
	Affinity            *Affinity112           `json:"affinity"`
	PodDisruptionBudget PodDisruptionBudget112 `json:"podDisruptionBudget"`
	Expose              Expose                 `json:"expose"`
	Resources           Resources112           `json:"resources"`
	VolumeSpec          *VolumeSpec112         `json:"volumeSpec"`
}

type Mongos112 struct {
	Size                int                    `json:"size"`
	Affinity            Affinity112            `json:"affinity"`
	PodDisruptionBudget PodDisruptionBudget112 `json:"podDisruptionBudget"`
	Resources           *Resources112          `json:"resources"`
	Expose              Expose112              `json:"expose"`
}

type Sharding112 struct {
	Enabled          bool                `json:"enabled"`
	ConfigsvrReplSet ConfigsvrReplSet112 `json:"configsvrReplSet"`
	Mongos           *Mongos112          `json:"mongos"`
}

type Pitr112 struct {
	Enabled          bool   `json:"enabled"`
	CompressionType  string `json:"compressionType"`
	CompressionLevel int    `json:"compressionLevel"`
}

type Backup112 struct {
	Enabled            bool    `json:"enabled"`
	Image              string  `json:"image"`
	ServiceAccountName string  `json:"serviceAccountName"`
	Pitr               Pitr112 `json:"pitr"`
}

type Spec112 struct {
	CrVersion                 string            `json:"crVersion"`
	Image                     string            `json:"image"`
	ImagePullPolicy           string            `json:"imagePullPolicy"`
	AllowUnsafeConfigurations bool              `json:"allowUnsafeConfigurations"`
	UpdateStrategy            string            `json:"updateStrategy"`
	UpgradeOptions            UpgradeOptions112 `json:"upgradeOptions"`
	Secrets                   *Secrets112       `json:"secrets"`
	PMM                       *Pmm112           `json:"pmm"`
	Replsets                  []*Replsets112    `json:"replsets"`
	Sharding                  *Sharding112      `json:"sharding"`
	Backup                    *Backup112        `json:"backup"`
}
