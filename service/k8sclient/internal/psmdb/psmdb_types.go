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

const (
	// AffinityOff turn off affinity.
	AffinityOff = "none"
	// PerconaServerMongoDBKind is a name of CRD for mongodb clusters.
	PerconaServerMongoDBKind = "PerconaServerMongoDB"
)

// PerconaServerMongoDB is the Schema for the perconaservermongodbs API.
type PerconaServerMongoDB struct {
	common.TypeMeta   // anonymous for embedding
	common.ObjectMeta `json:"metadata,omitempty"`

	Spec   *PerconaServerMongoDBSpec   `json:"spec,omitempty"`
	Status *PerconaServerMongoDBStatus `json:"status,omitempty"`
}

// SetDatabaseImage sets database image to appropriate image field.
func (p *PerconaServerMongoDB) SetDatabaseImage(image string) {
	if p.Spec == nil {
		p.Spec = new(PerconaServerMongoDBSpec)
	}
	p.Spec.Image = image
}

// DatabaseImage returns image of database software used.
func (p *PerconaServerMongoDB) DatabaseImage() string {
	return p.Spec.Image
}

// GetName returns name of the cluster.
func (p *PerconaServerMongoDB) GetName() string {
	return p.Name
}

// CRDName returns name of Custom Resource Definition -> cluster's kind.
func (p *PerconaServerMongoDB) CRDName() string {
	return string(PerconaServerMongoDBKind)
}

// DatabaseContainerNames returns container names that actually run the database.
func (p *PerconaServerMongoDB) DatabaseContainerNames() []string {
	return []string{"mongos", "mongod"}
}

// DatabasePodLabels return list of labels to get pods where database is running.
func (p *PerconaServerMongoDB) DatabasePodLabels() []string {
	return []string{"app.kubernetes.io/instance=" + p.Name, "app.kubernetes.io/part-of=percona-server-mongodb"}
}

// PerconaServerMongoDBList holds a list of PSMDB objects.
type PerconaServerMongoDBList struct {
	common.TypeMeta // anonymous for embedding

	Items []PerconaServerMongoDB `json:"items"`
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

type ShardingSpec struct {
	Enabled            bool                          `json:"enabled"`
	ConfigsvrReplSet   *ReplsetSpec                  `json:"configsvrReplSet"`
	Mongos             *ReplsetSpec                  `json:"mongos"`
	OperationProfiling *MongodSpecOperationProfiling `json:"operationProfiling"`
}

// UpgradeOptions specify how and to what version we update.
type UpgradeOptions struct {
	Apply                  string `json:"apply,omitempty"`
	VersionServiceEndpoint string `json:"versionServiceEndpoint,omitempty"`
	Schedule               string `json:"schedule,omitempty"`
}

// PerconaServerMongoDBSpec defines the desired state of PerconaServerMongoDB.
type PerconaServerMongoDBSpec struct {
	UpdateStrategy          string                 `json:"updateStrategy,omitempty"`
	UpgradeOptions          *common.UpgradeOptions `json:"upgradeOptions,omitempty"`
	CRVersion               string                 `json:"crVersion,omitempty"`
	Pause                   bool                   `json:"pause,omitempty"`
	UnsafeConf              bool                   `json:"allowUnsafeConfigurations"`
	RunUID                  int64                  `json:"runUid,omitempty"`
	Platform                *platform              `json:"platform,omitempty"`
	Image                   string                 `json:"image,omitempty"`
	Mongod                  *MongodSpec            `json:"mongod,omitempty"`
	Replsets                []*ReplsetSpec         `json:"replsets,omitempty"`
	Secrets                 *SecretsSpec           `json:"secrets,omitempty"`
	Backup                  *BackupSpec            `json:"backup,omitempty"`
	PMM                     *PmmSpec               `json:"pmm,omitempty"`
	SchedulerName           string                 `json:"schedulerName,omitempty"`
	ClusterServiceDNSSuffix string                 `json:"clusterServiceDNSSuffix,omitempty"`
	Sharding                *ShardingSpec          `json:"sharding,omitempty"`
}

type replsetMemberStatus struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type appStatus struct {
	Size   int32    `json:"size,omitempty"`
	Ready  int32    `json:"ready,omitempty"`
	Status AppState `json:"status,omitempty"`
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

// AppState application state.
type AppState string

const (
	// AppStateUnknown unknown application state.
	AppStateUnknown AppState = "unknown"
	// AppStatePending pending application state.
	AppStatePending AppState = "pending"
	// AppStateInit initializing application state.
	AppStateInit AppState = "initializing"
	// AppStateReady ready application state.
	AppStateReady AppState = "ready"
	// AppStateError error application state.
	AppStateError AppState = "error"
)

// PerconaServerMongoDBStatus defines the observed state of PerconaServerMongoDB.
type PerconaServerMongoDBStatus struct {
	Status             AppState                  `json:"state,omitempty"`
	Message            string                    `json:"message,omitempty"`
	Conditions         []clusterCondition        `json:"conditions,omitempty"`
	Mongos             appStatus                 `json:"mongos,omitempty"`
	Replsets           map[string]*replsetStatus `json:"replsets,omitempty"`
	ObservedGeneration int64                     `json:"observedGeneration,omitempty"`
	Host               string                    `json:"host,omitempty"`
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

// PmmSpec defines pmm specification.
type PmmSpec struct {
	Enabled    bool                 `json:"enabled,omitempty"`
	ServerHost string               `json:"serverHost,omitempty"`
	Image      string               `json:"image,omitempty"`
	Resources  *common.PodResources `json:"resources,omitempty"`
}

// MultiAZ defines multi availability zones.
type MultiAZ struct {
	Affinity          *PodAffinity      `json:"affinity,omitempty"`
	NodeSelector      map[string]string `json:"nodeSelector,omitempty"`
	PriorityClassName string            `json:"priorityClassName,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// PodAffinity define pod affinity.
type PodAffinity struct {
	TopologyKey *string `json:"antiAffinityTopologyKey,omitempty"`
}

// Expose holds information about how the cluster is exposed to the worl via ingress.
type Expose struct {
	Enabled    bool               `json:"enabled"`
	ExposeType common.ServiceType `json:"exposeType"`
}

// ReplsetSpec defines replicaton set specification.
type ReplsetSpec struct {
	Expose              Expose                          `json:"expose,omitempty"`
	Size                int32                           `json:"size"`
	Arbiter             Arbiter                         `json:"arbiter,omitempty"`
	Resources           *common.PodResources            `json:"resources,omitempty"`
	Name                string                          `json:"name,omitempty"`
	ClusterRole         clusterRole                     `json:"clusterRole,omitempty"`
	VolumeSpec          *common.VolumeSpec              `json:"volumeSpec,omitempty"`
	LivenessProbe       *livenessProbeExtended          `json:"livenessProbe,omitempty"`
	PodDisruptionBudget *common.PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	MultiAZ
}

type livenessProbeExtended struct {
	StartupDelaySeconds int `json:"startupDelaySeconds,omitempty"`
}

// SecretsSpec defines secrets specification.
type SecretsSpec struct {
	Users       string `json:"users,omitempty"`
	SSL         string `json:"ssl,omitempty"`
	SSLInternal string `json:"sslInternal,omitempty"`
}

// MongosSpec defines MongoDB specification.
type MongosSpec struct {
	*common.PodResources `json:"resources,omitempty"`
	Port                 int32 `json:"port,omitempty"`
	HostPort             int32 `json:"hostPort,omitempty"`
}

// MongodSpec defines mongod specification.
type MongodSpec struct {
	Net                *MongodSpecNet                `json:"net,omitempty"`
	AuditLog           *MongodSpecAuditLog           `json:"auditLog,omitempty"`
	OperationProfiling *MongodSpecOperationProfiling `json:"operationProfiling,omitempty"`
	Replication        *MongodSpecReplication        `json:"replication,omitempty"`
	Security           *MongodSpecSecurity           `json:"security,omitempty"`
	SetParameter       *MongodSpecSetParameter       `json:"setParameter,omitempty"`
	Storage            *MongodSpecStorage            `json:"storage,omitempty"`
}

// MongodSpecNet defines mongod specification of network.
type MongodSpecNet struct {
	Port     int32 `json:"port,omitempty"`
	HostPort int32 `json:"hostPort,omitempty"`
}

// MongodSpecReplication defines mongod specification of replication.
type MongodSpecReplication struct {
	OplogSizeMB int `json:"oplogSizeMB,omitempty"`
}

// mongodChiperMode is a cipher mode used by Data-at-Rest Encryption.
type mongodChiperMode string

const (
	// MongodChiperModeUnset mongod chiper mode is unset.
	MongodChiperModeUnset mongodChiperMode = ""
	// MongodChiperModeCBC mongod chiper mode is AES256-CBC.
	MongodChiperModeCBC mongodChiperMode = "AES256-CBC"
	// MongodChiperModeGCM mongod chiper mode is AES256-GCM.
	MongodChiperModeGCM mongodChiperMode = "AES256-GCM"
)

// MongodSpecSecurity defines mongod specification of security.
type MongodSpecSecurity struct {
	RedactClientLogData  bool             `json:"redactClientLogData,omitempty"`
	EnableEncryption     *bool            `json:"enableEncryption,omitempty"`
	EncryptionKeySecret  string           `json:"encryptionKeySecret,omitempty"`
	EncryptionCipherMode mongodChiperMode `json:"encryptionCipherMode,omitempty"`
}

// MongodSpecSetParameter defines mongod specification of parameter set.
type MongodSpecSetParameter struct {
	TTLMonitorSleepSecs                   int `json:"ttlMonitorSleepSecs,omitempty"`
	WiredTigerConcurrentReadTransactions  int `json:"wiredTigerConcurrentReadTransactions,omitempty"`
	WiredTigerConcurrentWriteTransactions int `json:"wiredTigerConcurrentWriteTransactions,omitempty"`
}

type storageEngine string

const (
	// StorageEngineWiredTiger WiredTiger storage engine.
	StorageEngineWiredTiger storageEngine = "wiredTiger"
	// StorageEngineInMemory inMemory storage engine.
	StorageEngineInMemory storageEngine = "inMemory"
	// StorageEngineMMAPv1 mmapv1 storage engine.
	StorageEngineMMAPv1 storageEngine = "mmapv1"
)

// MongodSpecStorage defines mongod specification of starage.
type MongodSpecStorage struct {
	Engine         storageEngine         `json:"engine,omitempty"`
	DirectoryPerDB bool                  `json:"directoryPerDB,omitempty"`
	SyncPeriodSecs int                   `json:"syncPeriodSecs,omitempty"`
	InMemory       *mongodSpecInMemory   `json:"inMemory,omitempty"`
	MMAPv1         *MongodSpecMMAPv1     `json:"mmapv1,omitempty"`
	WiredTiger     *MongodSpecWiredTiger `json:"wiredTiger,omitempty"`
}

// MongodSpecMMAPv1 defines mongod specification of MMAPv1.
type MongodSpecMMAPv1 struct {
	NsSize     int  `json:"nsSize,omitempty"`
	Smallfiles bool `json:"smallfiles,omitempty"`
}

type wiredTigerCompressor string

var (
	// WiredTigerCompressorNone is none compressor.
	WiredTigerCompressorNone wiredTigerCompressor = "none"
	// WiredTigerCompressorSnappy is snappy compressor.
	WiredTigerCompressorSnappy wiredTigerCompressor = "snappy"
	// WiredTigerCompressorZlib is zlib compressor.
	WiredTigerCompressorZlib wiredTigerCompressor = "zlib"
)

// MongodSpecWiredTigerEngineConfig defines mongod specification of WiredTiger engine configuration.
type MongodSpecWiredTigerEngineConfig struct {
	CacheSizeRatio      float64               `json:"cacheSizeRatio,omitempty"`
	DirectoryForIndexes bool                  `json:"directoryForIndexes,omitempty"`
	JournalCompressor   *wiredTigerCompressor `json:"journalCompressor,omitempty"`
}

// MongodSpecWiredTigerCollectionConfig defines mongod specification of WiredTiger collection configuration.
type MongodSpecWiredTigerCollectionConfig struct {
	BlockCompressor *wiredTigerCompressor `json:"blockCompressor,omitempty"`
}

// MongodSpecWiredTigerIndexConfig defines mongod specification of WiredTiger index configuration.
type MongodSpecWiredTigerIndexConfig struct {
	PrefixCompression bool `json:"prefixCompression,omitempty"`
}

// MongodSpecWiredTiger defines mongod specification of WiredTiger.
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

// MongodSpecAuditLog defines mongod specification of audit log.
type MongodSpecAuditLog struct {
	Destination auditLogDestination `json:"destination,omitempty"`
	Format      auditLogFormat      `json:"format,omitempty"`
	Filter      string              `json:"filter,omitempty"`
}

type operationProfilingMode string

const (
	// OperationProfilingModeAll mode all.
	OperationProfilingModeAll operationProfilingMode = "all"
	// OperationProfilingModeSlowOp mode slowOP.
	OperationProfilingModeSlowOp operationProfilingMode = "slowOp"
)

// MongodSpecOperationProfiling defines mongod specification of operation profiling.
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

// BackupSpec defines back up specification.
type BackupSpec struct {
	Enabled            bool                         `json:"enabled"`
	Storages           map[string]backupStorageSpec `json:"storages,omitempty"`
	Image              string                       `json:"image,omitempty"`
	Tasks              []backupTaskSpec             `json:"tasks,omitempty"`
	ServiceAccountName string                       `json:"serviceAccountName,omitempty"`
	Resources          *common.PodResources         `json:"resources,omitempty"`
}

// Arbiter defines Arbiter.
type Arbiter struct {
	Enabled bool  `json:"enabled"`
	Size    int32 `json:"size"`
	MultiAZ
}
