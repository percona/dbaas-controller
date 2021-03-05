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

// ContainerStatus contains container's status.
type ContainerStatus struct {
	Name  string                 `json:"name,omitempty"`
	State map[string]interface{} `json:"state,omitempty"`
}

// ContainerSpec represents a container definition.
type ContainerSpec struct {
	Name      string               `json:"name,omitempty"`
	Image     string               `json:"image,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty"`
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

	// List of init containers.
	InitContainers []ContainerSpec `json:"initContainers,omitempty"`
}

// PodPhase defines Pod's phase.
// It could be one of these values: Pending, Running, Succeeded, Failed, Unknown.
// See https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/.
type PodPhase string

const (
	// PodPhasePending indicates that the Pod has been accepted by the
	// Kubernetes cluster, but one or more of the containers has not been set up
	// and made ready to run. This includes time a Pod spends waiting to be
	// scheduled as well as the time spent downloading container images over the network.
	PodPhasePending PodPhase = "Pending"
	// PodPhaseSucceded indicates that all containers in the Pod have terminated
	// in success, and will not be restarted.
	PodPhaseSucceded PodPhase = "Succeeded"
	// PodPhaseFailed indicates that all ontainers in the Pod have terminated,
	// and at least one container has terminated in failure. That is,
	// the container either exited with non-zero status or was terminated by the system.
	PodPhaseFailed PodPhase = "Failed"
)

// ContainerState describes container's state - waiting, running, terminated.
type ContainerState string

const (
	// ContainerStateWaiting represents a state when container requires some
	// operations being done in order to complete start up.
	ContainerStateWaiting ContainerState = "waiting"
	// ContainerStateTerminated indicates that container began execution and
	// then either ran to completion or failed for some reason.
	ContainerStateTerminated ContainerState = "terminated"
)

// IsContainerInState returns true if container is in give state, otherwise false.
func IsContainerInState(containerStatuses []ContainerStatus, state ContainerState, containerName string) bool {
	for _, status := range containerStatuses {
		if status.Name == containerName {
			if _, ok := status.State[string(state)]; ok {
				return true
			}
		}
	}
	return false
}

// PodStatus holds pod status.
type PodStatus struct {
	// ContainerStatuses holds statuses of regular containers.
	ContainerStatuses []ContainerStatus `json:"containerStatuses,omitempty"`

	// InitContainerStatuses holds statuses of init containers.
	InitContainerStatuses []ContainerStatus `json:"initContainerStatuses,omitempty"`

	// Phase holds pod's phase.
	Phase PodPhase `json:"phase,omitempty"`
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

	// PodStatus contains status of the pod.
	Status PodStatus `json:"status,omitempty"`
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

// NodeStatus holds Kubernetes node status.
type NodeStatus struct {
	// Allocatable is amount of recources from node's capacity that is available
	// for allocation by pods. The difference between capacity and allocatable of
	// the node is reserved for Kubernetes overhead and non-Kubernetes processes.
	Allocatable ResourceList `json:"allocatable,omitempty"`

	// Images is a list of container images stored at node.
	Images []Image `json:"images,omitempty"`
}

// Taint reserves node for pods that tolerate the taint.
// See https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/.
type Taint struct {
	Effect string `json:"effect,omitempty"`
	Key    string `json:"key,omitempty"`
}

// Image holds continaer image names and image size.
type Image struct {
	Names     []string `json:"names,omitempty"`
	SizeBytes int64    `json:"sizeBytes,omitempty"`
}

// NodeSpec holds Kubernetes node specification.
type NodeSpec struct {
	Taints []Taint `json:"taints,omitempty"`
}

// Node holds information about Kubernetes node.
type Node struct {
	TypeMeta
	// Specification of the node.
	Spec NodeSpec `json:"spec,omitempty"`
	// Status of the node.
	Status NodeStatus `json:"status,omitempty"`
}
