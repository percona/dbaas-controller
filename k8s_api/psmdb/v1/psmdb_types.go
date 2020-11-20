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

// nolint:unused,deadcode,varcheck,gochecknoglobals
package v1

import (
	"github.com/percona-platform/dbaas-controller/k8s_api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const AffinityOff = "none"

type PerconaServerMongoDB struct {
	meta.TypeMeta   `json:",inline"`
	meta.ObjectMeta `json:"metadata,omitempty"`

	Spec   PerconaServerMongoDBSpec   `json:"spec,omitempty"`
	Status perconaServerMongoDBStatus `json:"status,omitempty"`
}

type clusterRole string

const (
	clusterRoleShardSvr  clusterRole = "shardsvr"
	clusterRoleConfigSvr clusterRole = "configsvr"
)

type platform string

const (
	platformUndef      platform = ""
	platformKubernetes platform = "kubernetes"
	platformOpenshift  platform = "openshift"
)

// PerconaServerMongoDBSpec defines the desired state of PerconaServerMongoDB.
type PerconaServerMongoDBSpec struct {
	Pause                   bool           `json:"pause,omitempty"`
	Platform                *platform      `json:"platform,omitempty"`
	Image                   string         `json:"image,omitempty"`
	RunUID                  int64          `json:"runUid,omitempty"`
	UnsafeConf              bool           `json:"allowUnsafeConfigurations"`
	Mongod                  *MongodSpec    `json:"mongod,omitempty"`
	Replsets                []*ReplsetSpec `json:"replsets,omitempty"`
	Secrets                 *SecretsSpec   `json:"secrets,omitempty"`
	Backup                  BackupSpec     `json:"backup,omitempty"`
	PMM                     PmmSpec        `json:"pmm,omitempty"`
	SchedulerName           string         `json:"schedulerName,omitempty"`
	ClusterServiceDNSSuffix string         `json:"clusterServiceDNSSuffix,omitempty"`
}

type replsetMemberStatus struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type replsetStatus struct {
	Members     []*replsetMemberStatus `json:"members,omitempty"`
	ClusterRole clusterRole            `json:"clusterRole,omitempty"`

	Initialized bool     `json:"initialized,omitempty"`
	Size        int32    `json:"size"`
	Ready       int32    `json:"ready"`
	Status      AppState `json:"status,omitempty"`
	Message     string   `json:"message,omitempty"`
}

type AppState string

const (
	AppStateUnknown AppState = "unknown"
	AppStatePending AppState = "pending"
	AppStateInit    AppState = "initializing"
	AppStateReady   AppState = "ready"
	AppStateError   AppState = "error"
)

// perconaServerMongoDBStatus defines the observed state of PerconaServerMongoDB.
type perconaServerMongoDBStatus struct {
	Status             AppState                  `json:"state,omitempty"`
	Message            string                    `json:"message,omitempty"`
	Conditions         []clusterCondition        `json:"conditions,omitempty"`
	Replsets           map[string]*replsetStatus `json:"replsets,omitempty"`
	ObservedGeneration int64                     `json:"observedGeneration,omitempty"`
}

type conditionStatus string

const (
	conditionTrue    conditionStatus = "True"
	conditionFalse   conditionStatus = "False"
	conditionUnknown conditionStatus = "Unknown"
)

type clusterConditionType string

const (
	clusterReady   clusterConditionType = "ClusterReady"
	clusterInit    clusterConditionType = "ClusterInitializing"
	clusterRSInit  clusterConditionType = "ReplsetInitialized"
	clusterRSReady clusterConditionType = "ReplsetReady"
	clusterError   clusterConditionType = "Error"
)

type clusterCondition struct {
	Status  conditionStatus      `json:"status"`
	Type    clusterConditionType `json:"type"`
	Reason  string               `json:"reason,omitempty"`
	Message string               `json:"message,omitempty"`
}

type PmmSpec struct {
	Enabled    bool           `json:"enabled,omitempty"`
	ServerHost string         `json:"serverHost,omitempty"`
	Image      string         `json:"image,omitempty"`
	Resources  *ResourcesSpec `json:"resources,omitempty"`
}

type MultiAZ struct {
	Affinity            *PodAffinity             `json:"affinity,omitempty"`
	NodeSelector        map[string]string        `json:"nodeSelector,omitempty"`
	PriorityClassName   string                   `json:"priorityClassName,omitempty"`
	Annotations         map[string]string        `json:"annotations,omitempty"`
	Labels              map[string]string        `json:"labels,omitempty"`
	PodDisruptionBudget *podDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
}

type podDisruptionBudgetSpec struct {
	MinAvailable   *intstr.IntOrString `json:"minAvailable,omitempty"`
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

type PodAffinity struct {
	TopologyKey *string `json:"antiAffinityTopologyKey,omitempty"`
}

type ReplsetSpec struct {
	Resources     *ResourcesSpec         `json:"resources,omitempty"`
	Name          string                 `json:"name"`
	Size          int32                  `json:"size"`
	ClusterRole   clusterRole            `json:"clusterRole,omitempty"`
	Arbiter       Arbiter                `json:"arbiter,omitempty"`
	Expose        expose                 `json:"expose,omitempty"`
	VolumeSpec    *VolumeSpec            `json:"volumeSpec,omitempty"`
	LivenessProbe *livenessProbeExtended `json:"livenessProbe,omitempty"`
	MultiAZ
}

type livenessProbeExtended struct {
	StartupDelaySeconds int `json:"startupDelaySeconds,omitempty"`
}

type VolumeSpec struct {

	// PersistentVolumeClaim represents a reference to a PersistentVolumeClaim.
	// It has the highest level of precedence, followed by HostPath and
	// EmptyDir. And represents the PVC specification.
	PersistentVolumeClaim *PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
}

type ResourceSpecRequirements struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type ResourcesSpec struct {
	Limits   *ResourceSpecRequirements `json:"limits,omitempty"`
	Requests *ResourceSpecRequirements `json:"requests,omitempty"`
}

type SecretsSpec struct {
	Users       string `json:"users,omitempty"`
	SSL         string `json:"ssl,omitempty"`
	SSLInternal string `json:"sslInternal,omitempty"`
}

type mongosSpec struct {
	*ResourcesSpec `json:"resources,omitempty"`
	Port           int32 `json:"port,omitempty"`
	HostPort       int32 `json:"hostPort,omitempty"`
}

type MongodSpec struct {
	Net                *MongodSpecNet                `json:"net,omitempty"`
	AuditLog           *MongodSpecAuditLog           `json:"auditLog,omitempty"`
	OperationProfiling *MongodSpecOperationProfiling `json:"operationProfiling,omitempty"`
	Replication        *MongodSpecReplication        `json:"replication,omitempty"`
	Security           *MongodSpecSecurity           `json:"security,omitempty"`
	SetParameter       *MongodSpecSetParameter       `json:"setParameter,omitempty"`
	Storage            *MongodSpecStorage            `json:"storage,omitempty"`
}

type MongodSpecNet struct {
	Port     int32 `json:"port,omitempty"`
	HostPort int32 `json:"hostPort,omitempty"`
}

type MongodSpecReplication struct {
	OplogSizeMB int `json:"oplogSizeMB,omitempty"`
}

// mongodChiperMode is a cipher mode used by Data-at-Rest Encryption.
type mongodChiperMode string

const (
	MongodChiperModeUnset mongodChiperMode = ""
	MongodChiperModeCBC   mongodChiperMode = "AES256-CBC"
	MongodChiperModeGCM   mongodChiperMode = "AES256-GCM"
)

type MongodSpecSecurity struct {
	RedactClientLogData  bool             `json:"redactClientLogData,omitempty"`
	EnableEncryption     *bool            `json:"enableEncryption,omitempty"`
	EncryptionKeySecret  string           `json:"encryptionKeySecret,omitempty"`
	EncryptionCipherMode mongodChiperMode `json:"encryptionCipherMode,omitempty"`
}

type MongodSpecSetParameter struct {
	TTLMonitorSleepSecs                   int `json:"ttlMonitorSleepSecs,omitempty"`
	WiredTigerConcurrentReadTransactions  int `json:"wiredTigerConcurrentReadTransactions,omitempty"`
	WiredTigerConcurrentWriteTransactions int `json:"wiredTigerConcurrentWriteTransactions,omitempty"`
}

type storageEngine string

const (
	StorageEngineWiredTiger storageEngine = "wiredTiger"
	StorageEngineInMemory   storageEngine = "inMemory"
	StorageEngineMMAPv1     storageEngine = "mmapv1"
)

type MongodSpecStorage struct {
	Engine         storageEngine         `json:"engine,omitempty"`
	DirectoryPerDB bool                  `json:"directoryPerDB,omitempty"`
	SyncPeriodSecs int                   `json:"syncPeriodSecs,omitempty"`
	InMemory       *mongodSpecInMemory   `json:"inMemory,omitempty"`
	MMAPv1         *MongodSpecMMAPv1     `json:"mmapv1,omitempty"`
	WiredTiger     *MongodSpecWiredTiger `json:"wiredTiger,omitempty"`
}

type MongodSpecMMAPv1 struct {
	NsSize     int  `json:"nsSize,omitempty"`
	Smallfiles bool `json:"smallfiles,omitempty"`
}

type wiredTigerCompressor string

var (
	WiredTigerCompressorNone   wiredTigerCompressor = "none"
	WiredTigerCompressorSnappy wiredTigerCompressor = "snappy"
	WiredTigerCompressorZlib   wiredTigerCompressor = "zlib"
)

type MongodSpecWiredTigerEngineConfig struct {
	CacheSizeRatio      float64               `json:"cacheSizeRatio,omitempty"`
	DirectoryForIndexes bool                  `json:"directoryForIndexes,omitempty"`
	JournalCompressor   *wiredTigerCompressor `json:"journalCompressor,omitempty"`
}

type MongodSpecWiredTigerCollectionConfig struct {
	BlockCompressor *wiredTigerCompressor `json:"blockCompressor,omitempty"`
}

type MongodSpecWiredTigerIndexConfig struct {
	PrefixCompression bool `json:"prefixCompression,omitempty"`
}

type MongodSpecWiredTiger struct {
	CollectionConfig *MongodSpecWiredTigerCollectionConfig `json:"collectionConfig,omitempty"`
	EngineConfig     *MongodSpecWiredTigerEngineConfig     `json:"engineConfig,omitempty"`
	IndexConfig      *MongodSpecWiredTigerIndexConfig      `json:"indexConfig,omitempty"`
}

type mongodSpecInMemoryEngineConfig struct {
	InMemorySizeRatio float64 `json:"inMemorySizeRatio,omitempty"`
}

type mongodSpecInMemory struct {
	EngineConfig *mongodSpecInMemoryEngineConfig `json:"engineConfig,omitempty"`
}

type auditLogDestination string

const auditLogDestinationFile auditLogDestination = auditLogDestination("file")

type auditLogFormat string

const (
	auditLogFormatBSON auditLogFormat = "BSON"
	auditLogFormatJSON auditLogFormat = "JSON"
)

type MongodSpecAuditLog struct {
	Destination auditLogDestination `json:"destination,omitempty"`
	Format      auditLogFormat      `json:"format,omitempty"`
	Filter      string              `json:"filter,omitempty"`
}

type operationProfilingMode string

const (
	OperationProfilingModeAll    operationProfilingMode = "all"
	OperationProfilingModeSlowOp operationProfilingMode = "slowOp"
)

type MongodSpecOperationProfiling struct {
	Mode              operationProfilingMode `json:"mode,omitempty"`
	SlowOpThresholdMs int                    `json:"slowOpThresholdMs,omitempty"`
	RateLimit         int                    `json:"rateLimit,omitempty"`
}

type compressionType string

const (
	compressionTypeNone   compressionType = "none"
	compressionTypeGZIP   compressionType = "gzip"
	compressionTypePGZIP  compressionType = "pgzip"
	compressionTypeSNAPPY compressionType = "snappy"
	compressionTypeLZ4    compressionType = "lz4"
	compressionTypeS2     compressionType = "s2"
)

type backupTaskSpec struct {
	Name            string          `json:"name"`
	Enabled         bool            `json:"enabled"`
	Schedule        string          `json:"schedule,omitempty"`
	StorageName     string          `json:"storageName,omitempty"`
	CompressionType compressionType `json:"compressionType,omitempty"`
}

type backupStorageS3Spec struct {
	Bucket            string `json:"bucket"`
	Prefix            string `json:"prefix,omitempty"`
	Region            string `json:"region,omitempty"`
	EndpointURL       string `json:"endpointUrl,omitempty"`
	CredentialsSecret string `json:"credentialsSecret"`
}

type backupStorageType string

const (
	backupStorageFilesystem backupStorageType = "filesystem"
	backupStorageS3         backupStorageType = "s3"
)

type backupStorageSpec struct {
	Type backupStorageType   `json:"type"`
	S3   backupStorageS3Spec `json:"s3,omitempty"`
}

type BackupSpec struct {
	Enabled            bool                         `json:"enabled"`
	Storages           map[string]backupStorageSpec `json:"storages,omitempty"`
	Image              string                       `json:"image,omitempty"`
	Tasks              []backupTaskSpec             `json:"tasks,omitempty"`
	ServiceAccountName string                       `json:"serviceAccountName,omitempty"`
	Resources          *ResourcesSpec               `json:"resources,omitempty"`
}

type Arbiter struct {
	Enabled bool  `json:"enabled"`
	Size    int32 `json:"size"`
	MultiAZ
}

type expose struct {
	Enabled bool `json:"enabled"`
}

// PersistentVolumeClaimSpec describes the common attributes of storage devices
// and allows a Source for provider-specific attributes.
type PersistentVolumeClaimSpec struct {
	// Resources represents the minimum resources the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,2,opt,name=resources"`
}

// ResourceRequirements describes the compute resource requirements.
type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Limits ResourceList `json:"limits,omitempty" protobuf:"bytes,1,rep,name=limits,casttype=ResourceList,castkey=resourceName"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
	// otherwise to an implementation-defined value.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/
	// +optional
	Requests ResourceList `json:"requests,omitempty" protobuf:"bytes,2,rep,name=requests,casttype=ResourceList,castkey=resourceName"`
}

// resourceName is the name identifying various resources in a resourceList.
type resourceName string

// Resource names must be not more than 63 characters, consisting of upper- or lower-case alphanumeric characters,
// with the -, _, and . characters allowed anywhere, except the first or last character.
// The default convention, matching that for annotations, is to use lower-case names, with dashes, rather than
// camel case, separating compound words.
// Fully-qualified resource typenames are constructed from a DNS-style subdomain, followed by a slash `/` and a name.
const (
	// CPU, in cores. (500m = .5 cores).
	ResourceCPU resourceName = "cpu"
	// Memory, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024).
	ResourceMemory resourceName = "memory"
	// Volume size, in bytes (e,g. 5Gi = 5GiB = 5 * 1024 * 1024 * 1024).
	ResourceStorage resourceName = "storage"
	// Local ephemeral storage, in bytes. (500Gi = 500GiB = 500 * 1024 * 1024 * 1024)
	// The resource name for ResourceEphemeralStorage is alpha and it can change across releases.
	ResourceEphemeralStorage resourceName = "ephemeral-storage"
)

// resourceList is a set of (resource name, quantity) pairs.
type ResourceList map[resourceName]resource.Quantity
