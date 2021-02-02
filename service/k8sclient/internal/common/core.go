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

package common

// Extracted from https://pkg.go.dev/k8s.io/api/core/v1

// HostPathVolumeSource represents a host path mapped into a pod.
// Host path volumes do not support ownership management or SELinux relabeling.
//
// https://pkg.go.dev/k8s.io/api/core/v1#HostPathVolumeSource
type HostPathVolumeSource struct {
	// Path of the directory on the host.
	// If the path is a symlink, it will follow the link to the real path.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	Path string `json:"path"`
}

// EmptyDirVolumeSource represents an empty directory for a pod.
// Empty directory volumes support ownership management and SELinux relabeling.
//
// https://pkg.go.dev/k8s.io/api/core/v1#EmptyDirVolumeSource
type EmptyDirVolumeSource struct{}

type ContainerSpec struct {
	Name string `json:"name,omitempty"`
}

// PodSpec is a description of a pod.
type PodSpec struct {
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	NodeName string `json:"nodeName,omitempty"`

	// Specifies the hostname of the Pod
	// If not specified, the pod's hostname will be set to a system-defined value.
	Hostname string `json:"hostname,omitempty"`

	// List of containers.
	Containers []ContainerSpec `json:"containers,omitempty"`
}

// Pod is a collection of containers that can run on a host. This resource is created
// by clients and scheduled onto hosts.
//
// https://pkg.go.dev/k8s.io/api/core/v1#Pod
type Pod struct {
	TypeMeta // anonymous for embedding

	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec PodSpec `json:"spec,omitempty"`
}

// Secret holds secret data of a certain type. The total bytes of the values in
// the Data field must be less than 1024 * 1024 bytes.
type Secret struct {
	TypeMeta
	// Standard object's metadata.
	ObjectMeta `json:"metadata,omitempty"`

	// Data contains the secret data. Each key must consist of alphanumeric
	// characters, '-', '_' or '.'. The serialized form of the secret data is a
	// base64 encoded string, representing the arbitrary (possibly non-string)
	// data value here. Described in https://tools.ietf.org/html/rfc4648#section-4
	Data map[string][]byte `json:"data,omitempty"`

	// Used to facilitate programmatic handling of secret data.
	Type SecretType `json:"type,omitempty"`
}

type SecretType string

const (
	// SecretTypeOpaque is the default. Arbitrary user-defined data.
	SecretTypeOpaque SecretType = "Opaque"
)
