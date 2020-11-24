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
package k8sclient

import (
	"github.com/percona-platform/dbaas-controller/k8_api/common"
)

const affinityOff = "none"

// TypeMeta describes an individual object in an API response or request
// with strings representing the type of the object and its API schema version.
// Structures that are versioned or persisted should inline TypeMeta.
//
// +k8s:deepcopy-gen=false
type TypeMeta struct {
	// Kind is a string value representing the REST resource this object represents.
	// Servers may infer this from the endpoint the client submits requests to.
	// Cannot be updated.
	// In CamelCase.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`

	// APIVersion defines the versioned schema of this representation of an object.
	// Servers should convert recognized schemas to the latest internal value, and
	// may reject unrecognized values.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
	// +optional
	APIVersion string `json:"apiVersion,omitempty" protobuf:"bytes,2,opt,name=apiVersion"`
}

// ObjectMeta is metadata that all persisted resources must have, which includes all objects
// users must create.
type ObjectMeta struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
}

// ListMeta describes metadata that synthetic resources must have, including lists and
// various status objects. A resource may have only one of {ObjectMeta, ListMeta}.
type ListMeta struct {
	// selfLink is a URL representing this object.
	// Populated by the system.
	// Read-only.
	//
	// DEPRECATED
	// Kubernetes will stop propagating this field in 1.20 release and the field is planned
	// to be removed in 1.21 release.
	// +optional
	SelfLink string `json:"selfLink,omitempty" protobuf:"bytes,1,opt,name=selfLink"`

	// String that identifies the server's internal version of this object that
	// can be used by clients to determine when objects have changed.
	// Value must be treated as opaque by clients and passed unmodified back to the server.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
	// +optional
	ResourceVersion string `json:"resourceVersion,omitempty" protobuf:"bytes,2,opt,name=resourceVersion"`

	// continue may be set if the user set a limit on the number of items returned, and indicates that
	// the server has more data available. The value is opaque and may be used to issue another request
	// to the endpoint that served this list to retrieve the next set of available objects. Continuing a
	// consistent list may not be possible if the server configuration has changed or more than a few
	// minutes have passed. The resourceVersion field returned when using this continue value will be
	// identical to the value in the first response, unless you have received this token from an error
	// message.
	Continue string `json:"continue,omitempty" protobuf:"bytes,3,opt,name=continue"`

	// remainingItemCount is the number of subsequent items in the list which are not included in this
	// list response. If the list request contained label or field selectors, then the number of
	// remaining items is unknown and the field will be left unset and omitted during serialization.
	// If the list is complete (either because it is not chunking or because this is the last chunk),
	// then there are no more remaining items and this field will be left unset and omitted during
	// serialization.
	// Servers older than v1.15 do not set this field.
	// The intended use of the remainingItemCount is *estimating* the size of a collection. Clients
	// should not rely on the remainingItemCount to be set or to be exact.
	// +optional
	RemainingItemCount *int64 `json:"remainingItemCount,omitempty" protobuf:"bytes,4,opt,name=remainingItemCount"`
}

type perconaServerMongoDB struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata,omitempty"`

	Spec   perconaServerMongoDBSpec   `json:"spec,omitempty"`
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

// perconaServerMongoDBSpec defines the desired state of PerconaServerMongoDB.
type perconaServerMongoDBSpec struct {
	Pause                   bool           `json:"pause,omitempty"`
	Platform                *platform      `json:"platform,omitempty"`
	Image                   string         `json:"image,omitempty"`
	RunUID                  int64          `json:"runUid,omitempty"`
	UnsafeConf              bool           `json:"allowUnsafeConfigurations"`
	Mongod                  *mongodSpec    `json:"mongod,omitempty"`
	Replsets                []*replsetSpec `json:"replsets,omitempty"`
	Secrets                 *secretsSpec   `json:"secrets,omitempty"`
	Backup                  backupSpec     `json:"backup,omitempty"`
	PMM                     pmmSpec        `json:"pmm,omitempty"`
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
	Status      appState `json:"status,omitempty"`
	Message     string   `json:"message,omitempty"`
}

type appState string

const (
	appStateUnknown appState = "unknown"
	appStatePending appState = "pending"
	appStateInit    appState = "initializing"
	appStateReady   appState = "ready"
	appStateError   appState = "error"
)

// perconaServerMongoDBStatus defines the observed state of PerconaServerMongoDB.
type perconaServerMongoDBStatus struct {
	Status             appState                  `json:"state,omitempty"`
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

type pmmSpec struct {
	Enabled    bool                 `json:"enabled,omitempty"`
	ServerHost string               `json:"serverHost,omitempty"`
	Image      string               `json:"image,omitempty"`
	Resources  *common.PodResources `json:"resources,omitempty"`
}

type multiAZ struct {
	Affinity          *podAffinity      `json:"affinity,omitempty"`
	NodeSelector      map[string]string `json:"nodeSelector,omitempty"`
	PriorityClassName string            `json:"priorityClassName,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

type podAffinity struct {
	TopologyKey *string `json:"antiAffinityTopologyKey,omitempty"`
}

type replsetSpec struct {
	Resources     *common.PodResources   `json:"resources,omitempty"`
	Name          string                 `json:"name"`
	Size          int32                  `json:"size"`
	ClusterRole   clusterRole            `json:"clusterRole,omitempty"`
	Arbiter       arbiter                `json:"arbiter,omitempty"`
	Expose        expose                 `json:"expose,omitempty"`
	VolumeSpec    *common.VolumeSpec     `json:"volumeSpec,omitempty"`
	LivenessProbe *livenessProbeExtended `json:"livenessProbe,omitempty"`
	multiAZ
}

type livenessProbeExtended struct {
	StartupDelaySeconds int `json:"startupDelaySeconds,omitempty"`
}

type secretsSpec struct {
	Users       string `json:"users,omitempty"`
	SSL         string `json:"ssl,omitempty"`
	SSLInternal string `json:"sslInternal,omitempty"`
}

type mongosSpec struct {
	*common.PodResources `json:"resources,omitempty"`
	Port                 int32 `json:"port,omitempty"`
	HostPort             int32 `json:"hostPort,omitempty"`
}

type mongodSpec struct {
	Net                *mongodSpecNet                `json:"net,omitempty"`
	AuditLog           *mongodSpecAuditLog           `json:"auditLog,omitempty"`
	OperationProfiling *mongodSpecOperationProfiling `json:"operationProfiling,omitempty"`
	Replication        *mongodSpecReplication        `json:"replication,omitempty"`
	Security           *mongodSpecSecurity           `json:"security,omitempty"`
	SetParameter       *mongodSpecSetParameter       `json:"setParameter,omitempty"`
	Storage            *mongodSpecStorage            `json:"storage,omitempty"`
}

type mongodSpecNet struct {
	Port     int32 `json:"port,omitempty"`
	HostPort int32 `json:"hostPort,omitempty"`
}

type mongodSpecReplication struct {
	OplogSizeMB int `json:"oplogSizeMB,omitempty"`
}

// mongodChiperMode is a cipher mode used by Data-at-Rest Encryption.
type mongodChiperMode string

const (
	mongodChiperModeUnset mongodChiperMode = ""
	mongodChiperModeCBC   mongodChiperMode = "AES256-CBC"
	mongodChiperModeGCM   mongodChiperMode = "AES256-GCM"
)

type mongodSpecSecurity struct {
	RedactClientLogData  bool             `json:"redactClientLogData,omitempty"`
	EnableEncryption     *bool            `json:"enableEncryption,omitempty"`
	EncryptionKeySecret  string           `json:"encryptionKeySecret,omitempty"`
	EncryptionCipherMode mongodChiperMode `json:"encryptionCipherMode,omitempty"`
}

type mongodSpecSetParameter struct {
	TTLMonitorSleepSecs                   int `json:"ttlMonitorSleepSecs,omitempty"`
	WiredTigerConcurrentReadTransactions  int `json:"wiredTigerConcurrentReadTransactions,omitempty"`
	WiredTigerConcurrentWriteTransactions int `json:"wiredTigerConcurrentWriteTransactions,omitempty"`
}

type storageEngine string

const (
	storageEngineWiredTiger storageEngine = "wiredTiger"
	storageEngineInMemory   storageEngine = "inMemory"
	storageEngineMMAPv1     storageEngine = "mmapv1"
)

type mongodSpecStorage struct {
	Engine         storageEngine         `json:"engine,omitempty"`
	DirectoryPerDB bool                  `json:"directoryPerDB,omitempty"`
	SyncPeriodSecs int                   `json:"syncPeriodSecs,omitempty"`
	InMemory       *mongodSpecInMemory   `json:"inMemory,omitempty"`
	MMAPv1         *mongodSpecMMAPv1     `json:"mmapv1,omitempty"`
	WiredTiger     *mongodSpecWiredTiger `json:"wiredTiger,omitempty"`
}

type mongodSpecMMAPv1 struct {
	NsSize     int  `json:"nsSize,omitempty"`
	Smallfiles bool `json:"smallfiles,omitempty"`
}

type wiredTigerCompressor string

var (
	wiredTigerCompressorNone   wiredTigerCompressor = "none"
	wiredTigerCompressorSnappy wiredTigerCompressor = "snappy"
	wiredTigerCompressorZlib   wiredTigerCompressor = "zlib"
)

type mongodSpecWiredTigerEngineConfig struct {
	CacheSizeRatio      float64               `json:"cacheSizeRatio,omitempty"`
	DirectoryForIndexes bool                  `json:"directoryForIndexes,omitempty"`
	JournalCompressor   *wiredTigerCompressor `json:"journalCompressor,omitempty"`
}

type mongodSpecWiredTigerCollectionConfig struct {
	BlockCompressor *wiredTigerCompressor `json:"blockCompressor,omitempty"`
}

type mongodSpecWiredTigerIndexConfig struct {
	PrefixCompression bool `json:"prefixCompression,omitempty"`
}

type mongodSpecWiredTiger struct {
	CollectionConfig *mongodSpecWiredTigerCollectionConfig `json:"collectionConfig,omitempty"`
	EngineConfig     *mongodSpecWiredTigerEngineConfig     `json:"engineConfig,omitempty"`
	IndexConfig      *mongodSpecWiredTigerIndexConfig      `json:"indexConfig,omitempty"`
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

type mongodSpecAuditLog struct {
	Destination auditLogDestination `json:"destination,omitempty"`
	Format      auditLogFormat      `json:"format,omitempty"`
	Filter      string              `json:"filter,omitempty"`
}

type operationProfilingMode string

const (
	operationProfilingModeAll    operationProfilingMode = "all"
	operationProfilingModeSlowOp operationProfilingMode = "slowOp"
)

type mongodSpecOperationProfiling struct {
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

type backupSpec struct {
	Enabled            bool                         `json:"enabled"`
	Storages           map[string]backupStorageSpec `json:"storages,omitempty"`
	Image              string                       `json:"image,omitempty"`
	Tasks              []backupTaskSpec             `json:"tasks,omitempty"`
	ServiceAccountName string                       `json:"serviceAccountName,omitempty"`
	Resources          *common.PodResources         `json:"resources,omitempty"`
}

type arbiter struct {
	Enabled bool  `json:"enabled"`
	Size    int32 `json:"size"`
	multiAZ
}

type expose struct {
	Enabled bool `json:"enabled"`
	// ExposeType ClusterIP, NodePort, LoadBalancer.
	// See: https://www.percona.com/doc/kubernetes-operator-for-psmongodb/expose.html.
	ExposeType string `json:"exposeType"`
}
